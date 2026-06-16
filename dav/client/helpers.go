package client

import (
	"crypto/x509"
	"fmt"
	"net/url"
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

// validateBlobID rejects blob IDs that could confuse path joining or enable
// path traversal: empty, leading/trailing slashes, double slashes, . or ..
// segments, and control characters.
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

func extractSignEndpoint(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parsing endpoint URL %q: %w", endpoint, err)
	}
	return fmt.Sprintf("%s://%s", u.Scheme, u.Host), nil
}

func extractDirectoryKey(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parsing endpoint URL %q: %w", endpoint, err)
	}

	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")

	for i, part := range pathParts {
		if part == "admin" && i+1 < len(pathParts) {
			return pathParts[i+1], nil
		}
	}

	for i := len(pathParts) - 1; i >= 0; i-- {
		if pathParts[i] != "" {
			return pathParts[i], nil
		}
	}

	return "", nil
}

// validatePrefix is like validateBlobID but allows a trailing slash.
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
