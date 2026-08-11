// Package config loads this service's configuration from environment
// variables — no config file, no flags, no library like viper.
package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	DatabaseURL       string
	InternalJWTSecret string
	Port              string
	LedgerBaseURL     string
	LedgerCallTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:       os.Getenv("CORE_DATABASE_URL"),
		InternalJWTSecret: os.Getenv("INTERNAL_JWT_SECRET"),
		Port:              os.Getenv("CONDUIT_CORE_PORT"),
		LedgerBaseURL:     os.Getenv("CONDUIT_LEDGER_URL"),
		LedgerCallTimeout: 5 * time.Second,
	}
	if cfg.Port == "" {
		cfg.Port = "8000"
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("CORE_DATABASE_URL is required")
	}
	if cfg.InternalJWTSecret == "" {
		return Config{}, fmt.Errorf("INTERNAL_JWT_SECRET is required")
	}
	if cfg.LedgerBaseURL == "" {
		return Config{}, fmt.Errorf("CONDUIT_LEDGER_URL is required")
	}
	return cfg, nil
}
