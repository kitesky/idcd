// Package config provides Gateway-specific configuration.
package config

import (
	"time"

	sharedconfig "github.com/kite365/idcd/lib/shared/config"
)

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
	Observability    sharedconfig.ObservabilityConfig `yaml:"observability"`
}

// Default returns a Config with sensible defaults for development.
func Default() *Config {
	return &Config{
		ListenAddr:       ":8081",
		RedisAddr:        "localhost:6379",
		RedisPassword:    "",
		RedisDB:          0,
		HeartbeatTimeout: 30 * time.Second,
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
