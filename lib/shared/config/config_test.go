package config_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kite365/idcd/lib/shared/config"
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
  admin_token: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
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

func TestRateLimitRule_OrDefault(t *testing.T) {
	defWindow := 30 * time.Second
	var defMax int64 = 99

	// Both fields zero → defaults apply.
	w, m := (config.RateLimitRule{}).OrDefault(defWindow, defMax)
	if w != defWindow || m != defMax {
		t.Errorf("zero rule: want (%v, %d), got (%v, %d)", defWindow, defMax, w, m)
	}

	// Only window set → max falls back.
	r := config.RateLimitRule{Window: config.Duration{Duration: 5 * time.Minute}}
	w, m = r.OrDefault(defWindow, defMax)
	if w != 5*time.Minute || m != defMax {
		t.Errorf("partial rule: want (5m, %d), got (%v, %d)", defMax, w, m)
	}

	// Both set → both override.
	r = config.RateLimitRule{Window: config.Duration{Duration: time.Hour}, MaxRequests: 7}
	w, m = r.OrDefault(defWindow, defMax)
	if w != time.Hour || m != 7 {
		t.Errorf("full rule: want (1h, 7), got (%v, %d)", w, m)
	}
}

func TestLoad_rateLimitOverrides(t *testing.T) {
	yaml := testYAML + `
rate_limit:
  auth:
    window: "2m"
    max_requests: 10
  twofa:
    max_requests: 3
`
	p := writeTempConfig(t, yaml)
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RateLimit.Auth.Window.Duration != 2*time.Minute {
		t.Errorf("auth.window: want 2m, got %v", cfg.RateLimit.Auth.Window.Duration)
	}
	if cfg.RateLimit.Auth.MaxRequests != 10 {
		t.Errorf("auth.max_requests: want 10, got %d", cfg.RateLimit.Auth.MaxRequests)
	}
	// Partial override: only max set; window should remain zero so OrDefault kicks in.
	if cfg.RateLimit.TwoFA.Window.Duration != 0 {
		t.Errorf("twofa.window: want 0 (unset), got %v", cfg.RateLimit.TwoFA.Window.Duration)
	}
	if cfg.RateLimit.TwoFA.MaxRequests != 3 {
		t.Errorf("twofa.max_requests: want 3, got %d", cfg.RateLimit.TwoFA.MaxRequests)
	}
}

func TestDuration_CombinedDayForm(t *testing.T) {
	// parseDuration must handle "7d12h", "1d30m", and pure "30d".
	cases := []struct {
		val  string
		want time.Duration
	}{
		{"30d", 30 * 24 * time.Hour},
		{"7d12h", 7*24*time.Hour + 12*time.Hour},
		{"1d30m", 24*time.Hour + 30*time.Minute},
	}
	baseYAML := `
database:
  main:
    dsn: "postgresql://dev:dev@localhost:5432/dev"
redis:
  addr: "localhost:6379"
server:
  port: 8080
  env: "development"
  admin_token: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
jwt:
  secret: "supersecretkey_at_least_32_chars!!"
  access_ttl: "15m"
  refresh_ttl: "%s"
`
	for _, tc := range cases {
		y := fmt.Sprintf(baseYAML, tc.val)
		p := writeTempConfig(t, y)
		cfg, err := config.Load(p)
		if err != nil {
			t.Fatalf("Load with refresh_ttl=%q: %v", tc.val, err)
		}
		if cfg.JWT.RefreshTTL.Duration != tc.want {
			t.Errorf("refresh_ttl=%q: want %v, got %v", tc.val, tc.want, cfg.JWT.RefreshTTL.Duration)
		}
	}
}
