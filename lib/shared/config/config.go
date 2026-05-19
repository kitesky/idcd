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
	Encryption    EncryptionConfig    `yaml:"encryption"`
	Payment       PaymentConfig       `yaml:"payment"`
}

// PaymentConfig holds credentials for the payment aggregation platform.
type PaymentConfig struct {
	// Enabled switches from StubProvider to the real PaymentHubProvider.
	Enabled bool `yaml:"enabled"`
	// BaseURL is the payment platform base URL, e.g. "https://pay.example.com".
	BaseURL string `yaml:"base_url"`
	// APIKey is the pk_xxx key from the platform admin panel.
	APIKey string `yaml:"api_key"`
	// APISecret is the sk_xxx secret used to sign requests.
	APISecret string `yaml:"api_secret"`
	// WebhookSecret is the callback secret used to verify incoming webhooks.
	WebhookSecret string `yaml:"webhook_secret"`
	// Channel is the default payment channel when the user does not specify one:
	// "alipay" or "wechat_pay". Method is derived automatically per channel.
	Channel string `yaml:"channel"`
	// Currency is the ISO 4217 code: "CNY", "USD".
	Currency string `yaml:"currency"`
}

// EncryptionConfig holds keys for field-level at-rest encryption.
type EncryptionConfig struct {
	// FieldKey is a 64-character hex-encoded 32-byte AES-256 key.
	// Generate with: openssl rand -hex 32
	// Required in production; if empty a dev-only all-zeros fallback is used.
	FieldKey string `yaml:"field_key"`
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
	// AdminIPAllowlist gates /internal/admin/* by direct or proxy-forwarded
	// client IP. Each entry is a CIDR ("10.0.0.0/8") or single IP. Empty
	// allows any IP (dev default). Production should pin VPN / office /
	// bastion ranges so a leaked admin_token alone is not enough to walk
	// in from the open internet.
	AdminIPAllowlist []string `yaml:"admin_ip_allowlist"`
	// AdminOrigins lists Origin / Referer hosts permitted to call
	// /internal/admin/* with a mutating method. Empty disables the check
	// (Bearer auth alone is considered enough — appropriate when callers
	// are server-to-server tools without a browser context). Set this when
	// you wire a browser-based admin console so a hijacked tab on another
	// origin can't ride a logged-in admin session.
	AdminOrigins []string `yaml:"admin_origins"`
	RPID         string   `yaml:"rp_id"`        // WebAuthn Relying Party ID (default: idcd.com)
	AppBaseURL   string   `yaml:"app_base_url"` // Frontend base URL for email deep-links, e.g. "https://app.idcd.com"
	// PublicAPIURL is the externally-reachable base URL for the API service
	// itself (e.g. "https://api.idcd.com"). Used to build webhook callback
	// URLs that the payment platform calls back into. MUST NOT be derived
	// from request headers like Origin/Host — a tampered client header
	// would redirect refund / payment webhooks to an attacker URL.
	PublicAPIURL string `yaml:"public_api_url"`
	// PinProbeNode forces every unspecified-node probe / diagnose task to be
	// routed to this node_id, bypassing the normal random-balance over all
	// active enrolled_nodes. Intended for shared-redis dev setups where
	// multiple gateways consume the same probe.tasks stream and the random
	// pick routes ~50% of tasks to nodes whose agent is connected to a
	// different gateway — leaving them stuck in PEL. Leave empty in
	// production so the load-balancer behaviour is preserved.
	PinProbeNode string `yaml:"pin_probe_node"`
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
	Addr        string `yaml:"addr"`
	CACert      string `yaml:"ca_cert"`
	PublicWSS   string `yaml:"public_wss"`    // returned to enrolled agents, e.g. wss://gateway.idcd.com
	InternalURL string `yaml:"internal_url"`  // internal HTTP base URL, e.g. http://gateway:8081
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
	if c.Server.AdminToken == "" {
		return fmt.Errorf("server.admin_token is required")
	}
	// Recommend `openssl rand -hex 32` (= 64 hex chars / 32 bytes entropy).
	// 32 bytes is the Go crypto/rand standard for HMAC-grade secrets and
	// matches the encryption.field_key requirement above; weaker tokens
	// expose /internal/admin/* to feasible online brute force given how
	// rarely operators rotate them. Empty was already rejected above.
	if len(c.Server.AdminToken) < 64 {
		return fmt.Errorf("server.admin_token must be at least 64 chars (use `openssl rand -hex 32`)")
	}
	return nil
}

// IsDev reports whether this is a development environment.
func (c *Config) IsDev() bool { return c.Server.Env == "development" }

// IsProd reports whether this is a production environment.
func (c *Config) IsProd() bool { return c.Server.Env == "production" }
