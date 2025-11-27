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
	"encoding/base64"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/storage-cli/gcs/config"
)

var _ = Describe("Integration", func() {
	var storageType string = "gcs"
	Context("static credentials configuration with a regional bucket", func() {
		var (
			ctx AssertContext
			cfg *config.GCSCli
		)
		BeforeEach(func() {
			cfg = getRegionalConfig()
			ctx = NewAssertContext(AsStaticCredentials)
			ctx.AddConfig(cfg)
		})
		AfterEach(func() {
			ctx.Cleanup()
		})

		It("can perform blobstore lifecycle", func() {
			AssertLifecycleWorks(gcsCLIPath, ctx)
		})

		It("validates the action is valid", func() {
			session, err := RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "sign", ctx.GCSFileName, "not-valid", "1h")
			Expect(err).NotTo(HaveOccurred())
			Expect(session.ExitCode()).ToNot(Equal(0))
		})

		It("can generate a signed url for a given object and action", func() {
			session, err := RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "sign", ctx.GCSFileName, "put", "1h")

			Expect(err).ToNot(HaveOccurred())
			Expect(session.ExitCode()).To(Equal(0))
			url := string(session.Out.Contents())
			Expect(url).To(MatchRegexp("https://"))

			body := strings.NewReader(`bar`)
			req, err := http.NewRequest("PUT", url, body)
			Expect(err).ToNot(HaveOccurred())

			resp, err := http.DefaultClient.Do(req)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))
			defer resp.Body.Close() //nolint:errcheck
		})

		Context("encryption key is set", func() {
			var key string

			BeforeEach(func() {
				// even though the config file holds a base64 encodeded key,
				// config at this point needs it to be decoded
				// openssl rand 32 | base64
				key = "PG+tLm6vjBZXpU6S5Oiv/rpkA4KLioQRTXU3AfVzyHc="
				data, err := base64.StdEncoding.DecodeString(key)
				Expect(err).NotTo(HaveOccurred())

				newcfg := ctx.Config
				newcfg.EncryptionKey = data
				ctx.AddConfig(newcfg)
			})

			It("can generate a signed url for encrypting later", func() {
				// echo -n key | base64 -D | shasum -a 256 | cut -f1 -d' ' | tr -d '\n' | xxd -r -p | base64
				hash := "bQOB9Mp048LRjpIoKm2njgQgiC3FRO2gn/+x6Vlfa4E="

				session, err := RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "sign", ctx.GCSFileName, "put", "1h")
				Expect(err).ToNot(HaveOccurred())
				signedPutUrl := string(session.Out.Contents())
				Expect(signedPutUrl).ToNot(BeNil())

				session, err = RunGCSCLI(gcsCLIPath, ctx.ConfigPath, storageType, "sign", ctx.GCSFileName, "get", "1h")
				Expect(err).ToNot(HaveOccurred())
				signedGetUrl := string(session.Out.Contents())
				Expect(signedGetUrl).ToNot(BeNil())

				stuff := strings.NewReader(`stuff`)                      //nolint:errcheck
				putReq, _ := http.NewRequest("PUT", signedPutUrl, stuff) //nolint:errcheck
				getReq, _ := http.NewRequest("GET", signedGetUrl, nil)   //nolint:errcheck

				headers := map[string][]string{
					"x-goog-encryption-algorithm":  []string{"AES256"},
					"x-goog-encryption-key":        []string{key},
					"x-goog-encryption-key-sha256": []string{hash},
				}

				putReq.Header = headers
				getReq.Header = headers

				resp, err := http.DefaultClient.Do(putReq)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))
				resp.Body.Close() //nolint:errcheck

				resp, err = http.DefaultClient.Do(getReq)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))
				resp.Body.Close() //nolint:errcheck
			})
		})
	})
})
