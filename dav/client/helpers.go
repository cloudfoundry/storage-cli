package client

import (
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	davconf "github.com/cloudfoundry/storage-cli/dav/config"
)

// getCertPool creates a certificate pool from the config
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

// validateBlobID ensures blob IDs are valid and safe to use in path construction
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

	segments := strings.Split(blobID, "/")
	for _, segment := range segments {
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

// validatePrefix ensures list prefixes are safe (more lenient than validateBlobID)
// Allows trailing slashes for directory-style prefixes (e.g., "foo/")
func validatePrefix(prefix string) error {
	if prefix == "" {
		return fmt.Errorf("prefix cannot be empty")
	}

	if strings.HasPrefix(prefix, "/") {
		return fmt.Errorf("prefix cannot start with slash: %q", prefix)
	}

	if strings.Contains(prefix, "//") {
		return fmt.Errorf("prefix cannot contain empty path segments (//): %q", prefix)
	}

	prefixForValidation := strings.TrimSuffix(prefix, "/")

	segments := strings.Split(prefixForValidation, "/")
	for _, segment := range segments {
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

func extractSignEndpoint(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	return fmt.Sprintf("%s://%s", u.Scheme, u.Host)
}

// extracts the directory key from the endpoint path
func extractDirectoryKey(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}

	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")

	for i, part := range pathParts {
		if part == "admin" && i+1 < len(pathParts) {
			return pathParts[i+1]
		}
	}

	for i := len(pathParts) - 1; i >= 0; i-- {
		if pathParts[i] != "" {
			return pathParts[i]
		}
	}

	return ""
}

// validates the endpoint configuration and provides helpful error messages
func validateEndpointConfig(config davconf.Config) error {
	if config.Endpoint == "" {
		return fmt.Errorf("endpoint cannot be empty")
	}

	u, err := url.Parse(config.Endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("endpoint must use http or https scheme, got: %s", u.Scheme)
	}

	if u.Path != "" {
		pathLower := strings.ToLower(u.Path)
		if !strings.Contains(pathLower, "/admin/") {
			slog.Warn("endpoint path does not contain '/admin/' - this may cause issues with WebDAV operations",
				"endpoint", config.Endpoint,
				"path", u.Path)
		}
	}

	if config.SignedURLFormat == "external-nginx-secure-link-signer" {
		if config.User == "" || config.Password == "" {
			return fmt.Errorf("external-nginx-secure-link-signer requires user and password for Basic Auth")
		}
		if config.PublicEndpoint == "" {
			return fmt.Errorf("external-nginx-secure-link-signer requires public_endpoint to be configured")
		}

		if config.Secret != "" {
			slog.Warn("secret is configured but not used with external-nginx-secure-link-signer",
				"signed_url_format", config.SignedURLFormat)
		}
	} else if config.SignedURLFormat == "hmac-sha256" {
		if config.Secret == "" {
			return fmt.Errorf("%s requires secret to be configured", config.SignedURLFormat)
		}
	}

	return nil
}

