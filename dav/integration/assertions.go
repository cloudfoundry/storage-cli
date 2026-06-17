package integration

import (
	"fmt"
	"os"

	"github.com/cloudfoundry/storage-cli/dav/config"

	. "github.com/onsi/gomega" //nolint:staticcheck
)

const storageType = "dav"

func AssertLifecycleWorks(cliPath string, cfg *config.Config) {
	expectedString := GenerateRandomString()
	blobName := GenerateRandomString()

	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	contentFile := MakeContentFile(expectedString)
	defer os.Remove(contentFile) //nolint:errcheck

	session, err := RunCli(cliPath, configPath, storageType, "put", contentFile, blobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	session, err = RunCli(cliPath, configPath, storageType, "exists", blobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

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

	session, err = RunCli(cliPath, configPath, storageType, "properties", blobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())
	Expect(session.Out.Contents()).To(ContainSubstring(fmt.Sprintf("\"content_length\": %d", len(expectedString))))
	Expect(session.Out.Contents()).To(ContainSubstring("\"etag\":"))
	Expect(session.Out.Contents()).To(ContainSubstring("\"last_modified\":"))

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

	session, err = RunCli(cliPath, configPath, storageType, "delete", blobName+"_copy")
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	session, err = RunCli(cliPath, configPath, storageType, "delete", blobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	session, err = RunCli(cliPath, configPath, storageType, "exists", blobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).ToNot(BeZero())

	session, err = RunCli(cliPath, configPath, storageType, "properties", blobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())
	Expect(session.Out.Contents()).To(ContainSubstring("{}"))
}

func AssertGetNonexistentFails(cliPath string, cfg *config.Config) {
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

func AssertDeleteNonexistentWorks(cliPath string, cfg *config.Config) {
	blobName := GenerateRandomString()

	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	session, err := RunCli(cliPath, configPath, storageType, "delete", blobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())
}

func AssertOnListDeleteLifecycle(cliPath string, cfg *config.Config) {
	prefix := GenerateRandomString()

	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	contentFiles := make([]string, 3)
	for i := range contentFiles {
		contentFiles[i] = MakeContentFile(GenerateRandomString())
	}
	defer func() {
		for _, f := range contentFiles {
			os.Remove(f) //nolint:errcheck
		}
	}()

	for i := 0; i < 3; i++ {
		blobName := fmt.Sprintf("%s-%d", prefix, i)
		session, err := RunCli(cliPath, configPath, storageType, "put", contentFiles[i], blobName)
		Expect(err).ToNot(HaveOccurred())
		Expect(session.ExitCode()).To(BeZero())
	}

	session, err := RunCli(cliPath, configPath, storageType, "list", prefix)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())
	for i := 0; i < 3; i++ {
		Expect(session.Out.Contents()).To(ContainSubstring(fmt.Sprintf("%s-%d", prefix, i)))
	}

	session, err = RunCli(cliPath, configPath, storageType, "delete-recursive", prefix)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	for i := 0; i < 3; i++ {
		blobName := fmt.Sprintf("%s-%d", prefix, i)
		session, err := RunCli(cliPath, configPath, storageType, "exists", blobName)
		Expect(err).ToNot(HaveOccurred())
		Expect(session.ExitCode()).ToNot(BeZero())
	}
}

func AssertListNonexistentPrefixReturnsEmpty(cliPath string, cfg *config.Config) {
	nonExistentPrefix := GenerateRandomString()

	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	session, err := RunCli(cliPath, configPath, storageType, "list", nonExistentPrefix)
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())
	Expect(string(session.Out.Contents())).To(BeEmpty())
}

func AssertEnsureStorageExists(cliPath string, cfg *config.Config) {
	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	session, err := RunCli(cliPath, configPath, storageType, "ensure-storage-exists")
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())

	// idempotent
	session, err = RunCli(cliPath, configPath, storageType, "ensure-storage-exists")
	Expect(err).ToNot(HaveOccurred())
	Expect(session.ExitCode()).To(BeZero())
}
