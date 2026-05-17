// Package config loads cert-svc settings from environment variables.
//
// Unlike apps/api (which uses lib/shared/config + YAML), cert-svc is a
// fresh service and reads CERT_* env vars directly to keep its boot
// surface small. The S2 milestone may revisit and merge into the shared
// config tree once it stabilises.
package config

import (
	"encoding/base64"
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
	envJWTSecret      = "CERT_JWT_SECRET"
	envMasterKey      = "CERT_MASTER_KEY"
	envDownloadSecret = "CERT_DOWNLOAD_SECRET"

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

	// JWTSecret is the HMAC secret cert-svc uses to verify JWTs issued
	// by apps/api. Must match apps/api's auth.jwt.secret in the
	// canonical config. Empty disables JWT auth entirely (every
	// /v1/cert request gets a 401), which is the safe default for
	// preview / unconfigured environments.
	JWTSecret string

	// MasterKey is the base64-encoded 32-byte master key passed
	// straight through to envmaster. Re-exposed on Config so callers
	// (server main, worker main, tests) can decide whether to fall
	// back to env-only lookup or fail fast on missing config.
	MasterKey string

	// DownloadSecret is the raw HMAC-SHA256 key cert-svc uses to sign
	// the W5 one-shot download tokens. Sourced from CERT_DOWNLOAD_SECRET
	// (base64). Empty means the download token endpoint is disabled —
	// /v1/cert/certs/{id}/download will return 503 rather than mint
	// tokens we can't verify. cmd/server should fail fast when unset.
	DownloadSecret []byte
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
	if v := strings.TrimSpace(os.Getenv(envJWTSecret)); v != "" {
		cfg.JWTSecret = v
	}
	if v := strings.TrimSpace(os.Getenv(envMasterKey)); v != "" {
		cfg.MasterKey = v
	}
	if v := strings.TrimSpace(os.Getenv(envDownloadSecret)); v != "" {
		// Base64 (std OR url) so operators can paste from any common
		// secret tool. We reject empty / undecodable payloads up front
		// rather than handing the service a forgeable signing key.
		raw, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			raw, err = base64.RawStdEncoding.DecodeString(v)
		}
		if err != nil {
			raw, err = base64.URLEncoding.DecodeString(v)
		}
		if err != nil {
			raw, err = base64.RawURLEncoding.DecodeString(v)
		}
		if err != nil {
			return nil, fmt.Errorf("config: %s must be base64-encoded: %w", envDownloadSecret, err)
		}
		if len(raw) < 16 {
			return nil, fmt.Errorf("config: %s decoded to %d bytes; need >=16", envDownloadSecret, len(raw))
		}
		cfg.DownloadSecret = raw
	}

	return cfg, nil
}

// Addr returns the host:port the HTTP server should bind.
func (c *Config) Addr() string {
	return fmt.Sprintf(":%d", c.Port)
}
