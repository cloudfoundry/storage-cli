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

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . StorageClient

type StorageClient interface {
	Get(path string) (content io.ReadCloser, err error)
	Put(path string, content io.ReadCloser, contentLength int64) (err error)
	Exists(path string) (bool, error)
	Delete(path string) (err error)
	DeleteRecursive(prefix string) error
	Sign(objectID, action string, duration time.Duration) (string, error)
	Copy(srcBlob, dstBlob string) error
	List(prefix string) ([]string, error)
	Properties(path string) error
	EnsureStorageExists() error
}

type BlobProperties struct {
	ETag          string    `json:"etag,omitempty"`
	LastModified  time.Time `json:"last_modified,omitempty"`
	ContentLength *int64    `json:"content_length,omitempty"`
}

// PROPFIND request body — sent as XML to ask the WebDAV server for the
// resourcetype of every child entry of a collection.
type propfindRequest struct {
	XMLName xml.Name        `xml:"D:propfind"`
	DAVNS   string          `xml:"xmlns:D,attr"`
	Prop    propfindReqProp `xml:"D:prop"`
}

type propfindReqProp struct {
	ResourceType struct{} `xml:"D:resourcetype"`
}

func newPropfindBody() (io.Reader, error) {
	body := propfindRequest{DAVNS: "DAV:"}
	out, err := xml.MarshalIndent(body, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling PROPFIND body: %w", err)
	}
	return strings.NewReader(xml.Header + string(out)), nil
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

type storageClient struct {
	config     davconf.Config
	httpClient httpclient.Client
}

func NewStorageClient(config davconf.Config, httpClient httpclient.Client) StorageClient {
	return &storageClient{
		config:     config,
		httpClient: httpClient,
	}
}

func (c *storageClient) Get(path string) (io.ReadCloser, error) {
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
	req, err := c.createReq("HEAD", path, nil)
	if err != nil {
		return false, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("checking if dav blob %q exists: %w", path, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("checking if dav blob %q exists: invalid status: %d", path, resp.StatusCode)
	}

	return true, nil
}

func (c *storageClient) Delete(path string) error {
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
	signer := URLsigner.NewSigner(c.config.Secret)
	signTime := time.Now()

	signedURL, err := signer.GenerateSignedURL(c.config.Endpoint, blobID, action, signTime, duration)
	if err != nil {
		return "", fmt.Errorf("pre-signing the url: %w", err)
	}

	return signedURL, nil
}

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

func (c *storageClient) createReq(method, blobID string, body io.Reader) (*http.Request, error) {
	rawURL, err := c.buildBlobURL(blobID)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return nil, err
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

func (c *storageClient) Copy(srcBlob, dstBlob string) error {
	dstURL, err := c.buildBlobURL(dstBlob)
	if err != nil {
		return fmt.Errorf("building destination URL: %w", err)
	}

	// PUT an empty file first so nginx (create_full_put_path on) creates any
	// missing parent directories before COPY overwrites the placeholder.
	putReq, err := c.createReq("PUT", dstBlob, http.NoBody)
	if err != nil {
		return fmt.Errorf("creating destination PUT request: %w", err)
	}
	putReq.ContentLength = 0

	putResp, err := c.httpClient.Do(putReq)
	if err != nil {
		return fmt.Errorf("creating destination placeholder: %w", err)
	}
	defer putResp.Body.Close() //nolint:errcheck

	if putResp.StatusCode != http.StatusCreated && putResp.StatusCode != http.StatusNoContent && putResp.StatusCode != http.StatusOK {
		return fmt.Errorf("creating destination placeholder %q: status %d, body: %s",
			dstBlob, putResp.StatusCode, c.readAndTruncateBody(putResp))
	}

	copyReq, err := c.createReq("COPY", srcBlob, nil)
	if err != nil {
		return fmt.Errorf("creating COPY request: %w", err)
	}
	copyReq.Header.Set("Destination", dstURL)
	copyReq.Header.Set("Overwrite", "T")

	copyResp, err := c.httpClient.Do(copyReq)
	if err != nil {
		return fmt.Errorf("performing COPY %q -> %q: %w", srcBlob, dstBlob, err)
	}
	defer copyResp.Body.Close() //nolint:errcheck

	// RFC 4918 §9.8: 201 Created (new) or 204 No Content (overwritten).
	if copyResp.StatusCode == http.StatusCreated || copyResp.StatusCode == http.StatusNoContent {
		return nil
	}

	return fmt.Errorf("COPY %q -> %q: status %d, body: %s",
		srcBlob, dstBlob, copyResp.StatusCode, c.readAndTruncateBody(copyResp))
}

func (c *storageClient) List(prefix string) ([]string, error) {
	rootURL, err := url.Parse(c.config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("parsing endpoint URL: %w", err)
	}
	if !strings.HasPrefix(rootURL.Path, "/") {
		rootURL.Path = "/" + rootURL.Path
	}

	return c.listRecursive(rootURL.String(), rootURL.Path, prefix)
}

func (c *storageClient) listRecursive(dirURL, endpointPath, prefix string) ([]string, error) {
	body, err := newPropfindBody()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PROPFIND", dirURL, body)
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
		return nil, fmt.Errorf("performing PROPFIND: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		return []string{}, nil
	}
	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PROPFIND %q: status %d, body: %s",
			dirURL, resp.StatusCode, c.readAndTruncateBody(resp))
	}

	var multi multistatusResponse
	if err := xml.NewDecoder(resp.Body).Decode(&multi); err != nil {
		return nil, fmt.Errorf("decoding PROPFIND response: %w", err)
	}

	parsedDirURL, err := url.Parse(dirURL)
	if err != nil {
		return nil, fmt.Errorf("parsing dirURL: %w", err)
	}
	currentPath := strings.TrimSuffix(parsedDirURL.Path, "/")

	var blobs []string
	for _, response := range multi.Responses {
		hrefURL, err := url.Parse(response.Href)
		if err != nil {
			slog.Warn("skipping unparseable href in PROPFIND response", "href", response.Href, "error", err)
			continue
		}
		hrefPath := strings.TrimSuffix(hrefURL.Path, "/")

		if hrefPath == currentPath {
			continue
		}

		if response.isCollection() {
			subURL := hrefURL.String()
			if !hrefURL.IsAbs() {
				subURL = parsedDirURL.ResolveReference(hrefURL).String()
			}
			sub, err := c.listRecursive(subURL, endpointPath, prefix)
			if err != nil {
				return nil, err
			}
			blobs = append(blobs, sub...)
			continue
		}

		blobID, err := blobIDFromHref(response.Href, endpointPath)
		if err != nil {
			slog.Warn("skipping href that could not be mapped to a blob ID", "href", response.Href, "error", err)
			continue
		}
		if prefix == "" || strings.HasPrefix(blobID, prefix) {
			blobs = append(blobs, blobID)
		}
	}

	return blobs, nil
}

// blobIDFromHref extracts the blob ID from a WebDAV href
// Returns the path relative to the endpoint
func blobIDFromHref(href, endpointPath string) (string, error) {
	if decoded, err := url.PathUnescape(href); err == nil {
		href = decoded
	}

	hrefURL, err := url.Parse(href)
	if err != nil {
		return "", fmt.Errorf("parsing href: %w", err)
	}

	hrefPath := strings.TrimPrefix(hrefURL.Path, "/")
	endpointClean := strings.Trim(endpointPath, "/")
	if endpointClean != "" {
		hrefPath = strings.TrimPrefix(hrefPath, endpointClean+"/")
	}

	if hrefPath == "" {
		return "", fmt.Errorf("href %q has no blob component after stripping endpoint %q", href, endpointPath)
	}
	return hrefPath, nil
}

func (c *storageClient) DeleteRecursive(prefix string) error {
	blobs, err := c.List(prefix)
	if err != nil {
		return fmt.Errorf("listing blobs under %q: %w", prefix, err)
	}

	if len(blobs) == 0 {
		slog.Warn("no blobs found for prefix, nothing deleted", "prefix", prefix)
		return nil
	}

	for _, blob := range blobs {
		if err := c.Delete(blob); err != nil {
			return fmt.Errorf("deleting %q: %w", blob, err)
		}
	}
	return nil
}

// Properties prints the blob's metadata (ETag, Last-Modified, Content-Length)
// as JSON to stdout. Returns nil with `{}` on 404 to mirror the behaviour of
// other backends (S3, Azure) for missing blobs.
func (c *storageClient) Properties(blobPath string) error {
	req, err := c.createReq("HEAD", blobPath, nil)
	if err != nil {
		return fmt.Errorf("creating HEAD request for %q: %w", blobPath, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching properties of %q: %w", blobPath, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		fmt.Println("{}")
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetching properties of %q: status %d", blobPath, resp.StatusCode)
	}

	props := BlobProperties{}
	if resp.ContentLength >= 0 {
		props.ContentLength = &resp.ContentLength
	}
	if etag := resp.Header.Get("ETag"); etag != "" {
		props.ETag = strings.Trim(etag, `"`)
	}
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		if t, err := time.Parse(time.RFC1123, lm); err == nil {
			props.LastModified = t
		} else {
			slog.Warn("could not parse Last-Modified header", "value", lm, "error", err)
		}
	}

	out, err := json.MarshalIndent(props, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling properties: %w", err)
	}
	fmt.Println(string(out))
	return nil
}

// EnsureStorageExists is a no-op for DAV. WebDAV has no "bucket" concept to
// provision: nginx auto-creates parent directories on first PUT (via
// `create_full_put_path on`), so there is nothing to do here. Matches the
// fog-based Ruby DavClient, whose ensure_bucket_exists is also empty. The
// method exists only to satisfy the StorageClient interface.
func (c *storageClient) EnsureStorageExists() error {
	return nil
}
