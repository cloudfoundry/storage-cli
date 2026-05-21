package client

import (
	"crypto/x509"
	"fmt"
	"strings"

	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	davconf "github.com/cloudfoundry/storage-cli/dav/config"
)

func getCertPool(config davconf.Config) (*x509.CertPool, error) {
	if config.TLS.Cert.CA == "" {
		return nil, nil
	}

	certPool, err := boshcrypto.CertPoolFromPEM([]byte(config.TLS.Cert.CA))
	if err != nil {
		return nil, err
	}

	return certPool, nil
}

// validateBlobID rejects blob IDs that are unsafe to splice into a request
// path. The rules are intentionally strict: blob IDs come from external
// callers (e.g. CCNG, Diego) and a malformed value can confuse path joining,
// produce ambiguous URLs, or — worst case — escape the configured endpoint
// via path traversal. We refuse:
//
//   - empty strings
//   - leading or trailing slashes (the path joiner adds them itself)
//   - empty path segments ("//")
//   - "." or ".." segments (traversal)
//   - control characters (CRLF / NUL injection into headers and URLs)
func validateBlobID(blobID string) error {
	if blobID == "" {
		return fmt.Errorf("blob ID cannot be empty")
	}

	if strings.HasPrefix(blobID, "/") || strings.HasSuffix(blobID, "/") {
		return fmt.Errorf("blob ID cannot start or end with slash: %q", blobID)
	}

	if strings.Contains(blobID, "//") {
		return fmt.Errorf("blob ID cannot contain empty path segments (//): %q", blobID)
	}

	for _, segment := range strings.Split(blobID, "/") {
		if segment == "." || segment == ".." {
			return fmt.Errorf("blob ID cannot contain path traversal segments (. or ..): %q", blobID)
		}
	}

	for _, r := range blobID {
		if r < 32 || r == 127 {
			return fmt.Errorf("blob ID cannot contain control characters: %q", blobID)
		}
	}

	return nil
}

// validatePrefix is the more lenient sibling of validateBlobID, used for List.
// A directory-style prefix may legitimately end in "/" (e.g. "cc-droplets/"),
// but everything else still applies. An empty prefix is allowed at the caller
// level — List uses "" to mean "no filtering" — so this helper is only invoked
// when a non-empty prefix was supplied.
func validatePrefix(prefix string) error {
	if strings.HasPrefix(prefix, "/") {
		return fmt.Errorf("prefix cannot start with slash: %q", prefix)
	}

	if strings.Contains(prefix, "//") {
		return fmt.Errorf("prefix cannot contain empty path segments (//): %q", prefix)
	}

	for _, segment := range strings.Split(strings.TrimSuffix(prefix, "/"), "/") {
		if segment == "." || segment == ".." {
			return fmt.Errorf("prefix cannot contain path traversal segments (. or ..): %q", prefix)
		}
	}

	for _, r := range prefix {
		if r < 32 || r == 127 {
			return fmt.Errorf("prefix cannot contain control characters: %q", prefix)
		}
	}

	return nil
}
