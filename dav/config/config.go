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
	TLS           TLS
	Secret        string
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
