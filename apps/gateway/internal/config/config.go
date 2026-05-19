// Package config provides Gateway-specific configuration.
package config

import (
	"os"
	"time"

	sharedconfig "github.com/kite365/idcd/lib/shared/config"
	"gopkg.in/yaml.v3"
)

// Load returns a Config populated from the shared dev.env.yaml, falling back
// to Default() values for gateway-specific fields not present in the shared config.
func Load() *Config {
	cfg := Default()
	shared, err := sharedconfig.Load(sharedconfig.DefaultPath())
	if err != nil {
		// Config file missing or invalid — fall back to defaults (e.g. in tests).
		return cfg
	}
	if shared.Redis.Addr != "" {
		cfg.RedisAddr = shared.Redis.Addr
		cfg.RedisPassword = shared.Redis.Password
		cfg.RedisDB = shared.Redis.DB
	}
	if shared.Database.Main.DSN != "" {
		cfg.PGDSN = shared.Database.Main.DSN
	}
	if shared.Server.Env != "" {
		cfg.Env = shared.Server.Env
	}
	if shared.AgentGateway.Addr != "" {
		cfg.ListenAddr = shared.AgentGateway.Addr
	}
	// Re-read the YAML file directly for gateway-only knobs that the shared
	// config schema does not model (cert_svc_url). Failure is non-fatal —
	// we keep the Default() value (empty string disables the proxy).
	if extra, ok := loadGatewayExtras(sharedconfig.DefaultPath()); ok {
		if extra.CertSvcURL != "" {
			cfg.CertSvcURL = extra.CertSvcURL
		}
	}
	return cfg
}

// gatewayExtras captures gateway-only YAML keys that live in dev.env.yaml
// but are not part of the shared config schema.
type gatewayExtras struct {
	CertSvcURL string `yaml:"cert_svc_url"`
}

// loadGatewayExtras decodes the gateway-only knobs from the YAML config file.
// Returns ok=false if the file is missing or cannot be parsed; callers fall
// back to Default() / Load() values in that case.
func loadGatewayExtras(path string) (gatewayExtras, bool) {
	var extras gatewayExtras
	f, err := os.Open(path)
	if err != nil {
		return extras, false
	}
	defer f.Close()
	dec := yaml.NewDecoder(f)
	dec.KnownFields(false)
	if err := dec.Decode(&extras); err != nil {
		return extras, false
	}
	return extras, true
}

// Config holds the Gateway service configuration.
type Config struct {
	ListenAddr       string                           `yaml:"listen_addr"`
	TLSCert          string                           `yaml:"tls_cert"`
	TLSKey           string                           `yaml:"tls_key"`
	RedisAddr        string                           `yaml:"redis_addr"`
	RedisPassword    string                           `yaml:"redis_password"`
	RedisDB          int                              `yaml:"redis_db"`
	PGDSN            string                           `yaml:"pg_dsn"`
	HeartbeatTimeout time.Duration                    `yaml:"heartbeat_timeout"`
	MaxConnections   int                              `yaml:"max_connections"`
	Env              string                           `yaml:"env"`
	// MetricsToken protects the /metrics endpoint. Requests must supply
	// Authorization: Bearer <token>. Leave empty in dev to skip the check.
	MetricsToken  string                           `yaml:"metrics_token"`
	Observability sharedconfig.ObservabilityConfig `yaml:"observability"`
	// CertSvcURL is the base URL (scheme://host:port) of the cert-svc
	// HTTP service that owns /v1/cert/*. When non-empty, the gateway
	// reverse-proxies /v1/cert/* and the unauthenticated one-shot
	// /v1/cert/certs/{id}/download endpoint to this upstream.
	// Empty disables the proxy (legacy / standalone gateway deploys).
	CertSvcURL string `yaml:"cert_svc_url"`
}

// Default returns a Config with sensible defaults for development.
func Default() *Config {
	return &Config{
		ListenAddr:       ":8081",
		RedisAddr:        "localhost:6379",
		RedisPassword:    "",
		RedisDB:          0,
		// Must be >= 2x the agent's heartbeat interval (apps/agent sends one
		// application-level "heartbeat" message every 30s). At the old 30s value
		// the comparison `now - LastHB > 30s` flickered into "true" whenever
		// the agent's next heartbeat slipped behind the gateway's check ticker
		// by even a few hundred ms — typical of a remote DB heartbeat handler
		// or scheduler jitter — and the hub evicted the still-healthy node,
		// causing the "WS connects → ~30s later disconnects → reconnects"
		// reconnect storm. 90s gives two full agent heartbeat windows of slack
		// before declaring a node stale.
		HeartbeatTimeout: 90 * time.Second,
		MaxConnections:   10000,
		Env:              "development",
	}
}

// IsDev reports whether this is a development environment.
func (c *Config) IsDev() bool {
	return c.Env == "development"
}

// UseTLS reports whether TLS is configured.
func (c *Config) UseTLS() bool {
	return c.TLSCert != "" && c.TLSKey != ""
}
