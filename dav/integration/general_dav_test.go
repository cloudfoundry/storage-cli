package integration_test

import (
	"os"

	"github.com/cloudfoundry/storage-cli/dav/config"
	"github.com/cloudfoundry/storage-cli/dav/integration"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("General testing for DAV", func() {
	Context("with DAV configurations", func() {
		endpoint := os.Getenv("DAV_ENDPOINT")
		user := os.Getenv("DAV_USER")
		password := os.Getenv("DAV_PASSWORD")
		ca := os.Getenv("DAV_CA_CERT")
		secret := os.Getenv("DAV_SECRET")

		BeforeEach(func() {
			Expect(endpoint).ToNot(BeEmpty(), "DAV_ENDPOINT must be set")
			Expect(user).ToNot(BeEmpty(), "DAV_USER must be set")
			Expect(password).ToNot(BeEmpty(), "DAV_PASSWORD must be set")
		})

		configurations := []TableEntry{
			Entry("with basic config", &config.Config{
				Endpoint: endpoint,
				User:     user,
				Password: password,
				TLS: config.TLS{
					Cert: config.Cert{
						CA: ca,
					},
				},
			}),
			Entry("with custom retry attempts", &config.Config{
				Endpoint:      endpoint,
				User:          user,
				Password:      password,
				RetryAttempts: 5,
				TLS: config.TLS{
					Cert: config.Cert{
						CA: ca,
					},
				},
			}),
		}

		DescribeTable("Blobstore lifecycle works",
			func(cfg *config.Config) { integration.AssertLifecycleWorks(cliPath, cfg) },
			configurations,
		)

		DescribeTable("Invoking `get` on a non-existent-key fails",
			func(cfg *config.Config) { integration.AssertGetNonexistentFails(cliPath, cfg) },
			configurations,
		)

		DescribeTable("Invoking `delete` on a non-existent-key does not fail",
			func(cfg *config.Config) { integration.AssertDeleteNonexistentWorks(cliPath, cfg) },
			configurations,
		)

		DescribeTable("Blobstore list and delete-recursive lifecycle works",
			func(cfg *config.Config) { integration.AssertOnListDeleteLifecycle(cliPath, cfg) },
			configurations,
		)

		DescribeTable("Invoking `ensure-storage-exists` works",
			func(cfg *config.Config) {
				Skip("ensure-storage-exists not applicable for WebDAV - root always exists")
				integration.AssertEnsureStorageExists(cliPath, cfg)
			},
			configurations,
		)

		Context("with signed URL support", func() {
			BeforeEach(func() {
				if secret == "" {
					Skip("DAV_SECRET not set - skipping signed URL tests")
				}
			})

			configurationsWithSecret := []TableEntry{
				Entry("with secret for signed URLs", &config.Config{
					Endpoint: endpoint,
					User:     user,
					Password: password,
					Secret:   secret,
					TLS: config.TLS{
						Cert: config.Cert{
							CA: ca,
						},
					},
				}),
			}

			DescribeTable("Invoking `sign` returns a signed URL",
				func(cfg *config.Config) { integration.AssertOnSignedURLs(cliPath, cfg) },
				configurationsWithSecret,
			)
		})
	})
})
