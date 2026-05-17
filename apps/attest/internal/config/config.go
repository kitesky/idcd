// Package config loads attest-svc settings from environment variables.
//
// Backend selection mirrors the cert-svc pattern (CERT_VAULT_BACKEND):
//
//   - ATTEST_SIGN_BACKEND = aws | aliyun  — picks the KMS adapter.
//   - ATTEST_TSA_PROVIDERS = digicert,globalsign — comma list, order
//     determines Multi fallover preference (primary first).
//
// All backend-specific env vars are validated in Load() so misconfigured
// production starts fail fast rather than silently degrading.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	envPort     = "ATTEST_PORT"
	envEnv      = "ATTEST_ENV"
	envLogLevel = "ATTEST_LOG_LEVEL"
	envDB       = "ATTEST_DB_DSN"
	envRedis    = "ATTEST_REDIS_ADDR"
	envS3Bucket = "ATTEST_S3_BUCKET" // WORM archive bucket; empty disables archival

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

	envTSAProviders = "ATTEST_TSA_PROVIDERS" // comma list: digicert,globalsign

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

	DatabaseDSN string
	RedisAddr   string
	S3Bucket    string

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

// Load reads ATTEST_* env vars and returns a populated Config.
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
	if v := strings.TrimSpace(os.Getenv(envS3Bucket)); v != "" {
		cfg.S3Bucket = v
	}

	cfg.SignBackend = strings.ToLower(strings.TrimSpace(os.Getenv(envSignBackend)))

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
	default:
		return nil, fmt.Errorf("config: %s=%q must be %q or %q",
			envSignBackend, cfg.SignBackend, SignBackendAWS, SignBackendAliyun)
	}

	return cfg, nil
}

// Addr returns the host:port the HTTP server should bind.
func (c *Config) Addr() string { return fmt.Sprintf(":%d", c.Port) }
