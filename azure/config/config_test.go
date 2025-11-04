package config_test

import (
	"bytes"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/storage-cli/azure/config"
)

var _ = Describe("Config", func() {

	It("contains account-name and account-name", func() {
		configJson := []byte(`{"account_name": "foo-account-name",
								"account_key": "bar-account-key",
								"container_name": "baz-container-name"}`)
		configReader := bytes.NewReader(configJson)

		config, err := config.NewFromReader(configReader)

		Expect(err).ToNot(HaveOccurred())
		Expect(config.AccountName).To(Equal("foo-account-name"))
		Expect(config.AccountKey).To(Equal("bar-account-key"))
		Expect(config.ContainerName).To(Equal("baz-container-name"))
		Expect(config.Environment).To(Equal("AzureCloud"))
		Expect(config.StorageEndpoint()).To(Equal("blob.core.windows.net"))
	})

	It("is empty if config cannot be parsed", func() {
		configJson := []byte(`~`)
		configReader := bytes.NewReader(configJson)

		config, err := config.NewFromReader(configReader)

		Expect(err.Error()).To(Equal("invalid character '~' looking for beginning of value"))
		Expect(config.AccountName).Should(BeEmpty())
		Expect(config.AccountKey).Should(BeEmpty())
	})

	Context("when the configuration file cannot be read", func() {
		It("returns an error", func() {
			f := explodingReader{}

			_, err := config.NewFromReader(f)
			Expect(err).To(MatchError("explosion"))
		})
	})

	Context("environment", func() {
		When("environment is invalid", func() {
			It("returns an error", func() {
				configJson := []byte(`{"environment": "invalid-cloud"}`)
				configReader := bytes.NewReader(configJson)

				config, err := config.NewFromReader(configReader)

				Expect(err.Error()).To(Equal("unknown cloud environment: invalid-cloud"))
				Expect(config.Environment).Should(BeEmpty())
			})
		})

		When("environment is AzureChinaCloud", func() {
			It("sets the endpoint for china", func() {
				configJson := []byte(`{"environment": "AzureChinaCloud"}`)
				configReader := bytes.NewReader(configJson)

				config, err := config.NewFromReader(configReader)

				Expect(err).ToNot(HaveOccurred())
				Expect(config.Environment).To(Equal("AzureChinaCloud"))
				Expect(config.StorageEndpoint()).To(Equal("blob.core.chinacloudapi.cn"))
			})
		})

		When("environment is AzureUSGovernment", func() {
			It("sets the endpoint for usgovernment", func() {
				configJson := []byte(`{"environment": "AzureUSGovernment"}`)
				configReader := bytes.NewReader(configJson)

				config, err := config.NewFromReader(configReader)

				Expect(err).ToNot(HaveOccurred())
				Expect(config.Environment).To(Equal("AzureUSGovernment"))
				Expect(config.StorageEndpoint()).To(Equal("blob.core.usgovcloudapi.net"))
			})
		})
	})
})

type explodingReader struct{}

func (e explodingReader) Read([]byte) (int, error) {
	return 0, errors.New("explosion")
}
