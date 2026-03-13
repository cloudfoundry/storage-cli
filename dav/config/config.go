package config

import (
	"encoding/json"
	"io"
)

type Config struct {
	User          string
	Password      string
	Endpoint      string
	RetryAttempts uint
	RetryDelay    uint `json:"retry_delay"` // Delay in seconds between retry attempts (default: 1)
	TLS           TLS
	Secret        string

	// SignedURLFormat specifies the signed URL format configured by the WebDAV server.
	// This must match the server configuration and should not be changed arbitrarily.
	// Supported values:
	//   - "hmac-sha256" (default): nginx secure_link_hmac format
	//   - "secure-link-md5": nginx secure_link format
	SignedURLFormat string `json:"signed_url_format"`

	// SignedURLExpiration is the signed URL lifetime in minutes (default: 15).
	SignedURLExpiration uint `json:"signed_url_expiration"`
}

type TLS struct {
	Cert Cert
}

type Cert struct {
	CA string
}

func NewFromReader(reader io.Reader) (Config, error) {
	config := Config{}

	configBytes, err := io.ReadAll(reader)
	if err != nil {
		return config, err
	}

	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		return config, err
	}

	return config, nil
}
