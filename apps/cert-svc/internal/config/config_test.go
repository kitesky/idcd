package config

import (
	"encoding/base64"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Make sure no env is leaking into the test.
	for _, k := range []string{envPort, envDB, envRedis, envLogLevel, envEnv} {
		t.Setenv(k, "")
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != defaultPort {
		t.Errorf("Port = %d, want %d", cfg.Port, defaultPort)
	}
	if cfg.DatabaseDSN != defaultDB {
		t.Errorf("DatabaseDSN = %q, want %q", cfg.DatabaseDSN, defaultDB)
	}
	if cfg.RedisURL != defaultRedis {
		t.Errorf("RedisURL = %q, want %q", cfg.RedisURL, defaultRedis)
	}
	if cfg.LogLevel != defaultLogLevel {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, defaultLogLevel)
	}
	if cfg.Env != defaultEnv {
		t.Errorf("Env = %q, want %q", cfg.Env, defaultEnv)
	}
	if got, want := cfg.Addr(), ":8080"; got != want {
		t.Errorf("Addr() = %q, want %q", got, want)
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv(envPort, "9090")
	t.Setenv(envDB, "postgres://u:p@db:5432/cert")
	t.Setenv(envRedis, "redis://cache:6380/3")
	t.Setenv(envLogLevel, "DEBUG")
	t.Setenv(envEnv, "production")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
	if cfg.DatabaseDSN != "postgres://u:p@db:5432/cert" {
		t.Errorf("DatabaseDSN = %q", cfg.DatabaseDSN)
	}
	if cfg.RedisURL != "redis://cache:6380/3" {
		t.Errorf("RedisURL = %q", cfg.RedisURL)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q (lowercased)", cfg.LogLevel, "debug")
	}
	if cfg.Env != "production" {
		t.Errorf("Env = %q", cfg.Env)
	}
	if got, want := cfg.Addr(), ":9090"; got != want {
		t.Errorf("Addr() = %q, want %q", got, want)
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	cases := []struct {
		name string
		val  string
	}{
		{"non-numeric", "abc"},
		{"zero", "0"},
		{"negative", "-1"},
		{"too-large", "70000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envPort, tc.val)
			if _, err := Load(); err == nil {
				t.Errorf("Load() with %s=%q expected error, got nil", envPort, tc.val)
			}
		})
	}
}

func TestLoad_DownloadSecret_DecodesBase64(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i)
	}
	t.Setenv(envDownloadSecret, base64.StdEncoding.EncodeToString(raw))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.DownloadSecret) != 32 {
		t.Errorf("DownloadSecret len = %d, want 32", len(cfg.DownloadSecret))
	}
}

func TestLoad_DownloadSecret_RejectsBadBase64(t *testing.T) {
	t.Setenv(envDownloadSecret, "!!!not-base64!!!")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected error for bad base64, got nil")
	}
}

func TestLoad_DownloadSecret_RejectsTooShort(t *testing.T) {
	t.Setenv(envDownloadSecret, base64.StdEncoding.EncodeToString([]byte("short")))
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected error for short secret, got nil")
	}
}

func TestLoad_DownloadSecret_DefaultsEmpty(t *testing.T) {
	t.Setenv(envDownloadSecret, "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.DownloadSecret) != 0 {
		t.Errorf("DownloadSecret should default to empty, got %d bytes", len(cfg.DownloadSecret))
	}
}

func TestLoad_MultiCA_ZeroSSLBothSet(t *testing.T) {
	t.Setenv(envZeroSSLEABKID, "kid-abc")
	t.Setenv(envZeroSSLEABHMACKey, "hmac-xyz")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ZeroSSLEABKID != "kid-abc" {
		t.Errorf("ZeroSSLEABKID = %q, want %q", cfg.ZeroSSLEABKID, "kid-abc")
	}
	if cfg.ZeroSSLEABHMACKey != "hmac-xyz" {
		t.Errorf("ZeroSSLEABHMACKey = %q, want %q", cfg.ZeroSSLEABHMACKey, "hmac-xyz")
	}
}

func TestLoad_MultiCA_ZeroSSLOnlyKID(t *testing.T) {
	// Only kid set — Load preserves the partial state; the wiring in
	// main.go decides whether to register the adapter.
	t.Setenv(envZeroSSLEABKID, "kid-only")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ZeroSSLEABKID != "kid-only" {
		t.Errorf("ZeroSSLEABKID = %q, want %q", cfg.ZeroSSLEABKID, "kid-only")
	}
	if cfg.ZeroSSLEABHMACKey != "" {
		t.Errorf("ZeroSSLEABHMACKey = %q, want empty", cfg.ZeroSSLEABHMACKey)
	}
}

func TestLoad_MultiCA_ZeroSSLOnlyHMAC(t *testing.T) {
	t.Setenv(envZeroSSLEABHMACKey, "hmac-only")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ZeroSSLEABKID != "" {
		t.Errorf("ZeroSSLEABKID = %q, want empty", cfg.ZeroSSLEABKID)
	}
	if cfg.ZeroSSLEABHMACKey != "hmac-only" {
		t.Errorf("ZeroSSLEABHMACKey = %q, want %q", cfg.ZeroSSLEABHMACKey, "hmac-only")
	}
}

func TestLoad_MultiCA_BuypassEnvSet(t *testing.T) {
	t.Setenv(envBuypassEnv, "staging")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.BuypassEnv != "staging" {
		t.Errorf("BuypassEnv = %q, want %q", cfg.BuypassEnv, "staging")
	}
}

func TestLoad_MultiCA_AllEmpty(t *testing.T) {
	t.Setenv(envZeroSSLEABKID, "")
	t.Setenv(envZeroSSLEABHMACKey, "")
	t.Setenv(envBuypassEnv, "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ZeroSSLEABKID != "" {
		t.Errorf("ZeroSSLEABKID = %q, want empty", cfg.ZeroSSLEABKID)
	}
	if cfg.ZeroSSLEABHMACKey != "" {
		t.Errorf("ZeroSSLEABHMACKey = %q, want empty", cfg.ZeroSSLEABHMACKey)
	}
	if cfg.BuypassEnv != "" {
		t.Errorf("BuypassEnv = %q, want empty", cfg.BuypassEnv)
	}
}

func TestLoad_WhitespaceTrimmed(t *testing.T) {
	t.Setenv(envPort, "  8081  ")
	t.Setenv(envEnv, "  staging  ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 8081 {
		t.Errorf("Port = %d, want 8081", cfg.Port)
	}
	if cfg.Env != "staging" {
		t.Errorf("Env = %q, want %q", cfg.Env, "staging")
	}
}

func TestLoad_VaultBackendDefault(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.VaultBackend != VaultBackendEnvMaster {
		t.Errorf("VaultBackend = %q, want %q", cfg.VaultBackend, VaultBackendEnvMaster)
	}
}

func TestLoad_VaultBackendAliKMS_AllSet(t *testing.T) {
	t.Setenv(envVaultBackend, "alikms")
	t.Setenv(envAliKMSRegionID, "cn-hangzhou")
	t.Setenv(envAliKMSAccessKeyID, "ak-id")
	t.Setenv(envAliKMSAccessKeySecret, "ak-secret")
	t.Setenv(envAliKMSKeyID, "alias/cert-master")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.VaultBackend != VaultBackendAliKMS {
		t.Errorf("VaultBackend = %q, want %q", cfg.VaultBackend, VaultBackendAliKMS)
	}
	if cfg.AliKMSRegionID != "cn-hangzhou" {
		t.Errorf("AliKMSRegionID = %q", cfg.AliKMSRegionID)
	}
	if cfg.AliKMSKeyID != "alias/cert-master" {
		t.Errorf("AliKMSKeyID = %q", cfg.AliKMSKeyID)
	}
}

func TestLoad_VaultBackendAliKMS_MissingField(t *testing.T) {
	t.Setenv(envVaultBackend, "alikms")
	t.Setenv(envAliKMSRegionID, "cn-hangzhou")
	// AccessKeyID intentionally unset

	_, err := Load()
	if err == nil {
		t.Fatal("Load() returned nil error; want missing-field rejection")
	}
}

func TestLoad_VaultBackendUnknown(t *testing.T) {
	t.Setenv(envVaultBackend, "vault-hashicorp")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() returned nil error; want unknown-backend rejection")
	}
}

func TestLoad_VaultBackendAWSKMS_AllSet(t *testing.T) {
	t.Setenv(envVaultBackend, "awskms")
	t.Setenv(envAWSKMSRegion, "us-east-1")
	t.Setenv(envAWSKMSAccessKeyID, "AKIDfake")
	t.Setenv(envAWSKMSSecretAccessKey, "secret-fake")
	t.Setenv(envAWSKMSKeyID, "alias/cert-master")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.VaultBackend != VaultBackendAWSKMS {
		t.Errorf("VaultBackend = %q", cfg.VaultBackend)
	}
	if cfg.AWSKMSRegion != "us-east-1" {
		t.Errorf("AWSKMSRegion = %q", cfg.AWSKMSRegion)
	}
	if cfg.AWSKMSKeyID != "alias/cert-master" {
		t.Errorf("AWSKMSKeyID = %q", cfg.AWSKMSKeyID)
	}
}

func TestLoad_VaultBackendAWSKMS_DefaultCredChain(t *testing.T) {
	t.Setenv(envVaultBackend, "awskms")
	t.Setenv(envAWSKMSRegion, "us-east-1")
	t.Setenv(envAWSKMSKeyID, "alias/cert-master")
	// no AccessKeyID / SecretAccessKey → default credential chain
	_, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoad_VaultBackendAWSKMS_HalfCredRejected(t *testing.T) {
	t.Setenv(envVaultBackend, "awskms")
	t.Setenv(envAWSKMSRegion, "us-east-1")
	t.Setenv(envAWSKMSKeyID, "alias/cert-master")
	t.Setenv(envAWSKMSAccessKeyID, "AKIDfake")
	// SecretAccessKey intentionally unset
	_, err := Load()
	if err == nil {
		t.Fatal("Load() returned nil; want half-credential rejection")
	}
}

func TestLoad_VaultBackendAWSKMS_MissingRegion(t *testing.T) {
	t.Setenv(envVaultBackend, "awskms")
	t.Setenv(envAWSKMSKeyID, "alias/cert-master")
	_, err := Load()
	if err == nil {
		t.Fatal("Load() returned nil; want missing region rejection")
	}
}

func TestLoad_VaultBackendHashiVault_AllSet(t *testing.T) {
	t.Setenv(envVaultBackend, "hashivault")
	t.Setenv(envHashiVaultAddress, "https://vault.example:8200")
	t.Setenv(envHashiVaultToken, "hvs.fake")
	t.Setenv(envHashiVaultKeyName, "cert-master")
	t.Setenv(envHashiVaultNamespace, "cert-svc/")
	t.Setenv(envHashiVaultMountPath, "transit-v2")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.VaultBackend != VaultBackendHashiVault {
		t.Errorf("VaultBackend = %q", cfg.VaultBackend)
	}
	if cfg.HashiVaultAddress != "https://vault.example:8200" {
		t.Errorf("HashiVaultAddress = %q", cfg.HashiVaultAddress)
	}
	if cfg.HashiVaultMountPath != "transit-v2" {
		t.Errorf("HashiVaultMountPath = %q", cfg.HashiVaultMountPath)
	}
}

func TestLoad_VaultBackendHashiVault_MissingToken(t *testing.T) {
	t.Setenv(envVaultBackend, "hashivault")
	t.Setenv(envHashiVaultAddress, "https://vault.example:8200")
	t.Setenv(envHashiVaultKeyName, "cert-master")
	_, err := Load()
	if err == nil {
		t.Fatal("Load() returned nil; want missing-token rejection")
	}
}

func TestLoad_MetricsPort_Default(t *testing.T) {
	t.Setenv(envMetricsPort, "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.MetricsPort != defaultMetricsPort {
		t.Errorf("MetricsPort = %d, want %d", cfg.MetricsPort, defaultMetricsPort)
	}
	if got, want := cfg.MetricsAddr(), ":9090"; got != want {
		t.Errorf("MetricsAddr() = %q, want %q", got, want)
	}
}

func TestLoad_MetricsPort_Override(t *testing.T) {
	t.Setenv(envMetricsPort, "19090")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.MetricsPort != 19090 {
		t.Errorf("MetricsPort = %d, want 19090", cfg.MetricsPort)
	}
	if got, want := cfg.MetricsAddr(), ":19090"; got != want {
		t.Errorf("MetricsAddr() = %q, want %q", got, want)
	}
}

func TestLoad_MetricsPort_Invalid(t *testing.T) {
	cases := []struct {
		name string
		val  string
	}{
		{"non-numeric", "abc"},
		{"zero", "0"},
		{"too-large", "70000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envMetricsPort, tc.val)
			if _, err := Load(); err == nil {
				t.Errorf("Load() with %s=%q expected error, got nil", envMetricsPort, tc.val)
			}
		})
	}
}
