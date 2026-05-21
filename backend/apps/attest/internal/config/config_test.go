package config

import (
	"os"
	"path/filepath"
	"testing"
)

// noYAML suppresses YAML loading for tests that only exercise env-var logic.
func noYAML(t *testing.T) {
	t.Helper()
	t.Setenv("ATTEST_CONFIG", "")
}

func TestLoad_Defaults(t *testing.T) {
	noYAML(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != defaultPort {
		t.Errorf("Port = %d, want %d", cfg.Port, defaultPort)
	}
	if cfg.Env != defaultEnv {
		t.Errorf("Env = %q, want %q", cfg.Env, defaultEnv)
	}
	if cfg.AWSKMSAlgorithm != defaultAlgorithm {
		t.Errorf("AWSKMSAlgorithm = %q", cfg.AWSKMSAlgorithm)
	}
	if len(cfg.TSAProviders) != 2 || cfg.TSAProviders[0] != "digicert" {
		t.Errorf("TSAProviders = %v", cfg.TSAProviders)
	}
}

func TestLoad_SignBackendAWS_Valid(t *testing.T) {
	t.Setenv(envSignBackend, "aws")
	t.Setenv(envAWSKMSRegion, "us-east-1")
	t.Setenv(envAWSKMSKeyID, "alias/cert-sign")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SignBackend != SignBackendAWS {
		t.Errorf("SignBackend = %q", cfg.SignBackend)
	}
}

func TestLoad_SignBackendAWS_MissingKeyID(t *testing.T) {
	t.Setenv(envSignBackend, "aws")
	t.Setenv(envAWSKMSRegion, "us-east-1")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing AWSKMSKeyID")
	}
}

func TestLoad_SignBackendAWS_HalfCredentials(t *testing.T) {
	t.Setenv(envSignBackend, "aws")
	t.Setenv(envAWSKMSRegion, "us-east-1")
	t.Setenv(envAWSKMSKeyID, "alias/x")
	t.Setenv(envAWSKMSAccessKeyID, "AKIDfake")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for half credential")
	}
}

func TestLoad_SignBackendAliyun_Valid(t *testing.T) {
	t.Setenv(envSignBackend, "aliyun")
	t.Setenv(envAliKMSRegionID, "cn-hangzhou")
	t.Setenv(envAliKMSAccessKeyID, "ak-id")
	t.Setenv(envAliKMSAccessKeySecret, "ak-secret")
	t.Setenv(envAliKMSKeyID, "alias/cert-sign")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SignBackend != SignBackendAliyun {
		t.Errorf("SignBackend = %q", cfg.SignBackend)
	}
}

func TestLoad_SignBackendUnknown(t *testing.T) {
	t.Setenv(envSignBackend, "azure")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestLoad_TSAProvidersOverride(t *testing.T) {
	t.Setenv(envTSAProviders, "globalsign , digicert ,, ntsc")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []string{"globalsign", "digicert", "ntsc"}
	if len(cfg.TSAProviders) != len(want) {
		t.Fatalf("TSAProviders = %v, want %v", cfg.TSAProviders, want)
	}
	for i := range want {
		if cfg.TSAProviders[i] != want[i] {
			t.Errorf("TSAProviders[%d] = %q, want %q", i, cfg.TSAProviders[i], want[i])
		}
	}
}

func TestLoad_TSAProvidersEmpty(t *testing.T) {
	t.Setenv(envTSAProviders, " , , ")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for empty TSA list")
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	t.Setenv(envPort, "999999")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid port")
	}
}

func TestLoad_PaymentHubWebhookSecret(t *testing.T) {
	t.Setenv(envPaymentHubWebhookSecret, "whsec_test")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.PaymentHubWebhookSecret != "whsec_test" {
		t.Errorf("PaymentHubWebhookSecret = %q", cfg.PaymentHubWebhookSecret)
	}
}

func TestLoad_PaymentHubWebhookSecret_Default(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.PaymentHubWebhookSecret != "" {
		t.Errorf("expected empty PaymentHubWebhookSecret default, got %q", cfg.PaymentHubWebhookSecret)
	}
}

func TestAddr(t *testing.T) {
	c := &Config{Port: 9090}
	if got := c.Addr(); got != ":9090" {
		t.Errorf("Addr() = %q", got)
	}
}

// TestLoad_ValidPort exercises the success branch of the port override
// (defaultPort is hit by every other test, but the "valid integer port"
// branch needs an explicit override).
func TestLoad_ValidPort(t *testing.T) {
	t.Setenv(envPort, "12345")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 12345 {
		t.Errorf("Port = %d, want 12345", cfg.Port)
	}
}

func TestLoad_PortNonNumeric(t *testing.T) {
	t.Setenv(envPort, "abc")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for non-numeric port")
	}
}

// TestLoad_AllScalarOverrides hits every "if v := strings.TrimSpace(...); v != ""
// {cfg.X = v}" branch in one shot so the per-field overrides all count
// as covered.
func TestLoad_AllScalarOverrides(t *testing.T) {
	t.Setenv(envEnv, "production")
	t.Setenv(envLogLevel, "DEBUG")
	t.Setenv(envDB, "postgres://example")
	t.Setenv(envRedis, "redis:6379")
	t.Setenv(envS3Bucket, "bucket-x")
	t.Setenv(envVerifyEndpoint, "https://attest.example/verify")
	t.Setenv(envAWSKMSAlgorithm, "ECDSA_SHA_384")
	t.Setenv(envAliKMSAlgorithm, "ECDSA_SHA_384")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Env != "production" {
		t.Errorf("Env = %q", cfg.Env)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q (expect lowercased)", cfg.LogLevel)
	}
	if cfg.DatabaseDSN != "postgres://example" {
		t.Errorf("DatabaseDSN = %q", cfg.DatabaseDSN)
	}
	if cfg.RedisAddr != "redis:6379" {
		t.Errorf("RedisAddr = %q", cfg.RedisAddr)
	}
	if cfg.S3Bucket != "bucket-x" {
		t.Errorf("S3Bucket = %q", cfg.S3Bucket)
	}
	if cfg.VerifyEndpoint != "https://attest.example/verify" {
		t.Errorf("VerifyEndpoint = %q", cfg.VerifyEndpoint)
	}
	if cfg.AWSKMSAlgorithm != "ECDSA_SHA_384" {
		t.Errorf("AWSKMSAlgorithm = %q", cfg.AWSKMSAlgorithm)
	}
	if cfg.AliKMSAlgorithm != "ECDSA_SHA_384" {
		t.Errorf("AliKMSAlgorithm = %q", cfg.AliKMSAlgorithm)
	}
}

// TestLoad_AWSKMSFullCredentials covers the "both halves set" branch of
// the credential check (existing TestLoad_SignBackendAWS_HalfCredentials
// only exercises the mismatch error branch).
func TestLoad_AWSKMSFullCredentials(t *testing.T) {
	t.Setenv(envSignBackend, "aws")
	t.Setenv(envAWSKMSRegion, "us-east-1")
	t.Setenv(envAWSKMSKeyID, "alias/x")
	t.Setenv(envAWSKMSAccessKeyID, "AKID")
	t.Setenv(envAWSKMSSecretAccessKey, "SECRET")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AWSKMSAccessKeyID != "AKID" || cfg.AWSKMSSecretAccessKey != "SECRET" {
		t.Errorf("AWS creds not populated: %+v", cfg)
	}
}

// TestLoad_SignBackendAliyun_MissingFields covers the validation branch
// for Aliyun when partial fields are supplied. Each subcase omits one
// required field.
func TestLoad_SignBackendAliyun_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		set  map[string]string
	}{
		{"no region", map[string]string{
			envAliKMSAccessKeyID:     "ak",
			envAliKMSAccessKeySecret: "sk",
			envAliKMSKeyID:           "key",
		}},
		{"no access key id", map[string]string{
			envAliKMSRegionID:        "cn-hangzhou",
			envAliKMSAccessKeySecret: "sk",
			envAliKMSKeyID:           "key",
		}},
		{"no secret", map[string]string{
			envAliKMSRegionID:    "cn-hangzhou",
			envAliKMSAccessKeyID: "ak",
			envAliKMSKeyID:       "key",
		}},
		{"no key id", map[string]string{
			envAliKMSRegionID:        "cn-hangzhou",
			envAliKMSAccessKeyID:     "ak",
			envAliKMSAccessKeySecret: "sk",
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(envSignBackend, "aliyun")
			for k, v := range c.set {
				t.Setenv(k, v)
			}
			_, err := Load()
			if err == nil {
				t.Fatal("expected error for partial aliyun config")
			}
		})
	}
}

func TestLoad_YAMLOverlay(t *testing.T) {
	const yamlContent = `
attest:
  port: 8282
  env: "staging"
  log_level: "warn"
  database:
    dsn: "postgres://attest:pw@yaml-db:5432/attest"
  redis:
    addr: "yaml-redis:6379"
    password: "yaml-pass"
  sign_backend: "aliyun"
  alikms:
    region_id: "cn-beijing"
    access_key_id: "yaml-ak"
    access_key_secret: "yaml-sk"
    key_id: "alias/yaml-key"
  tsa:
    providers:
      - "globalsign"
      - "digicert"
  s3:
    bucket: "yaml-bucket"
    region: "us-east-1"
  verify_endpoint: "https://verify.example"
  refund:
    initiate_stream: "yaml_initiate"
    retry_stream: "yaml_retry"
    group: "yaml-group"
`
	tmp := filepath.Join(t.TempDir(), "test.yaml")
	if err := os.WriteFile(tmp, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	t.Setenv("ATTEST_CONFIG", tmp)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 8282 {
		t.Errorf("Port = %d, want 8282", cfg.Port)
	}
	if cfg.Env != "staging" {
		t.Errorf("Env = %q, want staging", cfg.Env)
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want warn", cfg.LogLevel)
	}
	if cfg.DatabaseDSN != "postgres://attest:pw@yaml-db:5432/attest" {
		t.Errorf("DatabaseDSN = %q", cfg.DatabaseDSN)
	}
	if cfg.RedisAddr != "yaml-redis:6379" {
		t.Errorf("RedisAddr = %q", cfg.RedisAddr)
	}
	if cfg.SignBackend != SignBackendAliyun {
		t.Errorf("SignBackend = %q", cfg.SignBackend)
	}
	if cfg.AliKMSRegionID != "cn-beijing" {
		t.Errorf("AliKMSRegionID = %q", cfg.AliKMSRegionID)
	}
	if len(cfg.TSAProviders) != 2 || cfg.TSAProviders[0] != "globalsign" {
		t.Errorf("TSAProviders = %v", cfg.TSAProviders)
	}
	if cfg.S3Bucket != "yaml-bucket" {
		t.Errorf("S3Bucket = %q", cfg.S3Bucket)
	}
	if cfg.VerifyEndpoint != "https://verify.example" {
		t.Errorf("VerifyEndpoint = %q", cfg.VerifyEndpoint)
	}
	if cfg.RefundInitiateStream != "yaml_initiate" {
		t.Errorf("RefundInitiateStream = %q", cfg.RefundInitiateStream)
	}
	if cfg.RefundGroup != "yaml-group" {
		t.Errorf("RefundGroup = %q", cfg.RefundGroup)
	}
}

func TestLoad_YAMLEnvVarOverridesYAML(t *testing.T) {
	const yamlContent = `
attest:
  port: 8282
  database:
    dsn: "postgres://from-yaml"
`
	tmp := filepath.Join(t.TempDir(), "test.yaml")
	if err := os.WriteFile(tmp, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	t.Setenv("ATTEST_CONFIG", tmp)
	t.Setenv(envPort, "9999")
	t.Setenv(envDB, "postgres://from-env")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 9999 {
		t.Errorf("Port = %d; env var should beat YAML", cfg.Port)
	}
	if cfg.DatabaseDSN != "postgres://from-env" {
		t.Errorf("DatabaseDSN = %q; env var should beat YAML", cfg.DatabaseDSN)
	}
}

func TestLoad_YAMLMissingFileSkipped(t *testing.T) {
	t.Setenv("ATTEST_CONFIG", "/nonexistent/path/attest.yaml")
	_, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v; missing YAML should be silently skipped", err)
	}
}
