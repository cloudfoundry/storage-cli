package integration_test

import (
	"bytes"
	"os"

	"github.com/cloudfoundry/storage-cli/azurebs/config"
	"github.com/cloudfoundry/storage-cli/azurebs/integration"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("General testing for all Azure regions", func() {
	var defaultConfig config.AZStorageConfig
	storageType := "azurebs"

	BeforeEach(func() {
		defaultConfig = config.AZStorageConfig{
			AccountName:   os.Getenv("ACCOUNT_NAME"),
			AccountKey:    os.Getenv("ACCOUNT_KEY"),
			ContainerName: os.Getenv("CONTAINER_NAME"),
			Environment:   os.Getenv("ENVIRONMENT"),
		}
		if defaultConfig.Environment == "" {
			defaultConfig.Environment = "AzureCloud"
		}

		Expect(defaultConfig.AccountName).ToNot(BeEmpty(), "ACCOUNT_NAME must be set")
		Expect(defaultConfig.AccountKey).ToNot(BeEmpty(), "ACCOUNT_KEY must be set")
		Expect(defaultConfig.ContainerName).ToNot(BeEmpty(), "CONTAINER_NAME must be set")
	})

	configurations := []TableEntry{
		Entry("with default config", &defaultConfig),
	}
	DescribeTable("Assert Put Uses No Timeout When Not Specified",
		func(cfg *config.AZStorageConfig) { integration.AssertPutUsesNoTimeout(cliPath, cfg) },
		configurations,
	)
	DescribeTable("Assert Put Honors Custom Timeout",
		func(cfg *config.AZStorageConfig) { integration.AssertPutHonorsCustomTimeout(cliPath, cfg) },
		configurations,
	)
	DescribeTable("Assert Put Times Out",
		func(cfg *config.AZStorageConfig) { integration.AssertPutTimesOut(cliPath, cfg) },
		configurations,
	)
	DescribeTable("Assert Invalid Timeout Error",
		func(cfg *config.AZStorageConfig) { integration.AssertInvalidTimeoutIsError(cliPath, cfg) },
		configurations,
	)
	DescribeTable("Assert Signed URL Timeouts",
		func(cfg *config.AZStorageConfig) { integration.AssertSignedURLTimeouts(cliPath, cfg) },
		configurations,
	)
	DescribeTable("Rejects zero timeout",
		func(cfg *config.AZStorageConfig) { integration.AssertZeroTimeoutIsError(cliPath, cfg) },
		configurations,
	)
	DescribeTable("Rejects negative timeout",
		func(cfg *config.AZStorageConfig) { integration.AssertNegativeTimeoutIsError(cliPath, cfg) },
		configurations,
	)
	DescribeTable("Assert Ensure Bucket Idempotent",
		func(cfg *config.AZStorageConfig) { integration.AssertEnsureStorageIdempotent(cliPath, cfg) },
		configurations,
	)
	DescribeTable("Assert Put Get With Special Names",
		func(cfg *config.AZStorageConfig) { integration.AssertPutGetWithSpecialNames(cliPath, cfg) },
		configurations,
	)
	DescribeTable("Blobstore lifecycle works",
		func(cfg *config.AZStorageConfig) { integration.AssertLifecycleWorks(cliPath, cfg) },
		configurations,
	)
	DescribeTable("Invoking `get` on a non-existent-key fails",
		func(cfg *config.AZStorageConfig) { integration.AssertGetNonexistentFails(cliPath, cfg) },
		configurations,
	)
	DescribeTable("Invoking `delete` on a non-existent-key does not fail",
		func(cfg *config.AZStorageConfig) { integration.AssertDeleteNonexistentWorks(cliPath, cfg) },
		configurations,
	)
	DescribeTable("Invoking `sign` returns a signed URL",
		func(cfg *config.AZStorageConfig) { integration.AssertOnSignedURLs(cliPath, cfg) },
		configurations,
	)
	DescribeTable("Blobstore list and delete lifecycle works",
		func(cfg *config.AZStorageConfig) { integration.AssertOnListDeleteLifecyle(cliPath, cfg) },
		configurations,
	)

	DescribeTable("Server-side copy works",
		func(cfg *config.AZStorageConfig) { integration.AssertOnCopy(cliPath, cfg) },
		configurations,
	)

	Describe("Invoking `put`", func() {
		var blobName string
		var configPath string
		var contentFile string

		BeforeEach(func() {
			blobName = integration.GenerateRandomString()
			configPath = integration.MakeConfigFile(&defaultConfig)
			contentFile = integration.MakeContentFile("foo")
		})

		AfterEach(func() {
			os.Remove(configPath)  //nolint:errcheck
			os.Remove(contentFile) //nolint:errcheck
		})

		It("uploads a file", func() {
			defer func() {
				cliSession, err := integration.RunCli(cliPath, configPath, storageType, "delete", blobName)
				Expect(err).ToNot(HaveOccurred())
				Expect(cliSession.ExitCode()).To(BeZero())
			}()

			cliSession, err := integration.RunCli(cliPath, configPath, storageType, "put", contentFile, blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, storageType, "exists", blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())
			Expect(cliSession.Err).Should(gbytes.Say(`"msg":"Blob exists in container"`))
		})

		It("overwrites an existing file", func() {
			defer func() {
				cliSession, err := integration.RunCli(cliPath, configPath, storageType, "delete", blobName)
				Expect(err).ToNot(HaveOccurred())
				Expect(cliSession.ExitCode()).To(BeZero())
			}()

			tmpLocalFileName := "azure-storage-cli-download"
			defer os.Remove(tmpLocalFileName) //nolint:errcheck

			contentFile = integration.MakeContentFile("initial content")
			cliSession, err := integration.RunCli(cliPath, configPath, storageType, "put", contentFile, blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, storageType, "get", blobName, tmpLocalFileName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			gottenBytes, _ := os.ReadFile(tmpLocalFileName) //nolint:errcheck
			Expect(string(gottenBytes)).To(Equal("initial content"))

			contentFile = integration.MakeContentFile("updated content")
			cliSession, err = integration.RunCli(cliPath, configPath, storageType, "put", contentFile, blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, storageType, "get", blobName, tmpLocalFileName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			gottenBytes, _ = os.ReadFile(tmpLocalFileName) //nolint:errcheck
			Expect(string(gottenBytes)).To(Equal("updated content"))
		})

		It("returns the appropriate error message", func() {
			cfg := &config.AZStorageConfig{
				AccountName:   os.Getenv("ACCOUNT_NAME"),
				AccountKey:    os.Getenv("ACCOUNT_KEY"),
				ContainerName: "not-existing",
			}

			configPath = integration.MakeConfigFile(cfg)

			cliSession, err := integration.RunCli(cliPath, configPath, storageType, "put", contentFile, blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(Equal(1))

			consoleOutput := bytes.NewBuffer(cliSession.Err.Contents()).String()
			Expect(consoleOutput).To(ContainSubstring("upload failure"))
		})
	})
	Describe("Invoking `-v`", func() {
		It("returns the cli version", func() {
			integration.AssertOnCliVersion(cliPath, &defaultConfig)
		})
	})
})
