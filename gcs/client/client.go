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
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"

	"cloud.google.com/go/storage"

	"github.com/cloudfoundry/storage-cli/gcs/config"
)

// ErrInvalidROWriteOperation is returned when credentials associated with the
// client disallow an attempted write operation.
var ErrInvalidROWriteOperation = errors.New("the client operates in read only mode. Change 'credentials_source' parameter value ")

// To enforce concurent go routine numbers during delete-recursive operation
const maxConcurrency = 10

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
	dstFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer dstFile.Close() //nolint:errcheck

	reader, err := client.getReader(client.publicGCS, src)

	// If the public client fails, try using it as an authenticated actor
	if err != nil && client.authenticatedGCS != nil {
		reader, err = client.getReader(client.authenticatedGCS, src)
	}

	if err != nil {
		return err
	}

	_, err = io.Copy(dstFile, reader)
	return err
}

func (client *GCSBlobstore) getReader(gcs *storage.Client, src string) (*storage.Reader, error) {
	return client.getObjectHandle(gcs, src).NewReader(context.Background())
}

// Put uploads a blob to the GCS blobstore.
// Destination will be overwritten if it already exists.
//
// Put retries retryAttempts times
const retryAttempts = 3

func (client *GCSBlobstore) Put(sourceFilePath string, dest string) error {
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
		err := client.putOnce(src, dest)
		if err == nil {
			return nil
		}

		errs = append(errs, err)
		log.Printf("upload failed for %s, attempt %d/%d: %v\n", dest, i+1, retryAttempts, err)

		if _, err := src.Seek(pos, io.SeekStart); err != nil {
			return fmt.Errorf("restting buffer position after failed upload: %v", err)
		}
	}

	return fmt.Errorf("upload failed for %s after %d attempts: %v", dest, retryAttempts, errs)
}

func (client *GCSBlobstore) putOnce(src io.ReadSeeker, dest string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Clean up the context after the function completes

	remoteWriter := client.getObjectHandle(client.authenticatedGCS, dest).NewWriter(ctx) //nolint:staticcheck
	remoteWriter.ObjectAttrs.StorageClass = client.config.StorageClass                   //nolint:staticcheck

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
		log.Printf("File '%s' exists in bucket '%s'\n", dest, client.config.BucketName)
		return true, nil
	} else if errors.Is(err, storage.ErrObjectNotExist) {
		log.Printf("File '%s' does not exist in bucket '%s'\n", dest, client.config.BucketName)
		return false, nil
	}
	return false, err
}

func (client *GCSBlobstore) readOnly() bool {
	return client.authenticatedGCS == nil
}

func (client *GCSBlobstore) Sign(id string, action string, expiry time.Duration) (string, error) {
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
		log.Printf("Listing objects in bucket %s with prefix '%s'\n", client.config.BucketName, prefix)
	} else {
		log.Printf("Listing objects in bucket %s\n", client.config.BucketName)
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
	log.Printf("copying an object from %s to %s\n", srcBlob, dstBlob)
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
	log.Printf("Getting properties for object %s/%s\n", client.config.BucketName, dest)
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
	log.Printf("Ensuring bucket '%s' exists\n", client.config.BucketName)
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
		log.Printf("Deleting all objects in bucket %s with prefix '%s'\n",
			client.config.BucketName, prefix)
	} else {
		log.Printf("Deleting all objects in bucket %s\n", client.config.BucketName)
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
