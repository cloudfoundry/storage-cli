package integration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/cloudfoundry/storage-cli/s3/client"
	"github.com/cloudfoundry/storage-cli/s3/config"
	. "github.com/onsi/gomega" //nolint:staticcheck
)

var (
	// expectedPutUploadCalls represents the expected API calls for put requests
	expectedPutUploadCalls = []string{"PutObject"}
)

// isMultipartUploadPattern checks if calls follow the multipart upload pattern:
// starts with CreateMultipart, has one or more UploadPart calls, ends with CompleteMultipart
func isMultipartUploadPattern(calls []string) bool {
	if len(calls) < 3 {
		return false
	}
	if calls[0] != "CreateMultipart" {
		return false
	}
	if calls[len(calls)-1] != "CompleteMultipart" {
		return false
	}
	// Check all middle elements are UploadPart
	for _, call := range calls[1 : len(calls)-1] {
		if call != "UploadPart" {
			return false
		}
	}
	return true
}

// AssertLifecycleWorks tests the main blobstore object lifecycle from creation to deletion
func AssertLifecycleWorks(s3CLIPath string, cfg *config.S3Cli) {
	storageType := "s3"
	expectedString := GenerateRandomString()
	s3Filename := GenerateRandomString()

	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	contentFile := MakeContentFile(expectedString)
	defer os.Remove(contentFile) //nolint:errcheck

	s3CLISession, err := RunS3CLI(s3CLIPath, configPath, storageType, "put", contentFile, s3Filename)
	Expect(err).ToNot(HaveOccurred())
	Expect(s3CLISession.ExitCode()).To(BeZero())

	if len(cfg.FolderName) != 0 {
		folderName := cfg.FolderName
		cfg.FolderName = ""
		noFolderConfigPath := MakeConfigFile(cfg)
		defer os.Remove(noFolderConfigPath) //nolint:errcheck

		s3CLISession, err :=
			RunS3CLI(s3CLIPath, noFolderConfigPath, storageType, "exists", fmt.Sprintf("%s/%s", folderName, s3Filename))
		Expect(err).ToNot(HaveOccurred())
		Expect(s3CLISession.ExitCode()).To(BeZero())
	}

	s3CLISession, err = RunS3CLI(s3CLIPath, configPath, storageType, "exists", s3Filename)
	Expect(err).ToNot(HaveOccurred())
	Expect(s3CLISession.ExitCode()).To(BeZero())

	tmpLocalFile, err := os.CreateTemp("", "s3cli-download")
	Expect(err).ToNot(HaveOccurred())
	err = tmpLocalFile.Close()
	Expect(err).ToNot(HaveOccurred())
	defer os.Remove(tmpLocalFile.Name()) //nolint:errcheck

	s3CLISession, err = RunS3CLI(s3CLIPath, configPath, storageType, "get", s3Filename, tmpLocalFile.Name())
	Expect(err).ToNot(HaveOccurred())
	Expect(s3CLISession.ExitCode()).To(BeZero())

	gottenBytes, err := os.ReadFile(tmpLocalFile.Name())
	Expect(err).ToNot(HaveOccurred())
	Expect(string(gottenBytes)).To(Equal(expectedString))

	s3CLISession, err = RunS3CLI(s3CLIPath, configPath, storageType, "properties", s3Filename)
	Expect(err).ToNot(HaveOccurred())
	Expect(s3CLISession.ExitCode()).To(BeZero())
	Expect(s3CLISession.Out.Contents()).To(ContainSubstring(fmt.Sprintf("\"content_length\": %d", len(expectedString))))
	Expect(s3CLISession.Out.Contents()).To(ContainSubstring("\"etag\":"))
	Expect(s3CLISession.Out.Contents()).To(ContainSubstring("\"last_modified\":"))

	s3CLISession, err = RunS3CLI(s3CLIPath, configPath, storageType, "copy", s3Filename, s3Filename+"_copy")
	Expect(err).ToNot(HaveOccurred())
	Expect(s3CLISession.ExitCode()).To(BeZero())

	s3CLISession, err = RunS3CLI(s3CLIPath, configPath, storageType, "exists", s3Filename+"_copy")
	Expect(err).ToNot(HaveOccurred())
	Expect(s3CLISession.ExitCode()).To(BeZero())

	tmpCopiedFile, err := os.CreateTemp("", "s3cli-download-copy")
	Expect(err).ToNot(HaveOccurred())
	err = tmpCopiedFile.Close()
	Expect(err).ToNot(HaveOccurred())
	defer os.Remove(tmpCopiedFile.Name()) //nolint:errcheck

	s3CLISession, err = RunS3CLI(s3CLIPath, configPath, storageType, "get", s3Filename+"_copy", tmpCopiedFile.Name())
	Expect(err).ToNot(HaveOccurred())
	Expect(s3CLISession.ExitCode()).To(BeZero())

	copiedBytes, err := os.ReadFile(tmpCopiedFile.Name())
	Expect(err).ToNot(HaveOccurred())
	Expect(string(copiedBytes)).To(Equal(expectedString))

	s3CLISession, err = RunS3CLI(s3CLIPath, configPath, storageType, "delete", s3Filename+"_copy")
	Expect(err).ToNot(HaveOccurred())
	Expect(s3CLISession.ExitCode()).To(BeZero())

	s3CLISession, err = RunS3CLI(s3CLIPath, configPath, storageType, "delete", s3Filename)
	Expect(err).ToNot(HaveOccurred())
	Expect(s3CLISession.ExitCode()).To(BeZero())

	s3CLISession, err = RunS3CLI(s3CLIPath, configPath, storageType, "exists", s3Filename)
	Expect(err).ToNot(HaveOccurred())
	Expect(s3CLISession.ExitCode()).To(Equal(3))

	s3CLISession, err = RunS3CLI(s3CLIPath, configPath, storageType, "properties", s3Filename)
	Expect(err).ToNot(HaveOccurred())
	Expect(s3CLISession.ExitCode()).To(BeZero())
	Expect(s3CLISession.Out.Contents()).To(ContainSubstring("{}"))

}

func AssertOnBulkOperations(s3CLIPath string, cfg *config.S3Cli) {
	storageType := "s3"
	numFiles := 5
	s3FilenamePrefix := GenerateRandomString()
	localFile := MakeContentFile(GenerateRandomString())
	defer os.Remove(localFile) //nolint:errcheck

	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	for i := 0; i < numFiles; i++ {
		suffix := strings.Repeat("A", i)
		s3Filename := fmt.Sprintf("%s%s", s3FilenamePrefix, suffix)

		s3CLISession, err := RunS3CLI(s3CLIPath, configPath, storageType, "put", localFile, s3Filename)
		Expect(err).ToNot(HaveOccurred())
		Expect(s3CLISession.ExitCode()).To(BeZero())
	}

	s3CLISession, err := RunS3CLI(s3CLIPath, configPath, storageType, "list", s3FilenamePrefix)
	Expect(err).ToNot(HaveOccurred())
	Expect(s3CLISession.ExitCode()).To(BeZero())
	output := strings.TrimSpace(string(s3CLISession.Out.Contents()))
	Expect(strings.Split(output, "\n")).To(HaveLen(numFiles))

	s3CLISession, err = RunS3CLI(s3CLIPath, configPath, storageType, "delete-recursive", fmt.Sprintf("%sAAA", s3FilenamePrefix))
	Expect(err).ToNot(HaveOccurred())
	Expect(s3CLISession.ExitCode()).To(BeZero())

	s3CLISession, err = RunS3CLI(s3CLIPath, configPath, storageType, "list", s3FilenamePrefix)
	Expect(err).ToNot(HaveOccurred())
	Expect(s3CLISession.ExitCode()).To(BeZero())
	output = strings.TrimSpace(string(s3CLISession.Out.Contents()))
	Expect(strings.Split(output, "\n")).To(HaveLen(3))

	s3CLISession, err = RunS3CLI(s3CLIPath, configPath, storageType, "delete-recursive")
	Expect(err).ToNot(HaveOccurred())
	Expect(s3CLISession.ExitCode()).To(BeZero())

	s3CLISession, err = RunS3CLI(s3CLIPath, configPath, storageType, "list", s3FilenamePrefix)
	Expect(err).ToNot(HaveOccurred())
	Expect(s3CLISession.ExitCode()).To(BeZero())
	output = strings.TrimSpace(string(s3CLISession.Out.Contents()))
	Expect(output).To(BeEmpty())
}

func AssertOnStorageExists(s3CLIPath string, cfg *config.S3Cli) {
	cfgCopy := *cfg
	cfgCopy.BucketName = fmt.Sprintf("%s-%s", cfg.BucketName, strings.ToLower(GenerateRandomString(4)))

	configPath := MakeConfigFile(&cfgCopy)
	defer os.Remove(configPath) //nolint:errcheck

	// Create a single verification/cleanup client from the config file.
	// This ensures it has the exact same settings as the CLI will use.
	verificationCfgFile, err := os.Open(configPath)
	Expect(err).ToNot(HaveOccurred())
	defer verificationCfgFile.Close() //nolint:errcheck

	verificationCfg, err := config.NewFromReader(verificationCfgFile)
	Expect(err).ToNot(HaveOccurred())

	verificationClient, err := client.NewAwsS3Client(&verificationCfg)
	Expect(err).ToNot(HaveOccurred())

	// Defer the cleanup to ensure the bucket is deleted after the test.
	defer func() {
		_, err := verificationClient.DeleteBucket(context.TODO(), &s3.DeleteBucketInput{
			Bucket: aws.String(cfgCopy.BucketName),
		})
		// A NoSuchBucket error is acceptable if the bucket was never created or already cleaned up.
		var noSuchBucketErr *types.NoSuchBucket
		if err != nil && !errors.As(err, &noSuchBucketErr) {
			Expect(err).ToNot(HaveOccurred(), "Failed to clean up test bucket")
		}
	}()

	// --- Scenario 1: Bucket does not exist, should be created ---
	s3CLISession, err := RunS3CLI(s3CLIPath, configPath, "s3", "ensure-storage-exists")
	Expect(err).ToNot(HaveOccurred())
	Expect(s3CLISession.ExitCode()).To(BeZero())

	// Verify the bucket now exists using the client created earlier.
	_, headBucketErr := verificationClient.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(cfgCopy.BucketName),
	})
	Expect(headBucketErr).ToNot(HaveOccurred(), "Bucket should have been created by 'ensure-storage-exists'")

	// --- Scenario 2: Bucket already exists, command should still succeed (idempotency) ---
	s3CLISession, err = RunS3CLI(s3CLIPath, configPath, "s3", "ensure-storage-exists")
	Expect(err).ToNot(HaveOccurred())
	Expect(s3CLISession.ExitCode()).To(BeZero())
}

func AssertOnPutFailures(cfg *config.S3Cli, content, errorMessage string) {
	s3Filename := GenerateRandomString()
	sourceFile := MakeContentFile(content)

	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	configFile, err := os.Open(configPath)
	Expect(err).ToNot(HaveOccurred())

	s3Config, err := config.NewFromReader(configFile)
	Expect(err).ToNot(HaveOccurred())

	s3Client, err := CreateS3ClientWithFailureInjection(&s3Config)
	if err != nil {
		log.Fatalln(err)
	}
	blobstoreClient := client.New(s3Client, &s3Config)

	err = blobstoreClient.Put(sourceFile, s3Filename)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring(errorMessage))
}

// AssertPutOptionsApplied asserts that `s3cli put` uploads files with the requested encryption options
func AssertPutOptionsApplied(s3CLIPath string, cfg *config.S3Cli) {
	storageType := "s3"
	expectedString := GenerateRandomString()
	s3Filename := GenerateRandomString()

	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	contentFile := MakeContentFile(expectedString)
	defer os.Remove(contentFile) //nolint:errcheck

	configFile, err := os.Open(configPath)
	Expect(err).ToNot(HaveOccurred())

	s3CLISession, err := RunS3CLI(s3CLIPath, configPath, storageType, "put", contentFile, s3Filename) //nolint:ineffassign,staticcheck
	Expect(err).ToNot(HaveOccurred())
	Expect(s3CLISession.ExitCode()).To(BeZero())

	s3Config, err := config.NewFromReader(configFile)
	Expect(err).ToNot(HaveOccurred())

	s3Client, err := client.NewAwsS3Client(&s3Config)
	Expect(err).ToNot(HaveOccurred())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(cfg.BucketName),
		Key:    aws.String(s3Filename),
	})
	Expect(err).ToNot(HaveOccurred())

	if cfg.ServerSideEncryption == "" {
		Expect(resp.ServerSideEncryption).To(Or(BeNil(), HaveValue(Equal(types.ServerSideEncryptionAes256))))
	} else {
		Expect(string(resp.ServerSideEncryption)).To(Equal(cfg.ServerSideEncryption))
	}

	// Clean up the uploaded file
	_, err = RunS3CLI(s3CLIPath, configPath, storageType, "delete", s3Filename)
	Expect(err).ToNot(HaveOccurred())
}

// AssertGetNonexistentFails asserts that `s3cli get` on a non-existent object will fail
func AssertGetNonexistentFails(s3CLIPath string, cfg *config.S3Cli) {
	storageType := "s3"
	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	s3CLISession, err := RunS3CLI(s3CLIPath, configPath, storageType, "get", "non-existent-file", "/dev/null")
	Expect(err).ToNot(HaveOccurred())
	Expect(s3CLISession.ExitCode()).ToNot(BeZero())
	Expect(s3CLISession.Err.Contents()).To(ContainSubstring("NoSuchKey"))
}

// AssertDeleteNonexistentWorks asserts that `s3cli delete` on a non-existent
// object exits with status 0 (tests idempotency)
func AssertDeleteNonexistentWorks(s3CLIPath string, cfg *config.S3Cli) {
	storageType := "s3"
	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	s3CLISession, err := RunS3CLI(s3CLIPath, configPath, storageType, "delete", "non-existent-file")
	Expect(err).ToNot(HaveOccurred())
	Expect(s3CLISession.ExitCode()).To(BeZero())
}

func AssertOnMultipartUploads(s3CLIPath string, cfg *config.S3Cli, content string) {
	s3Filename := GenerateRandomString()
	sourceFile := MakeContentFile(content)

	storageType := "s3"
	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	configFile, err := os.Open(configPath)
	Expect(err).ToNot(HaveOccurred())

	s3Config, err := config.NewFromReader(configFile)
	Expect(err).ToNot(HaveOccurred())

	// Create S3 client with tracing middleware
	calls := []string{}
	s3Client, err := CreateTracingS3Client(&s3Config, &calls)
	if err != nil {
		log.Fatalln(err)
	}

	blobstoreClient := client.New(s3Client, &s3Config)

	err = blobstoreClient.Put(sourceFile, s3Filename)
	Expect(err).ToNot(HaveOccurred())

	switch config.Provider(cfg.Host) {
	// Google doesn't support multipart uploads as we use a normal put request instead when targeted host is Google.
	case "google":
		Expect(calls).To(Equal(expectedPutUploadCalls))
	default:
		Expect(isMultipartUploadPattern(calls)).To(BeTrue(), "Expected multipart upload pattern (CreateMultipart -> UploadPart(s) -> CompleteMultipart), got: %v", calls)
	}

	// Clean up the uploaded file
	_, err = RunS3CLI(s3CLIPath, configPath, storageType, "delete", s3Filename)
	Expect(err).ToNot(HaveOccurred())
}

// AssertOnSignedURLs asserts on using signed URLs for upload and download
func AssertOnSignedURLs(s3CLIPath string, cfg *config.S3Cli) {
	s3Filename := GenerateRandomString()
	expectedContent := GenerateRandomString()

	storageType := "s3"
	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	configFile, err := os.Open(configPath)
	Expect(err).ToNot(HaveOccurred())

	s3Config, err := config.NewFromReader(configFile)
	Expect(err).ToNot(HaveOccurred())

	s3Client, err := client.NewAwsS3Client(&s3Config)
	if err != nil {
		log.Fatalln(err)
	}

	blobstoreClient := client.New(s3Client, &s3Config)

	// First upload a test file using regular put operation
	contentFile := MakeContentFile(expectedContent)
	defer os.Remove(contentFile) //nolint:errcheck

	s3CLISession, err := RunS3CLI(s3CLIPath, configPath, storageType, "put", contentFile, s3Filename)
	Expect(err).ToNot(HaveOccurred())
	Expect(s3CLISession.ExitCode()).To(BeZero())

	regex := `(?m)((([A-Za-z]{3,9}:(?:\/\/?)?)(?:[-;:&=\+\$,\w]+@)?[A-Za-z0-9.-]+(:[0-9]+)?|(?:www.|[-;:&=\+\$,\w]+@)[A-Za-z0-9.-]+)((?:\/[\+~%\/.\w-_]*)?\??(?:[-\+=&;%@.\w_]*)#?(?:[\w]*))?)`

	// Test GET signed URL
	getURL, err := blobstoreClient.Sign(s3Filename, "get", 1*time.Minute)
	Expect(err).ToNot(HaveOccurred())
	Expect(getURL).To(MatchRegexp(regex))

	// Actually try to download from the GET signed URL
	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Get(getURL)
	Expect(err).ToNot(HaveOccurred())
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	Expect(err).ToNot(HaveOccurred())
	Expect(string(body)).To(Equal(expectedContent))

	// Test PUT signed URL
	putURL, err := blobstoreClient.Sign(s3Filename+"_put_test", "put", 1*time.Minute)
	Expect(err).ToNot(HaveOccurred())
	Expect(putURL).To(MatchRegexp(regex))

	// Actually try to upload to the PUT signed URL
	testUploadContent := "Test upload content via signed URL"
	putReq, err := http.NewRequest("PUT", putURL, strings.NewReader(testUploadContent))
	Expect(err).ToNot(HaveOccurred())

	putReq.Header.Set("Content-Type", "text/plain")
	putResp, err := httpClient.Do(putReq)
	Expect(err).ToNot(HaveOccurred())
	defer putResp.Body.Close() //nolint:errcheck
	Expect(putResp.StatusCode).To(Equal(200))

	// Clean up the test files
	_, err = RunS3CLI(s3CLIPath, configPath, storageType, "delete", s3Filename)
	Expect(err).ToNot(HaveOccurred())

	_, err = RunS3CLI(s3CLIPath, configPath, storageType, "delete", s3Filename+"_put_test")
	Expect(err).ToNot(HaveOccurred())
}
