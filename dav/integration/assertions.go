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
	Expect(session.Out.Contents()).To(ContainSubstring(fmt.Sprintf("\"ContentLength\": %d", len(expectedString))))
	Expect(session.Out.Contents()).To(ContainSubstring("\"ETag\":"))
	Expect(session.Out.Contents()).To(ContainSubstring("\"LastModified\":"))

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

// AssertOnSignedURLs tests signed URL generation
func AssertOnSignedURLs(cliPath string, cfg *config.Config) {
	storageType := "dav"
	blobName := GenerateRandomString()
	expectedString := GenerateRandomString()

	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	contentFile := MakeContentFile(expectedString)
	defer os.Remove(contentFile) //nolint:errcheck

	// Upload blob
	session, err := RunCli(cliPath, configPath, storageType, "put", contentFile, blobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	defer func() {
		session, err := RunCli(cliPath, configPath, storageType, "delete", blobName)
		Expect(err).ToNot(HaveOccurred())
		Expect(session.ExitCode()).To(BeZero())
	}()

	// Generate signed URL
	session, err = RunCli(cliPath, configPath, storageType, "sign", blobName, "get", "3600s")
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())
	Expect(session.Out.Contents()).To(ContainSubstring("http"))
	Expect(session.Out.Contents()).To(ContainSubstring("st="))
	Expect(session.Out.Contents()).To(ContainSubstring("ts="))
	Expect(session.Out.Contents()).To(ContainSubstring("e="))
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
