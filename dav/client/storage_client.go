package client

import (
	"crypto/sha1"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	URLsigner "github.com/cloudfoundry/storage-cli/dav/signer"

	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"github.com/cloudfoundry/bosh-utils/httpclient"

	davconf "github.com/cloudfoundry/storage-cli/dav/config"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . StorageClient

// StorageClient handles low-level HTTP operations for WebDAV
type StorageClient interface {
	Get(path string) (content io.ReadCloser, err error)
	Put(path string, content io.ReadCloser, contentLength int64) (err error)
	Exists(path string) (err error)
	Delete(path string) (err error)
	Sign(objectID, action string, duration time.Duration) (string, error)
	Copy(srcBlob, dstBlob string) error
	List(prefix string) ([]string, error)
	Properties(path string) error
	EnsureStorageExists() error
}

type storageClient struct {
	config     davconf.Config
	httpClient httpclient.Client
}

// NewStorageClient creates a new HTTP client for WebDAV operations
func NewStorageClient(config davconf.Config, httpClientBase httpclient.Client) StorageClient {
	return &storageClient{
		config:     config,
		httpClient: httpClientBase,
	}
}

// getCertPool creates a certificate pool from the config
func getCertPool(config davconf.Config) (*x509.CertPool, error) {
	if len(config.TLS.Cert.CA) == 0 {
		return nil, nil
	}

	certPool, err := boshcrypto.CertPoolFromPEM([]byte(config.TLS.Cert.CA))
	if err != nil {
		return nil, err
	}

	return certPool, nil
}

func (c *storageClient) Get(path string) (io.ReadCloser, error) {
	req, err := c.createReq("GET", path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "Getting dav blob %s", path)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Getting dav blob %s: Wrong response code: %d; body: %s", path, resp.StatusCode, c.readAndTruncateBody(resp)) //nolint:staticcheck
	}

	return resp.Body, nil
}

func (c *storageClient) Put(path string, content io.ReadCloser, contentLength int64) error {
	// Ensure the prefix directory exists
	if err := c.ensurePrefixDirExists(path); err != nil {
		return bosherr.WrapErrorf(err, "Ensuring prefix directory exists for blob %s", path)
	}

	req, err := c.createReq("PUT", path, content)
	if err != nil {
		return err
	}
	defer content.Close() //nolint:errcheck

	req.ContentLength = contentLength
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return bosherr.WrapErrorf(err, "Putting dav blob %s", path)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Putting dav blob %s: Wrong response code: %d; body: %s", path, resp.StatusCode, c.readAndTruncateBody(resp)) //nolint:staticcheck
	}

	return nil
}

func (c *storageClient) Exists(path string) error {
	req, err := c.createReq("HEAD", path, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return bosherr.WrapErrorf(err, "Checking if dav blob %s exists", path)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		err := fmt.Errorf("%s not found", path)
		return bosherr.WrapErrorf(err, "Checking if dav blob %s exists", path)
	}

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("invalid status: %d", resp.StatusCode)
		return bosherr.WrapErrorf(err, "Checking if dav blob %s exists", path)
	}

	return nil
}

func (c *storageClient) Delete(path string) error {
	req, err := c.createReq("DELETE", path, nil)
	if err != nil {
		return bosherr.WrapErrorf(err, "Creating delete request for blob '%s'", path)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return bosherr.WrapErrorf(err, "Deleting blob '%s'", path)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err := fmt.Errorf("invalid status: %d", resp.StatusCode)
		return bosherr.WrapErrorf(err, "Deleting blob '%s'", path)
	}

	return nil
}

func (c *storageClient) Sign(blobID, action string, duration time.Duration) (string, error) {
	signer := URLsigner.NewSigner(c.config.Secret)
	signTime := time.Now()

	prefixedBlob := fmt.Sprintf("%s/%s", getBlobPrefix(blobID), blobID)

	signedURL, err := signer.GenerateSignedURL(c.config.Endpoint, prefixedBlob, action, signTime, duration)
	if err != nil {
		return "", bosherr.WrapErrorf(err, "pre-signing the url")
	}

	return signedURL, err
}

// Copy copies a blob from source to destination within the same WebDAV server
func (c *storageClient) Copy(srcBlob, dstBlob string) error {
	// Ensure the destination prefix directory exists
	if err := c.ensurePrefixDirExists(dstBlob); err != nil {
		return bosherr.WrapErrorf(err, "Ensuring prefix directory exists for destination blob %s", dstBlob)
	}

	srcReq, err := c.createReq("GET", srcBlob, nil)
	if err != nil {
		return bosherr.WrapErrorf(err, "Creating request for source blob '%s'", srcBlob)
	}

	// Get the source blob content
	srcResp, err := c.httpClient.Do(srcReq)
	if err != nil {
		return bosherr.WrapErrorf(err, "Getting source blob '%s'", srcBlob)
	}
	defer srcResp.Body.Close() //nolint:errcheck

	if srcResp.StatusCode != http.StatusOK {
		return fmt.Errorf("Getting source blob '%s': Wrong response code: %d; body: %s", srcBlob, srcResp.StatusCode, c.readAndTruncateBody(srcResp)) //nolint:staticcheck
	}

	// Put the content to destination
	dstReq, err := c.createReq("PUT", dstBlob, srcResp.Body)
	if err != nil {
		return bosherr.WrapErrorf(err, "Creating request for destination blob '%s'", dstBlob)
	}

	dstReq.ContentLength = srcResp.ContentLength

	dstResp, err := c.httpClient.Do(dstReq)
	if err != nil {
		return bosherr.WrapErrorf(err, "Putting destination blob '%s'", dstBlob)
	}
	defer dstResp.Body.Close() //nolint:errcheck

	if dstResp.StatusCode != http.StatusOK && dstResp.StatusCode != http.StatusCreated && dstResp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Putting destination blob '%s': Wrong response code: %d; body: %s", dstBlob, dstResp.StatusCode, c.readAndTruncateBody(dstResp)) //nolint:staticcheck
	}

	return nil
}

// List returns a list of blob paths that match the given prefix
func (c *storageClient) List(prefix string) ([]string, error) {
	blobURL, err := url.Parse(c.config.Endpoint)
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "Parsing endpoint URL")
	}

	var allBlobs []string

	// Always list all prefix directories first
	dirPath := blobURL.Path
	if !strings.HasPrefix(dirPath, "/") {
		dirPath = "/" + dirPath
	}
	blobURL.Path = dirPath

	dirs, err := c.propfindDirs(blobURL.String())
	if err != nil {
		return nil, err
	}

	// For each prefix directory, list all blobs matching the prefix
	for _, dir := range dirs {
		dirURL := *blobURL
		dirURL.Path = path.Join(blobURL.Path, dir) + "/"
		blobs, err := c.propfindBlobs(dirURL.String(), prefix)
		if err != nil {
			continue // Skip directories we can't read
		}
		allBlobs = append(allBlobs, blobs...)
	}

	return allBlobs, nil
}

// propfindDirs returns a list of directory names (prefix directories like "8c")
func (c *storageClient) propfindDirs(urlStr string) ([]string, error) {
	propfindBody := `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:displayname/>
  </D:prop>
</D:propfind>`

	req, err := http.NewRequest("PROPFIND", urlStr, strings.NewReader(propfindBody))
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "Creating PROPFIND request")
	}

	if c.config.User != "" {
		req.SetBasicAuth(c.config.User, c.config.Password)
	}

	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", "application/xml")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "Performing PROPFIND request")
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PROPFIND request failed: status %d", resp.StatusCode) //nolint:staticcheck
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "Reading PROPFIND response")
	}

	var dirs []string
	responseStr := string(body)
	lines := strings.Split(responseStr, "\n")
	for _, line := range lines {
		if strings.Contains(line, "<D:href>") || strings.Contains(line, "<d:href>") {
			start := strings.Index(line, ">")
			end := strings.LastIndex(line, "<")
			if start != -1 && end != -1 && start < end {
				href := line[start+1 : end]
				decoded, err := url.PathUnescape(href)
				if err == nil {
					href = decoded
				}

				// Only include directories (ending with /)
				if strings.HasSuffix(href, "/") && href != "/" {
					parts := strings.Split(strings.TrimSuffix(href, "/"), "/")
					if len(parts) > 0 {
						dirName := parts[len(parts)-1]
						if dirName != "" {
							dirs = append(dirs, dirName)
						}
					}
				}
			}
		}
	}

	return dirs, nil
}

// propfindBlobs returns a list of blob names in a directory, filtered by prefix
func (c *storageClient) propfindBlobs(urlStr string, prefix string) ([]string, error) {
	propfindBody := `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:displayname/>
  </D:prop>
</D:propfind>`

	req, err := http.NewRequest("PROPFIND", urlStr, strings.NewReader(propfindBody))
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "Creating PROPFIND request")
	}

	if c.config.User != "" {
		req.SetBasicAuth(c.config.User, c.config.Password)
	}

	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", "application/xml")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "Performing PROPFIND request")
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PROPFIND request failed: status %d", resp.StatusCode) //nolint:staticcheck
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "Reading PROPFIND response")
	}

	var blobs []string
	responseStr := string(body)
	lines := strings.Split(responseStr, "\n")
	for _, line := range lines {
		if strings.Contains(line, "<D:href>") || strings.Contains(line, "<d:href>") {
			start := strings.Index(line, ">")
			end := strings.LastIndex(line, "<")
			if start != -1 && end != -1 && start < end {
				href := line[start+1 : end]
				decoded, err := url.PathUnescape(href)
				if err == nil {
					href = decoded
				}

				// Extract just the blob name (last part of path)
				parts := strings.Split(strings.TrimSuffix(href, "/"), "/")
				if len(parts) > 0 {
					blobName := parts[len(parts)-1]
					// Filter by prefix if provided, skip directories
					if !strings.HasSuffix(href, "/") && blobName != "" {
						if prefix == "" || strings.HasPrefix(blobName, prefix) {
							blobs = append(blobs, blobName)
						}
					}
				}
			}
		}
	}

	return blobs, nil
}

// Properties retrieves metadata/properties for a blob using HEAD request
func (c *storageClient) Properties(path string) error {
	req, err := c.createReq("HEAD", path, nil)
	if err != nil {
		return bosherr.WrapErrorf(err, "Creating HEAD request for blob '%s'", path)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return bosherr.WrapErrorf(err, "Getting properties for blob '%s'", path)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		fmt.Println(`{}`)
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Getting properties for blob '%s': status %d", path, resp.StatusCode) //nolint:staticcheck
	}

	// Extract properties from headers
	props := map[string]interface{}{
		"ContentLength": resp.ContentLength,
	}

	if etag := resp.Header.Get("ETag"); etag != "" {
		props["ETag"] = strings.Trim(etag, `"`)
	}

	if lastModified := resp.Header.Get("Last-Modified"); lastModified != "" {
		props["LastModified"] = lastModified
	}

	output, err := json.MarshalIndent(props, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal blob properties: %w", err)
	}

	fmt.Println(string(output))
	return nil
}

// EnsureStorageExists ensures the WebDAV directory structure exists
func (c *storageClient) EnsureStorageExists() error {
	blobURL, err := url.Parse(c.config.Endpoint)
	if err != nil {
		return bosherr.WrapErrorf(err, "Parsing endpoint URL")
	}

	// Try to check if the root path exists
	req, err := http.NewRequest("HEAD", blobURL.String(), nil)
	if err != nil {
		return bosherr.WrapErrorf(err, "Creating HEAD request for root")
	}

	if c.config.User != "" {
		req.SetBasicAuth(c.config.User, c.config.Password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return bosherr.WrapErrorf(err, "Checking if root exists")
	}
	defer resp.Body.Close() //nolint:errcheck

	// If the root exists, we're done
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	// If not found, try to create it using MKCOL
	if resp.StatusCode == http.StatusNotFound {
		mkcolReq, err := http.NewRequest("MKCOL", blobURL.String(), nil)
		if err != nil {
			return bosherr.WrapErrorf(err, "Creating MKCOL request")
		}

		if c.config.User != "" {
			mkcolReq.SetBasicAuth(c.config.User, c.config.Password)
		}

		mkcolResp, err := c.httpClient.Do(mkcolReq)
		if err != nil {
			return bosherr.WrapErrorf(err, "Creating root directory")
		}
		defer mkcolResp.Body.Close() //nolint:errcheck

		if mkcolResp.StatusCode != http.StatusCreated && mkcolResp.StatusCode != http.StatusOK {
			return fmt.Errorf("Creating root directory failed: status %d", mkcolResp.StatusCode) //nolint:staticcheck
		}
	}

	return nil
}

func (c *storageClient) createReq(method, blobID string, body io.Reader) (*http.Request, error) {
	blobURL, err := url.Parse(c.config.Endpoint)
	if err != nil {
		return nil, err
	}

	blobPrefix := getBlobPrefix(blobID)

	newPath := path.Join(blobURL.Path, blobPrefix, blobID)
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
	body := ""
	if resp.Body != nil {
		buf := make([]byte, 1024)
		n, err := resp.Body.Read(buf)
		if err == io.EOF || err == nil {
			body = string(buf[0:n])
		}
	}
	return body
}

func (c *storageClient) ensurePrefixDirExists(blobID string) error {
	blobURL, err := url.Parse(c.config.Endpoint)
	if err != nil {
		return err
	}

	blobPrefix := getBlobPrefix(blobID)
	prefixPath := path.Join(blobURL.Path, blobPrefix)
	if !strings.HasPrefix(prefixPath, "/") {
		prefixPath = "/" + prefixPath
	}
	// Add trailing slash for WebDAV collection
	if !strings.HasSuffix(prefixPath, "/") {
		prefixPath = prefixPath + "/"
	}

	blobURL.Path = prefixPath

	// Try MKCOL to create the directory
	req, err := http.NewRequest("MKCOL", blobURL.String(), nil)
	if err != nil {
		return err
	}

	if c.config.User != "" {
		req.SetBasicAuth(c.config.User, c.config.Password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	// Accept 200 (OK - already exists), 201 (Created), 405 (Method Not Allowed - already exists), or 409 (Conflict - already exists)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusMethodNotAllowed && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("creating prefix directory %s: status %d", prefixPath, resp.StatusCode)
	}

	return nil
}

func getBlobPrefix(blobID string) string {
	digester := sha1.New()
	digester.Write([]byte(blobID))
	return fmt.Sprintf("%02x", digester.Sum(nil)[0])
}
