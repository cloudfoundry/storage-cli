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
	// For copy operations: use multipart copy only when necessary (>5GB)
	// AWS CopyObject limit is 5GB, use 100MB parts for multipart copy
	defaultMultipartCopyThreshold = int64(5 * 1024 * 1024 * 1024) // 5 GB
	defaultMultipartCopyPartSize  = int64(100 * 1024 * 1024)      // 100 MB
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
	cfg := b.s3cliConfig

	copyThreshold := defaultMultipartCopyThreshold
	if cfg.MultipartCopyThreshold > 0 {
		copyThreshold = cfg.MultipartCopyThreshold
	}
	copyPartSize := defaultMultipartCopyPartSize
	if cfg.MultipartCopyPartSize > 0 {
		copyPartSize = cfg.MultipartCopyPartSize
	}

	headOutput, err := b.s3Client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(cfg.BucketName),
		Key:    b.key(srcBlob),
	})
	if err != nil {
		return fmt.Errorf("failed to get object metadata: %w", err)
	}
	if headOutput.ContentLength == nil {
		return errors.New("unable to determine object content length from S3 metadata")
	}

	objectSize := *headOutput.ContentLength
	copySource := fmt.Sprintf("%s/%s", cfg.BucketName, *b.key(srcBlob))

	// Use simple copy if file is below threshold or is empty
	if objectSize < copyThreshold {
		slog.Info("Copying object", "source", srcBlob, "destination", dstBlob, "size", objectSize)
		return b.simpleCopy(copySource, dstBlob)
	}

	// For large files, try multipart copy first (works for AWS, MinIO, Ceph, AliCloud)
	// Fall back to simple copy if provider doesn't support UploadPartCopy (e.g., GCS)
	slog.Info("Copying large object using multipart copy", "source", srcBlob, "destination", dstBlob, "size", objectSize)

	err = b.multipartCopy(copySource, dstBlob, objectSize, copyPartSize)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NotImplemented" {
			slog.Info("Multipart copy not supported by provider, falling back to simple copy", "source", srcBlob, "destination", dstBlob)
			return b.simpleCopy(copySource, dstBlob)
		}
		return err
	}

	return nil
}

// simpleCopy performs a single CopyObject request
func (b *awsS3Client) simpleCopy(copySource string, dstBlob string) error {
	cfg := b.s3cliConfig

	copyInput := &s3.CopyObjectInput{
		Bucket:     aws.String(cfg.BucketName),
		CopySource: aws.String(copySource),
		Key:        b.key(dstBlob),
	}
	if cfg.ServerSideEncryption != "" {
		copyInput.ServerSideEncryption = types.ServerSideEncryption(cfg.ServerSideEncryption)
	}
	if cfg.SSEKMSKeyID != "" {
		copyInput.SSEKMSKeyId = aws.String(cfg.SSEKMSKeyID)
	}

	_, err := b.s3Client.CopyObject(context.TODO(), copyInput)
	if err != nil {
		return fmt.Errorf("failed to copy object: %w", err)
	}
	return nil
}

// multipartCopy performs a multipart copy using CreateMultipartUpload, UploadPartCopy, and CompleteMultipartUpload
func (b *awsS3Client) multipartCopy(copySource string, dstBlob string, objectSize int64, copyPartSize int64) error {
	cfg := b.s3cliConfig
	// Calculate number of parts using ceiling division (avoids floating-point arithmetic).
	// Example: objectSize=550MB, partSize=100MB => (550 + 100 - 1) / 100 = 6 parts
	numParts := int((objectSize + copyPartSize - 1) / copyPartSize)

	createInput := &s3.CreateMultipartUploadInput{
		Bucket: aws.String(cfg.BucketName),
		Key:    b.key(dstBlob),
	}
	if cfg.ServerSideEncryption != "" {
		createInput.ServerSideEncryption = types.ServerSideEncryption(cfg.ServerSideEncryption)
	}
	if cfg.SSEKMSKeyID != "" {
		createInput.SSEKMSKeyId = aws.String(cfg.SSEKMSKeyID)
	}

	createOutput, err := b.s3Client.CreateMultipartUpload(context.TODO(), createInput)
	if err != nil {
		return fmt.Errorf("failed to create multipart upload: %w", err)
	}

	uploadID := *createOutput.UploadId

	var completed bool
	defer func() {
		if !completed {
			_, err := b.s3Client.AbortMultipartUpload(context.TODO(), &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(cfg.BucketName),
				Key:      b.key(dstBlob),
				UploadId: aws.String(uploadID),
			})
			if err != nil {
				slog.Warn("Failed to abort multipart upload", "uploadId", uploadID, "error", err)
			}
		}
	}()

	completedParts := make([]types.CompletedPart, 0, numParts)
	for i := 0; i < numParts; i++ {
		partNumber := int32(i + 1)
		start := int64(i) * copyPartSize
		end := start + copyPartSize - 1
		if end >= objectSize {
			end = objectSize - 1
		}
		byteRange := fmt.Sprintf("bytes=%d-%d", start, end)

		output, err := b.s3Client.UploadPartCopy(context.TODO(), &s3.UploadPartCopyInput{
			Bucket:          aws.String(cfg.BucketName),
			CopySource:      aws.String(copySource),
			CopySourceRange: aws.String(byteRange),
			Key:             b.key(dstBlob),
			PartNumber:      aws.Int32(partNumber),
			UploadId:        aws.String(uploadID),
		})
		if err != nil {
			return fmt.Errorf("failed to copy part %d: %w", partNumber, err)
		}

		completedParts = append(completedParts, types.CompletedPart{
			ETag:       output.CopyPartResult.ETag,
			PartNumber: aws.Int32(partNumber),
		})
		slog.Debug("Copied part", "part", partNumber, "range", byteRange)
	}

	_, err = b.s3Client.CompleteMultipartUpload(context.TODO(), &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(cfg.BucketName),
		Key:      b.key(dstBlob),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	completed = true
	slog.Debug("Multipart copy completed successfully", "parts", numParts)
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
