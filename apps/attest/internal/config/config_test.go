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
