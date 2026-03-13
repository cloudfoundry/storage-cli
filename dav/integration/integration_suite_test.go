package integration_test

import (
	"io"
	"log"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DAV Integration Suite")
}

var cliPath string

var _ = BeforeSuite(func() {
	// Suppress logs during integration tests
	log.SetOutput(io.Discard)

	var err error
	cliPath, err = gexec.Build("github.com/cloudfoundry/storage-cli")
	Expect(err).ShouldNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})
