package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	azBlob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	azContainer "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"

	"github.com/cloudfoundry/storage-cli/azure/config"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . StorageClient
type StorageClient interface {
	Upload(
		source io.ReadSeekCloser,
		dest string,
	) ([]byte, error)

	Download(
		source string,
		dest *os.File,
	) error

	Copy(
		srcBlob string,
		destBlob string,
	) error

	Delete(
		dest string,
	) error

	DeleteRecursive(
		dest string,
	) error

	Exists(
		dest string,
	) (bool, error)

	SignedUrl(
		requestType string,
		dest string,
		expiration time.Duration,
	) (string, error)

	List(
		prefix string,
	) ([]string, error)
	Properties(
		dest string,
	) error
	EnsureContainerExists() error
}

type DefaultStorageClient struct {
	credential    *azblob.SharedKeyCredential
	serviceURL    string
	storageConfig config.AZStorageConfig
}

func NewStorageClient(storageConfig config.AZStorageConfig) (StorageClient, error) {
	credential, err := azblob.NewSharedKeyCredential(storageConfig.AccountName, storageConfig.AccountKey)
	if err != nil {
		return nil, err
	}

	serviceURL := fmt.Sprintf("https://%s.%s/%s", storageConfig.AccountName, storageConfig.StorageEndpoint(), storageConfig.ContainerName)

	return DefaultStorageClient{credential: credential, serviceURL: serviceURL, storageConfig: storageConfig}, nil
}

func (dsc DefaultStorageClient) Upload(
	source io.ReadSeekCloser,
	dest string,
) ([]byte, error) {
	blobURL := fmt.Sprintf("%s/%s", dsc.serviceURL, dest)

	var ctx context.Context
	var cancel context.CancelFunc

	if dsc.storageConfig.Timeout != "" {
		timeoutInt, err := strconv.Atoi(dsc.storageConfig.Timeout)
		timeout := time.Duration(timeoutInt) * time.Second
		if timeout < 1 && err == nil {
			log.Printf("Invalid time \"%s\", need at least 1 second", dsc.storageConfig.Timeout)
			return nil, fmt.Errorf("invalid time: %w", err)
		}
		if err != nil {
			log.Printf("Invalid timeout format \"%s\", need \"<seconds in number>\" e.g. 30", dsc.storageConfig.Timeout)
			return nil, fmt.Errorf("invalid timeout format: %w", err)
		}
		log.Println(fmt.Sprintf("Uploading %s with a timeout of %s", blobURL, timeout)) //nolint:staticcheck
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	} else {
		log.Println(fmt.Sprintf("Uploading %s with no timeout", blobURL)) //nolint:staticcheck
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	client, err := blockblob.NewClientWithSharedKeyCredential(blobURL, dsc.credential, nil)
	if err != nil {
		return nil, err
	}
	uploadResponse, err := client.Upload(ctx, source, nil)
	if err != nil {
		if dsc.storageConfig.Timeout != "" && errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("upload failed: timeout of %s reached while uploading %s", dsc.storageConfig.Timeout, dest)
		}
		return nil, fmt.Errorf("upload failure: %w", err)
	}
	return uploadResponse.ContentMD5, err
}

func (dsc DefaultStorageClient) Download(
	source string,
	dest *os.File,
) error {

	blobURL := fmt.Sprintf("%s/%s", dsc.serviceURL, source)

	log.Println(fmt.Sprintf("Downloading %s", blobURL)) //nolint:staticcheck
	client, err := blockblob.NewClientWithSharedKeyCredential(blobURL, dsc.credential, nil)
	if err != nil {
		return err
	}

	blobSize, err := client.DownloadFile(context.Background(), dest, nil) //nolint:ineffassign,staticcheck
	if err != nil {
		return err
	}
	info, err := dest.Stat()
	if err != nil {
		return err
	}
	if blobSize != info.Size() {
		log.Printf("Truncating file according to the blob size %v", blobSize)
		dest.Truncate(blobSize) //nolint:errcheck
	}

	return nil
}

func (dsc DefaultStorageClient) Copy(
	srcBlob string,
	destBlob string,
) error {
	log.Printf("Copying blob from %s to %s", srcBlob, destBlob)

	srcURL := fmt.Sprintf("%s/%s", dsc.serviceURL, srcBlob)
	destURL := fmt.Sprintf("%s/%s", dsc.serviceURL, destBlob)

	destClient, err := blockblob.NewClientWithSharedKeyCredential(destURL, dsc.credential, nil)
	if err != nil {
		return fmt.Errorf("failed to create destination client: %w", err)
	}

	resp, err := destClient.StartCopyFromURL(context.Background(), srcURL, nil)
	if err != nil {
		return fmt.Errorf("failed to start copy: %w", err)
	}

	copyID := *resp.CopyID
	log.Printf("Copy started with CopyID: %s", copyID)

	// Wait for completion
	for {
		props, err := destClient.GetProperties(context.Background(), nil)
		if err != nil {
			return fmt.Errorf("failed to get properties: %w", err)
		}

		copyStatus := *props.CopyStatus
		log.Printf("Copy status: %s", copyStatus)

		switch copyStatus {
		case "success":
			log.Println("Copy completed successfully")
			return nil
		case "pending":
			time.Sleep(200 * time.Millisecond)
		default:
			return fmt.Errorf("copy failed or aborted with status: %s", copyStatus)
		}
	}
}

func (dsc DefaultStorageClient) Delete(
	dest string,
) error {

	blobURL := fmt.Sprintf("%s/%s", dsc.serviceURL, dest)

	log.Println(fmt.Sprintf("Deleting %s", blobURL)) //nolint:staticcheck
	client, err := blockblob.NewClientWithSharedKeyCredential(blobURL, dsc.credential, nil)
	if err != nil {
		return err
	}

	_, err = client.Delete(context.Background(), nil)

	if err == nil {
		return nil
	}

	if strings.Contains(err.Error(), "RESPONSE 404") {
		return nil
	}

	return err
}

func (dsc DefaultStorageClient) DeleteRecursive(
	prefix string,
) error {
	if prefix != "" {
		log.Printf("Deleting all blobs in container %s with prefix '%s'\n", dsc.storageConfig.ContainerName, prefix)
	} else {
		log.Printf("Deleting all blobs in container %s\n", dsc.storageConfig.ContainerName)
	}

	containerClient, err := azContainer.NewClientWithSharedKeyCredential(dsc.serviceURL, dsc.credential, nil)
	if err != nil {
		return fmt.Errorf("failed to create container client: %w", err)
	}

	options := &azContainer.ListBlobsFlatOptions{}
	if prefix != "" {
		options.Prefix = &prefix
	}

	pager := containerClient.NewListBlobsFlatPager(options)

	for pager.More() {
		resp, err := pager.NextPage(context.Background())
		if err != nil {
			return fmt.Errorf("error retrieving page of blobs: %w", err)
		}

		for _, blob := range resp.Segment.BlobItems {
			blobURL := fmt.Sprintf("%s/%s", dsc.serviceURL, *blob.Name)
			blobClient, err := blockblob.NewClientWithSharedKeyCredential(blobURL, dsc.credential, nil)
			if err != nil {
				log.Printf("Failed to create blob client for %s: %v\n", *blob.Name, err)
				continue
			}

			_, err = blobClient.BlobClient().Delete(context.Background(), nil)
			if err != nil && !strings.Contains(err.Error(), "RESPONSE 404") {
				log.Printf("Failed to delete blob %s: %v\n", *blob.Name, err)
			}
		}
	}

	return nil
}

func (dsc DefaultStorageClient) Exists(
	dest string,
) (bool, error) {

	blobURL := fmt.Sprintf("%s/%s", dsc.serviceURL, dest)

	log.Println(fmt.Sprintf("Checking if blob: %s exists", blobURL)) //nolint:staticcheck
	client, err := blockblob.NewClientWithSharedKeyCredential(blobURL, dsc.credential, nil)
	if err != nil {
		return false, err
	}

	_, err = client.BlobClient().GetProperties(context.Background(), nil)
	if err == nil {
		log.Printf("File '%s' exists in bucket '%s'\n", dest, dsc.storageConfig.ContainerName)
		return true, nil
	}
	if strings.Contains(err.Error(), "RESPONSE 404") {
		log.Printf("File '%s' does not exist in bucket '%s'\n", dest, dsc.storageConfig.ContainerName)
		return false, nil
	}

	return false, err
}

func (dsc DefaultStorageClient) SignedUrl(
	requestType string,
	dest string,
	expiration time.Duration,
) (string, error) {

	blobURL := fmt.Sprintf("%s/%s", dsc.serviceURL, dest)

	log.Println(fmt.Sprintf("Getting signed url for blob %s", blobURL)) //nolint:staticcheck
	client, err := azBlob.NewClientWithSharedKeyCredential(blobURL, dsc.credential, nil)
	if err != nil {
		return "", err
	}

	url, err := client.GetSASURL(sas.BlobPermissions{Read: true, Create: true}, time.Now().Add(expiration), nil)
	if err != nil {
		return "", err
	}

	// There could be occasional issues with the Azure Storage Account when requests hitting
	// the server are not responded to, and then BOSH hangs while expecting a reply from the server.
	// That's why we implement a server-side timeout here (30 mins for GET and 45 mins for PUT)
	// (see: https://learn.microsoft.com/en-us/rest/api/storageservices/setting-timeouts-for-blob-service-operations)
	if requestType == "GET" {
		url += "&timeout=1800"
	} else {
		url += "&timeout=2700"
	}

	return url, err
}

func (dsc DefaultStorageClient) List(
	prefix string,
) ([]string, error) {

	if prefix != "" {
		log.Println(fmt.Sprintf("Listing blobs in container %s with prefix '%s'", dsc.storageConfig.ContainerName, prefix)) //nolint:staticcheck
	} else {
		log.Println(fmt.Sprintf("Listing blobs in container %s", dsc.storageConfig.ContainerName)) //nolint:staticcheck
	}

	client, err := azContainer.NewClientWithSharedKeyCredential(dsc.serviceURL, dsc.credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create container client: %w", err)
	}

	options := &azContainer.ListBlobsFlatOptions{}
	if prefix != "" {
		options.Prefix = &prefix
	}

	pager := client.NewListBlobsFlatPager(options)
	var blobs []string

	for pager.More() {
		resp, err := pager.NextPage(context.Background())
		if err != nil {
			return nil, fmt.Errorf("error retrieving page of blobs: %w", err)
		}

		for _, blob := range resp.Segment.BlobItems {
			blobs = append(blobs, *blob.Name)
		}
	}

	return blobs, nil
}

type BlobProperties struct {
	ETag          string    `json:"etag,omitempty"`
	LastModified  time.Time `json:"last_modified,omitempty"`
	ContentLength int64     `json:"content_length,omitempty"`
}

func (dsc DefaultStorageClient) Properties(
	dest string,
) error {
	blobURL := fmt.Sprintf("%s/%s", dsc.serviceURL, dest)

	log.Println(fmt.Sprintf("Getting properties for blob %s", blobURL)) //nolint:staticcheck
	client, err := blockblob.NewClientWithSharedKeyCredential(blobURL, dsc.credential, nil)
	if err != nil {
		return err
	}

	resp, err := client.GetProperties(context.Background(), nil)
	if err != nil {
		if strings.Contains(err.Error(), "RESPONSE 404") {
			fmt.Println(`{}`)
			return nil
		}
		return fmt.Errorf("failed to get properties for blob %s: %w", dest, err)
	}

	props := BlobProperties{
		ETag:          strings.Trim(string(*resp.ETag), `"`),
		LastModified:  *resp.LastModified,
		ContentLength: *resp.ContentLength,
	}

	output, err := json.MarshalIndent(props, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal blob properties: %w", err)
	}

	fmt.Println(string(output))
	return nil
}

func (dsc DefaultStorageClient) EnsureContainerExists() error {
	log.Printf("Ensuring container '%s' exists\n", dsc.storageConfig.ContainerName)

	containerClient, err := azContainer.NewClientWithSharedKeyCredential(dsc.serviceURL, dsc.credential, nil)
	if err != nil {
		return fmt.Errorf("failed to create container client: %w", err)
	}

	_, err = containerClient.Create(context.Background(), nil)
	if err != nil {
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) && respErr.ErrorCode == string(bloberror.ContainerAlreadyExists) {
			log.Printf("Container '%s' already exists", dsc.storageConfig.ContainerName)
			return nil
		}
		return fmt.Errorf("failed to create container: %w", err)
	}

	log.Printf("Container '%s' created successfully", dsc.storageConfig.ContainerName)
	return nil
}
