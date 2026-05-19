package client

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
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

	retryClient := httpclient.NewRetryClient(
		httpClientBase,
		config.RetryAttempts,
		time.Duration(0),
		logger,
	)

	storageClient := NewStorageClient(config, retryClient)

	return NewWithStorageClient(storageClient), nil
}

func NewWithStorageClient(storageClient StorageClient) *DavBlobstore {
	return &DavBlobstore{storageClient: storageClient}
}

func (d *DavBlobstore) Put(sourceFilePath string, dest string) error {
	slog.Info("uploading file to webdav", "source", sourceFilePath, "dest", dest)

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

	slog.Debug("successfully uploaded file", "dest", dest)
	return nil
}

func (d *DavBlobstore) Get(source string, dest string) error {
	slog.Info("downloading file from webdav", "source", source, "dest", dest)

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

	slog.Debug("successfully downloaded file", "dest", dest)
	return nil
}

func (d *DavBlobstore) Delete(dest string) error {
	slog.Info("deleting file from webdav", "dest", dest)
	return d.storageClient.Delete(dest)
}

func (d *DavBlobstore) Exists(dest string) (bool, error) {
	slog.Debug("checking if file exists on webdav", "dest", dest)
	return d.storageClient.Exists(dest)
}

func (d *DavBlobstore) Sign(dest string, action string, expiration time.Duration) (string, error) {
	slog.Debug("signing url for webdav", "dest", dest, "action", action, "expiration", expiration)

	action = strings.ToUpper(action)
	switch action {
	case "GET", "PUT":
		signedURL, err := d.storageClient.Sign(dest, action, expiration)
		if err != nil {
			return "", fmt.Errorf("failed to sign URL: %w", err)
		}
		return signedURL, nil
	default:
		return "", fmt.Errorf("action not implemented: %s", action)
	}
}

// DeleteRecursive is not yet implemented in this refactoring
func (d *DavBlobstore) DeleteRecursive(prefix string) error {
	return fmt.Errorf("DeleteRecursive not yet implemented")
}

// List is not yet implemented in this refactoring
func (d *DavBlobstore) List(prefix string) ([]string, error) {
	return nil, fmt.Errorf("List not yet implemented")
}

// Copy is not yet implemented in this refactoring
func (d *DavBlobstore) Copy(srcBlob string, dstBlob string) error {
	return fmt.Errorf("Copy not yet implemented")
}

// Properties is not yet implemented in this refactoring
func (d *DavBlobstore) Properties(dest string) error {
	return fmt.Errorf("Properties not yet implemented")
}

// EnsureStorageExists is not yet implemented in this refactoring
func (d *DavBlobstore) EnsureStorageExists() error {
	return fmt.Errorf("EnsureStorageExists not yet implemented")
}
