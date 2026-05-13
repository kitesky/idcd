package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kite365/idcd/packages/shared/config"
)

const testYAML = `
database:
  main:
    dsn: "postgresql://dev:dev@localhost:5432/dev"
    max_open_conns: 10
    max_idle_conns: 3
    conn_max_lifetime: "5m"

redis:
  addr: "localhost:6379"
  password: "secret"
  db: 0

server:
  port: 8080
  env: "development"
  cors_origins:
    - "http://localhost:3000"

jwt:
  secret: "supersecretkey_at_least_32_chars!!"
  access_ttl: "15m"
  refresh_ttl: "7d"

email:
  smtp_host: "localhost"
  smtp_port: 25
  from_addr: "noreply@idcd.com"
  from_name: "idcd"

observability:
  prometheus_port: 9091
`

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad_valid(t *testing.T) {
	p := writeTempConfig(t, testYAML)
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Database.Main.DSN == "" {
		t.Error("DSN should not be empty")
	}
	if cfg.Redis.Addr != "localhost:6379" {
		t.Errorf("unexpected redis addr: %q", cfg.Redis.Addr)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
}

func TestLoad_duration_minutes(t *testing.T) {
	p := writeTempConfig(t, testYAML)
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JWT.AccessTTL.Duration != 15*time.Minute {
		t.Errorf("access_ttl: expected 15m, got %v", cfg.JWT.AccessTTL.Duration)
	}
}

func TestLoad_duration_days(t *testing.T) {
	p := writeTempConfig(t, testYAML)
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JWT.RefreshTTL.Duration != 7*24*time.Hour {
		t.Errorf("refresh_ttl: expected 7d, got %v", cfg.JWT.RefreshTTL.Duration)
	}
}

func TestLoad_missingFile(t *testing.T) {
	_, err := config.Load("/nonexistent/path.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoad_missingDSN(t *testing.T) {
	yaml := `
redis:
  addr: "localhost:6379"
  password: "x"
jwt:
  secret: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
`
	p := writeTempConfig(t, yaml)
	_, err := config.Load(p)
	if err == nil {
		t.Error("expected validation error for missing DSN")
	}
}

func TestIsDev(t *testing.T) {
	p := writeTempConfig(t, testYAML)
	cfg, _ := config.Load(p)
	if !cfg.IsDev() {
		t.Error("expected IsDev() = true")
	}
	if cfg.IsProd() {
		t.Error("expected IsProd() = false")
	}
}

func TestDefaultPath_envVar(t *testing.T) {
	t.Setenv("IDCD_CONFIG", "/custom/config.yaml")
	if got := config.DefaultPath(); got != "/custom/config.yaml" {
		t.Errorf("expected /custom/config.yaml, got %q", got)
	}
}

func TestDefaultPath_fallback(t *testing.T) {
	os.Unsetenv("IDCD_CONFIG")
	if got := config.DefaultPath(); got != "config/dev.env.yaml" {
		t.Errorf("expected default path, got %q", got)
	}
}
