package config_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kite365/idcd/lib/shared/config"
)

const testYAML = `
database:
  main:
    dsn: "postgresql://dev:dev@localhost:5432/dev"
    max_open_conns: 10
    max_idle_conns: 3
    conn_max_lifetime: "5m"

redis:
  addr: "localhost:6379"
  password: "secret"
  db: 0

server:
  port: 8080
  env: "development"
  admin_token: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  cors_origins:
    - "http://localhost:3000"

jwt:
  secret: "supersecretkey_at_least_32_chars!!"
  access_ttl: "15m"
  refresh_ttl: "7d"

email:
  smtp_host: "localhost"
  smtp_port: 25
  from_addr: "noreply@idcd.com"
  from_name: "idcd"

observability:
  prometheus_port: 9091
`

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad_valid(t *testing.T) {
	p := writeTempConfig(t, testYAML)
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Database.Main.DSN == "" {
		t.Error("DSN should not be empty")
	}
	if cfg.Redis.Addr != "localhost:6379" {
		t.Errorf("unexpected redis addr: %q", cfg.Redis.Addr)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
}

func TestLoad_duration_minutes(t *testing.T) {
	p := writeTempConfig(t, testYAML)
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JWT.AccessTTL.Duration != 15*time.Minute {
		t.Errorf("access_ttl: expected 15m, got %v", cfg.JWT.AccessTTL.Duration)
	}
}

func TestLoad_duration_days(t *testing.T) {
	p := writeTempConfig(t, testYAML)
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JWT.RefreshTTL.Duration != 7*24*time.Hour {
		t.Errorf("refresh_ttl: expected 7d, got %v", cfg.JWT.RefreshTTL.Duration)
	}
}

func TestLoad_missingFile(t *testing.T) {
	_, err := config.Load("/nonexistent/path.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoad_missingDSN(t *testing.T) {
	yaml := `
redis:
  addr: "localhost:6379"
  password: "x"
jwt:
  secret: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
`
	p := writeTempConfig(t, yaml)
	_, err := config.Load(p)
	if err == nil {
		t.Error("expected validation error for missing DSN")
	}
}

func TestIsDev(t *testing.T) {
	p := writeTempConfig(t, testYAML)
	cfg, _ := config.Load(p)
	if !cfg.IsDev() {
		t.Error("expected IsDev() = true")
	}
	if cfg.IsProd() {
		t.Error("expected IsProd() = false")
	}
}

func TestDefaultPath_envVar(t *testing.T) {
	t.Setenv("IDCD_CONFIG", "/custom/config.yaml")
	if got := config.DefaultPath(); got != "/custom/config.yaml" {
		t.Errorf("expected /custom/config.yaml, got %q", got)
	}
}

func TestDefaultPath_fallback(t *testing.T) {
	os.Unsetenv("IDCD_CONFIG")
	if got := config.DefaultPath(); got != "config/dev.env.yaml" {
		t.Errorf("expected default path, got %q", got)
	}
}

func TestRateLimitRule_OrDefault(t *testing.T) {
	defWindow := 30 * time.Second
	var defMax int64 = 99

	// Both fields zero → defaults apply.
	w, m := (config.RateLimitRule{}).OrDefault(defWindow, defMax)
	if w != defWindow || m != defMax {
		t.Errorf("zero rule: want (%v, %d), got (%v, %d)", defWindow, defMax, w, m)
	}

	// Only window set → max falls back.
	r := config.RateLimitRule{Window: config.Duration{Duration: 5 * time.Minute}}
	w, m = r.OrDefault(defWindow, defMax)
	if w != 5*time.Minute || m != defMax {
		t.Errorf("partial rule: want (5m, %d), got (%v, %d)", defMax, w, m)
	}

	// Both set → both override.
	r = config.RateLimitRule{Window: config.Duration{Duration: time.Hour}, MaxRequests: 7}
	w, m = r.OrDefault(defWindow, defMax)
	if w != time.Hour || m != 7 {
		t.Errorf("full rule: want (1h, 7), got (%v, %d)", w, m)
	}
}

func TestLoad_rateLimitOverrides(t *testing.T) {
	yaml := testYAML + `
rate_limit:
  auth:
    window: "2m"
    max_requests: 10
  twofa:
    max_requests: 3
`
	p := writeTempConfig(t, yaml)
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RateLimit.Auth.Window.Duration != 2*time.Minute {
		t.Errorf("auth.window: want 2m, got %v", cfg.RateLimit.Auth.Window.Duration)
	}
	if cfg.RateLimit.Auth.MaxRequests != 10 {
		t.Errorf("auth.max_requests: want 10, got %d", cfg.RateLimit.Auth.MaxRequests)
	}
	// Partial override: only max set; window should remain zero so OrDefault kicks in.
	if cfg.RateLimit.TwoFA.Window.Duration != 0 {
		t.Errorf("twofa.window: want 0 (unset), got %v", cfg.RateLimit.TwoFA.Window.Duration)
	}
	if cfg.RateLimit.TwoFA.MaxRequests != 3 {
		t.Errorf("twofa.max_requests: want 3, got %d", cfg.RateLimit.TwoFA.MaxRequests)
	}
}

// TestLoadRaw_CertSvcAndAttestSections pins the YAML tag mapping for
// every field in CertSvcServiceConfig and AttestServiceConfig that
// applyYAML (in apps/cert-svc and apps/attest) reads. A typo in any
// yaml struct tag would silently produce zero values in production and
// the test below would catch it before the bad config ships.
func TestLoadRaw_CertSvcAndAttestSections(t *testing.T) {
	yaml := `
cert_svc:
  port: 8081
  metrics_port: 9192
  env: "staging"
  le_env: "production"
  log_level: "DEBUG"
  acme_account_email: "ops@example.com"
  database:
    dsn: "postgres://cert:secret@db/cert"
  redis:
    addr: "redis:6379"
    password: "rp"
    db: 2
    master_name: "mymaster"
    sentinel_addrs: ["s1:26379", "s2:26379"]
    sentinel_password: "sp"
  vault:
    backend: "awskms"
    alikms:
      region_id: "cn-hangzhou"
      access_key_id: "ali-akid"
      access_key_secret: "ali-secret"
      key_id: "ali-key"
    awskms:
      region: "us-east-1"
      access_key_id: "aws-akid"
      secret_access_key: "aws-secret"
      key_id: "aws-key"
    hashivault:
      address: "https://vault.example.com"
      token: "hv-token"
      namespace: "ns"
      key_name: "kn"
      mount_path: "transit"
  zerossl_eab_kid: "kid"
  zerossl_eab_hmac_key: "hmac"
  buypass_env: "production"

attest:
  port: 8090
  env: "staging"
  log_level: "INFO"
  database:
    dsn: "postgres://att:secret@db/att"
  redis:
    addr: "redis:6379"
    password: "rp2"
    db: 3
    master_name: "att-master"
    sentinel_addrs: ["s3:26379"]
    sentinel_password: "ap"
  sign_backend: "aliyun"
  awskms:
    region: "us-west-1"
    access_key_id: "akid"
    secret_access_key: "asec"
    key_id: "akey"
    algorithm: "ECDSA_SHA_256"
  alikms:
    region_id: "cn-shanghai"
    access_key_id: "ali"
    access_key_secret: "alisec"
    key_id: "alikey"
    algorithm: "ECDSA_SHA_256"
  local_key_path: "/keys/dev.pem"
  local_algorithm: "RSASSA_PKCS1_V1_5_SHA_256"
  tsa:
    providers: ["digicert", "globalsign"]
  s3:
    endpoint: "https://s3.example.com"
    bucket: "att-archive"
    region: "ap-southeast-1"
    access_key: "ak"
    secret_key: "sk"
    object_lock_mode: "COMPLIANCE"
    object_lock_days: 3650
    key_prefix: "verdicts/"
  verify_endpoint: "https://attest.example.com/verify"
  archiver_backend: "s3"
  local_archive_dir: "/var/lib/attest"
  refund:
    initiate_stream: "rstream"
    retry_stream: "rretry"
    delay_zone_key: "rdelay"
    group: "rgroup"
    consumer: "rconsumer"
    notifier_addr: "naddr"
    notifier_queue: "nqueue"
`
	p := writeTempConfig(t, yaml)
	cfg, err := config.LoadRaw(p)
	if err != nil {
		t.Fatalf("LoadRaw: %v", err)
	}

	cs := cfg.CertSvc
	if cs.Port != 8081 || cs.MetricsPort != 9192 || cs.Env != "staging" {
		t.Errorf("cert_svc scalars: %+v", cs)
	}
	if cs.LEEnv != "production" || cs.LogLevel != "DEBUG" || cs.AccountEmail != "ops@example.com" {
		t.Errorf("cert_svc strings: %+v", cs)
	}
	if cs.Database.DSN != "postgres://cert:secret@db/cert" {
		t.Errorf("cert_svc.database.dsn: %q", cs.Database.DSN)
	}
	if cs.Redis.MasterName != "mymaster" || len(cs.Redis.SentinelAddrs) != 2 ||
		cs.Redis.SentinelPassword != "sp" {
		t.Errorf("cert_svc.redis sentinel: %+v", cs.Redis)
	}
	if cs.Vault.Backend != "awskms" {
		t.Errorf("vault.backend: %q", cs.Vault.Backend)
	}
	if cs.Vault.AliKMS.RegionID != "cn-hangzhou" || cs.Vault.AliKMS.KeyID != "ali-key" {
		t.Errorf("vault.alikms: %+v", cs.Vault.AliKMS)
	}
	if cs.Vault.AWSKMS.Region != "us-east-1" || cs.Vault.AWSKMS.SecretAccessKey != "aws-secret" {
		t.Errorf("vault.awskms: %+v", cs.Vault.AWSKMS)
	}
	if cs.Vault.HashiVault.Address != "https://vault.example.com" ||
		cs.Vault.HashiVault.Token != "hv-token" ||
		cs.Vault.HashiVault.MountPath != "transit" {
		t.Errorf("vault.hashivault: %+v", cs.Vault.HashiVault)
	}
	if cs.ZeroSSLEABKID != "kid" || cs.ZeroSSLEABHMACKey != "hmac" || cs.BuypassEnv != "production" {
		t.Errorf("cert_svc external: %+v", cs)
	}

	at := cfg.Attest
	if at.Port != 8090 || at.Env != "staging" || at.LogLevel != "INFO" {
		t.Errorf("attest scalars: %+v", at)
	}
	if at.Database.DSN != "postgres://att:secret@db/att" {
		t.Errorf("attest.database.dsn: %q", at.Database.DSN)
	}
	if at.Redis.MasterName != "att-master" || len(at.Redis.SentinelAddrs) != 1 {
		t.Errorf("attest.redis sentinel: %+v", at.Redis)
	}
	if at.SignBackend != "aliyun" {
		t.Errorf("attest.sign_backend: %q", at.SignBackend)
	}
	if at.AWSKMS.Region != "us-west-1" || at.AWSKMS.KeyID != "akey" || at.AWSKMS.Algorithm != "ECDSA_SHA_256" {
		t.Errorf("attest.awskms: %+v", at.AWSKMS)
	}
	if at.AliKMS.RegionID != "cn-shanghai" || at.AliKMS.KeyID != "alikey" {
		t.Errorf("attest.alikms: %+v", at.AliKMS)
	}
	if at.LocalKeyPath != "/keys/dev.pem" || at.LocalAlgorithm != "RSASSA_PKCS1_V1_5_SHA_256" {
		t.Errorf("attest.local: path=%q alg=%q", at.LocalKeyPath, at.LocalAlgorithm)
	}
	if len(at.TSA.Providers) != 2 || at.TSA.Providers[0] != "digicert" {
		t.Errorf("attest.tsa.providers: %+v", at.TSA.Providers)
	}
	if at.S3.Bucket != "att-archive" || at.S3.Region != "ap-southeast-1" ||
		at.S3.ObjectLockMode != "COMPLIANCE" || at.S3.ObjectLockDays != 3650 ||
		at.S3.KeyPrefix != "verdicts/" || at.S3.Endpoint != "https://s3.example.com" {
		t.Errorf("attest.s3: %+v", at.S3)
	}
	if at.VerifyEndpoint != "https://attest.example.com/verify" {
		t.Errorf("attest.verify_endpoint: %q", at.VerifyEndpoint)
	}
	if at.ArchiverBackend != "s3" || at.LocalArchiveDir != "/var/lib/attest" {
		t.Errorf("attest archiver: backend=%q dir=%q", at.ArchiverBackend, at.LocalArchiveDir)
	}
	r := at.Refund
	if r.InitiateStream != "rstream" || r.RetryStream != "rretry" || r.DelayZoneKey != "rdelay" ||
		r.Group != "rgroup" || r.Consumer != "rconsumer" || r.NotifierAddr != "naddr" || r.NotifierQueue != "nqueue" {
		t.Errorf("attest.refund: %+v", r)
	}
}

// TestLoadRaw_MissingFile ensures the dedicated raw loader surfaces
// fs errors instead of silently returning a zero-value Config.
func TestLoadRaw_MissingFile(t *testing.T) {
	_, err := config.LoadRaw("/nonexistent/path.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestDuration_CombinedDayForm(t *testing.T) {
	// parseDuration must handle "7d12h", "1d30m", and pure "30d".
	cases := []struct {
		val  string
		want time.Duration
	}{
		{"30d", 30 * 24 * time.Hour},
		{"7d12h", 7*24*time.Hour + 12*time.Hour},
		{"1d30m", 24*time.Hour + 30*time.Minute},
	}
	baseYAML := `
database:
  main:
    dsn: "postgresql://dev:dev@localhost:5432/dev"
redis:
  addr: "localhost:6379"
server:
  port: 8080
  env: "development"
  admin_token: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
jwt:
  secret: "supersecretkey_at_least_32_chars!!"
  access_ttl: "15m"
  refresh_ttl: "%s"
`
	for _, tc := range cases {
		y := fmt.Sprintf(baseYAML, tc.val)
		p := writeTempConfig(t, y)
		cfg, err := config.Load(p)
		if err != nil {
			t.Fatalf("Load with refresh_ttl=%q: %v", tc.val, err)
		}
		if cfg.JWT.RefreshTTL.Duration != tc.want {
			t.Errorf("refresh_ttl=%q: want %v, got %v", tc.val, tc.want, cfg.JWT.RefreshTTL.Duration)
		}
	}
}
