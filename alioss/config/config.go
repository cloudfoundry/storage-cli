package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

type AliStorageConfig struct {
	AccessKeyID        string `json:"access_key_id"`
	AccessKeySecret    string `json:"access_key_secret"`
	Endpoint           string `json:"endpoint"`
	BucketName         string `json:"bucket_name"`
	HTTPRequestTimeout string `json:"http_request_timeout"`
}

var errorNonPositiveHTTPRequestTimeout = errors.New("http_request_timeout must be greater than 0")

// NewFromReader returns a new ali-storage-cli configuration struct from the contents of reader.
// reader.Read() is expected to return valid JSON
func NewFromReader(reader io.Reader) (AliStorageConfig, error) {
	bytes, err := io.ReadAll(reader)
	if err != nil {
		return AliStorageConfig{}, err
	}
	config := AliStorageConfig{}

	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return AliStorageConfig{}, err
	}

	if _, err := config.HTTPRequestTimeoutSeconds(); err != nil {
		return AliStorageConfig{}, err
	}

	return config, nil
}

func (c AliStorageConfig) HTTPRequestTimeoutSeconds() (int64, error) {
	if c.HTTPRequestTimeout == "" {
		return 0, nil
	}

	httpRequestTimeout, err := time.ParseDuration(c.HTTPRequestTimeout)
	if err != nil {
		return 0, fmt.Errorf("invalid http_request_timeout: %w", err)
	}

	if httpRequestTimeout <= 0 {
		return 0, errorNonPositiveHTTPRequestTimeout
	}

	// round up if necessary
	timeoutSeconds := int64(httpRequestTimeout / time.Second)
	if httpRequestTimeout%time.Second != 0 {
		timeoutSeconds++
	}

	return timeoutSeconds, nil
}
