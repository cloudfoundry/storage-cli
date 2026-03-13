package signer

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type Signer interface {
	GenerateSignedURL(endpoint, prefixedBlobID, verb string, timeStamp time.Time, expiresAfter time.Duration) (string, error)
}

type signer struct {
	secret          string
	signedURLFormat string // "hmac-sha256" (default) or "secure-link-md5"
}

func NewSigner(secret string) Signer {
	return &signer{
		secret:          secret,
		signedURLFormat: "hmac-sha256",
	}
}

func NewSignerWithFormat(secret string, signedURLFormat string) (Signer, error) {
	if signedURLFormat == "" {
		signedURLFormat = "hmac-sha256"
	}

	normalized := strings.ToLower(signedURLFormat)
	switch normalized {
	case "sha256":
		normalized = "hmac-sha256"
	case "md5":
		normalized = "secure-link-md5"
	case "hmac-sha256", "secure-link-md5":
		// Valid format, already normalized
	default:
		return nil, fmt.Errorf("unsupported signed_url_format %q (supported: hmac-sha256, secure-link-md5)", signedURLFormat)
	}

	return &signer{
		secret:          secret,
		signedURLFormat: normalized,
	}, nil
}

// GenerateSignedURL generates nginx secure_link or secure_link_hmac compatible signed URLs
// Supports both HMAC-SHA256 and secure_link MD5 formats
func (s *signer) GenerateSignedURL(endpoint, prefixedBlobID, verb string, timeStamp time.Time, expiresAfter time.Duration) (string, error) {
	verb = strings.ToUpper(verb)
	if verb != "GET" && verb != "PUT" && verb != "HEAD" {
		return "", fmt.Errorf("action not implemented: %s. Available actions are 'GET', 'PUT', and 'HEAD'", verb)
	}

	if s.signedURLFormat == "secure-link-md5" {
		return s.generateMD5SignedURL(endpoint, prefixedBlobID, verb, timeStamp, expiresAfter)
	}
	return s.generateSHA256SignedURL(endpoint, prefixedBlobID, verb, timeStamp, expiresAfter)
}

// generateSHA256SignedURL generates BOSH-compatible SHA256 HMAC signed URLs
// Uses nginx secure_link_hmac module format
func (s *signer) generateSHA256SignedURL(endpoint, prefixedBlobID, verb string, timeStamp time.Time, expiresAfter time.Duration) (string, error) {
	endpoint = strings.TrimSuffix(endpoint, "/")
	timestamp := timeStamp.Unix()
	expiresAfterSeconds := int(expiresAfter.Seconds())

	blobURL, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}

	basePath := strings.TrimSuffix(blobURL.Path, "/")

	// Build the full path: /signed/basePath/blobID
	// The /signed prefix must come FIRST for nginx secure_link_hmac
	fullPath := path.Join("/signed", basePath, prefixedBlobID)

	// Generate HMAC-SHA256 signature using BOSH secure_link_hmac format:
	// hmac_sha256("{verb}{blobID}{timestamp}{duration}", secret)
	// Note: Uses duration in seconds, not absolute expiration timestamp
	signatureInput := fmt.Sprintf("%s%s%d%d", verb, prefixedBlobID, timestamp, expiresAfterSeconds)
	h := hmac.New(sha256.New, []byte(s.secret))
	h.Write([]byte(signatureInput))
	hmacSum := h.Sum(nil)
	signature := sanitizeBase64(base64.StdEncoding.EncodeToString(hmacSum))

	blobURL.Path = fullPath

	req, err := http.NewRequest(verb, blobURL.String(), nil)
	if err != nil {
		return "", err
	}

	q := req.URL.Query()
	q.Add("st", signature)
	q.Add("ts", fmt.Sprintf("%d", timestamp))
	q.Add("e", fmt.Sprintf("%d", expiresAfterSeconds))
	req.URL.RawQuery = q.Encode()

	return req.URL.String(), nil
}

// generateMD5SignedURL generates CAPI-compatible MD5 signed URLs
// Uses nginx secure_link module format
func (s *signer) generateMD5SignedURL(endpoint, prefixedBlobID, verb string, timeStamp time.Time, expiresAfter time.Duration) (string, error) {
	endpoint = strings.TrimSuffix(endpoint, "/")
	expires := timeStamp.Unix() + int64(expiresAfter.Seconds())

	var pathPrefix string
	if verb == "GET" || verb == "HEAD" {
		pathPrefix = "/read"
	} else {
		pathPrefix = "/write"
	}

	blobURL, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}

	basePath := strings.TrimSuffix(blobURL.Path, "/")

	// Build complete path: /write/cc-droplets/08/d3/... or /read/cc-droplets/08/d3/...
	// The path prefix (/write or /read) must come FIRST for nginx secure_link
	completePath := path.Join(pathPrefix, basePath, prefixedBlobID)

	// Generate MD5 signature using CAPI blobstore_url_signer format:
	// md5("{expires}{path} {secret}")
	signatureInput := fmt.Sprintf("%d%s %s", expires, completePath, s.secret)
	md5sum := md5.Sum([]byte(signatureInput))
	signature := sanitizeBase64(base64.StdEncoding.EncodeToString(md5sum[:]))

	blobURL.Path = completePath

	req, err := http.NewRequest(verb, blobURL.String(), nil)
	if err != nil {
		return "", err
	}

	q := req.URL.Query()
	q.Add("md5", signature)
	q.Add("expires", fmt.Sprintf("%d", expires))
	req.URL.RawQuery = q.Encode()

	return req.URL.String(), nil
}

// sanitizeBase64 converts base64 to URL-safe format for nginx secure_link_hmac
// Matches BOSH format: / -> _, + -> -, remove =
func sanitizeBase64(input string) string {
	str := strings.ReplaceAll(input, "/", "_")
	str = strings.ReplaceAll(str, "+", "-")
	str = strings.ReplaceAll(str, "=", "")
	return str
}
