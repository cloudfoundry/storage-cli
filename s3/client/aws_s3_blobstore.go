package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/cloudfoundry/storage-cli/s3/config"
)

var errorInvalidCredentialsSourceValue = errors.New("the client operates in read only mode. Change 'credentials_source' parameter value ")
var oneTB = int64(1000 * 1024 * 1024 * 1024)

// Default settings for transfer concurrency and part size.
// These values are chosen to align with typical AWS CLI and SDK defaults for efficient S3 uploads and downloads.
// See: https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/s3/manager#Downloader
const (
	defaultTransferConcurrency = 5
	defaultTransferPartSize    = int64(5 * 1024 * 1024) // 5 MB
)

// awsS3Client encapsulates AWS S3 blobstore interactions
type awsS3Client struct {
	s3Client    *s3.Client
	s3cliConfig *config.S3Cli
}

// Get fetches a blob, destination will be overwritten if exists
func (b *awsS3Client) Get(src string, dest io.WriterAt) error {
	cfg := b.s3cliConfig

	downloader := manager.NewDownloader(b.s3Client, func(d *manager.Downloader) { //nolint:staticcheck
		d.Concurrency = defaultTransferConcurrency
		if cfg.DownloadConcurrency > 0 {
			d.Concurrency = cfg.DownloadConcurrency
		}

		d.PartSize = defaultTransferPartSize
		if cfg.DownloadPartSize > 0 {
			d.PartSize = cfg.DownloadPartSize
		}
	})

	_, err := downloader.Download(context.TODO(), dest, &s3.GetObjectInput{ //nolint:staticcheck
		Bucket: aws.String(b.s3cliConfig.BucketName),
		Key:    b.key(src),
	})

	if err != nil {
		return err
	}

	return nil
}

// Put uploads a blob
func (b *awsS3Client) Put(src io.ReadSeeker, dest string) error {
	cfg := b.s3cliConfig
	if cfg.CredentialsSource == config.NoneCredentialsSource {
		return errorInvalidCredentialsSourceValue
	}

	uploader := manager.NewUploader(b.s3Client, func(u *manager.Uploader) { //nolint:staticcheck
		u.LeavePartsOnError = false

		u.Concurrency = defaultTransferConcurrency
		if cfg.UploadConcurrency > 0 {
			u.Concurrency = cfg.UploadConcurrency
		}

		// PartSize: if multipart uploads disabled, force a very large part to avoid multipart.
		// Otherwise, use configured upload part size if present, otherwise default.
		if !cfg.MultipartUpload {
			// disable multipart uploads by way of large PartSize configuration
			u.PartSize = oneTB
		} else {
			if cfg.UploadPartSize > 0 {
				u.PartSize = cfg.UploadPartSize
			} else {
				u.PartSize = defaultTransferPartSize
			}
		}

		if cfg.ShouldDisableUploaderRequestChecksumCalculation() {
			// Disable checksum calculation for Alicloud OSS (Object Storage Service)
			// Alicloud doesn't support AWS chunked encoding with checksum calculation
			u.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		}
	})
	uploadInput := &s3.PutObjectInput{
		Body:   src,
		Bucket: aws.String(cfg.BucketName),
		Key:    b.key(dest),
	}
	if cfg.ServerSideEncryption != "" {
		uploadInput.ServerSideEncryption = types.ServerSideEncryption(cfg.ServerSideEncryption)
	}
	if cfg.SSEKMSKeyID != "" {
		uploadInput.SSEKMSKeyId = aws.String(cfg.SSEKMSKeyID)
	}

	retry := 0
	maxRetries := 3
	for {
		putResult, err := uploader.Upload(context.TODO(), uploadInput) //nolint:staticcheck
		if err != nil {
			if _, ok := err.(manager.MultiUploadFailure); ok {
				if retry == maxRetries {
					return fmt.Errorf("upload retry limit exceeded: %s", err.Error())
				}
				retry++
				time.Sleep(time.Second * time.Duration(retry))
				continue
			}
			return fmt.Errorf("upload failure: %s", err.Error())
		}

		slog.Info("Successfully uploaded file", "location", putResult.Location)
		return nil
	}
}

// Delete removes a blob - no error is returned if the object does not exist
func (b *awsS3Client) Delete(dest string) error {
	if b.s3cliConfig.CredentialsSource == config.NoneCredentialsSource {
		return errorInvalidCredentialsSourceValue
	}

	deleteParams := &s3.DeleteObjectInput{
		Bucket: aws.String(b.s3cliConfig.BucketName),
		Key:    b.key(dest),
	}

	_, err := b.s3Client.DeleteObject(context.TODO(), deleteParams)

	if err == nil {
		return nil
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "NoSuchKey") {
		return nil
	}
	return err
}

// Exists checks if blob exists
func (b *awsS3Client) Exists(dest string) (bool, error) {
	existsParams := &s3.HeadObjectInput{
		Bucket: aws.String(b.s3cliConfig.BucketName),
		Key:    b.key(dest),
	}

	_, err := b.s3Client.HeadObject(context.TODO(), existsParams)

	if err == nil {
		slog.Info("Blob exists in bucket", "bucket", b.s3cliConfig.BucketName, "blob", dest)
		return true, nil
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NotFound" {
		slog.Info("Blob does not exist in bucket", "bucket", b.s3cliConfig.BucketName, "blob", dest)
		return false, nil
	}
	return false, err
}

// Sign creates a presigned URL
func (b *awsS3Client) Sign(objectID string, action string, expiration time.Duration) (string, error) {
	action = strings.ToUpper(action)
	switch action {
	case "GET":
		return b.getSigned(objectID, expiration)
	case "PUT":
		return b.putSigned(objectID, expiration)
	default:
		return "", fmt.Errorf("action not implemented: %s", action)
	}
}

func (b *awsS3Client) key(srcOrDest string) *string {
	formattedKey := aws.String(srcOrDest)
	if len(b.s3cliConfig.FolderName) != 0 {
		formattedKey = aws.String(fmt.Sprintf("%s/%s", b.s3cliConfig.FolderName, srcOrDest))
	}

	return formattedKey
}

func (b *awsS3Client) getSigned(objectID string, expiration time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(b.s3Client)
	signParams := &s3.GetObjectInput{
		Bucket: aws.String(b.s3cliConfig.BucketName),
		Key:    b.key(objectID),
	}

	req, err := presignClient.PresignGetObject(context.TODO(), signParams, s3.WithPresignExpires(expiration))
	if err != nil {
		return "", err
	}

	return req.URL, nil
}

func (b *awsS3Client) putSigned(objectID string, expiration time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(b.s3Client)
	signParams := &s3.PutObjectInput{
		Bucket: aws.String(b.s3cliConfig.BucketName),
		Key:    b.key(objectID),
	}

	req, err := presignClient.PresignPutObject(context.TODO(), signParams, s3.WithPresignExpires(expiration))
	if err != nil {
		return "", err
	}

	return req.URL, nil
}

func (b *awsS3Client) EnsureStorageExists() error {
	slog.Info("Ensuring bucket exists", "bucket", b.s3cliConfig.BucketName)
	_, err := b.s3Client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(b.s3cliConfig.BucketName),
	})

	if err == nil {
		slog.Info("Bucket exists", "bucket", b.s3cliConfig.BucketName)
		return nil
	}

	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "NotFound" {
		return fmt.Errorf("failed to check if bucket exists: %w", err)
	}

	slog.Info("Bucket does not exist, creating it", "bucket", b.s3cliConfig.BucketName)
	createBucketInput := &s3.CreateBucketInput{
		Bucket: aws.String(b.s3cliConfig.BucketName),
	}

	// For GCS and AWS region 'us-east-1', LocationConstraint must be omitted
	if !b.s3cliConfig.IsGoogle() && b.s3cliConfig.Region != "us-east-1" {
		createBucketInput.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(b.s3cliConfig.Region),
		}
	}

	_, err = b.s3Client.CreateBucket(context.TODO(), createBucketInput)
	if err != nil {
		var alreadyOwned *types.BucketAlreadyOwnedByYou
		var alreadyExists *types.BucketAlreadyExists
		if errors.As(err, &alreadyOwned) || errors.As(err, &alreadyExists) {
			slog.Warn("Bucket got created by another process", "bucket", b.s3cliConfig.BucketName)
			return nil
		}
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	slog.Info("Bucket created successfully", "bucket", b.s3cliConfig.BucketName)
	return nil
}

func (b *awsS3Client) Copy(srcBlob string, dstBlob string) error {
	slog.Info("Copying object within s3 bucket", "bucket", b.s3cliConfig.BucketName, "source_blob", srcBlob, "destination_blob", dstBlob)

	copySource := fmt.Sprintf("%s/%s", b.s3cliConfig.BucketName, *b.key(srcBlob))

	_, err := b.s3Client.CopyObject(context.TODO(), &s3.CopyObjectInput{
		Bucket:     aws.String(b.s3cliConfig.BucketName),
		CopySource: aws.String(copySource),
		Key:        b.key(dstBlob),
	})
	if err != nil {
		return fmt.Errorf("failed to copy object: %w", err)
	}

	waiter := s3.NewObjectExistsWaiter(b.s3Client)
	err = waiter.Wait(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(b.s3cliConfig.BucketName),
		Key:    b.key(dstBlob),
	}, 15*time.Minute)

	if err != nil {
		return fmt.Errorf("failed waiting for object to exist after copy: %w", err)
	}

	return nil
}

type BlobProperties struct {
	ETag          string    `json:"etag,omitempty"`
	LastModified  time.Time `json:"last_modified,omitempty"`
	ContentLength int64     `json:"content_length,omitempty"`
}

func (b *awsS3Client) Properties(dest string) error {
	slog.Info("Fetching blob properties", "bucket", b.s3cliConfig.BucketName, "blob", dest)

	headObjectOutput, err := b.s3Client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(b.s3cliConfig.BucketName),
		Key:    b.key(dest),
	})

	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NotFound" {
			fmt.Println(`{}`)
			return nil
		}
		return fmt.Errorf("failed to fetch blob properties: %w", err)
	}

	properties := BlobProperties{}
	if headObjectOutput.ETag != nil {
		properties.ETag = strings.Trim(*headObjectOutput.ETag, `"`)
	}
	if headObjectOutput.LastModified != nil {
		properties.LastModified = *headObjectOutput.LastModified
	}
	if headObjectOutput.ContentLength != nil {
		properties.ContentLength = *headObjectOutput.ContentLength
	}

	output, err := json.MarshalIndent(properties, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal blob properties: %w", err)
	}

	fmt.Println(string(output))

	return nil
}

func (b *awsS3Client) List(prefix string) ([]string, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(b.s3cliConfig.BucketName),
	}

	if prefix != "" {
		slog.Info("Listing all objects in bucket with prefix", "bucket", b.s3cliConfig.BucketName, "prefix", prefix)
		input.Prefix = b.key(prefix)
	} else {
		slog.Info("Listing all objects in bucket", "bucket", b.s3cliConfig.BucketName)
	}

	var names []string
	objectPaginator := s3.NewListObjectsV2Paginator(b.s3Client, input)
	for objectPaginator.HasMorePages() {
		page, err := objectPaginator.NextPage(context.TODO())
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		if len(page.Contents) == 0 {
			continue
		}

		for _, obj := range page.Contents {
			names = append(names, *obj.Key)
		}
	}

	return names, nil
}

func (b *awsS3Client) DeleteRecursive(prefix string) error {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(b.s3cliConfig.BucketName),
	}

	if prefix != "" {
		slog.Info("Deleting all objects in bucket with given prefix", "bucket", b.s3cliConfig.BucketName, "prefix", prefix)
		input.Prefix = b.key(prefix)
	} else {
		slog.Info("Deleting all objects in bucket", "bucket", b.s3cliConfig.BucketName)
	}

	objectPaginator := s3.NewListObjectsV2Paginator(b.s3Client, input)
	for objectPaginator.HasMorePages() {
		page, err := objectPaginator.NextPage(context.TODO())
		if err != nil {
			return fmt.Errorf("failed to list objects for deletion: %w", err)
		}

		for _, obj := range page.Contents {
			slog.Debug("Deleting object", "key", *obj.Key)
			_, err := b.s3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
				Bucket: aws.String(b.s3cliConfig.BucketName),
				Key:    obj.Key,
			})
			if err != nil {
				var apiErr smithy.APIError
				if errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "NoSuchKey") {
					continue // Object already deleted, which is fine
				}
				return fmt.Errorf("failed to delete object '%s': %w", *obj.Key, err)
			}
		}
	}
	return nil
}
