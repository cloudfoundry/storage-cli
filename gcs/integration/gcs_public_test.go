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
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/storage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/storage-cli/gcs/client"
	"github.com/cloudfoundry/storage-cli/gcs/config"
)

var _ = Describe("GCS Public Bucket", func() {
	storageType := "gcs"
	Context("with read-only configuration", func() {
		var (
			setupEnv  AssertContext
			publicEnv AssertContext
			cfg       *config.GCSCli
		)

		BeforeEach(func() {
			cfg = getPublicConfig()

			setupEnv = NewAssertContext(AsDefaultCredentials)
			setupEnv.AddConfig(cfg)
			Expect(setupEnv.Config.CredentialsSource).ToNot(Equal(config.NoneCredentialsSource), "Cannot use 'none' credentials to setup")

			publicEnv = setupEnv.Clone(AsReadOnlyCredentials)
		})
		AfterEach(func() {
			setupEnv.Cleanup()
			publicEnv.Cleanup()
		})

		Describe("with a public file", func() {
			BeforeEach(func() {
				// Place a file in the bucket
				RunGCSCLI(gcsCLIPath, setupEnv.ConfigPath, storageType, "put", setupEnv.ContentFile, setupEnv.GCSFileName) //nolint:errcheck

				// Make the file public
				rwClient, err := newSDK(setupEnv.ctx, *setupEnv.Config)
				Expect(err).ToNot(HaveOccurred())
				bucket := rwClient.Bucket(setupEnv.Config.BucketName)
				obj := bucket.Object(setupEnv.GCSFileName)
				Expect(obj.ACL().Set(context.Background(), storage.AllUsers, storage.RoleReader)).To(Succeed())
			})
			AfterEach(func() {
				RunGCSCLI(gcsCLIPath, setupEnv.ConfigPath, storageType, "delete", setupEnv.GCSFileName) //nolint:errcheck
				publicEnv.Cleanup()
			})

			It("can check if it exists", func() {
				session, err := RunGCSCLI(gcsCLIPath, publicEnv.ConfigPath, storageType, "exists", setupEnv.GCSFileName)
				Expect(err).ToNot(HaveOccurred())
				Expect(session.ExitCode()).To(BeZero())
			})

			It("can get", func() {
				tmpLocalFileName := "gcscli-download"
				defer os.Remove(tmpLocalFileName) //nolint:errcheck

				session, err := RunGCSCLI(gcsCLIPath, publicEnv.ConfigPath, storageType, "get", setupEnv.GCSFileName, tmpLocalFileName)
				Expect(err).ToNot(HaveOccurred())
				Expect(session.ExitCode()).To(BeZero(), fmt.Sprintf("unexpected '%s'", session.Err.Contents()))

				gottenBytes, err := os.ReadFile(tmpLocalFileName)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(gottenBytes)).To(Equal(setupEnv.ExpectedString))
			})
		})

		It("fails to get a missing file", func() {
			session, err := RunGCSCLI(gcsCLIPath, publicEnv.ConfigPath, storageType, "get", setupEnv.GCSFileName, "/dev/null")
			Expect(err).ToNot(HaveOccurred())
			Expect(session.ExitCode()).ToNot(BeZero())
			Expect(string(session.Err.Contents())).To(ContainSubstring("object doesn't exist"))
		})

		It("fails to put", func() {
			session, err := RunGCSCLI(gcsCLIPath, publicEnv.ConfigPath, storageType, "put", publicEnv.ContentFile, publicEnv.GCSFileName)
			Expect(err).ToNot(HaveOccurred())
			Expect(session.ExitCode()).ToNot(BeZero())
			Expect(string(session.Err.Contents())).To(ContainSubstring(client.ErrInvalidROWriteOperation.Error()))
		})

		It("fails to delete", func() {
			session, err := RunGCSCLI(gcsCLIPath, publicEnv.ConfigPath, storageType, "delete", publicEnv.GCSFileName)
			Expect(err).ToNot(HaveOccurred())
			Expect(session.ExitCode()).ToNot(BeZero())
			Expect(string(session.Err.Contents())).To(ContainSubstring(client.ErrInvalidROWriteOperation.Error()))
		})

		It("fails to list", func() {
			session, err := RunGCSCLI(gcsCLIPath, publicEnv.ConfigPath, storageType, "list", "prefix")
			Expect(err).ToNot(HaveOccurred())
			Expect(session.ExitCode()).ToNot(BeZero())
			Expect(string(session.Err.Contents())).To(ContainSubstring(client.ErrInvalidROWriteOperation.Error()))
		})

		It("fails to get properties", func() {
			session, err := RunGCSCLI(gcsCLIPath, publicEnv.ConfigPath, storageType, "properties", publicEnv.GCSFileName)
			Expect(err).ToNot(HaveOccurred())
			Expect(session.ExitCode()).ToNot(BeZero())
			Expect(string(session.Err.Contents())).To(ContainSubstring(client.ErrInvalidROWriteOperation.Error()))
		})

		It("fails to delete-recursive", func() {
			session, err := RunGCSCLI(gcsCLIPath, publicEnv.ConfigPath, storageType, "delete-recursive", "prefix")
			Expect(err).ToNot(HaveOccurred())
			Expect(session.ExitCode()).ToNot(BeZero())
			Expect(string(session.Err.Contents())).To(ContainSubstring(client.ErrInvalidROWriteOperation.Error()))
		})

		It("fails to copy", func() {
			session, err := RunGCSCLI(gcsCLIPath, publicEnv.ConfigPath, storageType, "copy", publicEnv.GCSFileName, "destination-object")
			Expect(err).ToNot(HaveOccurred())
			Expect(session.ExitCode()).ToNot(BeZero())
			Expect(string(session.Err.Contents())).To(ContainSubstring(client.ErrInvalidROWriteOperation.Error()))
		})

		It("fails to create bucket", func() {
			session, err := RunGCSCLI(gcsCLIPath, publicEnv.ConfigPath, storageType, "ensure-storage-exists")
			Expect(err).ToNot(HaveOccurred())
			Expect(session.ExitCode()).ToNot(BeZero())
			Expect(string(session.Err.Contents())).To(ContainSubstring(client.ErrInvalidROWriteOperation.Error()))
		})
	})
})
