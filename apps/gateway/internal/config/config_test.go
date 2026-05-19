package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.ListenAddr != ":8081" {
		t.Errorf("expected ListenAddr %q, got %q", ":8081", cfg.ListenAddr)
	}

	if cfg.RedisAddr != "localhost:6379" {
		t.Errorf("expected RedisAddr %q, got %q", "localhost:6379", cfg.RedisAddr)
	}

	if cfg.HeartbeatTimeout != 90*time.Second {
		t.Errorf("expected HeartbeatTimeout %v, got %v", 90*time.Second, cfg.HeartbeatTimeout)
	}

	if cfg.MaxConnections != 10000 {
		t.Errorf("expected MaxConnections %d, got %d", 10000, cfg.MaxConnections)
	}

	if cfg.Env != "development" {
		t.Errorf("expected Env %q, got %q", "development", cfg.Env)
	}

	// CertSvcURL should default to empty — the reverse-proxy mount is opt-in
	// so standalone gateway deploys (S1) don't accidentally try to talk to a
	// non-existent cert-svc upstream.
	if cfg.CertSvcURL != "" {
		t.Errorf("expected CertSvcURL %q, got %q", "", cfg.CertSvcURL)
	}
}

// TestLoadGatewayExtras_CertSvcURL verifies that the gateway-only
// cert_svc_url YAML key is decoded from the shared config file even though
// the shared config schema does not model it.
func TestLoadGatewayExtras_CertSvcURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dev.env.yaml")
	yamlBody := "" +
		"cert_svc_url: \"http://cert-svc:8082\"\n" +
		"redis:\n  addr: \"localhost:6379\"\n"
	if err := os.WriteFile(path, []byte(yamlBody), 0o600); err != nil {
		t.Fatalf("write tmp yaml: %v", err)
	}
	extras, ok := loadGatewayExtras(path)
	if !ok {
		t.Fatalf("loadGatewayExtras returned ok=false for valid file")
	}
	if extras.CertSvcURL != "http://cert-svc:8082" {
		t.Errorf("expected CertSvcURL %q, got %q", "http://cert-svc:8082", extras.CertSvcURL)
	}
}

// TestLoadGatewayExtras_Missing verifies graceful fallback when the YAML
// file is absent — gateway should not crash, just keep Default() values.
func TestLoadGatewayExtras_Missing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.yaml")
	extras, ok := loadGatewayExtras(path)
	if ok {
		t.Errorf("expected ok=false for missing file, got ok=true")
	}
	if extras.CertSvcURL != "" {
		t.Errorf("expected zero-value CertSvcURL, got %q", extras.CertSvcURL)
	}
}

// TestLoadGatewayExtras_NoKey verifies that omitting cert_svc_url leaves
// the field empty rather than triggering a decode error — backward compat
// with pre-S2 dev.env.yaml files.
func TestLoadGatewayExtras_NoKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dev.env.yaml")
	yamlBody := "redis:\n  addr: \"localhost:6379\"\n"
	if err := os.WriteFile(path, []byte(yamlBody), 0o600); err != nil {
		t.Fatalf("write tmp yaml: %v", err)
	}
	extras, ok := loadGatewayExtras(path)
	if !ok {
		t.Fatalf("loadGatewayExtras returned ok=false for valid file without cert_svc_url")
	}
	if extras.CertSvcURL != "" {
		t.Errorf("expected empty CertSvcURL when key absent, got %q", extras.CertSvcURL)
	}
}

func TestIsDev(t *testing.T) {
	tests := []struct {
		env      string
		expected bool
	}{
		{"development", true},
		{"staging", false},
		{"production", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			cfg := &Config{Env: tt.env}
			if got := cfg.IsDev(); got != tt.expected {
				t.Errorf("IsDev() = %v, expected %v for env %q", got, tt.expected, tt.env)
			}
		})
	}
}

func TestUseTLS(t *testing.T) {
	tests := []struct {
		name     string
		cert     string
		key      string
		expected bool
	}{
		{"both set", "/path/to/cert.pem", "/path/to/key.pem", true},
		{"cert only", "/path/to/cert.pem", "", false},
		{"key only", "", "/path/to/key.pem", false},
		{"neither set", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{TLSCert: tt.cert, TLSKey: tt.key}
			if got := cfg.UseTLS(); got != tt.expected {
				t.Errorf("UseTLS() = %v, expected %v", got, tt.expected)
			}
		})
	}
}
