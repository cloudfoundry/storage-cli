package config_test

import (
	"bytes"
	"errors"

	"github.com/cloudfoundry/storage-cli/ali/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {

	It("contains mandatory properties", func() {
		configJson := []byte(`{"access_key_id": "foo_access_key_id",
								"access_key_secret": "foo_access_key_secret",
                                "endpoint": "foo_endpoint",
								"bucket_name": "foo_bucket_name"}`)
		configReader := bytes.NewReader(configJson)

		config, err := config.NewFromReader(configReader)

		Expect(err).ToNot(HaveOccurred())
		Expect(config.AccessKeyID).To(Equal("foo_access_key_id"))
		Expect(config.AccessKeySecret).To(Equal("foo_access_key_secret"))
		Expect(config.Endpoint).To(Equal("foo_endpoint"))
		Expect(config.BucketName).To(Equal("foo_bucket_name"))
	})

	It("is empty if config cannot be parsed", func() {
		configJson := []byte(`~`)
		configReader := bytes.NewReader(configJson)

		config, err := config.NewFromReader(configReader)

		Expect(err.Error()).To(Equal("invalid character '~' looking for beginning of value"))
		Expect(config.AccessKeyID).Should(BeEmpty())
		Expect(config.AccessKeySecret).Should(BeEmpty())
		Expect(config.Endpoint).Should(BeEmpty())
		Expect(config.BucketName).Should(BeEmpty())
	})

	Context("when the configuration file cannot be read", func() {
		It("returns an error", func() {
			f := explodingReader{}

			_, err := config.NewFromReader(f)
			Expect(err).To(MatchError("explosion"))
		})
	})

})

type explodingReader struct{}

func (e explodingReader) Read([]byte) (int, error) {
	return 0, errors.New("explosion")
}
