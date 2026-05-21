// Package config loads attest-verify service settings from environment variables.
//
// D6 independence: this package is entirely separate from apps/attest/internal/config.
// All env vars use the ATTEST_VERIFIER_ prefix to avoid collisions.
package config

import (
	"fmt"
	"net/url"
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
	envBatchSize     = "ATTEST_VERIFIER_BATCH_SIZE"
	envS3Region      = "ATTEST_VERIFIER_S3_REGION"      // required to fetch s3:// pdf_url
	envS3Endpoint    = "ATTEST_VERIFIER_S3_ENDPOINT"    // optional, S3-compatible backends
	envAllowFileURLs = "ATTEST_VERIFIER_ALLOW_FILE_URLS" // dev only; never set in prod

	defaultBindAddr       = ":8090"
	defaultEnv            = "development"
	defaultLogLevel       = "info"
	defaultVerifyEndpoint = "https://attest.idcd.com/verify"
	defaultPollInterval   = 5 * time.Minute
	defaultBatchSize      = 20
)

// Config holds the attest-verify runtime configuration.
//
// S3Region / S3Endpoint configure the s3:// fetcher. Without S3Region,
// records whose pdf_url is s3://... will fail to fetch — apps/attest's
// s3Archiver writes s3:// URLs in production (see s3archiver.go:126),
// so S3Region MUST be set when ArchiverBackend=s3 anywhere in the fleet.
//
// AllowFileURLs gates the file:// scheme in the fetcher. localArchiver
// (dev/CI) writes file:// URLs (localarchiver.go:40); production runs
// with archiver_backend=s3 and MUST NOT have AllowFileURLs=true — the
// fetcher would otherwise let any future code path that writes into
// verdict_report.pdf_url turn this service into an arbitrary-file reader.
type Config struct {
	BindAddr       string
	Env            string
	LogLevel       string
	DatabaseDSN    string
	VerifyEndpoint string
	PollInterval   time.Duration
	BatchSize      int
	S3Region       string
	S3Endpoint     string
	AllowFileURLs  bool
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
	if v := strings.TrimSpace(os.Getenv(envS3Region)); v != "" {
		cfg.S3Region = v
	}
	if v := strings.TrimSpace(os.Getenv(envS3Endpoint)); v != "" {
		cfg.S3Endpoint = v
	}
	if v := strings.TrimSpace(os.Getenv(envAllowFileURLs)); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("config: invalid %s=%q", envAllowFileURLs, v)
		}
		cfg.AllowFileURLs = b
	}

	if cfg.DatabaseDSN == "" {
		return nil, fmt.Errorf("config: %s is required", envDBDSN)
	}

	// VerifyEndpoint must be a parseable URL. Outside development, TLS is
	// required so the multipart PDF upload is not sent in cleartext.
	u, err := url.Parse(cfg.VerifyEndpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("config: %s=%q is not a valid URL", envVerifyEndpoint, cfg.VerifyEndpoint)
	}
	if cfg.Env != "development" && u.Scheme != "https" {
		return nil, fmt.Errorf("config: %s must use https in env=%s (got scheme %q)",
			envVerifyEndpoint, cfg.Env, u.Scheme)
	}

	// AllowFileURLs is a development convenience that turns the fetcher
	// into a local-file reader. Outside development, refuse to start
	// rather than expose that capability behind an env-var typo.
	if cfg.AllowFileURLs && cfg.Env != "development" {
		return nil, fmt.Errorf("config: %s=true is only valid when env=development (got env=%s)",
			envAllowFileURLs, cfg.Env)
	}

	return cfg, nil
}
