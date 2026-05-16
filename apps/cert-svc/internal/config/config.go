// Package config loads cert-svc settings from environment variables.
//
// Unlike apps/api (which uses lib/shared/config + YAML), cert-svc is a
// fresh service and reads CERT_* env vars directly to keep its boot
// surface small. The S2 milestone may revisit and merge into the shared
// config tree once it stabilises.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	envPort         = "CERT_SVC_PORT"
	envDB           = "CERT_DB_DSN"
	envRedis        = "CERT_REDIS_URL"
	envRedisAddr    = "CERT_REDIS_ADDR"
	envLogLevel     = "CERT_LOG_LEVEL"
	envEnv          = "CERT_ENV"
	envLEEnv        = "CERT_LE_ENV"
	envAccountEmail = "CERT_ACME_ACCOUNT_EMAIL"

	defaultPort         = 8080
	defaultDB           = "postgres://idcd:idcd@localhost:5432/idcd?sslmode=disable"
	defaultRedis        = "redis://localhost:6379/0"
	defaultRedisAddr    = "localhost:6379"
	defaultLogLevel     = "info"
	defaultEnv          = "development"
	defaultLEEnv        = "staging"
	defaultAccountEmail = "acme@idcd.local"
)

// Config is the cert-svc runtime configuration.
type Config struct {
	Port         int
	DatabaseDSN  string
	RedisURL     string
	RedisAddr    string
	LogLevel     string
	Env          string
	LEEnv        string
	AccountEmail string
}

// Load reads CERT_* env vars and returns a populated Config.
// Unset vars fall back to sensible local-dev defaults; the only validation
// failure today is a non-numeric CERT_SVC_PORT.
func Load() (*Config, error) {
	cfg := &Config{
		Port:         defaultPort,
		DatabaseDSN:  defaultDB,
		RedisURL:     defaultRedis,
		RedisAddr:    defaultRedisAddr,
		LogLevel:     defaultLogLevel,
		Env:          defaultEnv,
		LEEnv:        defaultLEEnv,
		AccountEmail: defaultAccountEmail,
	}

	if v := strings.TrimSpace(os.Getenv(envPort)); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("config: invalid %s=%q: %w", envPort, v, err)
		}
		if port <= 0 || port > 65535 {
			return nil, fmt.Errorf("config: %s out of range: %d", envPort, port)
		}
		cfg.Port = port
	}

	if v := strings.TrimSpace(os.Getenv(envDB)); v != "" {
		cfg.DatabaseDSN = v
	}
	if v := strings.TrimSpace(os.Getenv(envRedis)); v != "" {
		cfg.RedisURL = v
	}
	if v := strings.TrimSpace(os.Getenv(envRedisAddr)); v != "" {
		cfg.RedisAddr = v
	}
	if v := strings.TrimSpace(os.Getenv(envLogLevel)); v != "" {
		cfg.LogLevel = strings.ToLower(v)
	}
	if v := strings.TrimSpace(os.Getenv(envEnv)); v != "" {
		cfg.Env = v
	}
	if v := strings.TrimSpace(os.Getenv(envLEEnv)); v != "" {
		cfg.LEEnv = v
	}
	if v := strings.TrimSpace(os.Getenv(envAccountEmail)); v != "" {
		cfg.AccountEmail = v
	}

	return cfg, nil
}

// Addr returns the host:port the HTTP server should bind.
func (c *Config) Addr() string {
	return fmt.Sprintf(":%d", c.Port)
}
