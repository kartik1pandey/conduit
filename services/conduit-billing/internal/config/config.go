// Package config loads this service's configuration from environment
// variables — no config file, no flags, matching every other Go service.
package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL       string
	InternalJWTSecret string
	Port              string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:       os.Getenv("BILLING_DATABASE_URL"),
		InternalJWTSecret: os.Getenv("INTERNAL_JWT_SECRET"),
		Port:              os.Getenv("CONDUIT_BILLING_PORT"),
	}
	if cfg.Port == "" {
		cfg.Port = "8004"
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("BILLING_DATABASE_URL is required")
	}
	if cfg.InternalJWTSecret == "" {
		return Config{}, fmt.Errorf("INTERNAL_JWT_SECRET is required")
	}
	return cfg, nil
}
