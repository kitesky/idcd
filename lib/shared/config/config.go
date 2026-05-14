// Package config loads idcd configuration from a YAML file.
// The canonical file for local dev is config/dev.env.yaml (gitignored).
// Use config/dev.env.example.yaml as the template.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration structure.
type Config struct {
	Database      DatabaseConfig      `yaml:"database"`
	Redis         RedisConfig         `yaml:"redis"`
	Server        ServerConfig        `yaml:"server"`
	JWT           JWTConfig           `yaml:"jwt"`
	Email         EmailConfig         `yaml:"email"`
	Observability ObservabilityConfig `yaml:"observability"`
	AgentGateway  AgentGatewayConfig  `yaml:"agent_gateway"`
	OAuth         OAuthConfig         `yaml:"oauth"`
}

// OAuthConfig holds third-party OAuth provider credentials.
type OAuthConfig struct {
	CallbackBase string          `yaml:"callback_base"`
	DingTalk     DingTalkConfig  `yaml:"dingtalk"`
	Feishu       FeishuConfig    `yaml:"feishu"`
}

// DingTalkConfig holds DingTalk app credentials.
type DingTalkConfig struct {
	AppID     string `yaml:"app_id"`
	AppSecret string `yaml:"app_secret"`
}

// FeishuConfig holds Feishu app credentials.
type FeishuConfig struct {
	AppID     string `yaml:"app_id"`
	AppSecret string `yaml:"app_secret"`
}

type DatabaseConfig struct {
	Main MainDBConfig `yaml:"main"`
}

type MainDBConfig struct {
	DSN             string   `yaml:"dsn"`
	MaxOpenConns    int32    `yaml:"max_open_conns"`
	MaxIdleConns    int32    `yaml:"max_idle_conns"`
	ConnMaxLifetime Duration `yaml:"conn_max_lifetime"` // e.g. "5m"
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type ServerConfig struct {
	Port        int      `yaml:"port"`
	Env         string   `yaml:"env"` // development | staging | production
	CORSOrigins []string `yaml:"cors_origins"`
	AdminToken  string   `yaml:"admin_token"`
}

type JWTConfig struct {
	Secret     string   `yaml:"secret"`
	AccessTTL  Duration `yaml:"access_ttl"`  // e.g. "15m"
	RefreshTTL Duration `yaml:"refresh_ttl"` // e.g. "7d"
}

type EmailConfig struct {
	SMTPHost string `yaml:"smtp_host"`
	SMTPPort int    `yaml:"smtp_port"`
	FromAddr string `yaml:"from_addr"`
	FromName string `yaml:"from_name"`
}

type ObservabilityConfig struct {
	PrometheusPort int       `yaml:"prometheus_port"`
	OTELEndpoint   string    `yaml:"otel_endpoint"`
	LokiEndpoint   string    `yaml:"loki_endpoint"`
	SentryDSN      string    `yaml:"sentry_dsn"`
	Telemetry      TelemetryConfig `yaml:"telemetry"`
}

type TelemetryConfig struct {
	Enabled      bool    `yaml:"enabled"`
	OTLPEndpoint string  `yaml:"otlp_endpoint"`
	SamplingRate float64 `yaml:"sampling_rate"`
}

type AgentGatewayConfig struct {
	Addr   string `yaml:"addr"`
	CACert string `yaml:"ca_cert"`
}

// Duration is a time.Duration that unmarshals from YAML strings like "15m", "7d", "24h".
// Extends Go's standard format with "d" for days.
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	dur, err := parseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("config: invalid duration %q: %w", value.Value, err)
	}
	d.Duration = dur
	return nil
}

func (d Duration) MarshalYAML() (any, error) {
	return d.Duration.String(), nil
}

// parseDuration extends time.ParseDuration with "d" (days) support.
func parseDuration(s string) (time.Duration, error) {
	if dayStr, ok := strings.CutSuffix(s, "d"); ok {
		days, err := strconv.Atoi(dayStr)
		if err != nil {
			return 0, fmt.Errorf("invalid day value %q", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// Load reads and parses the YAML config file at path.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("config: open %q: %w", path, err)
	}
	defer f.Close()

	var cfg Config
	dec := yaml.NewDecoder(f)
	dec.KnownFields(false) // ignore unknown keys for forward-compat
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("config: decode %q: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config: validate: %w", err)
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
	if c.Database.Main.DSN == "" {
		return fmt.Errorf("database.main.dsn is required")
	}
	if c.Redis.Addr == "" {
		return fmt.Errorf("redis.addr is required")
	}
	if c.JWT.Secret == "" {
		return fmt.Errorf("jwt.secret is required")
	}
	return nil
}

// IsDev reports whether this is a development environment.
func (c *Config) IsDev() bool { return c.Server.Env == "development" }

// IsProd reports whether this is a production environment.
func (c *Config) IsProd() bool { return c.Server.Env == "production" }
