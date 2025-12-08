package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/cloudfoundry/storage-cli/alioss/config"
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

	Copy(
		srcBlob string,
		destBlob string,
	) error

	Delete(
		object string,
	) error

	DeleteRecursive(
		objects string,
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

	List(
		bucketPrefix string,
	) ([]string, error)

	Properties(
		object string,
	) error

	EnsureBucketExists() error
}
type DefaultStorageClient struct {
	storageConfig config.AliStorageConfig
	client        *oss.Client
	bucket        *oss.Bucket
	bucketURL     string
}

func NewStorageClient(storageConfig config.AliStorageConfig) (StorageClient, error) {
	client, err := oss.New(
		storageConfig.Endpoint,
		storageConfig.AccessKeyID,
		storageConfig.AccessKeySecret,
	)
	if err != nil {
		return nil, err
	}

	bucket, err := client.Bucket(storageConfig.BucketName)
	if err != nil {
		return nil, err
	}

	endpoint := strings.TrimPrefix(storageConfig.Endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	bucketURL := fmt.Sprintf("https://%s.%s", storageConfig.BucketName, endpoint)

	return DefaultStorageClient{
		storageConfig: storageConfig,
		client:        client,
		bucket:        bucket,
		bucketURL:     bucketURL,
	}, nil
}

func (dsc DefaultStorageClient) Upload(
	sourceFilePath string,
	sourceFileMD5 string,
	destinationObject string,
) error {
	log.Printf("Uploading %s/%s\n", dsc.storageConfig.BucketName, destinationObject)

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
	log.Printf("Downloading %s/%s\n", dsc.storageConfig.BucketName, sourceObject)

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

func (dsc DefaultStorageClient) Copy(
	sourceObject string,
	destinationObject string,
) error {
	log.Printf("Copying object from %s to %s", sourceObject, destinationObject)
	srcURL := fmt.Sprintf("%s/%s", dsc.bucketURL, sourceObject)
	destURL := fmt.Sprintf("%s/%s", dsc.bucketURL, destinationObject)

	client, err := oss.New(dsc.storageConfig.Endpoint, dsc.storageConfig.AccessKeyID, dsc.storageConfig.AccessKeySecret)
	if err != nil {
		return err
	}

	bucket, err := client.Bucket(dsc.storageConfig.BucketName)
	if err != nil {
		return err
	}

	if _, err := bucket.CopyObject(sourceObject, destinationObject); err != nil {
		return fmt.Errorf("failed to copy object from %s to %s: %w", srcURL, destURL, err)
	}

	return nil
}

func (dsc DefaultStorageClient) Delete(
	object string,
) error {
	log.Printf("Deleting %s/%s\n", dsc.storageConfig.BucketName, object)

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

func (dsc DefaultStorageClient) DeleteRecursive(
	prefix string,
) error {
	if prefix != "" {
		log.Printf("Deleting all objects in bucket %s with prefix '%s'\n",
			dsc.storageConfig.BucketName, prefix)
	} else {
		log.Printf("Deleting all objects in bucket %s\n",
			dsc.storageConfig.BucketName)
	}

	marker := ""

	for {
		var listOptions []oss.Option
		if prefix != "" {
			listOptions = append(listOptions, oss.Prefix(prefix))
		}
		if marker != "" {
			listOptions = append(listOptions, oss.Marker(marker))
		}
		client, err := oss.New(dsc.storageConfig.Endpoint, dsc.storageConfig.AccessKeyID, dsc.storageConfig.AccessKeySecret)
		if err != nil {
			return err
		}

		bucket, err := client.Bucket(dsc.storageConfig.BucketName)
		if err != nil {
			return err
		}

		resp, err := bucket.ListObjects(listOptions...)
		if err != nil {
			return fmt.Errorf("error listing objects: %w", err)
		}

		for _, object := range resp.Objects {
			if err := bucket.DeleteObject(object.Key); err != nil {
				log.Printf("Failed to delete object %s: %v\n", object.Key, err)
			}
		}

		if !resp.IsTruncated {
			break
		}

		marker = resp.NextMarker
	}

	return nil
}

func (dsc DefaultStorageClient) Exists(object string) (bool, error) {
	log.Printf("Checking if blob: %s/%s\n", dsc.storageConfig.BucketName, object)

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

	log.Printf("Getting signed PUT url for blob %s/%s\n", dsc.storageConfig.BucketName, object)

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

	log.Printf("Getting signed GET url for blob %s/%s\n", dsc.storageConfig.BucketName, object)

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

func (dsc DefaultStorageClient) List(
	prefix string,
) ([]string, error) {
	if prefix != "" {
		log.Printf("Listing objects in bucket %s with prefix '%s'\n",
			dsc.storageConfig.BucketName, prefix)
	} else {
		log.Printf("Listing objects in bucket %s\n", dsc.storageConfig.BucketName)
	}

	var (
		objects []string
		marker  string
	)

	for {
		var opts []oss.Option
		if prefix != "" {
			opts = append(opts, oss.Prefix(prefix))
		}
		if marker != "" {
			opts = append(opts, oss.Marker(marker))
		}

		client, err := oss.New(dsc.storageConfig.Endpoint, dsc.storageConfig.AccessKeyID, dsc.storageConfig.AccessKeySecret)
		if err != nil {
			return nil, err
		}

		bucket, err := client.Bucket(dsc.storageConfig.BucketName)
		if err != nil {
			return nil, err
		}

		resp, err := bucket.ListObjects(opts...)
		if err != nil {
			return nil, fmt.Errorf("error retrieving page of objects: %w", err)
		}

		for _, obj := range resp.Objects {
			objects = append(objects, obj.Key)
		}

		if !resp.IsTruncated {
			break
		}
		marker = resp.NextMarker
	}

	return objects, nil
}

type BlobProperties struct {
	ETag          string    `json:"etag,omitempty"`
	LastModified  time.Time `json:"last_modified,omitempty"`
	ContentLength int64     `json:"content_length,omitempty"`
}

func (dsc DefaultStorageClient) Properties(
	object string,
) error {
	log.Printf("Getting properties for object %s/%s\n",
		dsc.storageConfig.BucketName, object)

	client, err := oss.New(dsc.storageConfig.Endpoint, dsc.storageConfig.AccessKeyID, dsc.storageConfig.AccessKeySecret)
	if err != nil {
		return err
	}

	bucket, err := client.Bucket(dsc.storageConfig.BucketName)
	if err != nil {
		return err
	}

	meta, err := bucket.GetObjectDetailedMeta(object)
	if err != nil {
		var ossErr oss.ServiceError
		if errors.As(err, &ossErr) && ossErr.StatusCode == 404 {
			fmt.Println(`{}`)
			return nil
		}

		return fmt.Errorf("failed to get properties for object %s: %w", object, err)
	}

	eTag := meta.Get("ETag")
	lastModifiedStr := meta.Get("Last-Modified")
	contentLengthStr := meta.Get("Content-Length")

	var (
		lastModified  time.Time
		contentLength int64
	)

	if lastModifiedStr != "" {
		t, parseErr := time.Parse(time.RFC1123, lastModifiedStr)
		if parseErr == nil {
			lastModified = t
		}
	}

	if contentLengthStr != "" {
		cl, convErr := strconv.ParseInt(contentLengthStr, 10, 64)
		if convErr == nil {
			contentLength = cl
		}
	}

	props := BlobProperties{
		ETag:          strings.Trim(eTag, `"`),
		LastModified:  lastModified,
		ContentLength: contentLength,
	}

	output, err := json.MarshalIndent(props, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal object properties: %w", err)
	}

	fmt.Println(string(output))
	return nil
}

func (dsc DefaultStorageClient) EnsureBucketExists() error {
	log.Printf("Ensuring bucket '%s' exists\n", dsc.storageConfig.BucketName)

	client, err := oss.New(dsc.storageConfig.Endpoint, dsc.storageConfig.AccessKeyID, dsc.storageConfig.AccessKeySecret)
	if err != nil {
		return err
	}

	exists, err := client.IsBucketExist(dsc.storageConfig.BucketName)
	if err != nil {
		return fmt.Errorf("failed to check if bucket exists: %w", err)
	}

	if exists {
		log.Printf("Bucket '%s' already exists\n", dsc.storageConfig.BucketName)
		return nil
	}

	if err := client.CreateBucket(dsc.storageConfig.BucketName); err != nil {
		return fmt.Errorf("failed to create bucket '%s': %w", dsc.storageConfig.BucketName, err)
	}

	log.Printf("Bucket '%s' created successfully\n", dsc.storageConfig.BucketName)
	return nil
}
