// Package config loads notifier-specific configuration.
package config

import (
	"fmt"
	"os"

	"github.com/kite365/idcd/lib/shared/config"
	"gopkg.in/yaml.v3"
)

// Config holds the complete configuration for the notifier service.
type Config struct {
	*config.Config `yaml:",inline"`
	Notifier       NotifierConfig `yaml:"notifier"`
}

// NotifierConfig holds notifier-specific configuration.
type NotifierConfig struct {
	SMTP     SMTPConfig     `yaml:"smtp"`
	SES      SESConfig      `yaml:"ses"`
	AsynqDSN string         `yaml:"asynq_redis_dsn"`
	Workers  int            `yaml:"workers"` // number of worker goroutines (default: 4)
	Queues   map[string]int `yaml:"queues"`  // queue name -> priority mapping

	// CertStreamEnabled toggles the S2 W8 cert:notifications Redis Stream
	// consumer. Pointer-typed so the default (true) can be distinguished from
	// an explicit "false" override in YAML — nil means "use default, on";
	// *false means the operator explicitly disabled it; *true means the
	// operator explicitly enabled it (no-op vs default).
	//
	// Callers should use CertStreamEnabledOrDefault() to read the resolved
	// boolean.
	CertStreamEnabled *bool `yaml:"cert_stream_enabled"`

	// CertStreamName is the Redis Stream key the cert consumer reads from.
	// Matches apps/cert-svc/internal/service/notifications.go
	// DefaultNotificationStream ("cert:notifications").
	CertStreamName string `yaml:"cert_stream_name"`

	// CertConsumerGroup is the Redis consumer group name used by XREADGROUP.
	// Defaults to "cert-notifier" so all notifier replicas share work via the
	// same group (each message handled by exactly one replica).
	CertConsumerGroup string `yaml:"cert_consumer_group"`
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
	baseConfig, err := config.Load(path)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	cfg.Config = baseConfig

	dec := yaml.NewDecoder(f)
	dec.KnownFields(false)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("notifier config: decode %q: %w", path, err)
	}

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

// CertStreamEnabledOrDefault resolves the pointer-typed CertStreamEnabled
// field to a concrete bool. nil → true (default on); otherwise honour the
// operator override.
func (n *NotifierConfig) CertStreamEnabledOrDefault() bool {
	if n == nil || n.CertStreamEnabled == nil {
		return true
	}
	return *n.CertStreamEnabled
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
			"billing":           5, // D5 refund retry — high priority, payment-impacting
		}
	} else if _, ok := n.Queues["billing"]; !ok {
		// Honour user-supplied queue overrides but ensure the billing queue
		// is always present so D5 refund retry tasks are processed.
		n.Queues["billing"] = 5
	}

	if n.AsynqDSN == "" {
		n.AsynqDSN = "localhost:6379" // default Redis host:port (no auth)
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

	// S2 W8 cert:notifications consumer defaults.  CertStreamEnabled is a
	// pointer so its zero value (nil) is distinguishable from explicit
	// false; CertStreamEnabledOrDefault() collapses nil to true.
	if n.CertStreamName == "" {
		n.CertStreamName = "cert:notifications"
	}
	if n.CertConsumerGroup == "" {
		n.CertConsumerGroup = "cert-notifier"
	}
}