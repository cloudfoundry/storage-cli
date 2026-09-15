package client_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/storage-cli/azurebs/client"
	"github.com/cloudfoundry/storage-cli/azurebs/config"
)

var _ = Describe("NewStorageClient", func() {
	baseConfig := func() config.AZStorageConfig {
		return config.AZStorageConfig{
			AccountName:   "account",
			AccountKey:    "Zm9vYmFy", // base64("foobar")
			ContainerName: "container",
		}
	}

	It("succeeds without any http timeout configured", func() {
		c, err := client.NewStorageClient(baseConfig())
		Expect(err).ToNot(HaveOccurred())
		Expect(c).ToNot(BeNil())
	})

	It("succeeds with http request and response header timeouts configured", func() {
		cfg := baseConfig()
		cfg.HTTPRequestTimeout = "30s"
		cfg.HTTPResponseHeaderTimeout = "10s"

		c, err := client.NewStorageClient(cfg)
		Expect(err).ToNot(HaveOccurred())
		Expect(c).ToNot(BeNil())
	})

	It("returns an error for an invalid http_request_timeout", func() {
		cfg := baseConfig()
		cfg.HTTPRequestTimeout = "30" // missing unit

		_, err := client.NewStorageClient(cfg)
		Expect(err).To(MatchError(ContainSubstring("missing duration unit")))
	})

	It("returns an error for an invalid http_response_header_timeout", func() {
		cfg := baseConfig()
		cfg.HTTPResponseHeaderTimeout = "-5s"

		_, err := client.NewStorageClient(cfg)
		Expect(err).To(MatchError(ContainSubstring("must be greater than 0")))
	})
})
