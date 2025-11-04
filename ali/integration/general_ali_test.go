package integration_test

import (
	"bytes"
	"io/ioutil"
	"os"

	"github.com/cloudfoundry/storage-cli/ali/config"
	"github.com/cloudfoundry/storage-cli/ali/integration"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("General testing for all Ali regions", func() {

	var blobName string
	var configPath string
	var contentFile string

	BeforeEach(func() {
		blobName = integration.GenerateRandomString()
		configPath = integration.MakeConfigFile(&defaultConfig)
		contentFile = integration.MakeContentFile("foo")
	})

	AfterEach(func() {
		defer func() { _ = os.Remove(configPath) }()
		defer func() { _ = os.Remove(contentFile) }()
	})

	Describe("Invoking `put`", func() {
		It("uploads a file", func() {
			defer func() {
				cliSession, err := integration.RunCli(cliPath, configPath, "delete", blobName)
				Expect(err).ToNot(HaveOccurred())
				Expect(cliSession.ExitCode()).To(BeZero())
			}()

			cliSession, err := integration.RunCli(cliPath, configPath, "put", contentFile, blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, "exists", blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			Expect(string(cliSession.Err.Contents())).To(MatchRegexp("File '" + blobName + "' exists in bucket '" + bucketName + "'"))
		})

		It("overwrites an existing file", func() {
			defer func() {
				cliSession, err := integration.RunCli(cliPath, configPath, "delete", blobName)
				Expect(err).ToNot(HaveOccurred())
				Expect(cliSession.ExitCode()).To(BeZero())
			}()

			tmpLocalFile, _ := os.CreateTemp("", "ali-storage-cli-download")
			tmpLocalFile.Close()
			defer func() { _ = os.Remove(tmpLocalFile.Name()) }()

			contentFile = integration.MakeContentFile("initial content")
			cliSession, err := integration.RunCli(cliPath, configPath, "put", contentFile, blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, "get", blobName, tmpLocalFile.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			gottenBytes, _ := os.ReadFile(tmpLocalFile.Name())
			Expect(string(gottenBytes)).To(Equal("initial content"))

			contentFile = integration.MakeContentFile("updated content")
			cliSession, err = integration.RunCli(cliPath, configPath, "put", contentFile, blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, "get", blobName, tmpLocalFile.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			gottenBytes, _ = os.ReadFile(tmpLocalFile.Name())
			Expect(string(gottenBytes)).To(Equal("updated content"))
		})

		It("returns the appropriate error message", func() {
			cfg := &config.AliStorageConfig{
				AccessKeyID:     accessKeyID,
				AccessKeySecret: accessKeySecret,
				Endpoint:        endpoint,
				BucketName:      "not-existing",
			}

			configPath = integration.MakeConfigFile(cfg)

			cliSession, err := integration.RunCli(cliPath, configPath, "put", contentFile, blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(Equal(1))

			consoleOutput := bytes.NewBuffer(cliSession.Err.Contents()).String()
			Expect(consoleOutput).To(ContainSubstring("upload failure"))
		})
	})

	Describe("Invoking `get`", func() {
		It("downloads a file", func() {
			outputFilePath := "/tmp/" + integration.GenerateRandomString()

			defer func() {
				cliSession, err := integration.RunCli(cliPath, configPath, "delete", blobName)
				Expect(err).ToNot(HaveOccurred())
				Expect(cliSession.ExitCode()).To(BeZero())

				_ = os.Remove(outputFilePath)
			}()

			cliSession, err := integration.RunCli(cliPath, configPath, "put", contentFile, blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, "get", blobName, outputFilePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			fileContent, _ := ioutil.ReadFile(outputFilePath)
			Expect(string(fileContent)).To(Equal("foo"))
		})
	})

	Describe("Invoking `delete`", func() {
		It("deletes a file", func() {
			defer func() {
				cliSession, err := integration.RunCli(cliPath, configPath, "delete", blobName)
				Expect(err).ToNot(HaveOccurred())
				Expect(cliSession.ExitCode()).To(BeZero())
			}()

			cliSession, err := integration.RunCli(cliPath, configPath, "put", contentFile, blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, "delete", blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, "exists", blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(Equal(3))
		})
	})

	Describe("Invoking `exists`", func() {
		It("returns 0 for an existing blob", func() {
			defer func() {
				cliSession, err := integration.RunCli(cliPath, configPath, "delete", blobName)
				Expect(err).ToNot(HaveOccurred())
				Expect(cliSession.ExitCode()).To(BeZero())
			}()

			cliSession, err := integration.RunCli(cliPath, configPath, "put", contentFile, blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, "exists", blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(Equal(0))
		})

		It("returns 3 for a not existing blob", func() {
			cliSession, err := integration.RunCli(cliPath, configPath, "exists", blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(Equal(3))
		})
	})

	Describe("Invoking `sign`", func() {
		It("returns 0 for an existing blob", func() {
			cliSession, err := integration.RunCli(cliPath, configPath, "sign", "some-blob", "get", "60s")
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			getUrl := bytes.NewBuffer(cliSession.Out.Contents()).String()
			Expect(getUrl).To(MatchRegexp("http://" + bucketName + "." + endpoint + "/some-blob"))

			cliSession, err = integration.RunCli(cliPath, configPath, "sign", "some-blob", "put", "60s")
			Expect(err).ToNot(HaveOccurred())

			putUrl := bytes.NewBuffer(cliSession.Out.Contents()).String()
			Expect(putUrl).To(MatchRegexp("http://" + bucketName + "." + endpoint + "/some-blob"))
		})

		It("returns 3 for a not existing blob", func() {
			cliSession, err := integration.RunCli(cliPath, configPath, "exists", blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(Equal(3))
		})
	})

	Describe("Invoking `-v`", func() {
		It("returns the cli version", func() {
			configPath := integration.MakeConfigFile(&defaultConfig)
			defer func() { _ = os.Remove(configPath) }()

			cliSession, err := integration.RunCli(cliPath, configPath, "-v")
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(Equal(0))

			consoleOutput := bytes.NewBuffer(cliSession.Out.Contents()).String()
			Expect(consoleOutput).To(ContainSubstring("version"))
		})
	})
})
