package client_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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

// Pinned against github.com/Azure/azure-sdk-for-go/sdk/azcore@v1.23.1.
// If this test fails: diff runtime/transport_default_http_client.go at the new
// version, update buildClientOptions in storage_client.go to reflect any
// changes, then update pinnedHash (and pinnedVersion for clarity) below.
var _ = Describe("azcore transport defaults drift detection", func() {
	It("transport_default_http_client.go has not changed since it was last reviewed", func() {
		const pinnedVersion = "v1.23.1"
		const pinnedHash = "e60db6ff9e71a7f778503c3628c8cda73c5fd555436b470696a0c491fb6db20d"

		out, err := exec.Command("go", "list", "-m", "-json", "github.com/Azure/azure-sdk-for-go/sdk/azcore").Output()
		Expect(err).NotTo(HaveOccurred(), "go list failed: %v", err)

		var modInfo struct {
			Version string
			Dir     string
		}
		Expect(json.Unmarshal(out, &modInfo)).To(Succeed())
		Expect(modInfo.Dir).NotTo(BeEmpty(), "azcore module directory not found")

		transportFile := filepath.Join(modInfo.Dir, "runtime", "transport_default_http_client.go")
		content, err := os.ReadFile(transportFile)
		Expect(err).NotTo(HaveOccurred(),
			"could not read azcore transport file at %s", transportFile)

		actualHash := fmt.Sprintf("%x", sha256.Sum256(content))
		Expect(actualHash).To(Equal(pinnedHash),
			"azcore transport defaults changed in %s (last reviewed at %s).\n"+
				"Diff runtime/transport_default_http_client.go, update buildClientOptions "+
				"in storage_client.go if needed, then update pinnedVersion and pinnedHash in this test.",
			modInfo.Version, pinnedVersion,
		)
	})
})
