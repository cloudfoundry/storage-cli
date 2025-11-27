package integration_test

import (
	"testing"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var cliPath string
var largeContent string //nolint:unused

var _ = BeforeSuite(func() {
	if len(cliPath) == 0 {
		var err error
		cliPath, err = gexec.Build("github.com/cloudfoundry/storage-cli")
		Expect(err).ShouldNot(HaveOccurred())
	}
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})
