package integration_test

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"time"

	"os"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/cloudfoundry/storage-cli/alioss/config"
	"github.com/cloudfoundry/storage-cli/alioss/integration"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
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
			Expect(cliSession.Err).Should(gbytes.Say(`"msg":"Object exists in OSS bucket"`))
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

			contentFile1 := integration.MakeContentFile("content-1")
			contentFile2 := integration.MakeContentFile("content-2")
			contentFileOther := integration.MakeContentFile("other-content")
			defer func() {
				_ = os.Remove(contentFile1)     //nolint:errcheck
				_ = os.Remove(contentFile2)     //nolint:errcheck
				_ = os.Remove(contentFileOther) //nolint:errcheck

				for _, b := range []string{blob1, blob2, otherBlob} {
					cliSession, err := integration.RunCli(cliPath, configPath, storageType, "delete", b)
					if err == nil && (cliSession.ExitCode() == 0 || cliSession.ExitCode() == 3) {
						continue
					}
				}
			}()

			cliSession, err := integration.RunCli(cliPath, configPath, storageType, "put", contentFile1, blob1)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, storageType, "put", contentFile2, blob2)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, storageType, "put", contentFileOther, otherBlob)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, storageType, "delete-recursive", prefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, storageType, "exists", blob1)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(Equal(3))

			cliSession, err = integration.RunCli(cliPath, configPath, storageType, "exists", blob2)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(Equal(3))

			cliSession, err = integration.RunCli(cliPath, configPath, storageType, "exists", otherBlob)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(Equal(0))
		})
	})

	Describe("Invoking `copy`", func() {
		It("copies the contents from one object to another", func() {
			srcBlob := blobName + "-src"
			destBlob := blobName + "-dest"

			defer func() {
				for _, b := range []string{srcBlob, destBlob} {
					cliSession, err := integration.RunCli(cliPath, configPath, storageType, "delete", b)
					if err != nil {
						GinkgoWriter.Printf("cleanup: error deleting %s: %v\n", b, err)
						continue
					}
					if cliSession.ExitCode() != 0 && cliSession.ExitCode() != 3 {
						GinkgoWriter.Printf("cleanup: delete %s exited with code %d\n", b, cliSession.ExitCode())
					}
				}
			}()

			contentFile = integration.MakeContentFile("copied content")
			cliSession, err := integration.RunCli(cliPath, configPath, storageType, "put", contentFile, srcBlob)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, storageType, "copy", srcBlob, destBlob)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			tmpLocalFile, _ := os.CreateTemp("", "ali-storage-cli-copy") //nolint:errcheck
			tmpLocalFile.Close()                                         //nolint:errcheck
			defer func() { _ = os.Remove(tmpLocalFile.Name()) }()        //nolint:errcheck

			cliSession, err = integration.RunCli(cliPath, configPath, storageType, "get", destBlob, tmpLocalFile.Name())
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
					_, err := integration.RunCli(cliPath, configPath, storageType, "delete", b)
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

			cliSession, err := integration.RunCli(cliPath, configPath, storageType, "put", contentFile1, blob1)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, storageType, "put", contentFile2, blob2)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, storageType, "put", contentFileOther, otherBlob)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			cliSession, err = integration.RunCli(cliPath, configPath, storageType, "list", prefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			output := bytes.NewBuffer(cliSession.Out.Contents()).String()

			Expect(output).To(ContainSubstring(blob1))
			Expect(output).To(ContainSubstring(blob2))
			Expect(output).NotTo(ContainSubstring(otherBlob))
		})

		It("lists all blobs across multiple pages", func() {
			prefix := integration.GenerateRandomString()
			const totalObjects = 120

			var blobNames []string
			var contentFiles []string

			for i := 0; i < totalObjects; i++ {
				blobName := fmt.Sprintf("%s/%03d", prefix, i)
				blobNames = append(blobNames, blobName)

				contentFile := integration.MakeContentFile(fmt.Sprintf("content-%d", i))
				contentFiles = append(contentFiles, contentFile)

				cliSession, err := integration.RunCli(cliPath, configPath, storageType, "put", contentFile, blobName)
				Expect(err).ToNot(HaveOccurred())
				Expect(cliSession.ExitCode()).To(BeZero())
			}

			defer func() {
				for _, f := range contentFiles {
					_ = os.Remove(f) //nolint:errcheck
				}

				for _, b := range blobNames {
					cliSession, err := integration.RunCli(cliPath, configPath, storageType, "delete", b)
					if err == nil && (cliSession.ExitCode() == 0 || cliSession.ExitCode() == 3) {
						continue
					}
				}
			}()

			cliSession, err := integration.RunCli(cliPath, configPath, storageType, "list", prefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(cliSession.ExitCode()).To(BeZero())

			output := bytes.NewBuffer(cliSession.Out.Contents()).String()

			for _, b := range blobNames {
				Expect(output).To(ContainSubstring(b))
			}
		})
	})

	const maxBucketLen = 63
	const suffixLen = 8

	Describe("Invoking `ensure-bucket-exists`", func() {
		It("creates a bucket that can be observed via the OSS API", func() {

			base := bucketName

			maxBaseLen := maxBucketLen - 1 - suffixLen
			if maxBaseLen < 1 {
				maxBaseLen = maxBucketLen - 1
			}
			if len(base) > maxBaseLen {
				base = base[:maxBaseLen]
			}

			rawSuffix := integration.GenerateRandomString()
			safe := strings.ToLower(rawSuffix)

			re := regexp.MustCompile(`[^a-z0-9]`)
			safe = re.ReplaceAllString(safe, "")
			if len(safe) < suffixLen {
				safe = safe + "0123456789abcdef"
			}
			safe = safe[:suffixLen]

			newBucketName := fmt.Sprintf("%s-%s", base, safe)

			cfg := defaultConfig
			cfg.BucketName = newBucketName

			configPath = integration.MakeConfigFile(&cfg)
			defer func() { _ = os.Remove(configPath) }() //nolint:errcheck

			ossClient, err := oss.New(cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret)
			Expect(err).ToNot(HaveOccurred())

			defer func() {
				if err := ossClient.DeleteBucket(newBucketName); err != nil {
					if svcErr, ok := err.(oss.ServiceError); ok && svcErr.StatusCode == 404 {
						return
					}
					fmt.Fprintf(GinkgoWriter, "cleanup: failed to delete bucket %s: %v\n", newBucketName, err) //nolint:errcheck
				}
			}()

			s1, err := integration.RunCli(cliPath, configPath, storageType, "ensure-storage-exists")
			Expect(err).ToNot(HaveOccurred())
			Expect(s1.ExitCode()).To(BeZero())

			Eventually(func(g Gomega) bool {
				exists, existsErr := ossClient.IsBucketExist(newBucketName)
				g.Expect(existsErr).ToNot(HaveOccurred())
				return exists
			}, 30*time.Second, 1*time.Second).Should(BeTrue())
		})

		It("is idempotent", func() {
			s1, err := integration.RunCli(cliPath, configPath, storageType, "ensure-storage-exists")
			Expect(err).ToNot(HaveOccurred())
			Expect(s1.ExitCode()).To(BeZero())

			s2, err := integration.RunCli(cliPath, configPath, storageType, "ensure-storage-exists")
			Expect(err).ToNot(HaveOccurred())
			Expect(s2.ExitCode()).To(BeZero())
		})
	})
})
