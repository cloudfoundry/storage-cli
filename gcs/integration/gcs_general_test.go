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
	"fmt"
	"os"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/storage-cli/gcs/client"
	"github.com/cloudfoundry/storage-cli/gcs/config"
)

var _ = Describe("Integration", func() {
	var storageType string = "gcs"
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
					writer, _ := os.OpenFile(pipePath, os.O_WRONLY, 0)
					if writer != nil {
						writer.Close()
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
	})
})
