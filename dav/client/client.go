package client

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	davconf "github.com/cloudfoundry/storage-cli/dav/config"
)

// DavBlobstore implements the storage.Storager interface for WebDAV
type DavBlobstore struct {
	storageClient StorageClient
}

// New creates a new DavBlobstore instance
func New(config davconf.Config) (*DavBlobstore, error) {
	logger := boshlog.NewLogger(boshlog.LevelNone)

	var httpClientBase httpclient.Client
	var certPool, err = getCertPool(config)
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "Failed to create certificate pool")
	}

	httpClientBase = httpclient.CreateDefaultClient(certPool)

	if config.RetryAttempts == 0 {
		config.RetryAttempts = 3
	}

	// Retry with 1 second delay between attempts
	duration := time.Duration(1) * time.Second
	retryClient := httpclient.NewRetryClient(
		httpClientBase,
		config.RetryAttempts,
		duration,
		logger,
	)

	storageClient := NewStorageClient(config, retryClient)

	return &DavBlobstore{
		storageClient: storageClient,
	}, nil
}

// Put uploads a file to the WebDAV server
func (d *DavBlobstore) Put(sourceFilePath string, dest string) error {
	slog.Debug("Uploading file to WebDAV", "source", sourceFilePath, "dest", dest)

	source, err := os.Open(sourceFilePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer source.Close() //nolint:errcheck

	fileInfo, err := source.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	err = d.storageClient.Put(dest, source, fileInfo.Size())
	if err != nil {
		return fmt.Errorf("upload failure: %w", err)
	}

	slog.Debug("Successfully uploaded file", "dest", dest)
	return nil
}

// Get downloads a file from the WebDAV server
func (d *DavBlobstore) Get(source string, dest string) error {
	slog.Debug("Downloading file from WebDAV", "source", source, "dest", dest)

	destFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close() //nolint:errcheck

	content, err := d.storageClient.Get(source)
	if err != nil {
		return fmt.Errorf("download failure: %w", err)
	}
	defer content.Close() //nolint:errcheck

	_, err = io.Copy(destFile, content)
	if err != nil {
		return fmt.Errorf("failed to write to destination file: %w", err)
	}

	slog.Debug("Successfully downloaded file", "dest", dest)
	return nil
}

// Delete removes a file from the WebDAV server
func (d *DavBlobstore) Delete(dest string) error {
	slog.Debug("Deleting file from WebDAV", "dest", dest)
	return d.storageClient.Delete(dest)
}

// DeleteRecursive deletes all files matching a prefix
func (d *DavBlobstore) DeleteRecursive(prefix string) error {
	slog.Debug("Deleting files recursively from WebDAV", "prefix", prefix)

	// List all blobs with the prefix
	blobs, err := d.storageClient.List(prefix)
	if err != nil {
		return fmt.Errorf("failed to list blobs with prefix '%s': %w", prefix, err)
	}

	slog.Debug("Found blobs to delete", "count", len(blobs), "prefix", prefix)

	// Delete each blob
	for _, blob := range blobs {
		if err := d.storageClient.Delete(blob); err != nil {
			return fmt.Errorf("failed to delete blob '%s': %w", blob, err)
		}
		slog.Debug("Deleted blob", "blob", blob)
	}

	slog.Debug("Successfully deleted all blobs", "prefix", prefix)
	return nil
}

// Exists checks if a file exists on the WebDAV server
func (d *DavBlobstore) Exists(dest string) (bool, error) {
	slog.Debug("Checking if file exists on WebDAV", "dest", dest)

	err := d.storageClient.Exists(dest)
	if err != nil {
		// Check if it's a "not found" error
		if bosherr.WrapError(err, "").Error() == fmt.Sprintf("%s not found", dest) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// Sign generates a pre-signed URL for the blob
func (d *DavBlobstore) Sign(dest string, action string, expiration time.Duration) (string, error) {
	slog.Debug("Signing URL for WebDAV", "dest", dest, "action", action, "expiration", expiration)

	signedURL, err := d.storageClient.Sign(dest, action, expiration)
	if err != nil {
		return "", fmt.Errorf("failed to sign URL: %w", err)
	}

	return signedURL, nil
}

// List returns a list of blob paths that match the given prefix
func (d *DavBlobstore) List(prefix string) ([]string, error) {
	slog.Debug("Listing files on WebDAV", "prefix", prefix)

	blobs, err := d.storageClient.List(prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list blobs: %w", err)
	}

	slog.Debug("Found blobs", "count", len(blobs), "prefix", prefix)
	return blobs, nil
}

// Copy copies a blob from source to destination
func (d *DavBlobstore) Copy(srcBlob string, dstBlob string) error {
	slog.Debug("Copying blob on WebDAV", "source", srcBlob, "dest", dstBlob)

	err := d.storageClient.Copy(srcBlob, dstBlob)
	if err != nil {
		return fmt.Errorf("copy failure: %w", err)
	}

	slog.Debug("Successfully copied blob", "source", srcBlob, "dest", dstBlob)
	return nil
}

// Properties retrieves metadata for a blob
func (d *DavBlobstore) Properties(dest string) error {
	slog.Debug("Getting properties for blob on WebDAV", "dest", dest)
	return d.storageClient.Properties(dest)
}

// EnsureStorageExists ensures the WebDAV directory structure exists
func (d *DavBlobstore) EnsureStorageExists() error {
	slog.Debug("Ensuring WebDAV storage exists")
	return d.storageClient.EnsureStorageExists()
}
