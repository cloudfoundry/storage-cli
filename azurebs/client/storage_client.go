package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
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

	"github.com/cloudfoundry/storage-cli/azurebs/config"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . StorageClient
type StorageClient interface {
	Upload(
		source io.ReadSeekCloser,
		dest string,
	) ([]byte, error)

	UploadStream(
		source io.ReadSeekCloser,
		dest string,
	) error

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

// 4 MB of block size
const blockSize = int64(4 * 1024 * 1024)

// number of go routines
const maxConcurrency = 5

func createContext(dsc DefaultStorageClient) (context.Context, context.CancelFunc, error) {
	var ctx context.Context
	var cancel context.CancelFunc

	if dsc.storageConfig.Timeout != "" {
		timeoutInt, err := strconv.Atoi(dsc.storageConfig.Timeout)
		timeout := time.Duration(timeoutInt) * time.Second
		if timeout < 1 && err == nil {
			slog.Info("Invalid time, need at least 1 second", "timeout", dsc.storageConfig.Timeout)
			return nil, nil, fmt.Errorf("invalid time: %w", err)
		}
		if err != nil {
			slog.Info("Invalid timeout format, need seconds as number e.g. 30s", "timeout", dsc.storageConfig.Timeout)
			return nil, nil, fmt.Errorf("invalid timeout format: %w", err)
		}
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}

	return ctx, cancel, nil

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

	if dsc.storageConfig.Timeout != "" {
		slog.Info("Uploading blob to container", "container", dsc.storageConfig.ContainerName, "blob", dest, "url", blobURL, "timeout", dsc.storageConfig.Timeout)
	} else {
		slog.Info("Uploading blob to container", "container", dsc.storageConfig.ContainerName, "blob", dest, "url", blobURL)
	}

	ctx, cancel, err := createContext(dsc)
	if err != nil {
		return nil, err
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

	slog.Info("Successfully uploaded blob", "container", dsc.storageConfig.ContainerName, "blob", dest)
	return uploadResponse.ContentMD5, nil
}

func (dsc DefaultStorageClient) UploadStream(
	source io.ReadSeekCloser,
	dest string,
) error {
	blobURL := fmt.Sprintf("%s/%s", dsc.serviceURL, dest)

	if dsc.storageConfig.Timeout != "" {
		slog.Info("UploadStreaming blob to container", "container", dsc.storageConfig.ContainerName, "blob", dest, "url", blobURL, "timeout", dsc.storageConfig.Timeout)
	} else {
		slog.Info("UploadStreaming blob to container", "container", dsc.storageConfig.ContainerName, "blob", dest, "url", blobURL)
	}

	ctx, cancel, err := createContext(dsc)
	if err != nil {
		return err
	}
	defer cancel()

	client, err := blockblob.NewClientWithSharedKeyCredential(blobURL, dsc.credential, nil)
	if err != nil {
		return err
	}

	_, err = client.UploadStream(ctx, source, &azblob.UploadStreamOptions{BlockSize: blockSize, Concurrency: maxConcurrency})
	if err != nil {
		if dsc.storageConfig.Timeout != "" && errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("upload failed: timeout of %s reached while uploading %s", dsc.storageConfig.Timeout, dest)
		}
		return fmt.Errorf("upload failure: %w", err)
	}

	slog.Info("Successfully uploaded blob", "container", dsc.storageConfig.ContainerName, "blob", dest)
	return nil
}

func (dsc DefaultStorageClient) Download(
	source string,
	dest *os.File,
) error {
	blobURL := fmt.Sprintf("%s/%s", dsc.serviceURL, source)
	slog.Info("Downloading blob from container", "container", dsc.storageConfig.ContainerName, "blob", source, "local_file", dest.Name())
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
		slog.Debug("Truncating file to blob size", "blob_size", blobSize)
		dest.Truncate(blobSize) //nolint:errcheck
	}

	return nil
}

func (dsc DefaultStorageClient) Copy(
	srcBlob string,
	destBlob string,
) error {
	slog.Info("Copying blob within container", "container", dsc.storageConfig.ContainerName, "source_blob", srcBlob, "dest_blob", destBlob)

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
	slog.Debug("Copy started", "copy_id", copyID)

	// Wait for completion
	for {
		props, err := destClient.GetProperties(context.Background(), nil)
		if err != nil {
			return fmt.Errorf("failed to get properties: %w", err)
		}

		copyStatus := *props.CopyStatus
		slog.Debug("Copy status", "status", copyStatus)

		switch copyStatus {
		case "success":
			slog.Info("Copy completed successfully", "container", dsc.storageConfig.ContainerName, "source_blob", srcBlob, "dest_blob", destBlob)
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

	slog.Info("Deleting blob from container", "container", dsc.storageConfig.ContainerName, "blob", dest, "url", blobURL)
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
		slog.Info("Deleting all blobs in container", "container", dsc.storageConfig.ContainerName, "prefix", prefix)
	} else {
		slog.Info("Deleting all blobs in container", "container", dsc.storageConfig.ContainerName)
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
				slog.Error("Failed to create blob client", "blob", *blob.Name, "error", err)
				continue
			}

			_, err = blobClient.BlobClient().Delete(context.Background(), nil)
			if err != nil && !strings.Contains(err.Error(), "RESPONSE 404") {
				slog.Error("Failed to delete blob", "blob", *blob.Name, "error", err)
			}
		}
	}

	return nil
}

func (dsc DefaultStorageClient) Exists(
	dest string,
) (bool, error) {

	blobURL := fmt.Sprintf("%s/%s", dsc.serviceURL, dest)

	slog.Info("Checking if blob exists", "container", dsc.storageConfig.ContainerName, "blob", dest, "url", blobURL)
	client, err := blockblob.NewClientWithSharedKeyCredential(blobURL, dsc.credential, nil)
	if err != nil {
		return false, err
	}

	_, err = client.BlobClient().GetProperties(context.Background(), nil)
	if err == nil {
		slog.Info("Blob exists in container", "container", dsc.storageConfig.ContainerName, "blob", dest)
		return true, nil
	}
	if strings.Contains(err.Error(), "RESPONSE 404") {
		slog.Info("Blob does not exist in container", "container", dsc.storageConfig.ContainerName, "blob", dest)
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

	slog.Info("Generating SAS URL for blob", "container", dsc.storageConfig.ContainerName, "blob", dest, "request_type", requestType, "expiration", expiration)
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
		slog.Info("Listing blobs in container", "container", dsc.storageConfig.ContainerName, "prefix", prefix)
	} else {
		slog.Info("Listing blobs in container", "container", dsc.storageConfig.ContainerName)
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

	slog.Info("Getting properties for blob", "container", dsc.storageConfig.ContainerName, "blob", dest, "url", blobURL)
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
	slog.Info("Ensuring container exists", "container", dsc.storageConfig.ContainerName)

	containerClient, err := azContainer.NewClientWithSharedKeyCredential(dsc.serviceURL, dsc.credential, nil)
	if err != nil {
		return fmt.Errorf("failed to create container client: %w", err)
	}

	_, err = containerClient.Create(context.Background(), nil)
	if err != nil {
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) && respErr.ErrorCode == string(bloberror.ContainerAlreadyExists) {
			slog.Info("Container already exists", "container", dsc.storageConfig.ContainerName)
			return nil
		}
		return fmt.Errorf("failed to create container: %w", err)
	}

	slog.Info("Container created successfully", "container", dsc.storageConfig.ContainerName)
	return nil
}
