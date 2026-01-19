package client

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type AliBlobstore struct {
	storageClient StorageClient
}

func New(storageClient StorageClient) (AliBlobstore, error) {
	return AliBlobstore{storageClient: storageClient}, nil
}

func (client *AliBlobstore) Put(sourceFilePath string, destinationObject string) error {
	sourceFileMD5, err := client.getMD5(sourceFilePath)
	if err != nil {
		return err
	}

	err = client.storageClient.Upload(sourceFilePath, sourceFileMD5, destinationObject)
	if err != nil {
		return fmt.Errorf("upload failure: %w", err)
	}
	return nil
}

func (client *AliBlobstore) Get(sourceObject string, dest string) error {
	return client.storageClient.Download(sourceObject, dest)
}

func (client *AliBlobstore) Delete(object string) error {
	return client.storageClient.Delete(object)
}

func (client *AliBlobstore) Exists(object string) (bool, error) {
	return client.storageClient.Exists(object)
}

func (client *AliBlobstore) Sign(object string, action string, expiration time.Duration) (string, error) {
	action = strings.ToUpper(action)
	expiredInSec := int64(expiration.Seconds())
	switch action {
	case "PUT":
		return client.storageClient.SignedUrlPut(object, expiredInSec)
	case "GET":
		return client.storageClient.SignedUrlGet(object, expiredInSec)
	default:
		return "", fmt.Errorf("action not implemented: %s", action)
	}
}

func (client *AliBlobstore) getMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}

	defer file.Close() //nolint:errcheck

	hash := md5.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return "", fmt.Errorf("failed to calculate md5: %w", err)
	}

	md5 := base64.StdEncoding.EncodeToString(hash.Sum(nil))

	return md5, nil
}

func (client *AliBlobstore) List(prefix string) ([]string, error) {
	return client.storageClient.List(prefix)
}

func (client *AliBlobstore) Copy(srcBlob string, dstBlob string) error {
	return client.storageClient.Copy(srcBlob, dstBlob)
}

func (client *AliBlobstore) Properties(dest string) error {
	return client.storageClient.Properties(dest)
}

func (client *AliBlobstore) EnsureStorageExists() error {
	return client.storageClient.EnsureBucketExists()
}

func (client *AliBlobstore) DeleteRecursive(prefix string) error {
	return client.storageClient.DeleteRecursive(prefix)
}
