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
	DatabaseURL            string
	RedisURL               string
	InternalJWTSecret      string
	Port                   string
	LedgerBaseURL          string
	LedgerCallTimeout      time.Duration
	WebhooksBaseURL        string
	RiskBaseURL            string
	RiskCallTimeout        time.Duration
	BillingBaseURL         string
	BillingCallTimeout     time.Duration
	DashboardSessionSecret string
	IdempotencyCacheTTL    time.Duration
	RateLimitPerMinute     int
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:            os.Getenv("CORE_DATABASE_URL"),
		RedisURL:               os.Getenv("REDIS_URL"),
		InternalJWTSecret:      os.Getenv("INTERNAL_JWT_SECRET"),
		Port:                   os.Getenv("CONDUIT_CORE_PORT"),
		LedgerBaseURL:          os.Getenv("CONDUIT_LEDGER_URL"),
		LedgerCallTimeout:      5 * time.Second,
		WebhooksBaseURL:        os.Getenv("CONDUIT_WEBHOOKS_URL"),
		RiskBaseURL:            os.Getenv("CONDUIT_RISK_URL"),
		RiskCallTimeout:        5 * time.Second,
		BillingBaseURL:         os.Getenv("CONDUIT_BILLING_URL"),
		BillingCallTimeout:     5 * time.Second,
		DashboardSessionSecret: os.Getenv("DASHBOARD_SESSION_SECRET"),
	}
	if cfg.Port == "" {
		cfg.Port = "8000"
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("CORE_DATABASE_URL is required")
	}
	if cfg.RedisURL == "" {
		return Config{}, fmt.Errorf("REDIS_URL is required")
	}
	if cfg.InternalJWTSecret == "" {
		return Config{}, fmt.Errorf("INTERNAL_JWT_SECRET is required")
	}
	if cfg.LedgerBaseURL == "" {
		return Config{}, fmt.Errorf("CONDUIT_LEDGER_URL is required")
	}
	if cfg.WebhooksBaseURL == "" {
		return Config{}, fmt.Errorf("CONDUIT_WEBHOOKS_URL is required")
	}
	if cfg.RiskBaseURL == "" {
		return Config{}, fmt.Errorf("CONDUIT_RISK_URL is required")
	}
	if cfg.BillingBaseURL == "" {
		return Config{}, fmt.Errorf("CONDUIT_BILLING_URL is required")
	}
	if cfg.DashboardSessionSecret == "" {
		return Config{}, fmt.Errorf("DASHBOARD_SESSION_SECRET is required")
	}

	ttlHours := os.Getenv("IDEMPOTENCY_KEY_TTL_HOURS")
	if ttlHours == "" {
		cfg.IdempotencyCacheTTL = 24 * time.Hour
	} else {
		n, err := strconv.Atoi(ttlHours)
		if err != nil {
			return Config{}, fmt.Errorf("IDEMPOTENCY_KEY_TTL_HOURS must be an integer: %w", err)
		}
		cfg.IdempotencyCacheTTL = time.Duration(n) * time.Hour
	}

	rateLimit := os.Getenv("RATE_LIMIT_REQUESTS_PER_MINUTE")
	if rateLimit == "" {
		cfg.RateLimitPerMinute = 100
	} else {
		n, err := strconv.Atoi(rateLimit)
		if err != nil {
			return Config{}, fmt.Errorf("RATE_LIMIT_REQUESTS_PER_MINUTE must be an integer: %w", err)
		}
		cfg.RateLimitPerMinute = n
	}

	return cfg, nil
}
