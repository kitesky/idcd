// Package config defines Aggregator-specific configuration.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/kite365/idcd/packages/shared/config"
	"gopkg.in/yaml.v3"
)

// Config extends the shared config with Aggregator-specific settings.
type Config struct {
	*config.Config `yaml:",inline"`
	Aggregator     AggregatorConfig `yaml:"aggregator"`
}

// AggregatorConfig contains settings specific to the Redis Stream aggregator.
type AggregatorConfig struct {
	RedisAddr     string        `yaml:"redis_addr"`
	StreamName    string        `yaml:"stream_name"`
	GroupName     string        `yaml:"group_name"`
	ConsumerCount int           `yaml:"consumer_count"`
	BatchSize     int64         `yaml:"batch_size"`
	BlockTimeout  time.Duration `yaml:"block_timeout"`
	PGDSN         string        `yaml:"pg_dsn"`
}

// Load reads and parses the Aggregator config file at path.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("aggregator config: open %q: %w", path, err)
	}
	defer f.Close()

	var cfg Config
	dec := yaml.NewDecoder(f)
	dec.KnownFields(false) // ignore unknown keys for forward-compat
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("aggregator config: decode %q: %w", path, err)
	}

	// Set defaults
	if cfg.Aggregator.StreamName == "" {
		cfg.Aggregator.StreamName = "probe.results"
	}
	if cfg.Aggregator.GroupName == "" {
		cfg.Aggregator.GroupName = "aggregator-group"
	}
	if cfg.Aggregator.ConsumerCount == 0 {
		cfg.Aggregator.ConsumerCount = 4
	}
	if cfg.Aggregator.BatchSize == 0 {
		cfg.Aggregator.BatchSize = 10
	}
	if cfg.Aggregator.BlockTimeout == 0 {
		cfg.Aggregator.BlockTimeout = 5 * time.Second
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("aggregator config: validate: %w", err)
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

// DefaultPath returns the aggregator config file path, honouring the AGGREGATOR_CONFIG env var.
func DefaultPath() string {
	if p := os.Getenv("AGGREGATOR_CONFIG"); p != "" {
		return p
	}
	return "config/aggregator.yaml"
}

func (c *Config) validate() error {
	// Validate base config first
	if c.Config != nil {
		if err := c.Config.Database.Main.DSN; err == "" {
			return fmt.Errorf("database.main.dsn is required")
		}
	}

	// Validate aggregator-specific config
	agg := c.Aggregator
	if agg.RedisAddr == "" {
		return fmt.Errorf("aggregator.redis_addr is required")
	}
	if agg.PGDSN == "" {
		return fmt.Errorf("aggregator.pg_dsn is required")
	}
	if agg.ConsumerCount <= 0 {
		return fmt.Errorf("aggregator.consumer_count must be > 0")
	}
	if agg.BatchSize <= 0 {
		return fmt.Errorf("aggregator.batch_size must be > 0")
	}
	if agg.BlockTimeout <= 0 {
		return fmt.Errorf("aggregator.block_timeout must be > 0")
	}

	return nil
}