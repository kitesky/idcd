// Package config defines the agent-specific configuration.
package config

import (
	"fmt"
	"os"

	sharedconfig "github.com/kite365/idcd/lib/shared/config"
	"gopkg.in/yaml.v3"
)

// Config holds agent-specific configuration.
type Config struct {
	NodeID        string                         `yaml:"node_id"`
	GatewayURL    string                         `yaml:"gateway_url"`
	CertPath      string                         `yaml:"cert_path"`
	Observability sharedconfig.ObservabilityConfig `yaml:"observability"`
	DataDir       string                           `yaml:"data_dir"`
	SecretKey     string                           `yaml:"secret_key"`
	PollInterval  string                           `yaml:"poll_interval"`
	BatchSize     int                              `yaml:"batch_size"`
}

// Load reads and parses the agent config file at path.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("config: open %q: %w", path, err)
	}
	defer f.Close()

	var cfg Config
	dec := yaml.NewDecoder(f)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("config: decode %q: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config: validate: %w", err)
	}

	// Apply environment variable overrides
	cfg.applyEnvOverrides()

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
	return "/etc/idcd-agent/config.yaml"
}

func (c *Config) validate() error {
	if c.NodeID == "" {
		return fmt.Errorf("node_id is required")
	}
	if c.GatewayURL == "" {
		return fmt.Errorf("gateway_url is required")
	}
	if c.SecretKey == "" {
		return fmt.Errorf("secret_key is required")
	}
	if c.DataDir == "" {
		c.DataDir = "/var/lib/idcd-agent"
	}
	if c.PollInterval == "" {
		c.PollInterval = "30s"
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 100
	}
	return nil
}

func (c *Config) applyEnvOverrides() {
	if nodeID := os.Getenv("AGENT_NODE_ID"); nodeID != "" {
		c.NodeID = nodeID
	}
	if dataDir := os.Getenv("AGENT_DATA_DIR"); dataDir != "" {
		c.DataDir = dataDir
	}
	if gatewayURL := os.Getenv("AGENT_GATEWAY_URL"); gatewayURL != "" {
		c.GatewayURL = gatewayURL
	}
}