package config

import (
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Make sure no env is leaking into the test.
	for _, k := range []string{envPort, envDB, envRedis, envLogLevel, envEnv} {
		t.Setenv(k, "")
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != defaultPort {
		t.Errorf("Port = %d, want %d", cfg.Port, defaultPort)
	}
	if cfg.DatabaseDSN != defaultDB {
		t.Errorf("DatabaseDSN = %q, want %q", cfg.DatabaseDSN, defaultDB)
	}
	if cfg.RedisURL != defaultRedis {
		t.Errorf("RedisURL = %q, want %q", cfg.RedisURL, defaultRedis)
	}
	if cfg.LogLevel != defaultLogLevel {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, defaultLogLevel)
	}
	if cfg.Env != defaultEnv {
		t.Errorf("Env = %q, want %q", cfg.Env, defaultEnv)
	}
	if got, want := cfg.Addr(), ":8080"; got != want {
		t.Errorf("Addr() = %q, want %q", got, want)
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv(envPort, "9090")
	t.Setenv(envDB, "postgres://u:p@db:5432/cert")
	t.Setenv(envRedis, "redis://cache:6380/3")
	t.Setenv(envLogLevel, "DEBUG")
	t.Setenv(envEnv, "production")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
	if cfg.DatabaseDSN != "postgres://u:p@db:5432/cert" {
		t.Errorf("DatabaseDSN = %q", cfg.DatabaseDSN)
	}
	if cfg.RedisURL != "redis://cache:6380/3" {
		t.Errorf("RedisURL = %q", cfg.RedisURL)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q (lowercased)", cfg.LogLevel, "debug")
	}
	if cfg.Env != "production" {
		t.Errorf("Env = %q", cfg.Env)
	}
	if got, want := cfg.Addr(), ":9090"; got != want {
		t.Errorf("Addr() = %q, want %q", got, want)
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	cases := []struct {
		name string
		val  string
	}{
		{"non-numeric", "abc"},
		{"zero", "0"},
		{"negative", "-1"},
		{"too-large", "70000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envPort, tc.val)
			if _, err := Load(); err == nil {
				t.Errorf("Load() with %s=%q expected error, got nil", envPort, tc.val)
			}
		})
	}
}

func TestLoad_WhitespaceTrimmed(t *testing.T) {
	t.Setenv(envPort, "  8081  ")
	t.Setenv(envEnv, "  staging  ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 8081 {
		t.Errorf("Port = %d, want 8081", cfg.Port)
	}
	if cfg.Env != "staging" {
		t.Errorf("Env = %q, want %q", cfg.Env, "staging")
	}
}
