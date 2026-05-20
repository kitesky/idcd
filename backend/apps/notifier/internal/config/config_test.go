package config

import (
	"os"
	"path/filepath"
	"testing"
)

// baseYAML is the minimum shared.Config snippet needed so the embedded
// validator (database.main.dsn / redis.addr / jwt.secret / server.admin_token)
// is satisfied.  Tests append notifier-specific sections to this.
const baseYAML = `
database:
  main:
    dsn: "postgresql://user:pass@localhost/idcd"
redis:
  addr: "localhost:6379"
jwt:
  secret: "test-secret"
server:
  admin_token: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  env: "development"
`

func writeConfig(t *testing.T, yaml string) string {
	t.Helper()
	tmp := t.TempDir()
	p := filepath.Join(tmp, "config.yaml")
	if err := os.WriteFile(p, []byte(yaml), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return p
}

func TestLoad_Defaults(t *testing.T) {
	// No notifier section at all → setDefaults must populate everything.
	cfg, err := Load(writeConfig(t, baseYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Notifier.Workers != 4 {
		t.Errorf("Workers = %d, want default 4", cfg.Notifier.Workers)
	}
	if cfg.Notifier.AsynqDSN != "localhost:6379" {
		t.Errorf("AsynqDSN = %q, want default localhost:6379", cfg.Notifier.AsynqDSN)
	}
	if cfg.Notifier.SMTP.Port != 587 {
		t.Errorf("SMTP.Port = %d, want default 587", cfg.Notifier.SMTP.Port)
	}
	if cfg.Notifier.SMTP.FromName != "idcd" {
		t.Errorf("SMTP.FromName = %q, want default idcd", cfg.Notifier.SMTP.FromName)
	}
	if cfg.Notifier.SES.Region != "us-east-1" {
		t.Errorf("SES.Region = %q, want default us-east-1", cfg.Notifier.SES.Region)
	}

	// Critical D5 wiring: the billing queue must be present in defaults so
	// payment:refund_retry tasks are picked up out of the box.
	if _, ok := cfg.Notifier.Queues["billing"]; !ok {
		t.Errorf("default Queues missing 'billing' — D5 refund retry won't fire: %v", cfg.Notifier.Queues)
	}
	if cfg.Notifier.Queues["billing"] != 5 {
		t.Errorf("Queues[billing] priority = %d, want 5", cfg.Notifier.Queues["billing"])
	}
	if _, ok := cfg.Notifier.Queues["notifier:default"]; !ok {
		t.Errorf("default Queues missing 'notifier:default'")
	}
}

func TestLoad_OverridesPreserveBillingQueue(t *testing.T) {
	// User supplies their own queues map without 'billing' — the default
	// should be injected so refund retry never silently breaks.
	yaml := baseYAML + `
notifier:
  workers: 8
  queues:
    custom: 3
`
	cfg, err := Load(writeConfig(t, yaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Notifier.Workers != 8 {
		t.Errorf("Workers = %d, want 8 (user override)", cfg.Notifier.Workers)
	}
	if cfg.Notifier.Queues["custom"] != 3 {
		t.Errorf("Queues[custom] = %d, want 3 (user override)", cfg.Notifier.Queues["custom"])
	}
	if cfg.Notifier.Queues["billing"] != 5 {
		t.Errorf("Queues[billing] = %d, want injected default 5", cfg.Notifier.Queues["billing"])
	}
}

func TestLoad_UserBillingQueueOverrideRespected(t *testing.T) {
	// If the user EXPLICITLY sets a billing priority, honour it — we only
	// inject the default when it's missing.
	yaml := baseYAML + `
notifier:
  queues:
    billing: 9
    notifier:default: 1
`
	cfg, err := Load(writeConfig(t, yaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Notifier.Queues["billing"] != 9 {
		t.Errorf("Queues[billing] = %d, want 9 (user override)", cfg.Notifier.Queues["billing"])
	}
}

func TestLoad_NotifierSectionDecoded(t *testing.T) {
	// Verify the YAML decoder actually populates the notifier section
	// (regression: prior to 2026-05-14 the section was silently ignored).
	yaml := baseYAML + `
notifier:
  asynq_redis_dsn: "redis://:pw@notifier-redis:6379/2"
  workers: 16
  smtp:
    host: smtp.example.com
    port: 465
    from: noreply@example.com
    from_name: Example
  ses:
    region: ap-southeast-1
    access_key: AKIA
    secret_key: SECRET
    from: noreply@example.com
`
	cfg, err := Load(writeConfig(t, yaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Notifier.AsynqDSN != "redis://:pw@notifier-redis:6379/2" {
		t.Errorf("AsynqDSN = %q", cfg.Notifier.AsynqDSN)
	}
	if cfg.Notifier.Workers != 16 {
		t.Errorf("Workers = %d", cfg.Notifier.Workers)
	}
	if cfg.Notifier.SMTP.Host != "smtp.example.com" {
		t.Errorf("SMTP.Host = %q", cfg.Notifier.SMTP.Host)
	}
	if cfg.Notifier.SMTP.Port != 465 {
		t.Errorf("SMTP.Port = %d", cfg.Notifier.SMTP.Port)
	}
	if cfg.Notifier.SES.Region != "ap-southeast-1" {
		t.Errorf("SES.Region = %q", cfg.Notifier.SES.Region)
	}
}

// TestLoad_CertStreamDefaults covers the S2 W8 defaults: with no notifier
// section the cert consumer is enabled and bound to the default stream /
// consumer group that match cert-svc's producer.
func TestLoad_CertStreamDefaults(t *testing.T) {
	cfg, err := Load(writeConfig(t, baseYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Notifier.CertStreamEnabledOrDefault(); !got {
		t.Errorf("CertStreamEnabledOrDefault() = %v, want true (default on)", got)
	}
	if cfg.Notifier.CertStreamName != "cert:notifications" {
		t.Errorf("CertStreamName = %q, want default cert:notifications", cfg.Notifier.CertStreamName)
	}
	if cfg.Notifier.CertConsumerGroup != "cert-notifier" {
		t.Errorf("CertConsumerGroup = %q, want default cert-notifier", cfg.Notifier.CertConsumerGroup)
	}
}

// TestLoad_CertStreamExplicitDisable covers an operator explicitly turning
// the consumer off via cert_stream_enabled: false.
func TestLoad_CertStreamExplicitDisable(t *testing.T) {
	yaml := baseYAML + `
notifier:
  cert_stream_enabled: false
`
	cfg, err := Load(writeConfig(t, yaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Notifier.CertStreamEnabledOrDefault(); got {
		t.Errorf("CertStreamEnabledOrDefault() = %v, want false (explicit disable)", got)
	}
}

// TestLoad_CertStreamExplicitEnableTrue covers explicit cert_stream_enabled:
// true. This is a no-op vs the default but exercises the pointer non-nil
// path.
func TestLoad_CertStreamExplicitEnableTrue(t *testing.T) {
	yaml := baseYAML + `
notifier:
  cert_stream_enabled: true
`
	cfg, err := Load(writeConfig(t, yaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Notifier.CertStreamEnabled == nil {
		t.Fatal("CertStreamEnabled should be non-nil after explicit true")
	}
	if *cfg.Notifier.CertStreamEnabled != true {
		t.Errorf("CertStreamEnabled = %v, want true", *cfg.Notifier.CertStreamEnabled)
	}
	if !cfg.Notifier.CertStreamEnabledOrDefault() {
		t.Errorf("CertStreamEnabledOrDefault() = false, want true")
	}
}

// TestLoad_CertStreamOverrideNames covers operator overrides for the stream
// name + consumer group (e.g. environment-scoped streams).
func TestLoad_CertStreamOverrideNames(t *testing.T) {
	yaml := baseYAML + `
notifier:
  cert_stream_name: "staging:cert:notifications"
  cert_consumer_group: "cert-notifier-staging"
`
	cfg, err := Load(writeConfig(t, yaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Notifier.CertStreamName != "staging:cert:notifications" {
		t.Errorf("CertStreamName = %q", cfg.Notifier.CertStreamName)
	}
	if cfg.Notifier.CertConsumerGroup != "cert-notifier-staging" {
		t.Errorf("CertConsumerGroup = %q", cfg.Notifier.CertConsumerGroup)
	}
}

// TestCertStreamEnabledOrDefault_NilReceiver guards against a nil pointer
// (matters when the embedded config wasn't constructed via Load — defensive
// for callers that allocate NotifierConfig{} directly in tests).
func TestCertStreamEnabledOrDefault_NilReceiver(t *testing.T) {
	var n *NotifierConfig
	if got := n.CertStreamEnabledOrDefault(); !got {
		t.Errorf("nil receiver: got %v, want true (default on)", got)
	}
}

func TestMustLoad_PanicsOnError(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("MustLoad: expected panic on missing file")
		}
	}()
	MustLoad("/nonexistent/path/to/config.yaml")
}
