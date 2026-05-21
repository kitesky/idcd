// Package config loads attest-verify service settings from environment variables.
//
// D6 independence: this package is entirely separate from apps/attest/internal/config.
// All env vars use the ATTEST_VERIFIER_ prefix to avoid collisions.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	envBindAddr       = "ATTEST_VERIFIER_BIND_ADDR"
	envEnv            = "ATTEST_VERIFIER_ENV"
	envLogLevel       = "ATTEST_VERIFIER_LOG_LEVEL"
	envDBDSN          = "ATTEST_VERIFIER_DB_DSN"
	envVerifyEndpoint = "ATTEST_VERIFIER_VERIFY_ENDPOINT"
	envPollInterval   = "ATTEST_VERIFIER_POLL_INTERVAL"
	envBatchSize      = "ATTEST_VERIFIER_BATCH_SIZE"

	defaultBindAddr       = ":8090"
	defaultEnv            = "development"
	defaultLogLevel       = "info"
	defaultVerifyEndpoint = "https://attest.idcd.com/verify"
	defaultPollInterval   = 5 * time.Minute
	defaultBatchSize      = 20
)

// Config holds the attest-verify runtime configuration.
type Config struct {
	BindAddr       string
	Env            string
	LogLevel       string
	DatabaseDSN    string
	VerifyEndpoint string
	PollInterval   time.Duration
	BatchSize      int
}

// Load reads ATTEST_VERIFIER_* env vars and returns a validated Config.
func Load() (*Config, error) {
	cfg := &Config{
		BindAddr:       defaultBindAddr,
		Env:            defaultEnv,
		LogLevel:       defaultLogLevel,
		VerifyEndpoint: defaultVerifyEndpoint,
		PollInterval:   defaultPollInterval,
		BatchSize:      defaultBatchSize,
	}

	if v := strings.TrimSpace(os.Getenv(envBindAddr)); v != "" {
		cfg.BindAddr = v
	}
	if v := strings.TrimSpace(os.Getenv(envEnv)); v != "" {
		cfg.Env = v
	}
	if v := strings.TrimSpace(os.Getenv(envLogLevel)); v != "" {
		cfg.LogLevel = strings.ToLower(v)
	}
	if v := strings.TrimSpace(os.Getenv(envDBDSN)); v != "" {
		cfg.DatabaseDSN = v
	}
	if v := strings.TrimSpace(os.Getenv(envVerifyEndpoint)); v != "" {
		cfg.VerifyEndpoint = v
	}
	if v := strings.TrimSpace(os.Getenv(envPollInterval)); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return nil, fmt.Errorf("config: invalid %s=%q: %w", envPollInterval, v, err)
		}
		cfg.PollInterval = d
	}
	if v := strings.TrimSpace(os.Getenv(envBatchSize)); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("config: invalid %s=%q", envBatchSize, v)
		}
		cfg.BatchSize = n
	}

	if cfg.DatabaseDSN == "" {
		return nil, fmt.Errorf("config: %s is required", envDBDSN)
	}

	return cfg, nil
}
