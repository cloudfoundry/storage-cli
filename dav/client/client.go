package client

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	davconf "github.com/cloudfoundry/storage-cli/dav/config"
)

type DavBlobstore struct {
	storageClient StorageClient
}

func New(config davconf.Config) (*DavBlobstore, error) {
	logger := boshlog.NewLogger(boshlog.LevelNone)

	var httpClientBase httpclient.Client
	var certPool, err = getCertPool(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate pool: %w", err)
	}

	httpClientBase = httpclient.CreateDefaultClient(certPool)

	if config.RetryAttempts == 0 {
		config.RetryAttempts = 3
	}

	retryDelay := time.Duration(1) * time.Second
	if config.RetryDelay > 0 {
		retryDelay = time.Duration(config.RetryDelay) * time.Second
	}

	retryClient := httpclient.NewRetryClient(
		httpClientBase,
		config.RetryAttempts,
		retryDelay,
		logger,
	)

	storageClient, err := NewStorageClient(config, retryClient)
	if err != nil {
		return nil, err
	}

	return NewWithStorageClient(storageClient), nil
}

func NewWithStorageClient(storageClient StorageClient) *DavBlobstore {
	return &DavBlobstore{storageClient: storageClient}
}

func (d *DavBlobstore) Put(sourceFilePath string, dest string) error {
	slog.Info("Uploading file to WebDAV", "source", sourceFilePath, "dest", dest)

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

	slog.Info("Successfully uploaded file", "dest", dest)
	return nil
}

func (d *DavBlobstore) Get(source string, dest string) error {
	slog.Info("Downloading file from WebDAV", "source", source, "dest", dest)

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

	slog.Info("Successfully downloaded file", "dest", dest)
	return nil
}

func (d *DavBlobstore) Delete(dest string) error {
	slog.Info("Deleting file from WebDAV", "dest", dest)
	return d.storageClient.Delete(dest)
}

func (d *DavBlobstore) DeleteRecursive(prefix string) error {
	slog.Info("Deleting files recursively from WebDAV", "prefix", prefix)

	blobs, err := d.storageClient.List(prefix)
	if err != nil {
		return fmt.Errorf("failed to list blobs with prefix '%s': %w", prefix, err)
	}

	slog.Info("Found blobs to delete", "count", len(blobs), "prefix", prefix)

	for _, blob := range blobs {
		if err := d.storageClient.Delete(blob); err != nil {
			return fmt.Errorf("failed to delete blob '%s': %w", blob, err)
		}
		slog.Info("Deleted blob", "blob", blob)
	}

	slog.Info("Successfully deleted all blobs", "prefix", prefix)
	return nil
}

func (d *DavBlobstore) Exists(dest string) (bool, error) {
	slog.Info("Checking if file exists on WebDAV", "dest", dest)
	return d.storageClient.Exists(dest)
}

func (d *DavBlobstore) Sign(dest string, action string, expiration time.Duration) (string, error) {
	slog.Info("Signing URL for WebDAV", "dest", dest, "action", action, "expiration", expiration)

	signedURL, err := d.storageClient.Sign(dest, action, expiration)
	if err != nil {
		return "", fmt.Errorf("failed to sign URL: %w", err)
	}

	return signedURL, nil
}

func (d *DavBlobstore) List(prefix string) ([]string, error) {
	slog.Info("Listing files on WebDAV", "prefix", prefix)

	blobs, err := d.storageClient.List(prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list blobs: %w", err)
	}

	slog.Info("Found blobs", "count", len(blobs), "prefix", prefix)
	return blobs, nil
}

func (d *DavBlobstore) Copy(srcBlob string, dstBlob string) error {
	slog.Info("Copying blob on WebDAV", "source", srcBlob, "dest", dstBlob)

	err := d.storageClient.Copy(srcBlob, dstBlob)
	if err != nil {
		return fmt.Errorf("copy failure: %w", err)
	}

	slog.Info("Successfully copied blob", "source", srcBlob, "dest", dstBlob)
	return nil
}

func (d *DavBlobstore) Properties(dest string) error {
	slog.Info("Getting properties for blob on WebDAV", "dest", dest)
	return d.storageClient.Properties(dest)
}

func (d *DavBlobstore) EnsureStorageExists() error {
	slog.Info("Ensuring WebDAV storage exists")
	return d.storageClient.EnsureStorageExists()
}
