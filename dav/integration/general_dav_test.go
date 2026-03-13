package integration_test

import (
	"os"

	"github.com/cloudfoundry/storage-cli/dav/config"
	"github.com/cloudfoundry/storage-cli/dav/integration"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("General testing for DAV", func() {
	Context("with DAV configurations", func() {
		var (
			endpoint string
			user     string
			password string
			ca       string
			secret   string
		)

		BeforeEach(func() {
			endpoint = os.Getenv("DAV_ENDPOINT")
			user = os.Getenv("DAV_USER")
			password = os.Getenv("DAV_PASSWORD")
			ca = os.Getenv("DAV_CA_CERT")
			secret = os.Getenv("DAV_SECRET")

			// Skip tests if environment variables are not set
			if endpoint == "" || user == "" || password == "" {
				Skip("Skipping DAV integration tests - environment variables not set (DAV_ENDPOINT, DAV_USER, DAV_PASSWORD required)")
			}
		})

		It("Blobstore lifecycle works with basic config", func() {
			cfg := &config.Config{
				Endpoint: endpoint,
				User:     user,
				Password: password,
				TLS: config.TLS{
					Cert: config.Cert{
						CA: ca,
					},
				},
			}
			integration.AssertLifecycleWorks(cliPath, cfg)
		})

		It("Blobstore lifecycle works with custom retry attempts", func() {
			cfg := &config.Config{
				Endpoint:      endpoint,
				User:          user,
				Password:      password,
				RetryAttempts: 5,
				TLS: config.TLS{
					Cert: config.Cert{
						CA: ca,
					},
				},
			}
			integration.AssertLifecycleWorks(cliPath, cfg)
		})

		It("Invoking `get` on a non-existent-key fails with basic config", func() {
			cfg := &config.Config{
				Endpoint: endpoint,
				User:     user,
				Password: password,
				TLS: config.TLS{
					Cert: config.Cert{
						CA: ca,
					},
				},
			}
			integration.AssertGetNonexistentFails(cliPath, cfg)
		})

		It("Invoking `get` on a non-existent-key fails with custom retry attempts", func() {
			cfg := &config.Config{
				Endpoint:      endpoint,
				User:          user,
				Password:      password,
				RetryAttempts: 5,
				TLS: config.TLS{
					Cert: config.Cert{
						CA: ca,
					},
				},
			}
			integration.AssertGetNonexistentFails(cliPath, cfg)
		})

		It("Invoking `delete` on a non-existent-key does not fail with basic config", func() {
			cfg := &config.Config{
				Endpoint: endpoint,
				User:     user,
				Password: password,
				TLS: config.TLS{
					Cert: config.Cert{
						CA: ca,
					},
				},
			}
			integration.AssertDeleteNonexistentWorks(cliPath, cfg)
		})

		It("Invoking `delete` on a non-existent-key does not fail with custom retry attempts", func() {
			cfg := &config.Config{
				Endpoint:      endpoint,
				User:          user,
				Password:      password,
				RetryAttempts: 5,
				TLS: config.TLS{
					Cert: config.Cert{
						CA: ca,
					},
				},
			}
			integration.AssertDeleteNonexistentWorks(cliPath, cfg)
		})

		It("Blobstore list and delete-recursive lifecycle works with basic config", func() {
			cfg := &config.Config{
				Endpoint: endpoint,
				User:     user,
				Password: password,
				TLS: config.TLS{
					Cert: config.Cert{
						CA: ca,
					},
				},
			}
			integration.AssertOnListDeleteLifecycle(cliPath, cfg)
		})

		It("Blobstore list and delete-recursive lifecycle works with custom retry attempts", func() {
			cfg := &config.Config{
				Endpoint:      endpoint,
				User:          user,
				Password:      password,
				RetryAttempts: 5,
				TLS: config.TLS{
					Cert: config.Cert{
						CA: ca,
					},
				},
			}
			integration.AssertOnListDeleteLifecycle(cliPath, cfg)
		})

		It("Invoking `ensure-storage-exists` works with basic config", func() {
			Skip("ensure-storage-exists not applicable for WebDAV - root always exists")
			cfg := &config.Config{
				Endpoint: endpoint,
				User:     user,
				Password: password,
				TLS: config.TLS{
					Cert: config.Cert{
						CA: ca,
					},
				},
			}
			integration.AssertEnsureStorageExists(cliPath, cfg)
		})

		It("Invoking `ensure-storage-exists` works with custom retry attempts", func() {
			Skip("ensure-storage-exists not applicable for WebDAV - root always exists")
			cfg := &config.Config{
				Endpoint:      endpoint,
				User:          user,
				Password:      password,
				RetryAttempts: 5,
				TLS: config.TLS{
					Cert: config.Cert{
						CA: ca,
					},
				},
			}
			integration.AssertEnsureStorageExists(cliPath, cfg)
		})

		Context("with signed URL support", func() {
			BeforeEach(func() {
				if secret == "" {
					Skip("DAV_SECRET not set - skipping signed URL tests")
				}
			})

			It("Invoking `sign` returns a signed URL with secret for signed URLs", func() {
				cfg := &config.Config{
					Endpoint: endpoint,
					User:     user,
					Password: password,
					Secret:   secret,
					TLS: config.TLS{
						Cert: config.Cert{
							CA: ca,
						},
					},
				}
				integration.AssertOnSignedURLs(cliPath, cfg)
			})
		})
	})
})
