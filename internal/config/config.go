package config

import (
	"os"
	"strconv"
)

type Config struct {
	Debug bool
}

func New() *Config {
	return &Config{
		Debug: false,
	}
}

func NewFromEnv() *Config {
	cfg := New()

	if debug := os.Getenv("DEBUG"); debug != "" {
		if parsed, err := strconv.ParseBool(debug); err == nil {
			cfg.Debug = parsed
		}
	}

	return cfg
}
