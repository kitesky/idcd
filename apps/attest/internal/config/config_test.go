package config

import "testing"

func TestLoad_Defaults(t *testing.T) {
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

func TestLoad_PaddleWebhookSecret(t *testing.T) {
	t.Setenv(envPaddleWebhookSecret, "whsec_test")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.PaddleWebhookSecret != "whsec_test" {
		t.Errorf("PaddleWebhookSecret = %q", cfg.PaddleWebhookSecret)
	}
}

func TestLoad_PaddleWebhookSecret_Default(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.PaddleWebhookSecret != "" {
		t.Errorf("expected empty PaddleWebhookSecret default, got %q", cfg.PaddleWebhookSecret)
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
