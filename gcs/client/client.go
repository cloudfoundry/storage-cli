/*
 * Copyright 2017 Google Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"

	"cloud.google.com/go/storage"
	"cloud.google.com/go/storage/transfermanager"

	"github.com/cloudfoundry/storage-cli/gcs/config"
)

// ErrInvalidROWriteOperation is returned when credentials associated with the
// client disallow an attempted write operation.
var ErrInvalidROWriteOperation = errors.New("the client operates in read only mode. Change 'credentials_source' parameter value ")

// 4 MB of block size.
// Used in concurrent download
const blockSize = int64(4 * 1024 * 1024)

// 100MB of resumable block size
// No concurrency, sequential upload still
// see, https://github.com/googleapis/google-api-ruby-client/blob/main/google-apis-core/lib/google/apis/options.rb#L142
const uploadChunkSize = int(100 * 1024 * 1024)

// number of go routines
const maxConcurrency = 5

// Put retries retryAttempts times
const retryAttempts = 3

type BlobProperties struct {
	ETag          string    `json:"etag,omitempty"`
	LastModified  time.Time `json:"last_modified,omitempty"`
	ContentLength int64     `json:"content_length,omitempty"`
}

// GCSBlobstore encapsulates interaction with the GCS blobstore
type GCSBlobstore struct {
	authenticatedGCS *storage.Client
	publicGCS        *storage.Client
	config           *config.GCSCli
}

// validateRemoteConfig determines if the configuration of the client matches
// against the remote configuration
//
// If operating in read-only mode, no mutations can be performed
// so the remote bucket location is always compatible.
func (client *GCSBlobstore) validateRemoteConfig() error {
	if client.readOnly() {
		return nil
	}

	bucket := client.authenticatedGCS.Bucket(client.config.BucketName)
	_, err := bucket.Attrs(context.Background())
	return err
}

// getObjectHandle returns a handle to an object named src
func (client *GCSBlobstore) getObjectHandle(gcs *storage.Client, src string) *storage.ObjectHandle {
	handle := gcs.Bucket(client.config.BucketName).Object(src)
	if client.config.EncryptionKey != nil {
		handle = handle.Key(client.config.EncryptionKey)
	}
	return handle
}

func (client *GCSBlobstore) getBucketHandle(gcs *storage.Client) *storage.BucketHandle {
	handle := gcs.Bucket(client.config.BucketName)
	return handle
}

// New returns a GCSBlobstore configured to operate using the given config
//
// non-nil error is returned on invalid Client or config. If the configuration
// is incompatible with the GCS bucket, a non-nil error is also returned.
func New(ctx context.Context, cfg *config.GCSCli) (*GCSBlobstore, error) {
	if cfg == nil {
		return nil, errors.New("expected non-nill config object")
	}

	authenticatedGCS, publicGCS, err := newStorageClients(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating storage client: %v", err)
	}

	return &GCSBlobstore{authenticatedGCS: authenticatedGCS, publicGCS: publicGCS, config: cfg}, nil
}

// Get fetches a blob from the GCS blobstore.
// Destination will be overwritten if it already exists.
func (client *GCSBlobstore) Get(src string, dest string) error {
	slog.Info("Getting object into file", "bucket", client.config.BucketName, "object_name", src, "local_path", dest)

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close() //nolint:errcheck

	gcsClient := client.publicGCS
	err = client.checkAccess(client.publicGCS, src)
	if err != nil && client.authenticatedGCS != nil {
		err = client.checkAccess(client.authenticatedGCS, src)
		if err == nil {
			gcsClient = client.authenticatedGCS
		}
	}

	if err != nil {
		return err
	}

	// If object is encrypted, we can't use transfermanager
	// Fall back to single-part download with encryption support
	if client.config.EncryptionKey != nil {
		return client.downloadEncrypted(gcsClient, src, destFile)
	}

	return client.downloadConcurrent(gcsClient, src, destFile)

}

// If the client can read object attributes,
// then it can download the object.
func (client *GCSBlobstore) checkAccess(gcsClient *storage.Client, src string) error {
	_, err := client.getObjectHandle(gcsClient, src).Attrs(context.Background())
	return err
}

func (client *GCSBlobstore) downloadConcurrent(gcsClient *storage.Client, src string, destFile *os.File) error {
	downloader, err := transfermanager.NewDownloader(gcsClient,
		transfermanager.WithPartSize(blockSize),
		transfermanager.WithWorkers(maxConcurrency))
	if err != nil {
		return fmt.Errorf("creating new downloader: %w", err)
	}

	in := &transfermanager.DownloadObjectInput{Bucket: client.config.BucketName, Object: src, Destination: destFile}

	if err := downloader.DownloadObject(context.Background(), in); err != nil {
		return fmt.Errorf("adding work into queue: %w", err)
	}

	results, err := downloader.WaitAndClose()
	if err != nil {
		return fmt.Errorf("finishing download and closing channels: %w", err)
	}

	if len(results) != 1 {
		return fmt.Errorf("expected 1 download result, got %d", len(results))
	}

	result := results[0]
	if result.Err != nil {
		return fmt.Errorf("download of object %v failed with error %w", result.Object, result.Err)
	}

	return nil
}

func (client *GCSBlobstore) downloadEncrypted(gcsClient *storage.Client, src string, destFile *os.File) error {
	reader, err := client.getObjectHandle(gcsClient, src).NewReader(context.Background())
	if err != nil {
		return err
	}
	defer reader.Close() //nolint:errcheck

	_, err = io.Copy(destFile, reader)
	return err
}

// Put uploads a blob to the GCS blobstore.
// Destination will be overwritten if it already exists.
func (client *GCSBlobstore) Put(sourceFilePath string, dest string) error {
	slog.Info("Putting file into object", "bucket", client.config.BucketName, "local_path", sourceFilePath, "object_name", dest)

	src, err := os.Open(sourceFilePath)
	if err != nil {
		return err
	}
	defer src.Close() //nolint:errcheck

	if client.readOnly() {
		return ErrInvalidROWriteOperation
	}

	if err := client.validateRemoteConfig(); err != nil {
		return err
	}

	pos, err := src.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("finding buffer position: %v", err)
	}

	var errs []error
	for i := range retryAttempts {
		err := client.putResumable(src, dest)
		if err == nil {
			return nil
		}

		errs = append(errs, err)
		slog.Error("Upload failed", "object_name", dest, "attempt", fmt.Sprintf("%d/%d", i+1, retryAttempts), "error", err)

		if _, err := src.Seek(pos, io.SeekStart); err != nil {
			return fmt.Errorf("resetting buffer position after failed upload: %v", err)
		}
	}

	return fmt.Errorf("upload failed for %s after %d attempts: %v", dest, retryAttempts, errs)
}

// putResumable performs a resumable upload in chunks of uploadChunkSize (100MB).
// Chunks are uploaded sequentially with automatic per-chunk retry on failure.
func (client *GCSBlobstore) putResumable(src io.ReadSeeker, dest string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Clean up the context after the function completes

	remoteWriter := client.getObjectHandle(client.authenticatedGCS, dest).NewWriter(ctx) //nolint:staticcheck
	remoteWriter.ObjectAttrs.StorageClass = client.config.StorageClass                   //nolint:staticcheck
	remoteWriter.ChunkSize = uploadChunkSize

	if _, err := io.Copy(remoteWriter, src); err != nil {
		remoteWriter.Close() //nolint:errcheck
		return err
	}

	return remoteWriter.Close()
}

// Delete removes a blob from from the GCS blobstore.
//
// If the object does not exist, Delete returns a nil error.
func (client *GCSBlobstore) Delete(dest string) error {
	slog.Info("Deleting object in bucket", "bucket", client.config.BucketName, "object_name", dest)

	if client.readOnly() {
		return ErrInvalidROWriteOperation
	}

	err := client.getObjectHandle(client.authenticatedGCS, dest).Delete(context.Background())
	if errors.Is(err, storage.ErrObjectNotExist) {
		return nil
	}
	return err
}

// Exists checks if a blob exists in the GCS blobstore.
func (client *GCSBlobstore) Exists(dest string) (exists bool, err error) {
	slog.Info("Checking object exists in bucket", "bucket", client.config.BucketName, "object_name", dest)

	if exists, err = client.exists(client.publicGCS, dest); err == nil {
		return exists, nil
	}

	// If the public client fails, try using it as an authenticated actor
	if client.authenticatedGCS != nil {
		return client.exists(client.authenticatedGCS, dest)
	}

	return
}

func (client *GCSBlobstore) exists(gcs *storage.Client, dest string) (bool, error) {
	_, err := client.getObjectHandle(gcs, dest).Attrs(context.Background())
	if err == nil {
		slog.Info("Object exists in bucket", "bucket", client.config.BucketName, "object_name", dest)
		return true, nil
	} else if errors.Is(err, storage.ErrObjectNotExist) {
		slog.Info("Object does not exist in bucket", "bucket", client.config.BucketName, "object_name", dest)
		return false, nil
	}
	return false, err
}

func (client *GCSBlobstore) readOnly() bool {
	return client.authenticatedGCS == nil
}

func (client *GCSBlobstore) Sign(id string, action string, expiry time.Duration) (string, error) {
	slog.Info("Signing object", "bucket", client.config.BucketName, "object_name", id, "method", action, "expiration", expiry.String())

	action = strings.ToUpper(action)
	token, err := google.JWTConfigFromJSON([]byte(client.config.ServiceAccountFile), storage.ScopeFullControl)
	if err != nil {
		return "", err
	}
	options := storage.SignedURLOptions{
		Method:         action,
		Expires:        time.Now().Add(expiry),
		PrivateKey:     token.PrivateKey,
		GoogleAccessID: token.Email,
		Scheme:         storage.SigningSchemeV4,
	}

	// GET/PUT to the resultant signed url must include, in addition to the below:
	// 'x-goog-encryption-key' and 'x-goog-encryption-key-sha256'
	willEncrypt := len(client.config.EncryptionKey) > 0
	if willEncrypt {
		options.Headers = []string{
			"x-goog-encryption-algorithm: AES256",
			fmt.Sprintf("x-goog-encryption-key: %s", client.config.EncryptionKeyEncoded),
			fmt.Sprintf("x-goog-encryption-key-sha256: %s", client.config.EncryptionKeySha256),
		}
	}
	return storage.SignedURL(client.config.BucketName, id, &options)
}

func (client *GCSBlobstore) List(prefix string) ([]string, error) {
	if prefix != "" {
		slog.Info("Listing all objects in bucket", "bucket", client.config.BucketName, "prefix", prefix)
	} else {
		slog.Info("Listing all objects in bucket", "bucket", client.config.BucketName)
	}
	if client.readOnly() {
		return nil, ErrInvalidROWriteOperation
	}

	bh := client.getBucketHandle(client.authenticatedGCS)

	it := bh.Objects(context.Background(), &storage.Query{Prefix: prefix})

	var names []string
	for {
		attr, err := it.Next()
		if err == iterator.Done {
			break
		}

		if err != nil {
			return nil, err
		}

		names = append(names, attr.Name)
	}

	return names, nil

}

func (client *GCSBlobstore) Copy(srcBlob string, dstBlob string) error {
	slog.Info("Copying object", "bucket", client.config.BucketName, "source_object", srcBlob, "destination_object", dstBlob)

	if client.readOnly() {
		return ErrInvalidROWriteOperation
	}

	srcHandle := client.getObjectHandle(client.authenticatedGCS, srcBlob)
	dstHandle := client.getObjectHandle(client.authenticatedGCS, dstBlob)

	_, err := dstHandle.CopierFrom(srcHandle).Run(context.Background())
	if err != nil {
		return fmt.Errorf("copying object: %w", err)
	}
	return nil
}

func (client *GCSBlobstore) Properties(dest string) error {
	slog.Info("Getting properties for object", "bucket", client.config.BucketName, "object_name", dest)

	if client.readOnly() {
		return ErrInvalidROWriteOperation
	}
	oh := client.getObjectHandle(client.authenticatedGCS, dest)
	attr, err := oh.Attrs(context.Background())

	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			fmt.Println(`{}`)
			return nil
		}
		return fmt.Errorf("getting attributes: %w", err)
	}

	props := BlobProperties{
		ETag:          strings.Trim(attr.Etag, `"`),
		LastModified:  attr.Updated,
		ContentLength: attr.Size,
	}

	output, err := json.MarshalIndent(props, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal blob properties: %w", err)
	}

	fmt.Println(string(output))
	return nil
}

func (client *GCSBlobstore) EnsureStorageExists() error {
	slog.Info("Ensuring bucket exists", "bucket", client.config.BucketName)

	if client.readOnly() {
		return ErrInvalidROWriteOperation
	}
	ctx := context.Background()
	bh := client.getBucketHandle(client.authenticatedGCS)

	_, err := bh.Attrs(ctx)
	if errors.Is(err, storage.ErrBucketNotExist) {
		battr := &storage.BucketAttrs{Name: client.config.BucketName}
		if client.config.StorageClass != "" {
			battr.StorageClass = client.config.StorageClass
		}

		if client.config.UniformBucketLevelAccess {
			battr.UniformBucketLevelAccess = storage.UniformBucketLevelAccess{Enabled: true}
		}

		projectID, err := extractProjectID(ctx, client.config)
		if err != nil {
			return fmt.Errorf("extracting project ID: %w", err)
		}

		err = bh.Create(ctx, projectID, battr)
		if err != nil {
			return fmt.Errorf("creating bucket: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("checking bucket: %w", err)
	}

	return nil
}

func (client *GCSBlobstore) DeleteRecursive(prefix string) error {
	if prefix != "" {
		slog.Info("Deleting all the objects in bucket", "bucket", client.config.BucketName, "prefix", prefix)
	} else {
		slog.Info("Deleting all the objects in bucket", "bucket", client.config.BucketName)
	}

	if client.readOnly() {
		return ErrInvalidROWriteOperation
	}

	names, err := client.List(prefix)
	if err != nil {
		return fmt.Errorf("listing objects: %w", err)
	}

	errChan := make(chan error, len(names))
	semaphore := make(chan struct{}, maxConcurrency)
	wg := &sync.WaitGroup{}
	for _, n := range names {
		name := n
		wg.Add(1)
		go func() {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			err := client.getObjectHandle(client.authenticatedGCS, name).Delete(context.Background())
			if err != nil && !errors.Is(err, storage.ErrObjectNotExist) {
				errChan <- fmt.Errorf("deleting object %s: %w", name, err)
			}
		}()
	}

	wg.Wait()
	close(errChan)

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}
