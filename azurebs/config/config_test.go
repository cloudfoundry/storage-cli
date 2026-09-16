package config_test

import (
	"bytes"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/storage-cli/azurebs/config"
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

	Context("http timeouts", func() {
		DescribeTable("HTTPRequestTimeoutValue",
			func(value string, expected time.Duration, errSubstring string) {
				c := config.AZStorageConfig{HTTPRequestTimeout: value}
				result, err := c.HTTPRequestTimeoutValue()
				if errSubstring == "" {
					Expect(err).ToNot(HaveOccurred())
					Expect(result).To(Equal(expected))
				} else {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(errSubstring))
				}
			},
			Entry("empty means unset", "", time.Duration(0), ""),
			Entry("valid duration", "30s", 30*time.Second, ""),
			Entry("valid minutes", "2m", 2*time.Minute, ""),
			Entry("bare number is rejected", "30", time.Duration(0), "missing duration unit"),
			Entry("garbage is rejected", "abc", time.Duration(0), "invalid http_request_timeout"),
			Entry("zero is rejected", "0s", time.Duration(0), "must be greater than 0"),
			Entry("negative is rejected", "-5s", time.Duration(0), "must be greater than 0"),
		)

		DescribeTable("HTTPResponseHeaderTimeoutValue",
			func(value string, expected time.Duration, errSubstring string) {
				c := config.AZStorageConfig{HTTPResponseHeaderTimeout: value}
				result, err := c.HTTPResponseHeaderTimeoutValue()
				if errSubstring == "" {
					Expect(err).ToNot(HaveOccurred())
					Expect(result).To(Equal(expected))
				} else {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(errSubstring))
				}
			},
			Entry("empty means unset", "", time.Duration(0), ""),
			Entry("valid duration", "10s", 10*time.Second, ""),
			Entry("bare number is rejected", "10", time.Duration(0), "missing duration unit"),
			Entry("garbage is rejected", "abc", time.Duration(0), "invalid http_response_header_timeout"),
			Entry("zero is rejected", "0s", time.Duration(0), "must be greater than 0"),
			Entry("negative is rejected", "-5s", time.Duration(0), "must be greater than 0"),
		)

		It("NewFromReader accepts valid timeout strings", func() {
			configJson := []byte(`{"http_request_timeout": "30s", "http_response_header_timeout": "10s"}`)
			c, err := config.NewFromReader(bytes.NewReader(configJson))
			Expect(err).ToNot(HaveOccurred())
			Expect(c.HTTPRequestTimeout).To(Equal("30s"))
			Expect(c.HTTPResponseHeaderTimeout).To(Equal("10s"))
		})

		It("NewFromReader rejects an invalid http_request_timeout", func() {
			configJson := []byte(`{"http_request_timeout": "30"}`)
			_, err := config.NewFromReader(bytes.NewReader(configJson))
			Expect(err).To(MatchError(ContainSubstring("missing duration unit")))
		})

		It("NewFromReader rejects an invalid http_response_header_timeout", func() {
			configJson := []byte(`{"http_response_header_timeout": "-1s"}`)
			_, err := config.NewFromReader(bytes.NewReader(configJson))
			Expect(err).To(MatchError(ContainSubstring("must be greater than 0")))
		})
	})
})

type explodingReader struct{}

func (e explodingReader) Read([]byte) (int, error) {
	return 0, errors.New("explosion")
}
