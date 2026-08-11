// Package config loads this service's configuration from environment
// variables — no config file, no flags, no library like viper.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL         string
	RedisURL            string
	InternalJWTSecret   string
	Port                string
	MaxDeliveryAttempts int
	DeliveryTimeout     time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:       os.Getenv("WEBHOOKS_DATABASE_URL"),
		RedisURL:          os.Getenv("REDIS_URL"),
		InternalJWTSecret: os.Getenv("INTERNAL_JWT_SECRET"),
		Port:              os.Getenv("CONDUIT_WEBHOOKS_PORT"),
		DeliveryTimeout:   5 * time.Second,
	}
	if cfg.Port == "" {
		cfg.Port = "8003"
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("WEBHOOKS_DATABASE_URL is required")
	}
	if cfg.RedisURL == "" {
		return Config{}, fmt.Errorf("REDIS_URL is required")
	}
	if cfg.InternalJWTSecret == "" {
		return Config{}, fmt.Errorf("INTERNAL_JWT_SECRET is required")
	}

	maxRetries := os.Getenv("WEBHOOK_MAX_RETRIES")
	if maxRetries == "" {
		cfg.MaxDeliveryAttempts = 8
	} else {
		n, err := strconv.Atoi(maxRetries)
		if err != nil {
			return Config{}, fmt.Errorf("WEBHOOK_MAX_RETRIES must be an integer: %w", err)
		}
		cfg.MaxDeliveryAttempts = n
	}

	return cfg, nil
}
