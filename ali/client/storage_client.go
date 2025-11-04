package client

import (
	"fmt"
	"log"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/cloudfoundry/storage-cli/ali/config"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . StorageClient
type StorageClient interface {
	Upload(
		sourceFilePath string,
		sourceFileMD5 string,
		destinationObject string,
	) error

	Download(
		sourceObject string,
		destinationFilePath string,
	) error

	Delete(
		object string,
	) error

	Exists(
		object string,
	) (bool, error)

	SignedUrlPut(
		object string,
		expiredInSec int64,
	) (string, error)

	SignedUrlGet(
		object string,
		expiredInSec int64,
	) (string, error)
}

type DefaultStorageClient struct {
	storageConfig config.AliStorageConfig
}

func NewStorageClient(storageConfig config.AliStorageConfig) (StorageClient, error) {
	return DefaultStorageClient{storageConfig: storageConfig}, nil
}

func (dsc DefaultStorageClient) Upload(
	sourceFilePath string,
	sourceFileMD5 string,
	destinationObject string,
) error {
	log.Println(fmt.Sprintf("Uploading %s/%s", dsc.storageConfig.BucketName, destinationObject))

	client, err := oss.New(dsc.storageConfig.Endpoint, dsc.storageConfig.AccessKeyID, dsc.storageConfig.AccessKeySecret)
	if err != nil {
		return err
	}

	bucket, err := client.Bucket(dsc.storageConfig.BucketName)
	if err != nil {
		return err
	}

	return bucket.PutObjectFromFile(destinationObject, sourceFilePath, oss.ContentMD5(sourceFileMD5))
}

func (dsc DefaultStorageClient) Download(
	sourceObject string,
	destinationFilePath string,
) error {
	log.Println(fmt.Sprintf("Downloading %s/%s", dsc.storageConfig.BucketName, sourceObject))

	client, err := oss.New(dsc.storageConfig.Endpoint, dsc.storageConfig.AccessKeyID, dsc.storageConfig.AccessKeySecret)
	if err != nil {
		return err
	}

	bucket, err := client.Bucket(dsc.storageConfig.BucketName)
	if err != nil {
		return err
	}

	return bucket.GetObjectToFile(sourceObject, destinationFilePath)
}

func (dsc DefaultStorageClient) Delete(
	object string,
) error {
	log.Println(fmt.Sprintf("Deleting %s/%s", dsc.storageConfig.BucketName, object))

	client, err := oss.New(dsc.storageConfig.Endpoint, dsc.storageConfig.AccessKeyID, dsc.storageConfig.AccessKeySecret)
	if err != nil {
		return err
	}

	bucket, err := client.Bucket(dsc.storageConfig.BucketName)
	if err != nil {
		return err
	}

	return bucket.DeleteObject(object)
}

func (dsc DefaultStorageClient) Exists(object string) (bool, error) {
	log.Println(fmt.Sprintf("Checking if blob: %s/%s", dsc.storageConfig.BucketName, object))

	client, err := oss.New(dsc.storageConfig.Endpoint, dsc.storageConfig.AccessKeyID, dsc.storageConfig.AccessKeySecret)
	if err != nil {
		return false, err
	}

	bucket, err := client.Bucket(dsc.storageConfig.BucketName)
	if err != nil {
		return false, err
	}

	objectExists, err := bucket.IsObjectExist(object)
	if err != nil {
		return false, err
	}

	if objectExists {
		log.Printf("File '%s' exists in bucket '%s'\n", object, dsc.storageConfig.BucketName)
		return true, nil
	} else {
		log.Printf("File '%s' does not exist in bucket '%s'\n", object, dsc.storageConfig.BucketName)
		return false, nil
	}
}

func (dsc DefaultStorageClient) SignedUrlPut(
	object string,
	expiredInSec int64,
) (string, error) {

	log.Println(fmt.Sprintf("Getting signed PUT url for blob %s/%s", dsc.storageConfig.BucketName, object))

	client, err := oss.New(dsc.storageConfig.Endpoint, dsc.storageConfig.AccessKeyID, dsc.storageConfig.AccessKeySecret)
	if err != nil {
		return "", err
	}

	bucket, err := client.Bucket(dsc.storageConfig.BucketName)
	if err != nil {
		return "", err
	}

	return bucket.SignURL(object, oss.HTTPPut, expiredInSec)
}

func (dsc DefaultStorageClient) SignedUrlGet(
	object string,
	expiredInSec int64,
) (string, error) {

	log.Println(fmt.Sprintf("Getting signed GET url for blob %s/%s", dsc.storageConfig.BucketName, object))

	client, err := oss.New(dsc.storageConfig.Endpoint, dsc.storageConfig.AccessKeyID, dsc.storageConfig.AccessKeySecret)
	if err != nil {
		return "", err
	}

	bucket, err := client.Bucket(dsc.storageConfig.BucketName)
	if err != nil {
		return "", err
	}

	return bucket.SignURL(object, oss.HTTPGet, expiredInSec)
}
