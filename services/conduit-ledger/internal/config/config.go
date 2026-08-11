// Package config loads this service's configuration from environment
// variables. No config file, no flags, no library like viper — a handful of
// required env vars doesn't justify the dependency (see .env.example at the
// repo root for the full documented set across all services).
package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL       string
	RedisURL          string
	InternalJWTSecret string
	Port              string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:       os.Getenv("LEDGER_DATABASE_URL"),
		RedisURL:          os.Getenv("REDIS_URL"),
		InternalJWTSecret: os.Getenv("INTERNAL_JWT_SECRET"),
		Port:              os.Getenv("CONDUIT_LEDGER_PORT"),
	}
	if cfg.Port == "" {
		cfg.Port = "8002"
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("LEDGER_DATABASE_URL is required")
	}
	if cfg.RedisURL == "" {
		return Config{}, fmt.Errorf("REDIS_URL is required")
	}
	if cfg.InternalJWTSecret == "" {
		return Config{}, fmt.Errorf("INTERNAL_JWT_SECRET is required")
	}
	return cfg, nil
}
