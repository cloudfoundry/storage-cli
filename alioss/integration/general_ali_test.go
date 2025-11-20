package integration_test

import (
	"bytes"

	"os"

	"github.com/cloudfoundry/storage-cli/alioss/config"
	"github.com/cloudfoundry/storage-cli/alioss/integration"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("General testing for all Ali regions", func() {

	var blobName string
	var configPath string
	var contentFile string
	var storageType = "alioss"

	BeforeEach(func() {
		blobName = integration.GenerateRandomString()
		configPath = integration.MakeConfigFile(&defaultConfig)
		contentFile = integration.MakeContentFile("foo")
	})

	AfterEach(func() {
		defer func() { _ = os.Remove(configPath) }()  //nolint:errcheck
		defer func() { _ = os.Remove(contentFile) }() //nolint:errcheck
	})

	Describe("Invoking `put`", func() {
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

			Expect(string(cliSession.Err.Contents())).To(MatchRegexp("File '" + blobName + "' exists in bucket '" + bucketName + "'"))
		})

		It("overwrites an existing file", func() {
			defer func() {
				cliSession, err := integration.RunCli(cliPath, configPath, storageType, "delete", blobName)
				Expect(err).ToNot(HaveOccurred())
				Expect(cliSession.ExitCode()).To(BeZero())
			}()

			tmpLocalFile, _ := os.CreateTemp("", "ali-storage-cli-download") //nolint:errcheck
			tmpLocalFile.Close()                                             //nolint:errcheck
			defer func() { _ = os.Remove(tmpLocalFile.Name()) }()            //nolint:errcheck

			contentFile = integration.MakeContentFile("initial content")
			cliSession, err := integration.RunCli(cliPath, configPath, storageType, "put", contentFile, blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, storageType, "get", blobName, tmpLocalFile.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			gottenBytes, _ := os.ReadFile(tmpLocalFile.Name()) //nolint:errcheck
			Expect(string(gottenBytes)).To(Equal("initial content"))

			contentFile = integration.MakeContentFile("updated content")
			cliSession, err = integration.RunCli(cliPath, configPath, storageType, "put", contentFile, blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, storageType, "get", blobName, tmpLocalFile.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			gottenBytes, _ = os.ReadFile(tmpLocalFile.Name()) //nolint:errcheck
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

			cliSession, err := integration.RunCli(cliPath, configPath, storageType, "put", contentFile, blobName)
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
				cliSession, err := integration.RunCli(cliPath, configPath, storageType, "delete", blobName)
				Expect(err).ToNot(HaveOccurred())
				Expect(cliSession.ExitCode()).To(BeZero())

				_ = os.Remove(outputFilePath) //nolint:errcheck
			}()

			cliSession, err := integration.RunCli(cliPath, configPath, storageType, "put", contentFile, blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, storageType, "get", blobName, outputFilePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			fileContent, _ := os.ReadFile(outputFilePath) //nolint:errcheck
			Expect(string(fileContent)).To(Equal("foo"))
		})
	})

	Describe("Invoking `delete`", func() {
		It("deletes a file", func() {
			defer func() {
				cliSession, err := integration.RunCli(cliPath, configPath, storageType, "delete", blobName)
				Expect(err).ToNot(HaveOccurred())
				Expect(cliSession.ExitCode()).To(BeZero())
			}()

			cliSession, err := integration.RunCli(cliPath, configPath, storageType, "put", contentFile, blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, storageType, "delete", blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, storageType, "exists", blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(Equal(3))
		})
	})

	Describe("Invoking `delete-recursive`", func() {
		It("deletes all objects with a given prefix", func() {
			prefix := integration.GenerateRandomString()
			blob1 := prefix + "/a"
			blob2 := prefix + "/b"
			otherBlob := integration.GenerateRandomString()

			// Create a temp file for uploads
			contentFile1 := integration.MakeContentFile("content-1")
			contentFile2 := integration.MakeContentFile("content-2")
			contentFileOther := integration.MakeContentFile("other-content")
			defer func() {
				_ = os.Remove(contentFile1)     //nolint:errcheck
				_ = os.Remove(contentFile2)     //nolint:errcheck
				_ = os.Remove(contentFileOther) //nolint:errcheck
				// make sure all are gone
				for _, b := range []string{blob1, blob2, otherBlob} {
					cliSession, err := integration.RunCli(cliPath, configPath, "delete", b)
					if err == nil && (cliSession.ExitCode() == 0 || cliSession.ExitCode() == 3) {
						continue
					}
				}
			}()

			// Put three blobs
			cliSession, err := integration.RunCli(cliPath, configPath, "put", contentFile1, blob1)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, "put", contentFile2, blob2)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, "put", contentFileOther, otherBlob)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			// Call delete-recursive
			cliSession, err = integration.RunCli(cliPath, configPath, "delete-recursive", prefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			// Objects with prefix should be gone (exit code 3)
			cliSession, err = integration.RunCli(cliPath, configPath, "exists", blob1)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(Equal(3))

			cliSession, err = integration.RunCli(cliPath, configPath, "exists", blob2)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(Equal(3))

			// Other blob should still exist (exit code 0)
			cliSession, err = integration.RunCli(cliPath, configPath, "exists", otherBlob)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(Equal(0))
		})
	})

	Describe("Invoking `copy`", func() {
		It("copies the contents from one object to another", func() {
			srcBlob := blobName + "-src"
			destBlob := blobName + "-dest"

			// Clean up both at the end
			defer func() {
				cliSession, err := integration.RunCli(cliPath, configPath, "delete", srcBlob)
				Expect(err).ToNot(HaveOccurred())
				Expect(cliSession.ExitCode()).To(BeZero())

				cliSession, err = integration.RunCli(cliPath, configPath, "delete", destBlob)
				Expect(err).ToNot(HaveOccurred())
				Expect(cliSession.ExitCode()).To(BeZero())
			}()

			// Upload source
			contentFile = integration.MakeContentFile("copied content")
			cliSession, err := integration.RunCli(cliPath, configPath, "put", contentFile, srcBlob)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			// Invoke copy
			cliSession, err = integration.RunCli(cliPath, configPath, "copy", srcBlob, destBlob)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			// Download destination and verify content
			tmpLocalFile, _ := os.CreateTemp("", "ali-storage-cli-copy") //nolint:errcheck
			tmpLocalFile.Close()                                         //nolint:errcheck
			defer func() { _ = os.Remove(tmpLocalFile.Name()) }()        //nolint:errcheck

			cliSession, err = integration.RunCli(cliPath, configPath, "get", destBlob, tmpLocalFile.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			gottenBytes, _ := os.ReadFile(tmpLocalFile.Name()) //nolint:errcheck
			Expect(string(gottenBytes)).To(Equal("copied content"))
		})
	})

	Describe("Invoking `exists`", func() {
		It("returns 0 for an existing blob", func() {
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
			Expect(cliSession.ExitCode()).To(Equal(0))
		})

		It("returns 3 for a not existing blob", func() {
			cliSession, err := integration.RunCli(cliPath, configPath, storageType, "exists", blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(Equal(3))
		})
	})

	Describe("Invoking `sign`", func() {
		It("returns 0 for an existing blob", func() {
			cliSession, err := integration.RunCli(cliPath, configPath, storageType, "sign", "some-blob", "get", "60s")
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			getUrl := bytes.NewBuffer(cliSession.Out.Contents()).String()
			Expect(getUrl).To(MatchRegexp("http://" + bucketName + "." + endpoint + "/some-blob"))

			cliSession, err = integration.RunCli(cliPath, configPath, storageType, "sign", "some-blob", "put", "60s")
			Expect(err).ToNot(HaveOccurred())

			putUrl := bytes.NewBuffer(cliSession.Out.Contents()).String()
			Expect(putUrl).To(MatchRegexp("http://" + bucketName + "." + endpoint + "/some-blob"))
		})

		It("returns 3 for a not existing blob", func() {
			cliSession, err := integration.RunCli(cliPath, configPath, storageType, "exists", blobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(Equal(3))
		})
	})

	Describe("Invoking `-v`", func() {
		It("returns the cli version", func() {
			configPath := integration.MakeConfigFile(&defaultConfig)
			defer func() { _ = os.Remove(configPath) }() //nolint:errcheck

			cliSession, err := integration.RunCli(cliPath, configPath, storageType, "-v")
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(Equal(0))

			consoleOutput := bytes.NewBuffer(cliSession.Out.Contents()).String()
			Expect(consoleOutput).To(ContainSubstring("version"))
		})
	})

	Describe("Invoking `list`", func() {
		It("lists all blobs with a given prefix", func() {
			prefix := integration.GenerateRandomString()
			blob1 := prefix + "/a"
			blob2 := prefix + "/b"
			otherBlob := integration.GenerateRandomString()

			defer func() {
				for _, b := range []string{blob1, blob2, otherBlob} {
					_, err := integration.RunCli(cliPath, configPath, "delete", b)
					Expect(err).ToNot(HaveOccurred())
				}
			}()

			contentFile1 := integration.MakeContentFile("list-1")
			contentFile2 := integration.MakeContentFile("list-2")
			contentFileOther := integration.MakeContentFile("list-other")
			defer func() {
				_ = os.Remove(contentFile1)     //nolint:errcheck
				_ = os.Remove(contentFile2)     //nolint:errcheck
				_ = os.Remove(contentFileOther) //nolint:errcheck
			}()

			// Put blobs
			cliSession, err := integration.RunCli(cliPath, configPath, "put", contentFile1, blob1)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, "put", contentFile2, blob2)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, "put", contentFileOther, otherBlob)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			// List with prefix
			cliSession, err = integration.RunCli(cliPath, configPath, "list", prefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			output := bytes.NewBuffer(cliSession.Out.Contents()).String()

			Expect(output).To(ContainSubstring(blob1))
			Expect(output).To(ContainSubstring(blob2))
			Expect(output).NotTo(ContainSubstring(otherBlob))
		})
	})

	Describe("Invoking `ensure-bucket-exists`", func() {
		It("is idempotent", func() {
			// first run
			s1, err := integration.RunCli(cliPath, configPath, "ensure-bucket-exists")
			Expect(err).ToNot(HaveOccurred())
			Expect(s1.ExitCode()).To(BeZero())

			// second run should also succeed (bucket already exists)
			s2, err := integration.RunCli(cliPath, configPath, "ensure-bucket-exists")
			Expect(err).ToNot(HaveOccurred())
			Expect(s2.ExitCode()).To(BeZero())
		})
	})

})
