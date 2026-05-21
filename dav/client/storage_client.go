package client

import (
	"fmt"
	"io"
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
	Sign(objectID, action string, duration time.Duration) (string, error)
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

func (c *storageClient) createReq(method, blobID string, body io.Reader) (*http.Request, error) {
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
