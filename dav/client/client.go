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

	if err := validateBlobID(dest); err != nil {
		return err
	}

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

	if err := validateBlobID(source); err != nil {
		return err
	}

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
	if err := validateBlobID(dest); err != nil {
		return err
	}
	return d.storageClient.Delete(dest)
}

func (d *DavBlobstore) Exists(dest string) (bool, error) {
	slog.Info("checking if file exists on webdav", "dest", dest)
	if err := validateBlobID(dest); err != nil {
		return false, err
	}
	return d.storageClient.Exists(dest)
}

func (d *DavBlobstore) Sign(dest string, action string, expiration time.Duration) (string, error) {
	slog.Info("signing url for webdav", "dest", dest, "action", action, "expiration", expiration)
	if err := validateBlobID(dest); err != nil {
		return "", err
	}
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

func (d *DavBlobstore) DeleteRecursive(prefix string) error {
	slog.Info("deleting blobs recursively from webdav", "prefix", prefix)
	return d.storageClient.DeleteRecursive(prefix)
}

func (d *DavBlobstore) List(prefix string) ([]string, error) {
	slog.Info("listing blobs on webdav", "prefix", prefix)
	if prefix != "" {
		if err := validatePrefix(prefix); err != nil {
			return nil, err
		}
	}
	return d.storageClient.List(prefix)
}

func (d *DavBlobstore) Copy(srcBlob string, dstBlob string) error {
	slog.Info("copying blob on webdav", "src", srcBlob, "dst", dstBlob)
	if err := validateBlobID(srcBlob); err != nil {
		return fmt.Errorf("invalid source blob ID: %w", err)
	}
	if err := validateBlobID(dstBlob); err != nil {
		return fmt.Errorf("invalid destination blob ID: %w", err)
	}
	return d.storageClient.Copy(srcBlob, dstBlob)
}

func (d *DavBlobstore) Properties(dest string) error {
	slog.Info("fetching blob properties from webdav", "dest", dest)
	if err := validateBlobID(dest); err != nil {
		return err
	}
	return d.storageClient.Properties(dest)
}

func (d *DavBlobstore) EnsureStorageExists() error {
	slog.Info("ensuring webdav storage root exists")
	return d.storageClient.EnsureStorageExists()
}
