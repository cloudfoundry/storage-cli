package integration_test

import (
	"os"
	"testing"

	"github.com/cloudfoundry/storage-cli/alioss/config"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var cliPath string
var accessKeyID string
var accessKeySecret string
var endpoint string
var bucketName string
var defaultConfig config.AliStorageConfig

var _ = BeforeSuite(func() {
	if len(cliPath) == 0 {
		var err error
		cliPath, err = gexec.Build("github.com/cloudfoundry/storage-cli")
		Expect(err).ShouldNot(HaveOccurred())
	}

	accessKeyID = os.Getenv("ACCESS_KEY_ID")
	Expect(accessKeyID).ToNot(BeEmpty(), "ACCESS_KEY_ID must be set")

	accessKeySecret = os.Getenv("ACCESS_KEY_SECRET")
	Expect(accessKeySecret).ToNot(BeEmpty(), "ACCESS_KEY_SECRET must be set")

	endpoint = os.Getenv("ENDPOINT")
	Expect(endpoint).ToNot(BeEmpty(), "ENDPOINT must be set")

	bucketName = os.Getenv("BUCKET_NAME")
	Expect(bucketName).ToNot(BeEmpty(), "BUCKET_NAME must be set")

	defaultConfig = config.AliStorageConfig{
		AccessKeyID:     accessKeyID,
		AccessKeySecret: accessKeySecret,
		Endpoint:        endpoint,
		BucketName:      bucketName,
	}
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})
