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
	RateLimit     RateLimitConfig     `yaml:"rate_limit"`
	StatusProbe   StatusProbeConfig   `yaml:"status_probe"`

	// CertSvcURL is the base URL of cert-svc (apps/cert-svc) used by
	// apps/api to reverse-proxy /v1/cert/* and /v1/admin/cert/* into
	// the certificate platform. Empty disables the proxy and the
	// stub handlers in apps/api/internal/handler/cert.go take over
	// (list endpoints return empty arrays, mutations return 501).
	CertSvcURL string `yaml:"cert_svc_url"`

	// CertSvc holds non-secret runtime settings for apps/cert-svc.
	// cert-svc reads this section when CERT_SVC_CONFIG or IDCD_CONFIG
	// points at this file. Individual values are overridable via CERT_* env vars.
	CertSvc CertSvcServiceConfig `yaml:"cert_svc"`

	// Attest holds non-secret runtime settings for apps/attest.
	// attest reads this section when ATTEST_CONFIG or IDCD_CONFIG
	// points at this file. Individual values are overridable via ATTEST_* env vars.
	Attest AttestServiceConfig `yaml:"attest"`
}

// ── Service-specific config sections ──────────────────────────────────────

// CertSvcServiceConfig holds non-secret runtime settings for apps/cert-svc.
type CertSvcServiceConfig struct {
	Port         int    `yaml:"port"`
	MetricsPort  int    `yaml:"metrics_port"`
	Env          string `yaml:"env"`
	LEEnv        string `yaml:"le_env"`
	LogLevel     string `yaml:"log_level"`
	AccountEmail string `yaml:"acme_account_email"`
	// Database holds cert-svc's own Postgres DSN (separate from the shared api DB).
	Database     ServiceDBConfig  `yaml:"database"`
	// Redis holds cert-svc's Redis settings (may differ from api Redis instance).
	Redis        RedisConfig      `yaml:"redis"`
	// Vault selects the KMS backend for certificate private-key operations.
	Vault        KMSVaultConfig   `yaml:"vault"`
	// ZeroSSLEABKID is the External Account Binding key ID issued by ZeroSSL.
	// Both ZeroSSLEABKID and ZeroSSLEABHMACKey must be set to enable the adapter.
	ZeroSSLEABKID    string `yaml:"zerossl_eab_kid"`
	ZeroSSLEABHMACKey string `yaml:"zerossl_eab_hmac_key"`
	// BuypassEnv enables the Buypass Go SSL CA: "production" | "staging" | "".
	BuypassEnv string `yaml:"buypass_env"`
}

// AttestServiceConfig holds non-secret runtime settings for apps/attest.
type AttestServiceConfig struct {
	Port         int    `yaml:"port"`
	Env          string `yaml:"env"`
	LogLevel     string `yaml:"log_level"`
	// Database holds attest-svc's own Postgres DSN.
	Database     ServiceDBConfig `yaml:"database"`
	// Redis holds attest-svc's Redis settings.
	Redis        RedisConfig     `yaml:"redis"`
	// SignBackend selects the KMS signing adapter: "aws" | "aliyun" | "local".
	SignBackend  string          `yaml:"sign_backend"`
	AWSKMS       AWSKMSConfig    `yaml:"awskms"`
	AliKMS       AliKMSConfig    `yaml:"alikms"`
	// LocalKeyPath is the PEM file path for SignBackend=local (dev-only).
	LocalKeyPath    string `yaml:"local_key_path"`
	LocalAlgorithm  string `yaml:"local_algorithm"`
	// TSA configures time-stamping authority providers.
	TSA          TSAConfig       `yaml:"tsa"`
	// S3 configures the WORM object-storage archival bucket.
	S3           S3Config        `yaml:"s3"`
	// VerifyEndpoint is the URL of the Self-Verify Worker (D6).
	VerifyEndpoint string `yaml:"verify_endpoint"`
	// ArchiverBackend selects the verdict archive storage: "local" (default) | "s3".
	ArchiverBackend string `yaml:"archiver_backend"`
	// LocalArchiveDir is the directory for the local archiver (default /var/lib/attest/archive).
	LocalArchiveDir string `yaml:"local_archive_dir"`
	// Refund holds Redis Stream keys for the D5 refund worker.
	Refund       AttestRefundConfig `yaml:"refund"`
}

// ServiceDBConfig holds the database DSN for a service with its own schema.
type ServiceDBConfig struct {
	DSN string `yaml:"dsn"`
}

// KMSVaultConfig picks the vault backend for cert-svc certificate key operations.
type KMSVaultConfig struct {
	// Backend selects the implementation: "envmaster" | "alikms" | "awskms" | "hashivault".
	Backend    string              `yaml:"backend"`
	AliKMS     AliKMSConfig        `yaml:"alikms"`
	AWSKMS     AWSKMSConfig        `yaml:"awskms"`
	HashiVault HashiVaultKMSConfig `yaml:"hashivault"`
}

// AliKMSConfig holds Aliyun KMS credentials and key reference.
type AliKMSConfig struct {
	RegionID        string `yaml:"region_id"`
	AccessKeyID     string `yaml:"access_key_id"`
	AccessKeySecret string `yaml:"access_key_secret"`
	KeyID           string `yaml:"key_id"`
	// Algorithm is optional (default "ECDSA_SHA_256" for attest).
	Algorithm string `yaml:"algorithm"`
}

// AWSKMSConfig holds AWS KMS credentials and key reference.
type AWSKMSConfig struct {
	Region          string `yaml:"region"`
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	KeyID           string `yaml:"key_id"`
	// Algorithm is optional (default "ECDSA_SHA_256" for attest).
	Algorithm string `yaml:"algorithm"`
}

// HashiVaultKMSConfig holds HashiCorp Vault transit-engine settings.
type HashiVaultKMSConfig struct {
	Address    string `yaml:"address"`
	Token      string `yaml:"token"`
	Namespace  string `yaml:"namespace"`
	KeyName    string `yaml:"key_name"`
	// MountPath defaults to "transit" in the adapter.
	MountPath string `yaml:"mount_path"`
}

// TSAConfig holds time-stamping authority configuration for attest-svc.
type TSAConfig struct {
	// Providers is an ordered list of TSA provider names: digicert, globalsign, freetsa.
	// Primary provider first; Multi-TSA tries in order.
	Providers []string `yaml:"providers"`
}

// S3Config holds object-storage settings for WORM archival in attest-svc.
type S3Config struct {
	Endpoint  string `yaml:"endpoint"`
	Bucket    string `yaml:"bucket"`
	Region    string `yaml:"region"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	// Object-lock retention fields (attest WORM archive only).
	ObjectLockMode string `yaml:"object_lock_mode"` // "COMPLIANCE" | "GOVERNANCE"; default "COMPLIANCE"
	ObjectLockDays int    `yaml:"object_lock_days"` // retention days; default 3650 (10 years)
	KeyPrefix      string `yaml:"key_prefix"`       // optional object key prefix
}

// AttestRefundConfig holds Redis Stream keys for the D5 refund-worker process.
type AttestRefundConfig struct {
	InitiateStream string `yaml:"initiate_stream"`
	RetryStream    string `yaml:"retry_stream"`
	DelayZoneKey   string `yaml:"delay_zone_key"`
	Group          string `yaml:"group"`
	Consumer       string `yaml:"consumer"`
	// NotifierAddr is the asynq broker used for apology-email enqueue (empty = disabled).
	NotifierAddr  string `yaml:"notifier_addr"`
	NotifierQueue string `yaml:"notifier_queue"`
}

// StatusProbeConfig drives the idcd.com/status data collector. The collector
// HTTP-probes each entry in Services every Interval and writes one row per
// service to status_uptime_5min. Disabled by default so dev environments
// without all services running don't accumulate junk uptime data.
type StatusProbeConfig struct {
	Enabled  bool              `yaml:"enabled"`
	Interval Duration          `yaml:"interval"` // default 5m if unset
	Timeout  Duration          `yaml:"timeout"`  // per-probe HTTP timeout, default 3s
	// Services maps service_key → probe URL. Common keys:
	//   api / cert-svc / gateway / aggregator / notifier / web
	// 2xx response = operational, slow (> degraded_ms) = degraded, else outage.
	Services map[string]string `yaml:"services"`
	// DegradedMs is the response-time threshold above which a 2xx response is
	// classified degraded instead of operational. Default 1000ms.
	DegradedMs int `yaml:"degraded_ms"`
}

// RateLimitConfig tunes the per-flow rate limiters previously hard-coded in
// server.go. Zero-valued fields fall back to the defaults documented on each
// field — production should override them to match the deployed traffic
// profile (e.g. lower auth.max_requests during an active credential-stuffing
// incident, raise twofa.window for legitimately laggy operators).
type RateLimitConfig struct {
	// Auth limits /v1/auth/* (login, register, password reset). Default
	// 5 requests / minute / client IP. Tuned for "human at a login form"
	// rather than scripted clients — automation should authenticate once
	// and reuse the resulting JWT/PAT.
	Auth RateLimitRule `yaml:"auth"`
	// TwoFA limits /v1/account/2fa/{verify,disable}. Default 5 attempts
	// per 15 minutes per authenticated user. A stolen JWT cannot
	// brute-force the 6-digit TOTP space (~10^6) without tripping this.
	TwoFA RateLimitRule `yaml:"twofa"`
}

// RateLimitRule pairs a sliding window with a request cap. Both fields must
// be > 0 to override the consuming code's default; partial overrides fall back
// to the default for the missing field, so an operator who only wants to
// lower the cap can omit window: entirely.
type RateLimitRule struct {
	Window      Duration `yaml:"window"`
	MaxRequests int64    `yaml:"max_requests"`
}

// OrDefault returns the rule's values, substituting the given defaults for
// any zero/missing field. Keeps the call site in server.go branch-free.
func (r RateLimitRule) OrDefault(defWindow time.Duration, defMax int64) (time.Duration, int64) {
	w := r.Window.Duration
	if w <= 0 {
		w = defWindow
	}
	m := r.MaxRequests
	if m <= 0 {
		m = defMax
	}
	return w, m
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
	// Sentinel mode (optional). When MasterName and SentinelAddrs are both
	// set, clients should use FailoverClient instead of single-node Client.
	MasterName       string   `yaml:"master_name"`
	SentinelAddrs    []string `yaml:"sentinel_addrs"`
	SentinelPassword string   `yaml:"sentinel_password"`
	// Optional connection tuning (zero values → go-redis defaults).
	DialTimeout  Duration `yaml:"dial_timeout"`
	ReadTimeout  Duration `yaml:"read_timeout"`
	WriteTimeout Duration `yaml:"write_timeout"`
	PoolSize     int      `yaml:"pool_size"`
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
// Accepts pure day forms ("7d") and combined forms ("7d12h", "1d30m").
func parseDuration(s string) (time.Duration, error) {
	if idx := strings.Index(s, "d"); idx > 0 {
		if days, err := strconv.Atoi(s[:idx]); err == nil {
			rest := s[idx+1:]
			if rest == "" {
				return time.Duration(days) * 24 * time.Hour, nil
			}
			if sub, err := time.ParseDuration(rest); err == nil {
				return time.Duration(days)*24*time.Hour + sub, nil
			}
		}
	}
	return time.ParseDuration(s)
}

// LoadRaw reads and parses YAML at path without validating required fields.
// Use when a service only needs its own named section (e.g. cert_svc: or
// attest:) and does not consume the shared database / redis / jwt fields.
func LoadRaw(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("config: open %q: %w", path, err)
	}
	defer f.Close()

	var cfg Config
	dec := yaml.NewDecoder(f)
	dec.KnownFields(false)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("config: decode %q: %w", path, err)
	}
	return &cfg, nil
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
