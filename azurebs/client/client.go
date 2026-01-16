package client

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

type AzBlobstore struct {
	storageClient StorageClient
}

func New(storageClient StorageClient) (AzBlobstore, error) {
	return AzBlobstore{storageClient: storageClient}, nil
}

func (client *AzBlobstore) Put(sourceFilePath string, dest string) error {
	sourceMD5, err := client.getMD5(sourceFilePath)
	if err != nil {
		return err
	}

	source, err := os.Open(sourceFilePath)
	if err != nil {
		return err
	}
	defer source.Close() //nolint:errcheck

	md5, err := client.storageClient.Upload(source, dest)
	if err != nil {
		return fmt.Errorf("upload failure: %w", err)
	}

	if !bytes.Equal(sourceMD5, md5) {
		slog.Error("Upload failed due to MD5 mismatch, deleting blob", "blob", dest, "expected_md5", fmt.Sprintf("%x", sourceMD5), "received_md5", fmt.Sprintf("%x", md5))

		err := client.storageClient.Delete(dest)
		if err != nil {
			slog.Error("Failed to delete blob after MD5 mismatch", "blob", dest, "error", err)

		}
		return fmt.Errorf("MD5 mismatch: expected %x, got %x", sourceMD5, md5)
	}

	slog.Debug("MD5 verification passed", "blob", dest, "md5", fmt.Sprintf("%x", md5))
	return nil
}

func (client *AzBlobstore) Get(source string, dest string) error {
	dstFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close() //nolint:errcheck

	return client.storageClient.Download(source, dstFile)
}

func (client *AzBlobstore) Delete(dest string) error {

	return client.storageClient.Delete(dest)
}

func (client *AzBlobstore) DeleteRecursive(prefix string) error {

	return client.storageClient.DeleteRecursive(prefix)
}

func (client *AzBlobstore) Exists(dest string) (bool, error) {

	return client.storageClient.Exists(dest)
}

func (client *AzBlobstore) Sign(dest string, action string, expiration time.Duration) (string, error) {
	action = strings.ToUpper(action)
	switch action {
	case "GET", "PUT":
		return client.storageClient.SignedUrl(action, dest, expiration)
	default:
		return "", fmt.Errorf("action not implemented: %s", action)
	}
}

func (client *AzBlobstore) getMD5(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	defer file.Close() //nolint:errcheck

	hash := md5.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate md5: %w", err)
	}

	return hash.Sum(nil), nil
}

func (client *AzBlobstore) List(prefix string) ([]string, error) {
	return client.storageClient.List(prefix)
}

func (client *AzBlobstore) Copy(srcBlob string, dstBlob string) error {

	return client.storageClient.Copy(srcBlob, dstBlob)
}

func (client *AzBlobstore) Properties(dest string) error {

	return client.storageClient.Properties(dest)
}

func (client *AzBlobstore) EnsureStorageExists() error {

	return client.storageClient.EnsureContainerExists()
}
