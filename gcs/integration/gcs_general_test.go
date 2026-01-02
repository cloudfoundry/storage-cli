/*
 * Copyright 2017 Google Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package integration

import (
	context "context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/cloudfoundry/storage-cli/gcs/client"
	"github.com/cloudfoundry/storage-cli/gcs/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Integration", func() {
	storageType := "gcs"

	Context("general (Default Applicaton Credentials) configuration", func() {
		var env AssertContext
		BeforeEach(func() {
			env = NewAssertContext(AsDefaultCredentials)
		})
		AfterEach(func() {
			env.Cleanup()
		})

		configurations := getBaseConfigs()

		DescribeTable("Blobstore lifecycle works",
			func(config *config.GCSCli) {
				env.AddConfig(config)
				AssertLifecycleWorks(gcsCLIPath, env)
			},
			configurations)

		DescribeTable("Delete silently ignores that the file doesn't exist",
			func(config *config.GCSCli) {
				env.AddConfig(config)

				session, err := RunGCSCLI(gcsCLIPath, env.ConfigPath, storageType, "delete", env.GCSFileName)
				Expect(err).ToNot(HaveOccurred())
				Expect(session.ExitCode()).To(BeZero())
			},
			configurations)

		Context("with a regional bucket", func() {
			var cfg *config.GCSCli
			BeforeEach(func() {
				cfg = getRegionalConfig()
				env.AddConfig(cfg)
			})
			AfterEach(func() {
				env.Cleanup()
			})

			It("can perform large file upload (multi-part)", func() {
				if os.Getenv(NoLongEnv) != "" {
					Skip(fmt.Sprintf(NoLongMsg, NoLongEnv))
				}

				const twoGB = 1024 * 1024 * 1024 * 2

				largeFile := MakeContentFile(GenerateRandomString(twoGB))
				defer os.Remove(largeFile) //nolint:errcheck

				blobstoreClient, err := client.New(env.ctx, env.Config)
				Expect(err).ToNot(HaveOccurred())

				err = blobstoreClient.Put(largeFile, env.GCSFileName)
				Expect(err).ToNot(HaveOccurred())

				blobstoreClient.Delete(env.GCSFileName) //nolint:errcheck
				Expect(err).ToNot(HaveOccurred())
			})
		})

		DescribeTable("Invalid Put should fail",
			func(config *config.GCSCli) {
				env.AddConfig(config)

				blobstoreClient, err := client.New(env.ctx, env.Config)
				Expect(err).ToNot(HaveOccurred())

				// create pipe to be open but not seekable
				pipePath := fmt.Sprintf("/tmp/%s", GenerateRandomString(10))
				err = syscall.Mkfifo(pipePath, 0666)
				Expect(err).ToNot(HaveOccurred())

				go func() {
					// This will block until the main test opens the pipe for reading.
					writer, _ := os.OpenFile(pipePath, os.O_WRONLY, 0) //nolint:errcheck
					if writer != nil {
						writer.Close() //nolint:errcheck
					}
				}()

				err = blobstoreClient.Put(pipePath, env.GCSFileName)
				Expect(err).To(MatchError(ContainSubstring("illegal seek")))
			},
			configurations)

		DescribeTable("Invalid Get should fail",
			func(config *config.GCSCli) {
				env.AddConfig(config)

				session, err := RunGCSCLI(gcsCLIPath, env.ConfigPath, storageType, "get", env.GCSFileName, "/dev/null")
				Expect(err).ToNot(HaveOccurred())
				Expect(session.ExitCode()).ToNot(BeZero())
				Expect(session.Err.Contents()).To(ContainSubstring("object doesn't exist"))
			},
			configurations)

		DescribeTable("copying will create same content with different name", func(config *config.GCSCli) {
			env.AddConfig(config)
			AssertCopyLifecycle(gcsCLIPath, env)
		}, configurations)

		Context("when bucket is not exist", func() {
			DescribeTable("ensure storage exist will create a new bucket", func(cfg *config.GCSCli) {
				// create new a newCfg instead of modifying shared cfg accross all tests
				newCfg := &config.GCSCli{
					BucketName: strings.ToLower(GenerateRandomString()),
				}

				env.AddConfig(newCfg)

				session, err := RunGCSCLI(gcsCLIPath, env.ConfigPath, storageType, "ensure-storage-exists")
				Expect(err).ToNot(HaveOccurred())
				Expect(session.ExitCode()).To(BeZero())

				deleteBucket(context.Background(), newCfg.BucketName, env.ConfigPath)
			}, configurations)
		})

		Context("when bucket exists", func() {
			DescribeTable("ensure storage exist will not create a new bucket", func(cfg *config.GCSCli) {
				env.AddConfig(cfg)
				session, err := RunGCSCLI(gcsCLIPath, env.ConfigPath, storageType, "ensure-storage-exists")
				Expect(err).ToNot(HaveOccurred())
				Expect(session.ExitCode()).To(BeZero())

			}, configurations)
		})

		Context("when working with multiple objects", func() {
			DescribeTable("recursive deleting will delete only the objects that have same prefix", func(config *config.GCSCli) {
				env.AddConfig(config)
				AssertDeleteRecursiveWithPrefixLifecycle(gcsCLIPath, env)
			},
				configurations)
			DescribeTable("list will output only the objects that have same prefix", func(config *config.GCSCli) {
				env.AddConfig(config)
				AssertListMultipleWithPrefixLifecycle(gcsCLIPath, env)
			}, configurations)

		})
	})
})
