package integration

import (
	"bytes"
	"os"

	"github.com/cloudfoundry/storage-cli/azurebs/config"

	. "github.com/onsi/gomega" //nolint:staticcheck
)

var storageType = "azurebs"

func AssertPutUsesNoTimeout(cliPath string, cfg *config.AZStorageConfig) {
	cfg2 := *cfg
	cfg2.Timeout = "" // unset -> no timeout
	configPath := MakeConfigFile(&cfg2)
	defer os.Remove(configPath) //nolint:errcheck

	content := MakeContentFile("hello")
	defer os.Remove(content) //nolint:errcheck
	blob := GenerateRandomString()

	sess, err := RunCli(cliPath, configPath, storageType, "put", content, blob)
	Expect(err).ToNot(HaveOccurred())
	Expect(sess.ExitCode()).To(BeZero())
	Expect(string(sess.Err.Contents())).To(ContainSubstring("Uploading ")) // stderr has log.Println
	Expect(string(sess.Err.Contents())).To(ContainSubstring("with no timeout"))

	sess, err = RunCli(cliPath, configPath, storageType, "delete", blob)
	Expect(err).ToNot(HaveOccurred())
	Expect(sess.ExitCode()).To(BeZero())
}

func AssertPutHonorsCustomTimeout(cliPath string, cfg *config.AZStorageConfig) {
	cfg2 := *cfg
	cfg2.Timeout = "3"
	configPath := MakeConfigFile(&cfg2)
	defer os.Remove(configPath) //nolint:errcheck

	content := MakeContentFile("ok")
	defer os.Remove(content) //nolint:errcheck
	blob := GenerateRandomString()

	sess, err := RunCli(cliPath, configPath, storageType, "put", content, blob)
	Expect(err).ToNot(HaveOccurred())
	Expect(sess.ExitCode()).To(BeZero())
	Expect(string(sess.Err.Contents())).To(ContainSubstring("with a timeout of 3s"))

	sess, err = RunCli(cliPath, configPath, storageType, "delete", blob)
	Expect(err).ToNot(HaveOccurred())
	Expect(sess.ExitCode()).To(BeZero())
}

func AssertPutTimesOut(cliPath string, cfg *config.AZStorageConfig) {
	cfg2 := *cfg
	cfg2.Timeout = "1"
	configPath := MakeConfigFile(&cfg2)
	defer os.Remove(configPath) //nolint:errcheck

	const mb = 1024 * 1024
	big := bytes.Repeat([]byte("x"), 250*mb)
	content := MakeContentFile(string(big))
	defer os.Remove(content) //nolint:errcheck
	blob := GenerateRandomString()

	sess, err := RunCli(cliPath, configPath, storageType, "put", content, blob)
	Expect(err).ToNot(HaveOccurred())
	Expect(sess.ExitCode()).ToNot(BeZero())
	Expect(string(sess.Err.Contents())).To(ContainSubstring("timeout of 1 reached while uploading"))
}

func AssertInvalidTimeoutIsError(cliPath string, cfg *config.AZStorageConfig) {
	cfg2 := *cfg
	cfg2.Timeout = "bananas"
	configPath := MakeConfigFile(&cfg2)
	defer os.Remove(configPath) //nolint:errcheck

	content := MakeContentFile("x")
	defer os.Remove(content) //nolint:errcheck
	blob := GenerateRandomString()

	sess, err := RunCli(cliPath, configPath, storageType, "put", content, blob)
	Expect(err).ToNot(HaveOccurred())
	Expect(sess.ExitCode()).ToNot(BeZero())
	Expect(string(sess.Err.Contents())).To(ContainSubstring(`Invalid timeout format "bananas"`))
}

func AssertZeroTimeoutIsError(cliPath string, cfg *config.AZStorageConfig) {
	cfg2 := *cfg
	cfg2.Timeout = "0"
	configPath := MakeConfigFile(&cfg2)
	defer os.Remove(configPath) //nolint:errcheck

	content := MakeContentFile("x")
	defer os.Remove(content) //nolint:errcheck
	blob := GenerateRandomString()

	sess, err := RunCli(cliPath, configPath, storageType, "put", content, blob)
	Expect(err).ToNot(HaveOccurred())
	Expect(sess.ExitCode()).ToNot(BeZero())

	Expect(string(sess.Err.Contents())).To(ContainSubstring(`Invalid time "0", need at least 1 second`))
}

func AssertNegativeTimeoutIsError(cliPath string, cfg *config.AZStorageConfig) {
	cfg2 := *cfg
	cfg2.Timeout = "-1"
	configPath := MakeConfigFile(&cfg2)
	defer os.Remove(configPath) //nolint:errcheck

	content := MakeContentFile("y")
	defer os.Remove(content) //nolint:errcheck
	blob := GenerateRandomString()

	sess, err := RunCli(cliPath, configPath, storageType, "put", content, blob)
	Expect(err).ToNot(HaveOccurred())
	Expect(sess.ExitCode()).ToNot(BeZero())

	Expect(string(sess.Err.Contents())).To(ContainSubstring(`Invalid time "-1", need at least 1 second`))
}

func AssertSignedURLTimeouts(cliPath string, cfg *config.AZStorageConfig) {
	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	sess, err := RunCli(cliPath, configPath, storageType, "sign", "some-blob", "get", "60s")
	Expect(err).ToNot(HaveOccurred())
	url := string(sess.Out.Contents())
	Expect(url).To(ContainSubstring("timeout=1800"))

	sess, err = RunCli(cliPath, configPath, storageType, "sign", "some-blob", "put", "60s")
	Expect(err).ToNot(HaveOccurred())
	url = string(sess.Out.Contents())
	Expect(url).To(ContainSubstring("timeout=2700"))
}

func AssertEnsureBucketIdempotent(cliPath string, cfg *config.AZStorageConfig) {
	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	s1, err := RunCli(cliPath, configPath, storageType, "ensure-bucket-exists")
	Expect(err).ToNot(HaveOccurred())
	Expect(s1.ExitCode()).To(BeZero())

	s2, err := RunCli(cliPath, configPath, storageType, "ensure-bucket-exists")
	Expect(err).ToNot(HaveOccurred())
	Expect(s2.ExitCode()).To(BeZero())
}

func AssertPutGetWithSpecialNames(cliPath string, cfg *config.AZStorageConfig) {
	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	name := "dir a/üñîçødë file.txt"
	content := "weird name content"
	f := MakeContentFile(content)
	defer os.Remove(f) //nolint:errcheck

	s, err := RunCli(cliPath, configPath, storageType, "put", f, name)
	Expect(err).ToNot(HaveOccurred())
	Expect(s.ExitCode()).To(BeZero())

	tmp, _ := os.CreateTemp("", "dl") //nolint:errcheck
	tmp.Close()                       //nolint:errcheck
	defer os.Remove(tmp.Name())       //nolint:errcheck

	s, err = RunCli(cliPath, configPath, storageType, "get", name, tmp.Name())
	Expect(err).ToNot(HaveOccurred())
	Expect(s.ExitCode()).To(BeZero())

	b, _ := os.ReadFile(tmp.Name()) //nolint:errcheck
	Expect(string(b)).To(Equal(content))

	s, err = RunCli(cliPath, configPath, storageType, "delete", name)
	Expect(err).ToNot(HaveOccurred())
	Expect(s.ExitCode()).To(BeZero())
}

func AssertLifecycleWorks(cliPath string, cfg *config.AZStorageConfig) {
	expectedString := GenerateRandomString()
	blobName := GenerateRandomString()

	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	contentFile := MakeContentFile(expectedString)
	defer os.Remove(contentFile) //nolint:errcheck

	// Ensure container/bucket exists
	cliSession, err := RunCli(cliPath, configPath, storageType, "ensure-bucket-exists")
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(BeZero())

	cliSession, err = RunCli(cliPath, configPath, storageType, "put", contentFile, blobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(BeZero())

	cliSession, err = RunCli(cliPath, configPath, storageType, "exists", blobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(BeZero())
	Expect(cliSession.Err.Contents()).To(MatchRegexp("File '.*' exists in bucket '.*'"))

	// Check blob properties
	cliSession, err = RunCli(cliPath, configPath, storageType, "properties", blobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(BeZero())
	output := string(cliSession.Out.Contents())
	Expect(output).To(MatchRegexp(`"etag":\s*".+?"`))
	Expect(output).To(MatchRegexp(`"last_modified":\s*".+?"`))
	Expect(output).To(MatchRegexp(`"content_length":\s*\d+`))

	tmpLocalFile, err := os.CreateTemp("", "azure-storage-cli-download")
	Expect(err).ToNot(HaveOccurred())
	err = tmpLocalFile.Close()
	Expect(err).ToNot(HaveOccurred())
	defer os.Remove(tmpLocalFile.Name()) //nolint:errcheck

	cliSession, err = RunCli(cliPath, configPath, storageType, "get", blobName, tmpLocalFile.Name())
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(BeZero())

	gottenBytes, err := os.ReadFile(tmpLocalFile.Name())
	Expect(err).ToNot(HaveOccurred())
	Expect(string(gottenBytes)).To(Equal(expectedString))

	cliSession, err = RunCli(cliPath, configPath, storageType, "delete", blobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(BeZero())

	cliSession, err = RunCli(cliPath, configPath, storageType, "exists", blobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(Equal(3))
	Expect(cliSession.Err.Contents()).To(MatchRegexp("File '.*' does not exist in bucket '.*'"))

	cliSession, err = RunCli(cliPath, configPath, storageType, "properties", blobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(Equal(0))
	Expect(cliSession.Out.Contents()).To(MatchRegexp("{}"))
}

func AssertOnCliVersion(cliPath string, cfg *config.AZStorageConfig) {
	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	cliSession, err := RunCli(cliPath, configPath, storageType, "-v")
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(Equal(0))

	consoleOutput := bytes.NewBuffer(cliSession.Out.Contents()).String()
	Expect(consoleOutput).To(ContainSubstring("version"))
}

func AssertGetNonexistentFails(cliPath string, cfg *config.AZStorageConfig) {
	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	cliSession, err := RunCli(cliPath, configPath, storageType, "get", "non-existent-file", "/dev/null")
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).ToNot(BeZero())
}

func AssertDeleteNonexistentWorks(cliPath string, cfg *config.AZStorageConfig) {
	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	cliSession, err := RunCli(cliPath, configPath, "delete", "non-existent-file")
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(BeZero())
}

func AssertOnSignedURLs(cliPath string, cfg *config.AZStorageConfig) {
	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	regex := "https://" + cfg.AccountName + ".blob.*/" + cfg.ContainerName + "/some-blob.*"

	cliSession, err := RunCli(cliPath, configPath, storageType, "sign", "some-blob", "get", "60s")
	Expect(err).ToNot(HaveOccurred())

	getUrl := bytes.NewBuffer(cliSession.Out.Contents()).String()
	Expect(getUrl).To(MatchRegexp(regex))

	cliSession, err = RunCli(cliPath, configPath, storageType, "sign", "some-blob", "put", "60s")
	Expect(err).ToNot(HaveOccurred())

	putUrl := bytes.NewBuffer(cliSession.Out.Contents()).String()
	Expect(putUrl).To(MatchRegexp(regex))
}

func AssertOnListDeleteLifecyle(cliPath string, cfg *config.AZStorageConfig) {
	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	cli, err := RunCli(cliPath, configPath, storageType, "delete-recursive", "")
	Expect(err).ToNot(HaveOccurred())
	Expect(cli.ExitCode()).To(BeZero())
	cliSession, err := RunCli(cliPath, configPath, storageType, "list")
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(BeZero())

	Expect(len(cliSession.Out.Contents())).To(BeZero())

	CreateRandomBlobs(cliPath, cfg, 4, "")

	customPrefix := "custom-prefix-"
	CreateRandomBlobs(cliPath, cfg, 4, customPrefix)

	otherPrefix := "other-prefix-"
	CreateRandomBlobs(cliPath, cfg, 2, otherPrefix)

	// Assert that the blobs are listed correctly
	cliSession, err = RunCli(cliPath, configPath, storageType, "list")
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(BeZero())
	Expect(len(bytes.FieldsFunc(cliSession.Out.Contents(), func(r rune) bool { return r == '\n' || r == '\r' }))).To(BeNumerically("==", 10))

	// Assert that the all blobs with custom prefix are listed correctly
	cliSession, err = RunCli(cliPath, configPath, storageType, "list", customPrefix)
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(BeZero())
	Expect(len(bytes.FieldsFunc(cliSession.Out.Contents(), func(r rune) bool { return r == '\n' || r == '\r' }))).To(BeNumerically("==", 4))

	// Delete all blobs with custom prefix
	cliSession, err = RunCli(cliPath, configPath, storageType, "delete-recursive", customPrefix)
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(BeZero())

	// Assert that the blobs with custom prefix are deleted
	cliSession, err = RunCli(cliPath, configPath, storageType, "list", customPrefix)
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(BeZero())
	Expect(len(cliSession.Out.Contents())).To(BeZero())

	// Assert that the other prefixed blobs are still listed
	cliSession, err = RunCli(cliPath, configPath, storageType, "list", otherPrefix)
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(BeZero())
	Expect(len(bytes.FieldsFunc(cliSession.Out.Contents(), func(r rune) bool { return r == '\n' || r == '\r' }))).To(BeNumerically("==", 2))

	// Delete all other blobs
	cliSession, err = RunCli(cliPath, configPath, "delete-recursive", "")
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(BeZero())

	// Assert that all blobs are deleted
	cliSession, err = RunCli(cliPath, configPath, storageType, "list")
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(BeZero())
	Expect(len(cliSession.Out.Contents())).To(BeZero())
}

func AssertOnCopy(cliPath string, cfg *config.AZStorageConfig) {
	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	// Create a blob to copy
	blobName := GenerateRandomString()
	blobContent := GenerateRandomString()
	contentFile := MakeContentFile(blobContent)
	defer os.Remove(contentFile) //nolint:errcheck

	cliSession, err := RunCli(cliPath, configPath, storageType, "put", contentFile, blobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(BeZero())

	// Copy the blob to a new name
	copiedBlobName := GenerateRandomString()
	cliSession, err = RunCli(cliPath, configPath, "copy", blobName, copiedBlobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(BeZero())

	// Assert that the copied blob exists
	cliSession, err = RunCli(cliPath, configPath, storageType, "exists", copiedBlobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(BeZero())

	// Compare the content of the original and copied blobs
	tmpLocalFile, err := os.CreateTemp("", "download-copy")
	Expect(err).ToNot(HaveOccurred())
	err = tmpLocalFile.Close()
	Expect(err).ToNot(HaveOccurred())
	defer os.Remove(tmpLocalFile.Name()) //nolint:errcheck
	cliSession, err = RunCli(cliPath, configPath, storageType, "get", blobName, tmpLocalFile.Name())
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(BeZero())
	gottenBytes, err := os.ReadFile(tmpLocalFile.Name())
	Expect(err).ToNot(HaveOccurred())
	Expect(string(gottenBytes)).To(Equal(blobContent))

	// Clean up
	cliSession, err = RunCli(cliPath, configPath, storageType, "delete", blobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(BeZero())
	cliSession, err = RunCli(cliPath, configPath, storageType, "delete", copiedBlobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(cliSession.ExitCode()).To(BeZero())
}

func CreateRandomBlobs(cliPath string, cfg *config.AZStorageConfig, count int, prefix string) {
	configPath := MakeConfigFile(cfg)
	defer os.Remove(configPath) //nolint:errcheck

	for i := 0; i < count; i++ {
		blobName := GenerateRandomString()
		if prefix != "" {
			blobName = prefix + blobName
		}
		contentFile := MakeContentFile(GenerateRandomString())
		defer os.Remove(contentFile) //nolint:errcheck

		cliSession, err := RunCli(cliPath, configPath, storageType, "put", contentFile, blobName)
		Expect(err).ToNot(HaveOccurred())
		Expect(cliSession.ExitCode()).To(BeZero())
	}
}
