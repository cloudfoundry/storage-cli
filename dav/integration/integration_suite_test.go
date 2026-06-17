package integration_test

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DAV Integration Suite")
}

var (
	cliPath       string
	containerName string
	davEndpoint   string
	davCA         string
)

const (
	davUser     = "testuser"
	davPassword = "testpassword"
	// davDirectoryKey is the bucket name embedded in the endpoint path under /admin/.
	// nginx config serves from root, so we use /admin/testbucket to match the
	// extractDirectoryKey logic (path: /admin/testbucket -> key: testbucket).
	davDirectoryKey = "testbucket"
)

var _ = BeforeSuite(func() {
	log.SetOutput(io.Discard)

	var err error
	cliPath, err = gexec.Build("github.com/cloudfoundry/storage-cli")
	Expect(err).ShouldNot(HaveOccurred())

	_, filename, _, _ := runtime.Caller(0)
	testdataDir := filepath.Join(filepath.Dir(filename), "testdata")

	containerName = fmt.Sprintf("dav-integration-%d", time.Now().UnixNano())

	// Build the Docker image
	buildCmd := exec.Command("docker", "build", "-t", containerName, testdataDir)
	buildOut, err := buildCmd.CombinedOutput()
	Expect(err).ShouldNot(HaveOccurred(), "docker build failed: %s", string(buildOut))

	// Start the container, mapping port 443 to a random host port
	runCmd := exec.Command("docker", "run", "-d",
		"--name", containerName,
		"-p", "443",
		containerName,
	)
	runOut, err := runCmd.CombinedOutput()
	Expect(err).ShouldNot(HaveOccurred(), "docker run failed: %s", string(runOut))

	// Get the mapped host port
	portCmd := exec.Command("docker", "port", containerName, "443")
	portOut, err := portCmd.Output()
	Expect(err).ShouldNot(HaveOccurred())
	// output: "0.0.0.0:PORT\n" or ":::PORT\n"
	hostPort := strings.TrimSpace(string(portOut))
	hostPort = hostPort[strings.LastIndex(hostPort, ":")+1:]

	davEndpoint = fmt.Sprintf("https://localhost:%s/admin/%s", hostPort, davDirectoryKey)

	// Read the CA cert from testdata
	caBytes, err := os.ReadFile(filepath.Join(testdataDir, "certs", "server.crt"))
	Expect(err).ShouldNot(HaveOccurred())
	davCA = string(caBytes)

	// Wait for nginx to be ready
	Eventually(func() error {
		cmd := exec.Command("curl", "-sk", "--max-time", "2",
			"-u", fmt.Sprintf("%s:%s", davUser, davPassword),
			fmt.Sprintf("https://localhost:%s/", hostPort))
		return cmd.Run()
	}, 15*time.Second, 500*time.Millisecond).Should(Succeed())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()

	if containerName != "" {
		exec.Command("docker", "rm", "-f", containerName).Run()  //nolint:errcheck
		exec.Command("docker", "rmi", "-f", containerName).Run() //nolint:errcheck
	}
})
