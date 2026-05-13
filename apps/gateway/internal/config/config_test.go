package config

import (
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

	if cfg.HeartbeatTimeout != 30*time.Second {
		t.Errorf("expected HeartbeatTimeout %v, got %v", 30*time.Second, cfg.HeartbeatTimeout)
	}

	if cfg.MaxConnections != 10000 {
		t.Errorf("expected MaxConnections %d, got %d", 10000, cfg.MaxConnections)
	}

	if cfg.Env != "development" {
		t.Errorf("expected Env %q, got %q", "development", cfg.Env)
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
