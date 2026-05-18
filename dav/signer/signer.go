package signer

import (
	"crypto/hmac"
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
	GenerateSignedURL(endpointBase, directoryKey, prefixedBlobID, verb string, timeStamp time.Time, expiresAfter time.Duration) (string, error)
}

type signer struct {
	secret string
}

func NewSigner(secret string) Signer {
	return &signer{
		secret: secret,
	}
}

func NewSignerWithFormat(secret string, signedURLFormat string) (Signer, error) {
	if signedURLFormat == "" {
		signedURLFormat = "hmac-sha256"
	}

	normalized := strings.ToLower(signedURLFormat)
	switch normalized {
	case "sha256":
		// Alias for hmac-sha256
	case "hmac-sha256":
		// Valid format, already normalized
	default:
		return nil, fmt.Errorf("unsupported signed_url_format %q (supported: hmac-sha256)", signedURLFormat)
	}

	return &signer{
		secret: secret,
	}, nil
}

// GenerateSignedURL generates nginx secure_link_hmac compatible signed URLs
// Uses HMAC-SHA256 format for BOSH
// endpointBase: base URL with scheme and host (e.g., "https://blobstore.service.cf.internal:4443")
// directoryKey: the directory key / bucket name (e.g., "cc-droplets")
// prefixedBlobID: the blob ID with path partitioning (e.g., "dr/op/droplet-guid")
func (s *signer) GenerateSignedURL(endpointBase, directoryKey, prefixedBlobID, verb string, timeStamp time.Time, expiresAfter time.Duration) (string, error) {
	verb = strings.ToUpper(verb)
	if verb != "GET" && verb != "PUT" && verb != "HEAD" {
		return "", fmt.Errorf("action not implemented: %s. Available actions are 'GET', 'PUT', and 'HEAD'", verb)
	}

	return s.generateSHA256SignedURL(endpointBase, directoryKey, prefixedBlobID, verb, timeStamp, expiresAfter)
}

// generateSHA256SignedURL generates BOSH-compatible SHA256 HMAC signed URLs
// Uses nginx secure_link_hmac module format
func (s *signer) generateSHA256SignedURL(endpointBase, directoryKey, prefixedBlobID, verb string, timeStamp time.Time, expiresAfter time.Duration) (string, error) {
	endpointBase = strings.TrimSuffix(endpointBase, "/")
	timestamp := timeStamp.Unix()
	expiresAfterSeconds := int(expiresAfter.Seconds())

	blobURL, err := url.Parse(endpointBase)
	if err != nil {
		return "", err
	}

	// Build the full path: /signed/{directoryKey}/{blobID}
	// The /signed prefix must come FIRST for nginx secure_link_hmac
	// Do NOT include /admin prefix - just the directory key
	fullPath := path.Join("/signed", directoryKey, prefixedBlobID)

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

// sanitizeBase64 converts base64 to URL-safe format for nginx secure_link_hmac
// Matches BOSH format: / -> _, + -> -, remove =
func sanitizeBase64(input string) string {
	str := strings.ReplaceAll(input, "/", "_")
	str = strings.ReplaceAll(str, "+", "-")
	str = strings.ReplaceAll(str, "=", "")
	return str
}
