package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kite365/idcd/apps/attest-verify/internal/config"
)

// resetEnv clears every ATTEST_VERIFIER_* env var consulted by Load so a
// previous test's exports don't leak into the next. We set the DB DSN
// here too since most tests need it and it's the only required var.
func resetEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"ATTEST_VERIFIER_BIND_ADDR",
		"ATTEST_VERIFIER_ENV",
		"ATTEST_VERIFIER_LOG_LEVEL",
		"ATTEST_VERIFIER_DB_DSN",
		"ATTEST_VERIFIER_VERIFY_ENDPOINT",
		"ATTEST_VERIFIER_POLL_INTERVAL",
		"ATTEST_VERIFIER_BATCH_SIZE",
		"ATTEST_VERIFIER_S3_REGION",
		"ATTEST_VERIFIER_S3_ENDPOINT",
		"ATTEST_VERIFIER_ALLOW_FILE_URLS",
	} {
		t.Setenv(k, "")
	}
}

func TestLoad_DefaultsWithDBDSN(t *testing.T) {
	resetEnv(t)
	t.Setenv("ATTEST_VERIFIER_DB_DSN", "postgres://x:y@localhost/db")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BindAddr != ":8090" {
		t.Errorf("BindAddr default = %q, want :8090", cfg.BindAddr)
	}
	if cfg.Env != "development" {
		t.Errorf("Env default = %q, want development", cfg.Env)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel default = %q, want info", cfg.LogLevel)
	}
	if cfg.VerifyEndpoint != "https://attest.idcd.com/verify" {
		t.Errorf("VerifyEndpoint default = %q", cfg.VerifyEndpoint)
	}
	if cfg.PollInterval != 5*time.Minute {
		t.Errorf("PollInterval default = %v, want 5m", cfg.PollInterval)
	}
	if cfg.BatchSize != 20 {
		t.Errorf("BatchSize default = %d, want 20", cfg.BatchSize)
	}
	if cfg.S3Region != "" || cfg.S3Endpoint != "" {
		t.Errorf("S3 unset by default, got region=%q endpoint=%q", cfg.S3Region, cfg.S3Endpoint)
	}
	if cfg.AllowFileURLs {
		t.Error("AllowFileURLs should default false")
	}
}

func TestLoad_MissingDBDSN(t *testing.T) {
	resetEnv(t)
	_, err := config.Load()
	if err == nil || !strings.Contains(err.Error(), "DB_DSN") {
		t.Fatalf("expected DB_DSN required error, got %v", err)
	}
}

func TestLoad_PollInterval_Invalid(t *testing.T) {
	resetEnv(t)
	t.Setenv("ATTEST_VERIFIER_DB_DSN", "postgres://x:y@localhost/db")
	t.Setenv("ATTEST_VERIFIER_POLL_INTERVAL", "not-a-duration")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLoad_PollInterval_NonPositive(t *testing.T) {
	resetEnv(t)
	t.Setenv("ATTEST_VERIFIER_DB_DSN", "postgres://x:y@localhost/db")
	t.Setenv("ATTEST_VERIFIER_POLL_INTERVAL", "0s")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected non-positive duration error")
	}
}

func TestLoad_BatchSize_Invalid(t *testing.T) {
	resetEnv(t)
	t.Setenv("ATTEST_VERIFIER_DB_DSN", "postgres://x:y@localhost/db")
	t.Setenv("ATTEST_VERIFIER_BATCH_SIZE", "xyz")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLoad_BatchSize_NonPositive(t *testing.T) {
	resetEnv(t)
	t.Setenv("ATTEST_VERIFIER_DB_DSN", "postgres://x:y@localhost/db")
	t.Setenv("ATTEST_VERIFIER_BATCH_SIZE", "0")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected non-positive batch size error")
	}
}

func TestLoad_LogLevelLowercased(t *testing.T) {
	resetEnv(t)
	t.Setenv("ATTEST_VERIFIER_DB_DSN", "postgres://x:y@localhost/db")
	t.Setenv("ATTEST_VERIFIER_LOG_LEVEL", "DEBUG")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", cfg.LogLevel)
	}
}

func TestLoad_VerifyEndpointHTTPSRequiredInProd(t *testing.T) {
	resetEnv(t)
	t.Setenv("ATTEST_VERIFIER_DB_DSN", "postgres://x:y@localhost/db")
	t.Setenv("ATTEST_VERIFIER_ENV", "production")
	t.Setenv("ATTEST_VERIFIER_VERIFY_ENDPOINT", "http://insecure/verify")
	_, err := config.Load()
	if err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("expected https-required error, got %v", err)
	}
}

func TestLoad_VerifyEndpointHTTPAllowedInDev(t *testing.T) {
	resetEnv(t)
	t.Setenv("ATTEST_VERIFIER_DB_DSN", "postgres://x:y@localhost/db")
	t.Setenv("ATTEST_VERIFIER_ENV", "development")
	t.Setenv("ATTEST_VERIFIER_VERIFY_ENDPOINT", "http://127.0.0.1:8080/verify")
	if _, err := config.Load(); err != nil {
		t.Fatalf("expected http allowed in dev, got %v", err)
	}
}

func TestLoad_VerifyEndpointMalformed(t *testing.T) {
	resetEnv(t)
	t.Setenv("ATTEST_VERIFIER_DB_DSN", "postgres://x:y@localhost/db")
	t.Setenv("ATTEST_VERIFIER_VERIFY_ENDPOINT", "::::not-a-url")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected URL parse error")
	}
}

func TestLoad_AllowFileURLsRejectedInProd(t *testing.T) {
	resetEnv(t)
	t.Setenv("ATTEST_VERIFIER_DB_DSN", "postgres://x:y@localhost/db")
	t.Setenv("ATTEST_VERIFIER_ENV", "production")
	t.Setenv("ATTEST_VERIFIER_ALLOW_FILE_URLS", "true")
	_, err := config.Load()
	if err == nil || !strings.Contains(err.Error(), "development") {
		t.Fatalf("expected ALLOW_FILE_URLS rejected in prod, got %v", err)
	}
}

func TestLoad_AllowFileURLsAcceptedInDev(t *testing.T) {
	resetEnv(t)
	t.Setenv("ATTEST_VERIFIER_DB_DSN", "postgres://x:y@localhost/db")
	t.Setenv("ATTEST_VERIFIER_ENV", "development")
	t.Setenv("ATTEST_VERIFIER_ALLOW_FILE_URLS", "true")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.AllowFileURLs {
		t.Error("AllowFileURLs should be true after env=true")
	}
}

func TestLoad_S3Overrides(t *testing.T) {
	resetEnv(t)
	t.Setenv("ATTEST_VERIFIER_DB_DSN", "postgres://x:y@localhost/db")
	t.Setenv("ATTEST_VERIFIER_S3_REGION", "us-east-1")
	t.Setenv("ATTEST_VERIFIER_S3_ENDPOINT", "http://minio:9000")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.S3Region != "us-east-1" {
		t.Errorf("S3Region = %q, want us-east-1", cfg.S3Region)
	}
	if cfg.S3Endpoint != "http://minio:9000" {
		t.Errorf("S3Endpoint = %q", cfg.S3Endpoint)
	}
}

func TestLoad_AllScalarOverrides(t *testing.T) {
	resetEnv(t)
	t.Setenv("ATTEST_VERIFIER_DB_DSN", "postgres://x:y@localhost/db")
	t.Setenv("ATTEST_VERIFIER_BIND_ADDR", ":9091")
	t.Setenv("ATTEST_VERIFIER_ENV", "staging")
	t.Setenv("ATTEST_VERIFIER_VERIFY_ENDPOINT", "https://attest.staging.idcd.com/verify")
	t.Setenv("ATTEST_VERIFIER_POLL_INTERVAL", "30s")
	t.Setenv("ATTEST_VERIFIER_BATCH_SIZE", "5")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BindAddr != ":9091" {
		t.Errorf("BindAddr = %q", cfg.BindAddr)
	}
	if cfg.Env != "staging" {
		t.Errorf("Env = %q", cfg.Env)
	}
	if cfg.PollInterval != 30*time.Second {
		t.Errorf("PollInterval = %v", cfg.PollInterval)
	}
	if cfg.BatchSize != 5 {
		t.Errorf("BatchSize = %d", cfg.BatchSize)
	}
}
