// Package config loads attest-svc settings with a two-layer strategy:
//
//  1. YAML file (attest: section of the shared YAML; path from
//     ATTEST_CONFIG, then IDCD_CONFIG, then "config/dev.env.yaml").
//  2. ATTEST_* environment variables override individual YAML values.
//
// Secrets (ATTEST_PAYMENT_HUB_WEBHOOK_SECRET) remain env-only.
//
// Backend selection:
//   - ATTEST_SIGN_BACKEND (or attest.sign_backend in YAML) = aws | aliyun | local.
//   - ATTEST_TSA_PROVIDERS (or attest.tsa.providers) = comma list.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	sharedconfig "github.com/kite365/idcd/lib/shared/config"
)

const (
	envPort     = "ATTEST_PORT"
	envEnv      = "ATTEST_ENV"
	envLogLevel = "ATTEST_LOG_LEVEL"
	envDB       = "ATTEST_DB_DSN"
	envRedis    = "ATTEST_REDIS_ADDR"
	envS3Bucket          = "ATTEST_S3_BUCKET"           // WORM archive bucket; empty disables archival
	envS3Region          = "ATTEST_S3_REGION"           // AWS region for S3 bucket
	envS3Endpoint        = "ATTEST_S3_ENDPOINT"         // optional S3-compatible endpoint (MinIO/LocalStack)
	envS3ObjectLockMode  = "ATTEST_S3_OBJECT_LOCK_MODE" // "COMPLIANCE" | "GOVERNANCE"
	envS3ObjectLockDays  = "ATTEST_S3_OBJECT_LOCK_DAYS" // retention days (default 3650)
	envS3KeyPrefix       = "ATTEST_S3_KEY_PREFIX"       // optional object key prefix
	envArchiverBackend   = "ATTEST_ARCHIVER_BACKEND"    // "local" (default) | "s3"
	envLocalArchiveDir   = "ATTEST_LOCAL_ARCHIVE_DIR"   // directory for local archiver
	envRedisPasswd  = "ATTEST_REDIS_PASSWORD"
	envRedisDB      = "ATTEST_REDIS_DB"
	envRedisMasterName     = "ATTEST_REDIS_MASTER_NAME"
	envRedisSentinelAddrs  = "ATTEST_REDIS_SENTINEL_ADDRS"
	envRedisSentinelPasswd = "ATTEST_REDIS_SENTINEL_PASSWORD"

	envSignBackend = "ATTEST_SIGN_BACKEND" // "aws" | "aliyun"

	envAWSKMSRegion          = "ATTEST_AWSKMS_REGION"
	envAWSKMSAccessKeyID     = "ATTEST_AWSKMS_ACCESS_KEY_ID"
	envAWSKMSSecretAccessKey = "ATTEST_AWSKMS_SECRET_ACCESS_KEY"
	envAWSKMSKeyID           = "ATTEST_AWSKMS_KEY_ID"
	envAWSKMSAlgorithm       = "ATTEST_AWSKMS_ALGORITHM" // optional, default ECDSA_SHA_256

	envAliKMSRegionID        = "ATTEST_ALIKMS_REGION_ID"
	envAliKMSAccessKeyID     = "ATTEST_ALIKMS_ACCESS_KEY_ID"
	envAliKMSAccessKeySecret = "ATTEST_ALIKMS_ACCESS_KEY_SECRET"
	envAliKMSKeyID           = "ATTEST_ALIKMS_KEY_ID"
	envAliKMSAlgorithm       = "ATTEST_ALIKMS_ALGORITHM" // optional, default ECDSA_SHA_256

	envLocalKeyPath   = "ATTEST_LOCAL_KEY_PATH"  // SignBackend=local: PEM 私钥路径，不存在自动生成
	envLocalAlgorithm = "ATTEST_LOCAL_ALGORITHM" // optional, default RSASSA_PKCS1_V1_5_SHA_256

	envTSAProviders = "ATTEST_TSA_PROVIDERS" // comma list: digicert,globalsign,freetsa

	envVerifyEndpoint = "ATTEST_VERIFY_ENDPOINT" // Self-Verify Worker target (D6)

	envPaymentHubWebhookSecret = "ATTEST_PAYMENT_HUB_WEBHOOK_SECRET" // HMAC secret for /webhooks/paymenthub

	// D5 refund worker — Redis Stream wiring. Defaults match the producer
	// constants in apps/attest/cmd/verifier/refund_enqueue.go and
	// apps/attest/internal/handler/paymenthub/paymenthub.go so the consumer and
	// the two producers share keys without any further configuration.
	envRefundInitiateStream = "ATTEST_REFUND_INITIATE_STREAM"
	envRefundRetryStream    = "ATTEST_REFUND_RETRY_STREAM"
	envRefundDelayZoneKey   = "ATTEST_REFUND_DELAY_ZONE"
	envRefundGroup          = "ATTEST_REFUND_GROUP"
	envRefundConsumer       = "ATTEST_REFUND_CONSUMER"
	envRefundNotifierAddr   = "ATTEST_REFUND_NOTIFIER_REDIS_ADDR" // asynq broker for apology emails (empty = mailer disabled)
	envRefundNotifierQueue  = "ATTEST_REFUND_NOTIFIER_QUEUE"      // asynq queue name; default "billing"

	SignBackendAWS    = "aws"
	SignBackendAliyun = "aliyun"
	// SignBackendLocal 是 dev-only fallback：从 ATTEST_LOCAL_KEY_PATH 指向
	// 的 PEM 文件读 RSA-2048 私钥（不存在则生成）。不要用于生产 — 私钥
	// 裸存磁盘，无 KMS audit / HSM / 轮换机制。
	SignBackendLocal = "local"

	defaultPort      = 8080
	defaultEnv       = "development"
	defaultLogLevel  = "info"
	defaultAlgorithm = "ECDSA_SHA_256"
	defaultTSAList   = "digicert,globalsign"

	defaultRefundInitiateStream = "refund_initiate_queue"
	defaultRefundRetryStream    = "refund_retry_queue"
	defaultRefundDelayZoneKey   = "refund_delay_zone"
	defaultRefundGroup          = "attest-refund-worker"
	defaultRefundConsumer       = "attest-refund-worker-1"
	defaultRefundNotifierQueue  = "billing"
)

// Config is the attest-svc runtime configuration.
type Config struct {
	Port     int
	Env      string
	LogLevel string

	DatabaseDSN           string
	RedisAddr             string
	RedisPassword         string
	RedisDB               int
	RedisMasterName       string
	RedisSentinelAddrs    []string
	RedisSentinelPassword string
	S3Bucket              string
	S3Region              string
	S3Endpoint            string
	S3ObjectLockMode      string
	S3ObjectLockDays      int
	S3KeyPrefix           string
	ArchiverBackend       string // "local" | "s3"
	LocalArchiveDir       string

	SignBackend string // "aws" | "aliyun"

	AWSKMSRegion          string
	AWSKMSAccessKeyID     string
	AWSKMSSecretAccessKey string
	AWSKMSKeyID           string
	AWSKMSAlgorithm       string

	AliKMSRegionID        string
	AliKMSAccessKeyID     string
	AliKMSAccessKeySecret string
	AliKMSKeyID           string
	AliKMSAlgorithm       string

	// LocalKeyPath / LocalAlgorithm 仅在 SignBackend=local 时使用。
	LocalKeyPath   string
	LocalAlgorithm string

	TSAProviders []string // ordered: primary first

	VerifyEndpoint string // Self-Verify Worker only

	PaymentHubWebhookSecret string // empty disables /webhooks/paymenthub (D5)

	// D5 refund worker (cmd/refund-worker only — verify-only / generator
	// processes ignore these). DelayZoneKey is the Redis ZSET key
	// holding scheduled retries; Group / Consumer are XREADGROUP
	// identifiers; NotifierRedisAddr is the asynq broker URL used for
	// apology-email enqueue (empty disables the mailer for early S2
	// deploys where notifier is not yet co-located).
	RefundInitiateStream string
	RefundRetryStream    string
	RefundDelayZoneKey   string
	RefundGroup          string
	RefundConsumer       string
	RefundNotifierAddr   string
	RefundNotifierQueue  string
}

// yamlConfigPath returns the YAML file attest should load for its
// attest: section.  Priority: ATTEST_CONFIG > IDCD_CONFIG >
// "config/dev.env.yaml".  Returns "" when an empty explicit var is set,
// which suppresses YAML loading (useful in unit tests).
func yamlConfigPath() string {
	if v, ok := os.LookupEnv("ATTEST_CONFIG"); ok {
		return v
	}
	if v, ok := os.LookupEnv("IDCD_CONFIG"); ok {
		return v
	}
	return "config/dev.env.yaml"
}

// applyYAML overlays non-zero values from the attest: YAML section onto cfg.
// Missing file is silently ignored; any other parse error is fatal.
func applyYAML(cfg *Config, path string) error {
	if path == "" {
		return nil
	}
	raw, err := sharedconfig.LoadRaw(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	y := raw.Attest
	if y.Port != 0 {
		cfg.Port = y.Port
	}
	if y.Env != "" {
		cfg.Env = y.Env
	}
	if y.LogLevel != "" {
		cfg.LogLevel = strings.ToLower(y.LogLevel)
	}
	if y.Database.DSN != "" {
		cfg.DatabaseDSN = y.Database.DSN
	}
	if y.Redis.Addr != "" {
		cfg.RedisAddr = y.Redis.Addr
	}
	if y.Redis.Password != "" {
		cfg.RedisPassword = y.Redis.Password
	}
	if y.Redis.DB != 0 {
		cfg.RedisDB = y.Redis.DB
	}
	if y.Redis.MasterName != "" {
		cfg.RedisMasterName = y.Redis.MasterName
	}
	if len(y.Redis.SentinelAddrs) > 0 {
		cfg.RedisSentinelAddrs = y.Redis.SentinelAddrs
	}
	if y.Redis.SentinelPassword != "" {
		cfg.RedisSentinelPassword = y.Redis.SentinelPassword
	}
	if y.S3.Bucket != "" {
		cfg.S3Bucket = y.S3.Bucket
	}
	if y.S3.Region != "" {
		cfg.S3Region = y.S3.Region
	}
	if y.S3.Endpoint != "" {
		cfg.S3Endpoint = y.S3.Endpoint
	}
	if y.S3.ObjectLockMode != "" {
		cfg.S3ObjectLockMode = y.S3.ObjectLockMode
	}
	if y.S3.ObjectLockDays != 0 {
		cfg.S3ObjectLockDays = y.S3.ObjectLockDays
	}
	if y.S3.KeyPrefix != "" {
		cfg.S3KeyPrefix = y.S3.KeyPrefix
	}
	if y.ArchiverBackend != "" {
		cfg.ArchiverBackend = y.ArchiverBackend
	}
	if y.LocalArchiveDir != "" {
		cfg.LocalArchiveDir = y.LocalArchiveDir
	}
	if y.SignBackend != "" {
		cfg.SignBackend = y.SignBackend
	}
	if y.AWSKMS.Region != "" {
		cfg.AWSKMSRegion = y.AWSKMS.Region
	}
	if y.AWSKMS.AccessKeyID != "" {
		cfg.AWSKMSAccessKeyID = y.AWSKMS.AccessKeyID
	}
	if y.AWSKMS.SecretAccessKey != "" {
		cfg.AWSKMSSecretAccessKey = y.AWSKMS.SecretAccessKey
	}
	if y.AWSKMS.KeyID != "" {
		cfg.AWSKMSKeyID = y.AWSKMS.KeyID
	}
	if y.AWSKMS.Algorithm != "" {
		cfg.AWSKMSAlgorithm = y.AWSKMS.Algorithm
	}
	if y.AliKMS.RegionID != "" {
		cfg.AliKMSRegionID = y.AliKMS.RegionID
	}
	if y.AliKMS.AccessKeyID != "" {
		cfg.AliKMSAccessKeyID = y.AliKMS.AccessKeyID
	}
	if y.AliKMS.AccessKeySecret != "" {
		cfg.AliKMSAccessKeySecret = y.AliKMS.AccessKeySecret
	}
	if y.AliKMS.KeyID != "" {
		cfg.AliKMSKeyID = y.AliKMS.KeyID
	}
	if y.AliKMS.Algorithm != "" {
		cfg.AliKMSAlgorithm = y.AliKMS.Algorithm
	}
	if y.LocalKeyPath != "" {
		cfg.LocalKeyPath = y.LocalKeyPath
	}
	if y.LocalAlgorithm != "" {
		cfg.LocalAlgorithm = y.LocalAlgorithm
	}
	if len(y.TSA.Providers) > 0 {
		cfg.TSAProviders = y.TSA.Providers
	}
	if y.VerifyEndpoint != "" {
		cfg.VerifyEndpoint = y.VerifyEndpoint
	}
	r := y.Refund
	if r.InitiateStream != "" {
		cfg.RefundInitiateStream = r.InitiateStream
	}
	if r.RetryStream != "" {
		cfg.RefundRetryStream = r.RetryStream
	}
	if r.DelayZoneKey != "" {
		cfg.RefundDelayZoneKey = r.DelayZoneKey
	}
	if r.Group != "" {
		cfg.RefundGroup = r.Group
	}
	if r.Consumer != "" {
		cfg.RefundConsumer = r.Consumer
	}
	if r.NotifierAddr != "" {
		cfg.RefundNotifierAddr = r.NotifierAddr
	}
	if r.NotifierQueue != "" {
		cfg.RefundNotifierQueue = r.NotifierQueue
	}
	return nil
}

// Load builds the attest Config with three layers:
//  1. Hard-coded defaults (safe local-dev values).
//  2. YAML overlay from the attest: section (path resolved by yamlConfigPath).
//  3. ATTEST_* environment variables (override YAML; secrets stay env-only).
func Load() (*Config, error) {
	cfg := &Config{
		Port:                 defaultPort,
		Env:                  defaultEnv,
		LogLevel:             defaultLogLevel,
		AWSKMSAlgorithm:      defaultAlgorithm,
		AliKMSAlgorithm:      defaultAlgorithm,
		TSAProviders:         []string{"digicert", "globalsign"},
		RefundInitiateStream: defaultRefundInitiateStream,
		RefundRetryStream:    defaultRefundRetryStream,
		RefundDelayZoneKey:   defaultRefundDelayZoneKey,
		RefundGroup:          defaultRefundGroup,
		RefundConsumer:       defaultRefundConsumer,
		RefundNotifierQueue:  defaultRefundNotifierQueue,
	}

	if err := applyYAML(cfg, yamlConfigPath()); err != nil {
		return nil, fmt.Errorf("config: yaml overlay: %w", err)
	}

	if v := strings.TrimSpace(os.Getenv(envPort)); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil || port <= 0 || port > 65535 {
			return nil, fmt.Errorf("config: invalid %s=%q", envPort, v)
		}
		cfg.Port = port
	}
	if v := strings.TrimSpace(os.Getenv(envEnv)); v != "" {
		cfg.Env = v
	}
	if v := strings.TrimSpace(os.Getenv(envLogLevel)); v != "" {
		cfg.LogLevel = strings.ToLower(v)
	}
	if v := strings.TrimSpace(os.Getenv(envDB)); v != "" {
		cfg.DatabaseDSN = v
	}
	if v := strings.TrimSpace(os.Getenv(envRedis)); v != "" {
		cfg.RedisAddr = v
	}
	if v := os.Getenv(envRedisPasswd); v != "" {
		cfg.RedisPassword = v
	}
	if v := strings.TrimSpace(os.Getenv(envRedisDB)); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("config: invalid %s=%q", envRedisDB, v)
		}
		cfg.RedisDB = n
	}
	if v := strings.TrimSpace(os.Getenv(envS3Bucket)); v != "" {
		cfg.S3Bucket = v
	}
	if v := strings.TrimSpace(os.Getenv(envS3Region)); v != "" {
		cfg.S3Region = v
	}
	if v := strings.TrimSpace(os.Getenv(envS3Endpoint)); v != "" {
		cfg.S3Endpoint = v
	}
	if v := strings.TrimSpace(os.Getenv(envS3ObjectLockMode)); v != "" {
		cfg.S3ObjectLockMode = v
	}
	if v := strings.TrimSpace(os.Getenv(envS3ObjectLockDays)); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("config: invalid %s=%q", envS3ObjectLockDays, v)
		}
		cfg.S3ObjectLockDays = n
	}
	if v := strings.TrimLeft(os.Getenv(envS3KeyPrefix), " \t"); v != "" {
		cfg.S3KeyPrefix = v
	}
	if v := strings.ToLower(strings.TrimSpace(os.Getenv(envArchiverBackend))); v != "" {
		cfg.ArchiverBackend = v
	}
	if v := strings.TrimSpace(os.Getenv(envLocalArchiveDir)); v != "" {
		cfg.LocalArchiveDir = v
	}
	if v := strings.TrimSpace(os.Getenv(envRedisMasterName)); v != "" {
		cfg.RedisMasterName = v
	}
	if v := strings.TrimSpace(os.Getenv(envRedisSentinelAddrs)); v != "" {
		for _, addr := range strings.Split(v, ",") {
			if a := strings.TrimSpace(addr); a != "" {
				cfg.RedisSentinelAddrs = append(cfg.RedisSentinelAddrs, a)
			}
		}
	}
	if v := strings.TrimSpace(os.Getenv(envRedisSentinelPasswd)); v != "" {
		cfg.RedisSentinelPassword = v
	}

	if v := strings.ToLower(strings.TrimSpace(os.Getenv(envSignBackend))); v != "" {
		cfg.SignBackend = v
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
	if v := strings.TrimSpace(os.Getenv(envAWSKMSAlgorithm)); v != "" {
		cfg.AWSKMSAlgorithm = v
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
	if v := strings.TrimSpace(os.Getenv(envAliKMSAlgorithm)); v != "" {
		cfg.AliKMSAlgorithm = v
	}

	if v := strings.TrimSpace(os.Getenv(envLocalKeyPath)); v != "" {
		cfg.LocalKeyPath = v
	}
	if v := strings.TrimSpace(os.Getenv(envLocalAlgorithm)); v != "" {
		cfg.LocalAlgorithm = v
	}

	if v := strings.TrimSpace(os.Getenv(envTSAProviders)); v != "" {
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(strings.ToLower(p))
			if p != "" {
				out = append(out, p)
			}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("config: %s parsed to empty list", envTSAProviders)
		}
		cfg.TSAProviders = out
	}

	if v := strings.TrimSpace(os.Getenv(envVerifyEndpoint)); v != "" {
		cfg.VerifyEndpoint = v
	}

	if v := strings.TrimSpace(os.Getenv(envPaymentHubWebhookSecret)); v != "" {
		cfg.PaymentHubWebhookSecret = v
	}

	if v := strings.TrimSpace(os.Getenv(envRefundInitiateStream)); v != "" {
		cfg.RefundInitiateStream = v
	}
	if v := strings.TrimSpace(os.Getenv(envRefundRetryStream)); v != "" {
		cfg.RefundRetryStream = v
	}
	if v := strings.TrimSpace(os.Getenv(envRefundDelayZoneKey)); v != "" {
		cfg.RefundDelayZoneKey = v
	}
	if v := strings.TrimSpace(os.Getenv(envRefundGroup)); v != "" {
		cfg.RefundGroup = v
	}
	if v := strings.TrimSpace(os.Getenv(envRefundConsumer)); v != "" {
		cfg.RefundConsumer = v
	}
	if v := strings.TrimSpace(os.Getenv(envRefundNotifierAddr)); v != "" {
		cfg.RefundNotifierAddr = v
	}
	if v := strings.TrimSpace(os.Getenv(envRefundNotifierQueue)); v != "" {
		cfg.RefundNotifierQueue = v
	}

	switch cfg.SignBackend {
	case "":
		// Sign backend optional for cmd/server (verify-only); cmd/generator
		// validates separately.
	case SignBackendAWS:
		if cfg.AWSKMSRegion == "" || cfg.AWSKMSKeyID == "" {
			return nil, fmt.Errorf("config: %s=%s requires %s + %s",
				envSignBackend, SignBackendAWS, envAWSKMSRegion, envAWSKMSKeyID)
		}
		akid := cfg.AWSKMSAccessKeyID != ""
		sec := cfg.AWSKMSSecretAccessKey != ""
		if akid != sec {
			return nil, fmt.Errorf("config: %s=%s requires both %s and %s or neither",
				envSignBackend, SignBackendAWS, envAWSKMSAccessKeyID, envAWSKMSSecretAccessKey)
		}
	case SignBackendAliyun:
		if cfg.AliKMSRegionID == "" || cfg.AliKMSAccessKeyID == "" ||
			cfg.AliKMSAccessKeySecret == "" || cfg.AliKMSKeyID == "" {
			return nil, fmt.Errorf("config: %s=%s requires %s / %s / %s / %s",
				envSignBackend, SignBackendAliyun,
				envAliKMSRegionID, envAliKMSAccessKeyID, envAliKMSAccessKeySecret, envAliKMSKeyID)
		}
	case SignBackendLocal:
		if cfg.LocalKeyPath == "" {
			return nil, fmt.Errorf("config: %s=%s requires %s",
				envSignBackend, SignBackendLocal, envLocalKeyPath)
		}
	default:
		return nil, fmt.Errorf("config: %s=%q must be %q, %q or %q",
			envSignBackend, cfg.SignBackend, SignBackendAWS, SignBackendAliyun, SignBackendLocal)
	}

	return cfg, nil
}

// Addr returns the host:port the HTTP server should bind.
func (c *Config) Addr() string { return fmt.Sprintf(":%d", c.Port) }
