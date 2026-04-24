package client

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/cloudfoundry/bosh-utils/httpclient"
	davconf "github.com/cloudfoundry/storage-cli/dav/config"
	URLsigner "github.com/cloudfoundry/storage-cli/dav/signer"
)

type propfindRequest struct {
	XMLName xml.Name        `xml:"D:propfind"`
	DAVNS   string          `xml:"xmlns:D,attr"`
	Prop    propfindReqProp `xml:"D:prop"`
}

type propfindReqProp struct {
	ResourceType struct{} `xml:"D:resourcetype"`
}

var propfindBodyXML = func() string {
	reqBody := propfindRequest{DAVNS: "DAV:"}
	out, err := xml.MarshalIndent(reqBody, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("failed to marshal PROPFIND request body: %v", err))
	}
	return xml.Header + string(out)
}()

func newPropfindBody() *strings.Reader {
	return strings.NewReader(propfindBodyXML)
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . StorageClient

type StorageClient interface {
	Get(path string) (content io.ReadCloser, err error)
	Put(path string, content io.ReadCloser, contentLength int64) (err error)
	Exists(path string) (bool, error)
	Delete(path string) (err error)
	Sign(objectID, action string, duration time.Duration) (string, error)
	Copy(srcBlob, dstBlob string) error
	List(prefix string) ([]string, error)
	Properties(path string) error
	EnsureStorageExists() error
}

type multistatusResponse struct {
	XMLName   xml.Name      `xml:"multistatus"`
	Responses []davResponse `xml:"response"`
}

type davResponse struct {
	Href      string        `xml:"href"`
	PropStats []davPropStat `xml:"propstat"`
}

type davPropStat struct {
	Prop davProp `xml:"prop"`
}

type davProp struct {
	ResourceType davResourceType `xml:"resourcetype"`
}

type davResourceType struct {
	Collection *struct{} `xml:"collection"`
}

func (r davResponse) isCollection() bool {
	for _, ps := range r.PropStats {
		if ps.Prop.ResourceType.Collection != nil {
			return true
		}
	}
	return false
}

type davHTTPError struct {
	Operation  string // e.g., "COPY", "MKCOL", "PROPFIND"
	StatusCode int
	Body       string
}

func (e *davHTTPError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("%s request failed: status %d, body: %s", e.Operation, e.StatusCode, e.Body)
	}
	return fmt.Sprintf("%s request failed: status %d", e.Operation, e.StatusCode)
}

type BlobProperties struct {
	ETag          string    `json:"etag,omitempty"`
	LastModified  time.Time `json:"last_modified,omitempty"`
	ContentLength int64     `json:"content_length,omitempty"`
}

type storageClient struct {
	config     davconf.Config
	httpClient httpclient.Client
	signer     URLsigner.Signer
}

func NewStorageClient(config davconf.Config, httpClientBase httpclient.Client) (StorageClient, error) {
	var urlSigner URLsigner.Signer
	if config.Secret != "" {
		if config.SignedURLFormat != "" {
			signer, err := URLsigner.NewSignerWithFormat(config.Secret, config.SignedURLFormat)
			if err != nil {
				return nil, fmt.Errorf("invalid signed_url_format: %w", err)
			}
			urlSigner = signer
		} else {
			urlSigner = URLsigner.NewSigner(config.Secret)
		}
	}

	return &storageClient{
		config:     config,
		httpClient: httpClientBase,
		signer:     urlSigner,
	}, nil
}

func (c *storageClient) Get(path string) (io.ReadCloser, error) {
	if err := validateBlobID(path); err != nil {
		return nil, err
	}

	req, err := c.createReq("GET", path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("getting dav blob %q: %w", path, err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close() //nolint:errcheck
		return nil, fmt.Errorf("getting dav blob %q: wrong response code: %d; body: %s", path, resp.StatusCode, c.readAndTruncateBody(resp))
	}

	return resp.Body, nil
}

func (c *storageClient) Put(path string, content io.ReadCloser, contentLength int64) error {
	defer content.Close() //nolint:errcheck

	if err := validateBlobID(path); err != nil {
		return err
	}

	req, err := c.createReq("PUT", path, content)
	if err != nil {
		return err
	}

	req.ContentLength = contentLength
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("putting dav blob %q: %w", path, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("putting dav blob %q: wrong response code: %d; body: %s", path, resp.StatusCode, c.readAndTruncateBody(resp))
	}

	return nil
}

func (c *storageClient) Exists(path string) (bool, error) {
	if err := validateBlobID(path); err != nil {
		return false, err
	}

	req, err := c.createReq("HEAD", path, nil)
	if err != nil {
		return false, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("checking if dav blob %s exists: %w", path, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("checking if dav blob %s exists: wrong response code: %d; body: %s", path, resp.StatusCode, c.readAndTruncateBody(resp))
	}

	return true, nil
}

func (c *storageClient) Delete(path string) error {
	if err := validateBlobID(path); err != nil {
		return err
	}

	req, err := c.createReq("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("creating delete request for blob %q: %w", path, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("deleting blob %q: %w", path, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("deleting blob %q: invalid status %d", path, resp.StatusCode)
	}

	return nil
}

func (c *storageClient) Sign(blobID, action string, duration time.Duration) (string, error) {
	if err := validateBlobID(blobID); err != nil {
		return "", err
	}

	action = strings.ToUpper(action)
	if action != "GET" && action != "PUT" {
		return "", fmt.Errorf("action not implemented: %s (only GET and PUT are supported)", action)
	}

	if c.signer == nil {
		return "", fmt.Errorf("signing is not configured (no secret provided)")
	}

	signTime := time.Now()
	signedURL, err := c.signer.GenerateSignedURL(c.config.Endpoint, blobID, action, signTime, duration)
	if err != nil {
		return "", fmt.Errorf("pre-signing the url: %w", err)
	}

	return signedURL, nil
}

func (c *storageClient) Copy(srcBlob, dstBlob string) error {
	if err := validateBlobID(srcBlob); err != nil {
		return fmt.Errorf("invalid source blob ID: %w", err)
	}
	if err := validateBlobID(dstBlob); err != nil {
		return fmt.Errorf("invalid destination blob ID: %w", err)
	}

	err := c.copyNative(srcBlob, dstBlob)
	if err == nil {
		return nil
	}

	// nginx WebDAV handles directory creation automatically, so no need to handle
	// 409 Conflict errors by manually creating parent directories and retrying.
	// Return the COPY error directly.
	return fmt.Errorf("WebDAV COPY failed: %w", err)
}

func (c *storageClient) copyNative(srcBlob, dstBlob string) error {
	srcURL, err := c.buildBlobURL(srcBlob)
	if err != nil {
		return fmt.Errorf("building source URL: %w", err)
	}

	dstURL, err := c.buildBlobURL(dstBlob)
	if err != nil {
		return fmt.Errorf("building destination URL: %w", err)
	}

	req, err := http.NewRequest("COPY", srcURL, nil)
	if err != nil {
		return fmt.Errorf("creating COPY request: %w", err)
	}

	if c.config.User != "" {
		req.SetBasicAuth(c.config.User, c.config.Password)
	}

	req.Header.Set("Destination", dstURL)
	req.Header.Set("Overwrite", "T") // Allow overwriting existing destination

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("performing COPY request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	// Per RFC 4918 section 9.8, standard COPY success responses:
	// 201 Created - destination resource was created
	// 204 No Content - destination resource was overwritten
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024)) //nolint:errcheck
	bodyPreview := string(bodyBytes)
	if len(bodyPreview) > 200 {
		bodyPreview = bodyPreview[:200] + "..."
	}

	return &davHTTPError{
		Operation:  "COPY",
		StatusCode: resp.StatusCode,
		Body:       bodyPreview,
	}
}

func (c *storageClient) List(prefix string) ([]string, error) {
	if prefix != "" {
		if err := validatePrefix(prefix); err != nil {
			return nil, err
		}
	}

	blobURL, err := url.Parse(c.config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("parsing endpoint URL: %w", err)
	}

	dirPath := blobURL.Path
	if !strings.HasPrefix(dirPath, "/") {
		dirPath = "/" + dirPath
	}
	blobURL.Path = dirPath

	return c.listRecursive(blobURL.String(), blobURL.Path, prefix)
}

func (c *storageClient) listRecursive(dirURL string, endpointPath string, prefix string) ([]string, error) {
	propfindBody := newPropfindBody()

	req, err := http.NewRequest("PROPFIND", dirURL, propfindBody)
	if err != nil {
		return nil, fmt.Errorf("creating PROPFIND request: %w", err)
	}

	if c.config.User != "" {
		req.SetBasicAuth(c.config.User, c.config.Password)
	}

	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", "application/xml")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing PROPFIND request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		return []string{}, nil
	}

	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 512)) //nolint:errcheck
		return nil, fmt.Errorf("PROPFIND failed: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var propfindResp multistatusResponse
	if err := xml.NewDecoder(resp.Body).Decode(&propfindResp); err != nil {
		return nil, fmt.Errorf("decoding PROPFIND response: %w", err)
	}

	reqURL, err := url.Parse(dirURL)
	if err != nil {
		return nil, fmt.Errorf("parsing request URL: %w", err)
	}
	requestPath := strings.TrimSuffix(reqURL.Path, "/")

	var allBlobs []string
	for _, response := range propfindResp.Responses {
		hrefURL, err := url.Parse(response.Href)
		if err != nil {
			continue
		}

		hrefPath := strings.TrimSuffix(hrefURL.Path, "/")

		if hrefPath == requestPath {
			continue
		}

		if response.isCollection() {
			subdirURL := hrefURL.String()
			if !hrefURL.IsAbs() {
				baseURL, err := url.Parse(dirURL)
				if err != nil {
					continue
				}
				subdirURL = baseURL.ResolveReference(hrefURL).String()
			}

			subBlobs, err := c.listRecursive(subdirURL, endpointPath, prefix)
			if err != nil {
				return nil, err
			}
			allBlobs = append(allBlobs, subBlobs...)
		} else {
			blobID, err := c.extractBlobIDFromHref(response.Href, endpointPath)
			if err != nil {
				continue
			}

			if prefix == "" || strings.HasPrefix(blobID, prefix) {
				allBlobs = append(allBlobs, blobID)
			}
		}
	}

	return allBlobs, nil
}

// extractBlobIDFromHref extracts the blob ID from a WebDAV href
// Returns the path relative to the endpoint
func (c *storageClient) extractBlobIDFromHref(href, endpointPath string) (string, error) {
	decoded, err := url.PathUnescape(href)
	if err == nil {
		href = decoded
	}

	hrefURL, err := url.Parse(href)
	if err != nil {
		return "", fmt.Errorf("parsing href: %w", err)
	}

	hrefPath := hrefURL.Path

	hrefPath = strings.TrimPrefix(hrefPath, "/")

	endpointPathClean := strings.TrimPrefix(strings.TrimSuffix(endpointPath, "/"), "/")
	if endpointPathClean != "" {
		hrefPath = strings.TrimPrefix(hrefPath, endpointPathClean+"/")
	}

	if hrefPath == "" {
		return "", fmt.Errorf("no blob ID after stripping endpoint path")
	}

	return hrefPath, nil
}

func (c *storageClient) Properties(path string) error {
	if err := validateBlobID(path); err != nil {
		return err
	}

	req, err := c.createReq("HEAD", path, nil)
	if err != nil {
		return fmt.Errorf("creating HEAD request for blob %q: %w", path, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("getting properties for blob %q: %w", path, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		fmt.Println(`{}`)
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("getting properties for blob %q: status %d", path, resp.StatusCode)
	}

	properties := BlobProperties{
		ContentLength: resp.ContentLength,
	}

	if etag := resp.Header.Get("ETag"); etag != "" {
		properties.ETag = strings.Trim(etag, `"`)
	}

	if lastModified := resp.Header.Get("Last-Modified"); lastModified != "" {
		// nginx always sends Last-Modified in RFC1123 format
		if t, err := time.Parse(time.RFC1123, lastModified); err == nil {
			properties.LastModified = t
		} else {
			slog.Warn("Failed to parse Last-Modified header", "value", lastModified, "error", err)
		}
	}

	output, err := json.MarshalIndent(properties, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal blob properties: %w", err)
	}

	fmt.Println(string(output))
	return nil
}

func (c *storageClient) EnsureStorageExists() error {
	// When using signed URLs (secret present), the storage always exists.
	// PROPFIND to the signed URL endpoint is not supported by nginx secure_link module.
	// Skip the check in this case as the /read and /write paths are handled by nginx.
	if c.config.Secret != "" {
		return nil
	}

	blobURL, err := url.Parse(c.config.Endpoint)
	if err != nil {
		return fmt.Errorf("parsing endpoint URL: %w", err)
	}

	propfindBody := newPropfindBody()

	req, err := http.NewRequest("PROPFIND", blobURL.String(), propfindBody)
	if err != nil {
		return fmt.Errorf("creating PROPFIND request for root: %w", err)
	}

	if c.config.User != "" {
		req.SetBasicAuth(c.config.User, c.config.Password)
	}

	req.Header.Set("Depth", "0")
	req.Header.Set("Content-Type", "application/xml")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("checking if root exists: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusMultiStatus || resp.StatusCode == http.StatusOK {
		return nil
	}

	if resp.StatusCode == http.StatusNotFound {
		mkcolReq, err := http.NewRequest("MKCOL", blobURL.String(), nil)
		if err != nil {
			return fmt.Errorf("creating MKCOL request: %w", err)
		}

		if c.config.User != "" {
			mkcolReq.SetBasicAuth(c.config.User, c.config.Password)
		}

		mkcolResp, err := c.httpClient.Do(mkcolReq)
		if err != nil {
			return fmt.Errorf("creating root directory: %w", err)
		}
		defer mkcolResp.Body.Close() //nolint:errcheck

		// Per RFC 4918, only accept standard MKCOL success responses:
		// 201 Created - collection created successfully
		// 405 Method Not Allowed - already exists (standard "already exists" case)
		if mkcolResp.StatusCode == http.StatusCreated || mkcolResp.StatusCode == http.StatusMethodNotAllowed {
			return nil
		}

		bodyBytes, _ := io.ReadAll(io.LimitReader(mkcolResp.Body, 512)) //nolint:errcheck
		bodyPreview := string(bodyBytes)
		if len(bodyPreview) > 200 {
			bodyPreview = bodyPreview[:200] + "..."
		}

		return &davHTTPError{
			Operation:  "MKCOL",
			StatusCode: mkcolResp.StatusCode,
			Body:       bodyPreview,
		}
	}

	return &davHTTPError{
		Operation:  "PROPFIND",
		StatusCode: resp.StatusCode,
		Body:       "",
	}
}

// createReq creates an HTTP request for a blob operation
// IMPORTANT: blobID must be validated with validateBlobID before calling this function
func (c *storageClient) createReq(method, blobID string, body io.Reader) (*http.Request, error) {
	// When using signed URLs, generate the signed URL with the signer
	if c.signer != nil {
		// Default to 15 minutes if not specified
		expirationMinutes := c.config.SignedURLExpiration
		if expirationMinutes == 0 {
			expirationMinutes = 15
		}

		signedURL, err := c.signer.GenerateSignedURL(
			c.config.Endpoint,
			blobID,
			method,
			time.Now(),
			time.Duration(expirationMinutes)*time.Minute,
		)
		if err != nil {
			return nil, fmt.Errorf("generating signed URL: %w", err)
		}

		req, err := http.NewRequest(method, signedURL, body)
		if err != nil {
			return nil, err
		}
		return req, nil
	}

	// Basic auth mode (no signer)
	blobURL, err := url.Parse(c.config.Endpoint)
	if err != nil {
		return nil, err
	}

	newPath := path.Join(blobURL.Path, blobID)
	if !strings.HasPrefix(newPath, "/") {
		newPath = "/" + newPath
	}

	blobURL.Path = newPath

	req, err := http.NewRequest(method, blobURL.String(), body)
	if err != nil {
		return req, err
	}

	if c.config.User != "" {
		req.SetBasicAuth(c.config.User, c.config.Password)
	}
	return req, nil
}

func (c *storageClient) readAndTruncateBody(resp *http.Response) string {
	if resp.Body == nil {
		return ""
	}
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024)) //nolint:errcheck
	return string(bodyBytes)
}

// buildBlobURL constructs the full URL for a blob
// IMPORTANT: blobID must be validated with validateBlobID before calling this function
func (c *storageClient) buildBlobURL(blobID string) (string, error) {
	blobURL, err := url.Parse(c.config.Endpoint)
	if err != nil {
		return "", err
	}

	newPath := path.Join(blobURL.Path, blobID)
	if !strings.HasPrefix(newPath, "/") {
		newPath = "/" + newPath
	}
	blobURL.Path = newPath

	return blobURL.String(), nil
}
