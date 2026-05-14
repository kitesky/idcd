// Package config loads scheduler configuration.
package config

import (
	"fmt"
	"os"
	"time"

	sharedconfig "github.com/kite365/idcd/lib/shared/config"
	"gopkg.in/yaml.v3"
)

// Config holds the scheduler service configuration.
type Config struct {
	Redis         RedisConfig                      `yaml:"redis"`
	Database      DatabaseConfig                   `yaml:"database"`
	Leader        LeaderConfig                     `yaml:"leader"`
	Worker        WorkerConfig                     `yaml:"worker"`
	Observability sharedconfig.ObservabilityConfig `yaml:"observability"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	DSN string `yaml:"dsn"`
}

// LeaderConfig holds leader election settings.
type LeaderConfig struct {
	Key string        `yaml:"key"`         // Redis key for leader lock, default: "scheduler:leader"
	TTL time.Duration `yaml:"ttl"`         // Leader lock TTL, default: 10s
}

// WorkerConfig holds worker pool settings.
type WorkerConfig struct {
	Count int `yaml:"count"` // Number of worker goroutines, default: 4
}

// Load reads and parses the YAML config file at path.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("config.Load: open %q: %w", path, err)
	}
	defer f.Close()

	var cfg Config
	dec := yaml.NewDecoder(f)
	dec.KnownFields(false) // ignore unknown keys for forward-compat
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("config.Load: decode %q: %w", path, err)
	}

	// Apply defaults
	if cfg.Leader.Key == "" {
		cfg.Leader.Key = "scheduler:leader"
	}
	if cfg.Leader.TTL == 0 {
		cfg.Leader.TTL = 10 * time.Second
	}
	if cfg.Worker.Count == 0 {
		cfg.Worker.Count = 4
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config.Load: validate: %w", err)
	}

	return &cfg, nil
}

// MustLoad calls Load and panics on error — use in main() only.
func MustLoad(path string) *Config {
	cfg, err := Load(path)
	if err != nil {
		panic(err)
	}
	return cfg
}

// DefaultPath returns the config file path, honouring the IDCD_CONFIG env var.
func DefaultPath() string {
	if p := os.Getenv("IDCD_CONFIG"); p != "" {
		return p
	}
	return "config/dev.env.yaml"
}

func (c *Config) validate() error {
	if c.Redis.Addr == "" {
		return fmt.Errorf("redis.addr is required")
	}
	if c.Database.DSN == "" {
		return fmt.Errorf("database.dsn is required")
	}
	if c.Leader.TTL < time.Second {
		return fmt.Errorf("leader.ttl must be at least 1s")
	}
	if c.Worker.Count < 1 {
		return fmt.Errorf("worker.count must be at least 1")
	}
	return nil
}
