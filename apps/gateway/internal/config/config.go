// Package config provides Gateway-specific configuration.
package config

import (
	"time"
)

// Config holds the Gateway service configuration.
type Config struct {
	ListenAddr       string        `yaml:"listen_addr"`        // e.g. ":8081"
	TLSCert          string        `yaml:"tls_cert"`           // path to TLS cert (optional)
	TLSKey           string        `yaml:"tls_key"`            // path to TLS key (optional)
	RedisAddr        string        `yaml:"redis_addr"`         // Redis address for streams
	RedisPassword    string        `yaml:"redis_password"`     // Redis password
	RedisDB          int           `yaml:"redis_db"`           // Redis DB number
	PGDSN            string        `yaml:"pg_dsn"`             // PostgreSQL DSN (for future use)
	HeartbeatTimeout time.Duration `yaml:"heartbeat_timeout"`  // e.g. 30s
	MaxConnections   int           `yaml:"max_connections"`    // max concurrent WSS connections
	Env              string        `yaml:"env"`                // development | staging | production
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
