package config

import (
	"encoding/json"
	"io"
)

type AliStorageConfig struct {
	AccessKeyID     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
	Endpoint        string `json:"endpoint"`
	BucketName      string `json:"bucket_name"`
}

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

	return config, nil
}
