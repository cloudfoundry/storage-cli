package integration

import (
	"fmt"
	"os"

	"github.com/cloudfoundry/storage-cli/dav/config"

	. "github.com/onsi/gomega" //nolint:staticcheck
)

// AssertLifecycleWorks tests the main blobstore object lifecycle from creation to deletion
func AssertLifecycleWorks(cliPath string, cfg *config.Config) {
	storageType := "dav"
	expectedString := GenerateRandomString()
	blobName := GenerateRandomString()

	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	contentFile := MakeContentFile(expectedString)
	defer os.Remove(contentFile) //nolint:errcheck

	// Test PUT
	session, err := RunCli(cliPath, configPath, storageType, "put", contentFile, blobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	// Test EXISTS
	session, err = RunCli(cliPath, configPath, storageType, "exists", blobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	// Test GET
	tmpLocalFile, err := os.CreateTemp("", "davcli-download")
	Expect(err).ToNot(HaveOccurred())
	err = tmpLocalFile.Close()
	Expect(err).ToNot(HaveOccurred())
	defer os.Remove(tmpLocalFile.Name()) //nolint:errcheck

	session, err = RunCli(cliPath, configPath, storageType, "get", blobName, tmpLocalFile.Name())
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	gottenBytes, err := os.ReadFile(tmpLocalFile.Name())
	Expect(err).ToNot(HaveOccurred())
	Expect(string(gottenBytes)).To(Equal(expectedString))

	// Test PROPERTIES
	session, err = RunCli(cliPath, configPath, storageType, "properties", blobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())
	Expect(session.Out.Contents()).To(ContainSubstring(fmt.Sprintf("\"content_length\": %d", len(expectedString))))
	Expect(session.Out.Contents()).To(ContainSubstring("\"etag\":"))
	Expect(session.Out.Contents()).To(ContainSubstring("\"last_modified\":"))

	// Test COPY
	session, err = RunCli(cliPath, configPath, storageType, "copy", blobName, blobName+"_copy")
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	session, err = RunCli(cliPath, configPath, storageType, "exists", blobName+"_copy")
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	tmpCopiedFile, err := os.CreateTemp("", "davcli-download-copy")
	Expect(err).ToNot(HaveOccurred())
	err = tmpCopiedFile.Close()
	Expect(err).ToNot(HaveOccurred())
	defer os.Remove(tmpCopiedFile.Name()) //nolint:errcheck

	session, err = RunCli(cliPath, configPath, storageType, "get", blobName+"_copy", tmpCopiedFile.Name())
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	copiedBytes, err := os.ReadFile(tmpCopiedFile.Name())
	Expect(err).ToNot(HaveOccurred())
	Expect(string(copiedBytes)).To(Equal(expectedString))

	// Test DELETE (copied blob)
	session, err = RunCli(cliPath, configPath, storageType, "delete", blobName+"_copy")
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	// Test DELETE (original blob)
	session, err = RunCli(cliPath, configPath, storageType, "delete", blobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	// Verify blob no longer exists
	session, err = RunCli(cliPath, configPath, storageType, "exists", blobName)
	Expect(err).ToNot(HaveOccurred())
	// Exit code should be non-zero (blob doesn't exist)
	Expect(session.ExitCode()).ToNot(BeZero())

	// Properties should return empty for non-existent blob
	session, err = RunCli(cliPath, configPath, storageType, "properties", blobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())
	Expect(session.Out.Contents()).To(ContainSubstring("{}"))
}

// AssertGetNonexistentFails tests that getting a non-existent blob fails
func AssertGetNonexistentFails(cliPath string, cfg *config.Config) {
	storageType := "dav"
	blobName := GenerateRandomString()

	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	tmpLocalFile, err := os.CreateTemp("", "davcli-download")
	Expect(err).ToNot(HaveOccurred())
	err = tmpLocalFile.Close()
	Expect(err).ToNot(HaveOccurred())
	defer os.Remove(tmpLocalFile.Name()) //nolint:errcheck

	session, err := RunCli(cliPath, configPath, storageType, "get", blobName, tmpLocalFile.Name())
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).ToNot(BeZero())
}

// AssertDeleteNonexistentWorks tests that deleting a non-existent blob succeeds
func AssertDeleteNonexistentWorks(cliPath string, cfg *config.Config) {
	storageType := "dav"
	blobName := GenerateRandomString()

	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	session, err := RunCli(cliPath, configPath, storageType, "delete", blobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())
}

// AssertOnListDeleteLifecycle tests list and delete-recursive functionality
func AssertOnListDeleteLifecycle(cliPath string, cfg *config.Config) {
	storageType := "dav"
	prefix := GenerateRandomString()

	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	// Create multiple blobs with the same prefix
	for i := 0; i < 3; i++ {
		content := GenerateRandomString()
		contentFile := MakeContentFile(content)
		defer os.Remove(contentFile) //nolint:errcheck

		blobName := fmt.Sprintf("%s-%d", prefix, i)
		session, err := RunCli(cliPath, configPath, storageType, "put", contentFile, blobName)
		Expect(err).ToNot(HaveOccurred())
		Expect(session.ExitCode()).To(BeZero())
	}

	// Test LIST
	session, err := RunCli(cliPath, configPath, storageType, "list", prefix)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())
	for i := 0; i < 3; i++ {
		Expect(session.Out.Contents()).To(ContainSubstring(fmt.Sprintf("%s-%d", prefix, i)))
	}

	// Test DELETE-RECURSIVE
	session, err = RunCli(cliPath, configPath, storageType, "delete-recursive", prefix)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	// Verify all blobs are deleted
	for i := 0; i < 3; i++ {
		blobName := fmt.Sprintf("%s-%d", prefix, i)
		session, err := RunCli(cliPath, configPath, storageType, "exists", blobName)
		Expect(err).ToNot(HaveOccurred())
		// Exit code should be non-zero (blob doesn't exist)
		// DAV returns 3 for NotExistsError, but may return 1 for other "not found" scenarios
		Expect(session.ExitCode()).ToNot(BeZero())
	}
}

// AssertListNonexistentPrefixReturnsEmpty tests that listing a non-existent prefix returns empty list
func AssertListNonexistentPrefixReturnsEmpty(cliPath string, cfg *config.Config) {
	storageType := "dav"
	nonExistentPrefix := GenerateRandomString()

	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	// List with a prefix that doesn't exist - should return empty, not error
	session, err := RunCli(cliPath, configPath, storageType, "list", nonExistentPrefix)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	// Output should be empty (no blobs found)
	output := string(session.Out.Contents())
	Expect(output).To(BeEmpty())
}

// AssertOnSignedURLs tests signed URL generation with hmac-sha256 format
// Note: This test only validates that signed URLs are generated with correct format.
// It does not test actual signed URL usage since that requires nginx with secure_link module,
// which is not available in the Apache WebDAV test environment.
func AssertOnSignedURLs(cliPath string, cfg *config.Config) {
	storageType := "dav"
	blobName := GenerateRandomString()

	// Create config with secret for signing
	configWithSecret := MakeConfigFile(cfg)
	defer os.Remove(configWithSecret) //nolint:errcheck

	// Generate signed PUT URL
	session, err := RunCli(cliPath, configWithSecret, storageType, "sign", blobName, "put", "3600s")
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	signedPutURL := string(session.Out.Contents())
	Expect(signedPutURL).To(ContainSubstring("http"))
	Expect(signedPutURL).To(ContainSubstring("st="))
	Expect(signedPutURL).To(ContainSubstring("ts="))
	Expect(signedPutURL).To(ContainSubstring("e="))

	// Verify PUT URL contains /signed/ path prefix for hmac-sha256 format
	Expect(signedPutURL).To(ContainSubstring("/signed/"))

	// Generate signed GET URL
	session, err = RunCli(cliPath, configWithSecret, storageType, "sign", blobName, "get", "3600s")
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	signedGetURL := string(session.Out.Contents())
	Expect(signedGetURL).To(ContainSubstring("http"))
	Expect(signedGetURL).To(ContainSubstring("st="))
	Expect(signedGetURL).To(ContainSubstring("ts="))
	Expect(signedGetURL).To(ContainSubstring("e="))

	// Verify GET URL contains /signed/ path prefix for hmac-sha256 format
	Expect(signedGetURL).To(ContainSubstring("/signed/"))
}

// AssertOnSignedURLsSecureLinkMD5 tests signed URL generation with secure-link-md5 format
func AssertOnSignedURLsSecureLinkMD5(cliPath string, cfg *config.Config) {
	storageType := "dav"
	blobName := GenerateRandomString()

	configWithSecret := MakeConfigFile(cfg)
	defer os.Remove(configWithSecret) //nolint:errcheck

	// Generate signed PUT URL
	session, err := RunCli(cliPath, configWithSecret, storageType, "sign", blobName, "put", "3600s")
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	signedPutURL := string(session.Out.Contents())
	Expect(signedPutURL).To(ContainSubstring("http"))
	Expect(signedPutURL).To(ContainSubstring("md5="))
	Expect(signedPutURL).To(ContainSubstring("expires="))

	// Verify URL does NOT contain /signed/ prefix (secure-link-md5 format)
	Expect(signedPutURL).ToNot(ContainSubstring("/signed/"))

	// Generate signed GET URL
	session, err = RunCli(cliPath, configWithSecret, storageType, "sign", blobName, "get", "3600s")
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	signedGetURL := string(session.Out.Contents())
	Expect(signedGetURL).To(ContainSubstring("http"))
	Expect(signedGetURL).To(ContainSubstring("md5="))
	Expect(signedGetURL).To(ContainSubstring("expires="))

	// Verify URL does NOT contain /signed/ prefix (secure-link-md5 format)
	Expect(signedGetURL).ToNot(ContainSubstring("/signed/"))
}

// AssertOnSignedURLsWithCustomExpiration tests signed URL generation with custom expiration
func AssertOnSignedURLsWithCustomExpiration(cliPath string, cfg *config.Config, expectedExpirationMinutes uint) {
	storageType := "dav"
	blobName := GenerateRandomString()

	configWithSecret := MakeConfigFile(cfg)
	defer os.Remove(configWithSecret) //nolint:errcheck

	// Generate signed URL with explicit duration
	durationStr := fmt.Sprintf("%ds", expectedExpirationMinutes*60)
	session, err := RunCli(cliPath, configWithSecret, storageType, "sign", blobName, "put", durationStr)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	signedURL := string(session.Out.Contents())
	Expect(signedURL).To(ContainSubstring("http"))

	// Verify URL contains expiration parameter
	// For hmac-sha256: e=<seconds>
	// For secure-link-md5: expires=<unix_timestamp>
	expectedSeconds := fmt.Sprintf("%d", expectedExpirationMinutes*60)
	if cfg.SignedURLFormat == "secure-link-md5" {
		Expect(signedURL).To(ContainSubstring("expires="))
	} else {
		Expect(signedURL).To(ContainSubstring(fmt.Sprintf("e=%s", expectedSeconds)))
	}
}

// AssertEnsureStorageExists tests ensure-storage-exists command
func AssertEnsureStorageExists(cliPath string, cfg *config.Config) {
	storageType := "dav"

	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	session, err := RunCli(cliPath, configPath, storageType, "ensure-storage-exists")
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	// Should be idempotent - run again
	session, err = RunCli(cliPath, configPath, storageType, "ensure-storage-exists")
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())
}
