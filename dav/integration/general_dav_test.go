package integration_test

import (
	"github.com/cloudfoundry/storage-cli/dav/config"
	"github.com/cloudfoundry/storage-cli/dav/integration"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("General testing for DAV", func() {
	Context("with DAV configurations", func() {

		It("Blobstore lifecycle works with basic config", func() {
			cfg := &config.Config{
				Endpoint: davEndpoint,
				User:     davUser,
				Password: davPassword,
				TLS:      config.TLS{Cert: config.Cert{CA: davCA}},
			}
			integration.AssertLifecycleWorks(cliPath, cfg)
		})

		It("Blobstore lifecycle works with custom retry attempts", func() {
			cfg := &config.Config{
				Endpoint:      davEndpoint,
				User:          davUser,
				Password:      davPassword,
				RetryAttempts: 5,
				TLS:           config.TLS{Cert: config.Cert{CA: davCA}},
			}
			integration.AssertLifecycleWorks(cliPath, cfg)
		})

		It("Invoking `get` on a non-existent key fails", func() {
			cfg := &config.Config{
				Endpoint: davEndpoint,
				User:     davUser,
				Password: davPassword,
				TLS:      config.TLS{Cert: config.Cert{CA: davCA}},
			}
			integration.AssertGetNonexistentFails(cliPath, cfg)
		})

		It("Invoking `delete` on a non-existent key does not fail", func() {
			cfg := &config.Config{
				Endpoint: davEndpoint,
				User:     davUser,
				Password: davPassword,
				TLS:      config.TLS{Cert: config.Cert{CA: davCA}},
			}
			integration.AssertDeleteNonexistentWorks(cliPath, cfg)
		})

		It("Blobstore list and delete-recursive lifecycle works", func() {
			cfg := &config.Config{
				Endpoint: davEndpoint,
				User:     davUser,
				Password: davPassword,
				TLS:      config.TLS{Cert: config.Cert{CA: davCA}},
			}
			integration.AssertOnListDeleteLifecycle(cliPath, cfg)
		})

		It("Invoking `list` on non-existent prefix returns empty list", func() {
			cfg := &config.Config{
				Endpoint: davEndpoint,
				User:     davUser,
				Password: davPassword,
				TLS:      config.TLS{Cert: config.Cert{CA: davCA}},
			}
			integration.AssertListNonexistentPrefixReturnsEmpty(cliPath, cfg)
		})

		It("Invoking `ensure-storage-exists` is a no-op and succeeds", func() {
			cfg := &config.Config{
				Endpoint: davEndpoint,
				User:     davUser,
				Password: davPassword,
				TLS:      config.TLS{Cert: config.Cert{CA: davCA}},
			}
			integration.AssertEnsureStorageExists(cliPath, cfg)
		})
	})
})
