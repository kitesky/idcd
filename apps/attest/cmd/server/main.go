// attest-server is the Evidence/Attestation HTTP API entry point.
//
// Routes (S2):
//
//	GET  /healthz             — liveness probe
//	POST /verify              — multipart PDF upload, returns VerifyResult
//	GET  /verify/{report_id}  — re-publishes stored signature metadata
//
// The verifier (KMS Verifier) is selected at startup by
// ATTEST_SIGN_BACKEND. ATTEST_DB_DSN drives the Postgres pool used by
// the GET-by-id lookup. Generator and Self-Verify workers run as
// separate binaries (cmd/generator, cmd/verifier) so a misbehaving
// KMS / DB on the verdict pipeline cannot DoS the public verify path.
//
// POST /webhooks/paymenthub is mounted when ATTEST_PAYMENT_HUB_WEBHOOK_SECRET
// is set, driving D5 refund processing (refund_retry_queue enqueue on
// transient DB failure). When the secret is unset the route is omitted
// and verify still comes up — PaymentHub integration is optional in dev.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/attest/internal/config"
	"github.com/kite365/idcd/apps/attest/internal/handler/paymenthub"
	"github.com/kite365/idcd/apps/attest/internal/handler/verify"
	"github.com/kite365/idcd/apps/attest/internal/repo"
	"github.com/kite365/idcd/lib/attest/sign"
	"github.com/kite365/idcd/lib/attest/sign/alikms"
	"github.com/kite365/idcd/lib/attest/sign/awskms"
)

// maxVerifyPDFBytes caps multipart upload size at 32 MiB — see
// verify.DefaultMaxPDFBytes for rationale.
const maxVerifyPDFBytes = 32 << 20

// httpShutdownGrace is how long Shutdown waits for in-flight requests
// to complete after SIGTERM. 5s matches the pattern used by other
// idcd HTTP binaries (apps/api, apps/gateway).
const httpShutdownGrace = 5 * time.Second

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "attest-server: load config: %v\n", err)
		os.Exit(1)
	}

	log := newLogger(cfg.LogLevel)
	log.Info("attest-server starting",
		"port", cfg.Port,
		"env", cfg.Env,
		"sign_backend", cfg.SignBackend,
		"tsa_providers", cfg.TSAProviders,
	)

	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseDSN)
	if err != nil {
		log.Error("attest-server: pgxpool init failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	repos := repo.New(pool)

	verifier, err := buildVerifier(cfg)
	if err != nil {
		log.Error("attest-server: verifier init", "err", err)
		os.Exit(1)
	}
	log.Info("attest-server: verifier wired",
		"key_id", verifier.KeyID(),
		"algorithm", verifier.Algorithm(),
	)

	vh := &verify.Handler{
		Verifier: verifier,
		ReportLookup: &reportLookupAdapter{
			reports: repos.Reports,
		},
		Logger:      log,
		MaxPDFBytes: maxVerifyPDFBytes,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	// Mount verify at both "/verify" (POST) and "/verify/" (GET by id).
	// ServeHTTP does internal path normalisation; the trailing-slash
	// pattern catches everything under /verify/<id>.
	mux.Handle("/verify", vh)
	mux.Handle("/verify/", vh)

	if cfg.PaymentHubWebhookSecret == "" {
		log.Warn("attest-server: ATTEST_PAYMENT_HUB_WEBHOOK_SECRET unset; /webhooks/paymenthub disabled")
	} else {
		rdb := redis.NewClient(&redis.Options{
			Addr:         cfg.RedisAddr,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
		})
		defer func() { _ = rdb.Close() }()

		ph := &paymenthub.Handler{
			Secret: []byte(cfg.PaymentHubWebhookSecret),
			Lookup: &extOrderLookupAdapter{pool: pool},
			Orders: repos.Orders,
			Redis:  rdb,
			Logger: log,
		}
		mux.Handle("/webhooks/paymenthub", ph)
		log.Info("attest-server: paymenthub webhook mounted")
	}

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("attest-server: listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case <-ctx.Done():
		log.Info("attest-server: shutdown signal received")
	case err, ok := <-serverErr:
		if ok && err != nil {
			log.Error("attest-server: ListenAndServe failed", "err", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), httpShutdownGrace)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Warn("attest-server: graceful shutdown failed", "err", err)
	}
	log.Info("attest-server: exited cleanly")
}

// buildVerifier picks the KMS Verifier implementation based on
// cfg.SignBackend. The server can verify-only, but for S2 we require
// a backend to be configured: a misconfigured deploy that fell back to
// a stub would silently make every verify call return false, which is
// worse than refusing to start.
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

// reportLookupAdapter satisfies verify.ReportLookup over the verdict
// reports repo. It translates repo.ErrNotFound to
// verify.ErrReportNotFound (the handler maps the latter to HTTP 404).
type reportLookupAdapter struct {
	reports *repo.VerdictReportsRepo
}

func (a *reportLookupAdapter) LookupByID(ctx context.Context, reportID string) (*verify.KnownReport, error) {
	r, err := a.reports.GetByID(ctx, reportID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, verify.ErrReportNotFound
		}
		return nil, err
	}
	return &verify.KnownReport{
		ID:             r.ID,
		ContentHash:    r.ContentHash,
		Signature:      r.Signature,
		SignatureKeyID: r.SignatureKeyID,
		TSAProvider:    r.TSAProvider,
		TSATime:        r.TSATime,
		ReportType:     r.ReportType,
	}, nil
}

// extOrderLookupAdapter satisfies paymenthub.OrderLookup by querying
// idcd_attest.verdict_order directly on the pool. We bypass the repo
// layer because Lookup-by-ext_order_id is webhook-handler-specific
// and adding it to the shared repo would broaden that package's API.
type extOrderLookupAdapter struct {
	pool *pgxpool.Pool
}

func (a *extOrderLookupAdapter) LookupByExtOrderID(ctx context.Context, extOrderID string) (string, string, error) {
	const q = `SELECT id, status FROM idcd_attest.verdict_order WHERE paddle_order_id = $1`
	var id, status string
	if err := a.pool.QueryRow(ctx, q, extOrderID).Scan(&id, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", paymenthub.ErrOrderNotFound
		}
		return "", "", err
	}
	return id, status, nil
}

// newLogger mirrors cmd/verifier so log-level env handling is uniform.
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
