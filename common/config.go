package common

import "sync"

type Config struct {
	Debug bool
}

var (
	instance *Config
	once     sync.Once
)

func InitConfig(debug bool) {
	once.Do(func() {
		instance = &Config{Debug: debug}
	})
}

func GetConfig() *Config {
	return instance
}

func IsDebug() bool {
	if instance == nil {
		return false
	}
	return instance.Debug
}
