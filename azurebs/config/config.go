package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
)

var errorNonPositiveHTTPRequestTimeout = errors.New("http_request_timeout must be greater than 0")
var errorNonPositiveHTTPResponseHeaderTimeout = errors.New("http_response_header_timeout must be greater than 0")

const storage cloud.ServiceName = "storage"

var cloudConfig cloud.Configuration

func init() {
	// Configure the cloud endpoints for the storage service
	// as the SDK does not have a configuration for it
	cloud.AzurePublic.Services[storage] = cloud.ServiceConfiguration{
		Endpoint: "blob.core.windows.net",
	}
	cloud.AzureChina.Services[storage] = cloud.ServiceConfiguration{
		Endpoint: "blob.core.chinacloudapi.cn",
	}
	cloud.AzureGovernment.Services[storage] = cloud.ServiceConfiguration{
		Endpoint: "blob.core.usgovcloudapi.net",
	}
}

type AZStorageConfig struct {
	AccountName               string `json:"account_name"`
	AccountKey                string `json:"account_key"`
	ContainerName             string `json:"container_name"`
	Environment               string `json:"environment"`
	Timeout                   string `json:"put_timeout_in_seconds"`
	HTTPRequestTimeout        string `json:"http_request_timeout"`
	HTTPResponseHeaderTimeout string `json:"http_response_header_timeout"`
}

// NewFromReader returns a new azure-storage-cli configuration struct from the contents of reader.
// reader.Read() is expected to return valid JSON
func NewFromReader(reader io.Reader) (AZStorageConfig, error) {
	bytes, err := io.ReadAll(reader)
	if err != nil {
		return AZStorageConfig{}, err
	}
	config := AZStorageConfig{}

	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return AZStorageConfig{}, err
	}

	err = config.configureCloud()
	if err != nil {
		return AZStorageConfig{}, err
	}

	if _, err := config.HTTPRequestTimeoutValue(); err != nil {
		return AZStorageConfig{}, err
	}

	if _, err := config.HTTPResponseHeaderTimeoutValue(); err != nil {
		return AZStorageConfig{}, err
	}

	return config, nil
}

func (c AZStorageConfig) StorageEndpoint() string {
	return cloudConfig.Services[storage].Endpoint
}

func (c *AZStorageConfig) configureCloud() error {
	switch c.Environment {
	case "AzureCloud", "":
		c.Environment = "AzureCloud"
		cloudConfig = cloud.AzurePublic
	case "AzureChinaCloud":
		cloudConfig = cloud.AzureChina
	case "AzureUSGovernment":
		cloudConfig = cloud.AzureGovernment
	default:
		return errors.New("unknown cloud environment: " + c.Environment)
	}
	return nil
}

// parseOptionalPositiveDuration parses a Go duration string (e.g. "30s", "2m").
// An empty value means "unset" and returns a zero duration with no error.
// A bare number without a unit is rejected, as is a non-positive duration.
func parseOptionalPositiveDuration(fieldName, value string, nonPositiveErr error) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}

	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return 0, fmt.Errorf("invalid %s: missing duration unit", fieldName)
	}

	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", fieldName, err)
	}

	if d <= 0 {
		return 0, nonPositiveErr
	}

	return d, nil
}

func (c AZStorageConfig) HTTPRequestTimeoutValue() (time.Duration, error) {
	return parseOptionalPositiveDuration("http_request_timeout", c.HTTPRequestTimeout, errorNonPositiveHTTPRequestTimeout)
}

func (c AZStorageConfig) HTTPResponseHeaderTimeoutValue() (time.Duration, error) {
	return parseOptionalPositiveDuration("http_response_header_timeout", c.HTTPResponseHeaderTimeout, errorNonPositiveHTTPResponseHeaderTimeout)
}
