package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
)

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
	AccountName        string `json:"account_name"`
	AccountKey         string `json:"account_key"`
	ContainerName      string `json:"container_name"`
	Environment        string `json:"environment"`
	Timeout            string `json:"put_timeout_in_seconds"`
	HTTPRequestTimeout string `json:"http_request_timeout"`
}

// ErrNonPositiveHTTPRequestTimeout is returned when http_request_timeout is <= 0.
var ErrNonPositiveHTTPRequestTimeout = errors.New("http_request_timeout must be greater than 0")

// HTTPRequestTimeoutValue parses HTTPRequestTimeout as a Go duration string.
// Returns 0 (no timeout) if the field is empty.
func (c *AZStorageConfig) HTTPRequestTimeoutValue() (time.Duration, error) {
	if c.HTTPRequestTimeout == "" {
		return 0, nil
	}

	d, err := time.ParseDuration(c.HTTPRequestTimeout)
	if err != nil {
		return 0, fmt.Errorf("invalid http_request_timeout: %w", err)
	}

	if d <= 0 {
		return 0, ErrNonPositiveHTTPRequestTimeout
	}

	return d, nil
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

	if _, err = config.HTTPRequestTimeoutValue(); err != nil {
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
