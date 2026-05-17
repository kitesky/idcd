// attest-generator is the Verdict-generation worker.
//
// Consumes Redis Stream "verdict_generation_queue" — one message per
// paid verdict order — and runs the 10-step pipeline implemented by
// apps/attest/internal/service. The pipeline is idempotent (D4 WAL via
// attestation_record), so a worker crash mid-run is recovered safely:
// Redis redelivers the unacked message, the orchestrator re-reads the
// WAL, and externally-effecting steps (KMS sign, TSA stamp, WORM
// archive) are skipped if already complete.
//
// Wiring overview:
//
//   - Postgres pool         → repo.New(pool) → service via serviceadapter.
//   - KMS Signer (aws|ali)  → chosen by ATTEST_SIGN_BACKEND.
//   - tsa.Multi of providers → primaryAdapter wraps it back to tsa.Provider.
//   - Local file archiver   → S2 MVP; S3+ObjectLock lands in W7+.
//   - Signer X.509 cert     → wireSignerCert prefers ATTEST_SIGNER_CERT_PEM;
//     falls back to loadDevSignerCert (self-signed RSA-2048) in pre-prod.
//     When a production cert is configured wireSignerCert also binds
//     the cert's public key to the KMS signer's public key — start-up
//     refuses to proceed on mismatch.
package main

import (
	"context"
	"crypto"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/attest/internal/config"
	"github.com/kite365/idcd/apps/attest/internal/repo"
	"github.com/kite365/idcd/apps/attest/internal/service"
	"github.com/kite365/idcd/apps/attest/internal/serviceadapter"
	"github.com/kite365/idcd/apps/attest/internal/streamconsumer"
	"github.com/kite365/idcd/lib/attest/sign"
	"github.com/kite365/idcd/lib/attest/sign/alikms"
	"github.com/kite365/idcd/lib/attest/sign/awskms"
	"github.com/kite365/idcd/lib/attest/tsa"
	"github.com/kite365/idcd/lib/attest/tsa/digicert"
	"github.com/kite365/idcd/lib/attest/tsa/globalsign"
)

// verdictStream is the Redis Streams key the API enqueues on every
// successful payment. Naming follows the snake_case stream-name
// convention used by the api → attest contract.
const verdictStream = "verdict_generation_queue"

// verdictGroup is the consumer-group name for the attest-generator
// fleet. All replicas share this group so each message is delivered to
// exactly one worker.
const verdictGroup = "attest-generator"

// defaultArchiveDir is the local WORM directory used in S2 MVP. The
// directory is created on first archive. S3 + Object Lock lands in W7+
// and will swap localArchiver for an S3 implementation.
const defaultArchiveDir = "/var/lib/attest/archive"

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "attest-generator: load config: %v\n", err)
		os.Exit(1)
	}

	log := newLogger(cfg.LogLevel)
	log.Info("attest-generator starting",
		"env", cfg.Env,
		"sign_backend", cfg.SignBackend,
		"tsa_providers", cfg.TSAProviders,
		"redis_addr", cfg.RedisAddr,
	)

	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// --- Postgres -----------------------------------------------------
	pool, err := pgxpool.New(ctx, cfg.DatabaseDSN)
	if err != nil {
		log.Error("attest-generator: pgxpool init failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	repos := repo.New(pool)

	// --- Signer (KMS) -------------------------------------------------
	signer, err := buildSigner(cfg)
	if err != nil {
		log.Error("attest-generator: signer init", "err", err)
		os.Exit(1)
	}
	log.Info("attest-generator: signer wired",
		"key_id", signer.KeyID(),
		"algorithm", signer.Algorithm(),
	)

	// --- Verifier (KMS) -----------------------------------------------
	// We build a sign.Verifier alongside the sign.Signer so wireSignerCert
	// can bind the loaded X.509 cert's public key to the KMS-held key at
	// start-up (the Signer interface itself does not expose PublicKey;
	// the Verifier does). Both adapters point at the same KeyID so the
	// bind check is meaningful.
	verifier, err := buildVerifier(cfg)
	if err != nil {
		log.Error("attest-generator: verifier init", "err", err)
		os.Exit(1)
	}

	// --- TSA ----------------------------------------------------------
	multi, primary, err := buildTSA(cfg, log)
	if err != nil {
		log.Error("attest-generator: tsa init", "err", err)
		os.Exit(1)
	}
	log.Info("attest-generator: tsa providers wired",
		"order", multi.Names(),
		"primary_endpoint", primary.endpoint,
	)
	tsaProvider := &multiProviderAdapter{multi: multi, fallbackName: primary.name}

	// --- Signer X.509 cert -------------------------------------------
	signerCert, certChain, err := wireSignerCert(log, verifier)
	if err != nil {
		log.Error("attest-generator: signer cert load", "err", err)
		os.Exit(1)
	}

	// --- Archiver ----------------------------------------------------
	archiver, err := wireArchiver(log)
	if err != nil {
		log.Error("attest-generator: archiver init", "err", err)
		os.Exit(1)
	}

	// --- Observation pool (cross-schema read on idcd_main) -----------
	// Owned by main() and Close()d on shutdown — D1 cross-schema reads
	// go through this dedicated pool rather than reusing the attest
	// repos.New(pool) which talks to idcd_attest.
	obsPool, err := service.NewObservationPoolFromEnv(ctx)
	if err != nil {
		log.Error("attest-generator: observation pool init", "err", err)
		os.Exit(1)
	}
	defer obsPool.Close()
	log.Info("attest-generator: observation pool wired")

	// --- Service orchestrator ----------------------------------------
	svc := service.New(service.Config{
		Orders:             serviceadapter.WrapOrders(repos.Orders),
		Reports:            serviceadapter.WrapReports(repos.Reports),
		AttestationRecords: repos.AttestationRecords,
		Observations:       obsPool,
		Signer:             signer,
		TSA:                tsaProvider,
		SignerCert:         signerCert,
		CertChain:          certChain,
		TSAEndpoint:        primary.endpoint,
		Archiver:           archiver,
		Logger:             log,
	})

	// --- Redis Stream consumer ---------------------------------------
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  6 * time.Second, // > consumer BLOCK (5s) so we don't time out the read.
		WriteTimeout: 3 * time.Second,
	})
	defer func() { _ = rdb.Close() }()

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "attest-generator"
	}

	consumer := streamconsumer.New(streamconsumer.Config{
		Redis:    rdb,
		Stream:   verdictStream,
		Group:    verdictGroup,
		Consumer: hostname,
		Logger:   log,
		Handler: func(handlerCtx context.Context, fields map[string]any) error {
			orderID, ok := fields["order_id"].(string)
			if !ok || strings.TrimSpace(orderID) == "" {
				// Drop poison messages (no order_id) — non-retryable.
				// We return nil so XACK fires and the message stops
				// blocking the queue.
				log.Error("attest-generator: missing or non-string order_id; dropping",
					"fields", fmt.Sprintf("%v", fields))
				return nil
			}
			return svc.GenerateVerdict(handlerCtx, orderID)
		},
	})

	log.Info("attest-generator: consuming",
		"stream", verdictStream,
		"group", verdictGroup,
		"consumer", hostname,
	)
	if err := consumer.Run(ctx); err != nil {
		log.Error("attest-generator: consumer exited with error", "err", err)
		os.Exit(1)
	}
	log.Info("attest-generator: exited cleanly")
}

// --- buildSigner ----------------------------------------------------

func buildSigner(cfg *config.Config) (sign.Signer, error) {
	switch cfg.SignBackend {
	case config.SignBackendAWS:
		return awskms.New(awskms.Config{
			Region:          cfg.AWSKMSRegion,
			AccessKeyID:     cfg.AWSKMSAccessKeyID,
			SecretAccessKey: cfg.AWSKMSSecretAccessKey,
			KeyID:           cfg.AWSKMSKeyID,
			Algorithm:       cfg.AWSKMSAlgorithm,
		})
	case config.SignBackendAliyun:
		return alikms.New(alikms.Config{
			RegionID:        cfg.AliKMSRegionID,
			AccessKeyID:     cfg.AliKMSAccessKeyID,
			AccessKeySecret: cfg.AliKMSAccessKeySecret,
			KeyID:           cfg.AliKMSKeyID,
			Algorithm:       cfg.AliKMSAlgorithm,
		})
	default:
		return nil, fmt.Errorf("ATTEST_SIGN_BACKEND must be set to %q or %q",
			config.SignBackendAWS, config.SignBackendAliyun)
	}
}

// --- buildVerifier --------------------------------------------------

// buildVerifier mirrors buildSigner but constructs a sign.Verifier
// pointed at the same KMS KeyID. It's used at start-up only — to bind
// the loaded X.509 signer cert's public key to the KMS-held key. The
// generator does NOT verify signatures at runtime (Self-Verify Worker
// owns that, D6); this verifier exists solely for the pubkey bind.
func buildVerifier(cfg *config.Config) (sign.Verifier, error) {
	switch cfg.SignBackend {
	case config.SignBackendAWS:
		return awskms.NewVerifier(awskms.Config{
			Region:          cfg.AWSKMSRegion,
			AccessKeyID:     cfg.AWSKMSAccessKeyID,
			SecretAccessKey: cfg.AWSKMSSecretAccessKey,
			KeyID:           cfg.AWSKMSKeyID,
			Algorithm:       cfg.AWSKMSAlgorithm,
		})
	case config.SignBackendAliyun:
		return alikms.NewVerifier(alikms.Config{
			RegionID:        cfg.AliKMSRegionID,
			AccessKeyID:     cfg.AliKMSAccessKeyID,
			AccessKeySecret: cfg.AliKMSAccessKeySecret,
			KeyID:           cfg.AliKMSKeyID,
			Algorithm:       cfg.AliKMSAlgorithm,
		})
	default:
		return nil, fmt.Errorf("ATTEST_SIGN_BACKEND must be set to %q or %q",
			config.SignBackendAWS, config.SignBackendAliyun)
	}
}

// --- buildTSA -------------------------------------------------------

// tsaProviderInfo bundles a constructed Provider with the endpoint URL
// the orchestrator stamps onto pdfsign.SignRequest.TSAEndpoint (so the
// embed step can re-fetch a fresh token; see orchestrator.go step 8).
type tsaProviderInfo struct {
	name     string
	endpoint string
	provider tsa.Provider
}

func buildTSA(cfg *config.Config, log *slog.Logger) (*tsa.Multi, *tsaProviderInfo, error) {
	if len(cfg.TSAProviders) == 0 {
		return nil, nil, fmt.Errorf("ATTEST_TSA_PROVIDERS must list at least one provider")
	}
	infos := make([]*tsaProviderInfo, 0, len(cfg.TSAProviders))
	for _, name := range cfg.TSAProviders {
		info, err := newTSAProvider(name)
		if err != nil {
			return nil, nil, err
		}
		infos = append(infos, info)
	}
	provs := make([]tsa.Provider, len(infos))
	for i, info := range infos {
		provs[i] = info.provider
	}
	multi := tsa.NewMulti(provs...)
	multi.Logger = log
	return multi, infos[0], nil
}

// newTSAProvider constructs a single provider by short name. Endpoints
// fall back to the package's public defaults; bespoke endpoints can be
// added later via ATTEST_TSA_<NAME>_ENDPOINT.
func newTSAProvider(name string) (*tsaProviderInfo, error) {
	switch name {
	case "digicert":
		return &tsaProviderInfo{
			name:     "digicert",
			endpoint: digicert.DefaultEndpoint,
			provider: digicert.New(digicert.Config{}),
		}, nil
	case "globalsign":
		return &tsaProviderInfo{
			name:     "globalsign",
			endpoint: globalsign.DefaultEndpoint,
			provider: globalsign.New(globalsign.Config{}),
		}, nil
	default:
		return nil, fmt.Errorf("unknown TSA provider %q (supported: digicert, globalsign)", name)
	}
}

// multiProviderAdapter exposes a *tsa.Multi as a tsa.Provider. Multi's
// own Stamp() returns the chosen providerName as an extra value so it
// does not directly satisfy Provider; the adapter drops that value and
// reports `multi` as the Name() for upstream consumers that just want a
// "TSA stamped" indication.
//
// Name() returns the primary provider's name before any Stamp call so
// the orchestrator's verdict_report row has *something* recognisable
// when fall-over has not happened. The actual provider that produced a
// given token can be recovered from Multi's logs or from the embedded
// TSA certificate in the token blob.
type multiProviderAdapter struct {
	multi        *tsa.Multi
	fallbackName string
}

func (m *multiProviderAdapter) Name() string {
	if m.fallbackName != "" {
		return m.fallbackName
	}
	return "multi"
}

func (m *multiProviderAdapter) Stamp(ctx context.Context, hashAlg crypto.Hash, digest []byte) ([]byte, time.Time, error) {
	tok, ts, _, err := m.multi.Stamp(ctx, hashAlg, digest)
	return tok, ts, err
}

// newLogger picks a slog.Level from the config string and writes text
// format to stderr. Matches cmd/server / cmd/verifier wording.
func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}
