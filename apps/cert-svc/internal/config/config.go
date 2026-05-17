// Package config loads cert-svc settings from environment variables.
//
// Unlike apps/api (which uses lib/shared/config + YAML), cert-svc is a
// fresh service and reads CERT_* env vars directly to keep its boot
// surface small. The S2 milestone may revisit and merge into the shared
// config tree once it stabilises.
package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	envPort         = "CERT_SVC_PORT"
	envDB           = "CERT_DB_DSN"
	envRedis        = "CERT_REDIS_URL"
	envRedisAddr    = "CERT_REDIS_ADDR"
	envLogLevel     = "CERT_LOG_LEVEL"
	envEnv          = "CERT_ENV"
	envLEEnv        = "CERT_LE_ENV"
	envAccountEmail = "CERT_ACME_ACCOUNT_EMAIL"
	envJWTSecret      = "CERT_JWT_SECRET"
	envMasterKey      = "CERT_MASTER_KEY"
	envDownloadSecret = "CERT_DOWNLOAD_SECRET"

	// S2 multi-CA wiring. Each adapter is optional — empty values mean
	// the CA is not registered with the Router and cert.orders.ca rows
	// pointing at it will fail with ErrUnknownCA. cmd/server +
	// cmd/worker read these and conditionally construct the adapter.
	envZeroSSLEABKID     = "CERT_ZEROSSL_EAB_KID"
	envZeroSSLEABHMACKey = "CERT_ZEROSSL_EAB_HMAC_KEY"
	envBuypassEnv        = "CERT_BUYPASS_ENV" // "production" | "staging" | "" (disabled)

	// S2 vault backend selection. "envmaster" (default) keeps the S1
	// process-local AES master key; "alikms" switches to Aliyun KMS
	// envelope encryption (D-FC-04 国内主路径). The four CERT_ALIKMS_*
	// vars are required when VaultBackend=="alikms".
	envVaultBackend      = "CERT_VAULT_BACKEND"
	envAliKMSRegionID    = "CERT_ALIKMS_REGION_ID"
	envAliKMSAccessKeyID = "CERT_ALIKMS_ACCESS_KEY_ID"
	envAliKMSAccessKeySecret = "CERT_ALIKMS_ACCESS_KEY_SECRET"
	envAliKMSKeyID       = "CERT_ALIKMS_KEY_ID"

	envAWSKMSRegion          = "CERT_AWSKMS_REGION"
	envAWSKMSAccessKeyID     = "CERT_AWSKMS_ACCESS_KEY_ID"
	envAWSKMSSecretAccessKey = "CERT_AWSKMS_SECRET_ACCESS_KEY"
	envAWSKMSKeyID           = "CERT_AWSKMS_KEY_ID"

	envHashiVaultAddress   = "CERT_HASHIVAULT_ADDRESS"
	envHashiVaultToken     = "CERT_HASHIVAULT_TOKEN"
	envHashiVaultNamespace = "CERT_HASHIVAULT_NAMESPACE"
	envHashiVaultKeyName   = "CERT_HASHIVAULT_KEY_NAME"
	envHashiVaultMountPath = "CERT_HASHIVAULT_MOUNT_PATH"

	VaultBackendEnvMaster  = "envmaster"
	VaultBackendAliKMS     = "alikms"
	VaultBackendAWSKMS     = "awskms"
	VaultBackendHashiVault = "hashivault"

	defaultPort         = 8080
	defaultDB           = "postgres://idcd:idcd@localhost:5432/idcd?sslmode=disable"
	defaultRedis        = "redis://localhost:6379/0"
	defaultRedisAddr    = "localhost:6379"
	defaultLogLevel     = "info"
	defaultEnv          = "development"
	defaultLEEnv        = "staging"
	defaultAccountEmail = "acme@idcd.local"
)

// Config is the cert-svc runtime configuration.
type Config struct {
	Port         int
	DatabaseDSN  string
	RedisURL     string
	RedisAddr    string
	LogLevel     string
	Env          string
	LEEnv        string
	AccountEmail string

	// JWTSecret is the HMAC secret cert-svc uses to verify JWTs issued
	// by apps/api. Must match apps/api's auth.jwt.secret in the
	// canonical config. Empty disables JWT auth entirely (every
	// /v1/cert request gets a 401), which is the safe default for
	// preview / unconfigured environments.
	JWTSecret string

	// MasterKey is the base64-encoded 32-byte master key passed
	// straight through to envmaster. Re-exposed on Config so callers
	// (server main, worker main, tests) can decide whether to fall
	// back to env-only lookup or fail fast on missing config.
	MasterKey string

	// DownloadSecret is the raw HMAC-SHA256 key cert-svc uses to sign
	// the W5 one-shot download tokens. Sourced from CERT_DOWNLOAD_SECRET
	// (base64). Empty means the download token endpoint is disabled —
	// /v1/cert/certs/{id}/download will return 503 rather than mint
	// tokens we can't verify. cmd/server should fail fast when unset.
	DownloadSecret []byte

	// ZeroSSLEABKID / ZeroSSLEABHMACKey carry the External Account
	// Binding credentials issued by ZeroSSL's portal. Both must be set
	// for the ZeroSSL adapter to register with the Router; either
	// missing → adapter disabled (no partial registration). The values
	// are passed through verbatim — validation lives in the adapter.
	ZeroSSLEABKID     string
	ZeroSSLEABHMACKey string

	// BuypassEnv selects the Buypass Go SSL endpoint. "production" or
	// "staging" enable the adapter; empty disables it. The string is
	// passed through to the adapter, which is responsible for any
	// further validation.
	BuypassEnv string

	// VaultBackend picks the vault.Vault implementation. "envmaster"
	// (default) reads CERT_MASTER_KEY as the S1 single-process master
	// key; "alikms" uses Aliyun KMS envelope encryption — production
	// deploys MUST switch to alikms per D-FC-04.
	VaultBackend string

	// AliKMS* — only consulted when VaultBackend == "alikms". All four
	// are required; Load returns an error if backend=alikms and any
	// is empty so misconfigured production starts fail fast rather
	// than silently falling through to envmaster.
	AliKMSRegionID        string
	AliKMSAccessKeyID     string
	AliKMSAccessKeySecret string
	AliKMSKeyID           string

	// AWSKMS* — only consulted when VaultBackend == "awskms". Region
	// and KeyID are required; static IAM keys are optional (empty pair
	// falls back to the SDK's default credential chain — IRSA /
	// instance profile / shared config). Setting only one of the two
	// IAM fields is a hard error.
	AWSKMSRegion          string
	AWSKMSAccessKeyID     string
	AWSKMSSecretAccessKey string
	AWSKMSKeyID           string

	// HashiVault* — only consulted when VaultBackend == "hashivault".
	// Address / Token / KeyName are required; Namespace + MountPath
	// are optional (MountPath defaults to "transit" in the adapter).
	HashiVaultAddress   string
	HashiVaultToken     string
	HashiVaultNamespace string
	HashiVaultKeyName   string
	HashiVaultMountPath string
}

// Load reads CERT_* env vars and returns a populated Config.
// Unset vars fall back to sensible local-dev defaults; the only validation
// failure today is a non-numeric CERT_SVC_PORT.
func Load() (*Config, error) {
	cfg := &Config{
		Port:         defaultPort,
		DatabaseDSN:  defaultDB,
		RedisURL:     defaultRedis,
		RedisAddr:    defaultRedisAddr,
		LogLevel:     defaultLogLevel,
		Env:          defaultEnv,
		LEEnv:        defaultLEEnv,
		AccountEmail: defaultAccountEmail,
	}

	if v := strings.TrimSpace(os.Getenv(envPort)); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("config: invalid %s=%q: %w", envPort, v, err)
		}
		if port <= 0 || port > 65535 {
			return nil, fmt.Errorf("config: %s out of range: %d", envPort, port)
		}
		cfg.Port = port
	}

	if v := strings.TrimSpace(os.Getenv(envDB)); v != "" {
		cfg.DatabaseDSN = v
	}
	if v := strings.TrimSpace(os.Getenv(envRedis)); v != "" {
		cfg.RedisURL = v
	}
	if v := strings.TrimSpace(os.Getenv(envRedisAddr)); v != "" {
		cfg.RedisAddr = v
	}
	if v := strings.TrimSpace(os.Getenv(envLogLevel)); v != "" {
		cfg.LogLevel = strings.ToLower(v)
	}
	if v := strings.TrimSpace(os.Getenv(envEnv)); v != "" {
		cfg.Env = v
	}
	if v := strings.TrimSpace(os.Getenv(envLEEnv)); v != "" {
		cfg.LEEnv = v
	}
	if v := strings.TrimSpace(os.Getenv(envAccountEmail)); v != "" {
		cfg.AccountEmail = v
	}
	if v := strings.TrimSpace(os.Getenv(envJWTSecret)); v != "" {
		cfg.JWTSecret = v
	}
	if v := strings.TrimSpace(os.Getenv(envMasterKey)); v != "" {
		cfg.MasterKey = v
	}
	if v := strings.TrimSpace(os.Getenv(envDownloadSecret)); v != "" {
		// Base64 (std OR url) so operators can paste from any common
		// secret tool. We reject empty / undecodable payloads up front
		// rather than handing the service a forgeable signing key.
		raw, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			raw, err = base64.RawStdEncoding.DecodeString(v)
		}
		if err != nil {
			raw, err = base64.URLEncoding.DecodeString(v)
		}
		if err != nil {
			raw, err = base64.RawURLEncoding.DecodeString(v)
		}
		if err != nil {
			return nil, fmt.Errorf("config: %s must be base64-encoded: %w", envDownloadSecret, err)
		}
		if len(raw) < 16 {
			return nil, fmt.Errorf("config: %s decoded to %d bytes; need >=16", envDownloadSecret, len(raw))
		}
		cfg.DownloadSecret = raw
	}

	if v := strings.TrimSpace(os.Getenv(envZeroSSLEABKID)); v != "" {
		cfg.ZeroSSLEABKID = v
	}
	if v := strings.TrimSpace(os.Getenv(envZeroSSLEABHMACKey)); v != "" {
		cfg.ZeroSSLEABHMACKey = v
	}
	if v := strings.TrimSpace(os.Getenv(envBuypassEnv)); v != "" {
		cfg.BuypassEnv = v
	}

	cfg.VaultBackend = VaultBackendEnvMaster
	if v := strings.TrimSpace(os.Getenv(envVaultBackend)); v != "" {
		cfg.VaultBackend = strings.ToLower(v)
	}
	if v := strings.TrimSpace(os.Getenv(envAliKMSRegionID)); v != "" {
		cfg.AliKMSRegionID = v
	}
	if v := strings.TrimSpace(os.Getenv(envAliKMSAccessKeyID)); v != "" {
		cfg.AliKMSAccessKeyID = v
	}
	if v := strings.TrimSpace(os.Getenv(envAliKMSAccessKeySecret)); v != "" {
		cfg.AliKMSAccessKeySecret = v
	}
	if v := strings.TrimSpace(os.Getenv(envAliKMSKeyID)); v != "" {
		cfg.AliKMSKeyID = v
	}
	if v := strings.TrimSpace(os.Getenv(envAWSKMSRegion)); v != "" {
		cfg.AWSKMSRegion = v
	}
	if v := strings.TrimSpace(os.Getenv(envAWSKMSAccessKeyID)); v != "" {
		cfg.AWSKMSAccessKeyID = v
	}
	if v := strings.TrimSpace(os.Getenv(envAWSKMSSecretAccessKey)); v != "" {
		cfg.AWSKMSSecretAccessKey = v
	}
	if v := strings.TrimSpace(os.Getenv(envAWSKMSKeyID)); v != "" {
		cfg.AWSKMSKeyID = v
	}
	if v := strings.TrimSpace(os.Getenv(envHashiVaultAddress)); v != "" {
		cfg.HashiVaultAddress = v
	}
	if v := strings.TrimSpace(os.Getenv(envHashiVaultToken)); v != "" {
		cfg.HashiVaultToken = v
	}
	if v := strings.TrimSpace(os.Getenv(envHashiVaultNamespace)); v != "" {
		cfg.HashiVaultNamespace = v
	}
	if v := strings.TrimSpace(os.Getenv(envHashiVaultKeyName)); v != "" {
		cfg.HashiVaultKeyName = v
	}
	if v := strings.TrimSpace(os.Getenv(envHashiVaultMountPath)); v != "" {
		cfg.HashiVaultMountPath = v
	}

	switch cfg.VaultBackend {
	case VaultBackendEnvMaster:
		// ok; CERT_MASTER_KEY validated lazily by envmaster.
	case VaultBackendAliKMS:
		if cfg.AliKMSRegionID == "" || cfg.AliKMSAccessKeyID == "" ||
			cfg.AliKMSAccessKeySecret == "" || cfg.AliKMSKeyID == "" {
			return nil, fmt.Errorf("config: %s=%s requires %s / %s / %s / %s",
				envVaultBackend, VaultBackendAliKMS,
				envAliKMSRegionID, envAliKMSAccessKeyID, envAliKMSAccessKeySecret, envAliKMSKeyID)
		}
	case VaultBackendAWSKMS:
		if cfg.AWSKMSRegion == "" || cfg.AWSKMSKeyID == "" {
			return nil, fmt.Errorf("config: %s=%s requires %s + %s",
				envVaultBackend, VaultBackendAWSKMS, envAWSKMSRegion, envAWSKMSKeyID)
		}
		akidSet := cfg.AWSKMSAccessKeyID != ""
		secretSet := cfg.AWSKMSSecretAccessKey != ""
		if akidSet != secretSet {
			return nil, fmt.Errorf("config: %s=%s requires both %s and %s or neither (default credential chain)",
				envVaultBackend, VaultBackendAWSKMS, envAWSKMSAccessKeyID, envAWSKMSSecretAccessKey)
		}
	case VaultBackendHashiVault:
		if cfg.HashiVaultAddress == "" || cfg.HashiVaultToken == "" || cfg.HashiVaultKeyName == "" {
			return nil, fmt.Errorf("config: %s=%s requires %s + %s + %s",
				envVaultBackend, VaultBackendHashiVault,
				envHashiVaultAddress, envHashiVaultToken, envHashiVaultKeyName)
		}
	default:
		return nil, fmt.Errorf("config: %s=%q must be one of %q / %q / %q / %q",
			envVaultBackend, cfg.VaultBackend,
			VaultBackendEnvMaster, VaultBackendAliKMS, VaultBackendAWSKMS, VaultBackendHashiVault)
	}

	return cfg, nil
}

// Addr returns the host:port the HTTP server should bind.
func (c *Config) Addr() string {
	return fmt.Sprintf(":%d", c.Port)
}
