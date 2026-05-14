// Package config loads notifier-specific configuration.
package config

import (
	"os"

	"github.com/kite365/idcd/lib/shared/config"
)

// Config holds the complete configuration for the notifier service.
type Config struct {
	*config.Config `yaml:",inline"`
	Notifier       NotifierConfig `yaml:"notifier"`
}

// NotifierConfig holds notifier-specific configuration.
type NotifierConfig struct {
	SMTP      SMTPConfig      `yaml:"smtp"`
	SES       SESConfig       `yaml:"ses"`
	AsynqDSN  string          `yaml:"asynq_redis_dsn"`
	Workers   int             `yaml:"workers"`          // number of worker goroutines (default: 4)
	Queues    map[string]int  `yaml:"queues"`           // queue name -> priority mapping
}

// SMTPConfig holds SMTP server configuration.
type SMTPConfig struct {
	Host     string `yaml:"host"`      // SMTP server host
	Port     int    `yaml:"port"`      // SMTP server port (587 for STARTTLS, 465 for TLS)
	Username string `yaml:"username"`  // SMTP authentication username
	Password string `yaml:"password"`  // SMTP authentication password
	From     string `yaml:"from"`      // sender email address
	FromName string `yaml:"from_name"` // sender display name
}

// SESConfig holds AWS SES configuration.
type SESConfig struct {
	Region    string `yaml:"region"`     // AWS region (e.g., "us-east-1")
	AccessKey string `yaml:"access_key"` // AWS access key ID
	SecretKey string `yaml:"secret_key"` // AWS secret access key
	From      string `yaml:"from"`       // sender email address
	FromName  string `yaml:"from_name"`  // sender display name
}

// Load reads and parses the configuration file.
func Load(path string) (*Config, error) {
	// Load base config first
	baseConfig, err := config.Load(path)
	if err != nil {
		return nil, err
	}

	// Load the full config including notifier section
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	cfg.Config = baseConfig

	// Set defaults for notifier config
	setDefaults(&cfg.Notifier)

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

// setDefaults sets default values for notifier configuration.
func setDefaults(n *NotifierConfig) {
	if n.Workers == 0 {
		n.Workers = 4 // default worker count
	}

	if n.Queues == nil {
		n.Queues = map[string]int{
			"notifier:default":  1, // lower priority
			"notifier:critical": 2, // higher priority
		}
	}

	if n.AsynqDSN == "" {
		n.AsynqDSN = "redis://localhost:6379/0" // default Redis connection
	}

	// SMTP defaults
	if n.SMTP.Port == 0 {
		n.SMTP.Port = 587 // default to STARTTLS
	}
	if n.SMTP.FromName == "" {
		n.SMTP.FromName = "idcd" // default sender name
	}

	// SES defaults
	if n.SES.Region == "" {
		n.SES.Region = "us-east-1" // default region
	}
	if n.SES.FromName == "" {
		n.SES.FromName = "idcd" // default sender name
	}
}