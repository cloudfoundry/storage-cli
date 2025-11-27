package client

import (
	"errors"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/cloudfoundry/storage-cli/s3/config"
)

type S3CompatibleClient struct {
	s3cliConfig             *config.S3Cli
	awsS3BlobstoreClient    *awsS3Client
	openstackSwiftBlobstore *openstackSwiftS3Client
}

// New returns an S3CompatibleClient
func New(s3Client *s3.Client, s3cliConfig *config.S3Cli) *S3CompatibleClient {
	return &S3CompatibleClient{
		s3cliConfig: s3cliConfig,
		openstackSwiftBlobstore: &openstackSwiftS3Client{
			s3cliConfig: s3cliConfig,
		},
		awsS3BlobstoreClient: &awsS3Client{
			s3Client:    s3Client,
			s3cliConfig: s3cliConfig,
		},
	}
}

func (c *S3CompatibleClient) Get(src string, dest string) error {
	dstFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer dstFile.Close() //nolint:errcheck
	return c.awsS3BlobstoreClient.Get(src, dstFile)
}

func (c *S3CompatibleClient) Put(src string, dest string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		log.Fatalln(err)
	}
	defer sourceFile.Close() //nolint:errcheck
	return c.awsS3BlobstoreClient.Put(sourceFile, dest)
}

func (c *S3CompatibleClient) Delete(dest string) error {
	return c.awsS3BlobstoreClient.Delete(dest)
}

func (c *S3CompatibleClient) Exists(dest string) (bool, error) {
	return c.awsS3BlobstoreClient.Exists(dest)
}

func (c *S3CompatibleClient) Sign(objectID string, action string, expiration time.Duration) (string, error) {
	if c.s3cliConfig.SwiftAuthAccount != "" {
		return c.openstackSwiftBlobstore.Sign(objectID, action, expiration)
	}

	return c.awsS3BlobstoreClient.Sign(objectID, action, expiration)
}

func (c *S3CompatibleClient) EnsureStorageExists() error {
	return errors.New("Not implemented")

}

func (c *S3CompatibleClient) Copy(srcBlob string, dstBlob string) error {
	return errors.New("Not implemented")

}

func (c *S3CompatibleClient) Properties(dest string) error {
	return errors.New("Not implemented")

}

func (c *S3CompatibleClient) List(prefix string) ([]string, error) {
	return nil, errors.New("Not implemented")

}

func (c *S3CompatibleClient) DeleteRecursive(prefix string) error {
	return errors.New("Not implemented")
}
