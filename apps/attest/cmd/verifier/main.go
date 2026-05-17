// attest-verifier is the Self-Verify Worker entry point.
//
// D6 (DECISIONS.md): this binary runs as an INDEPENDENT process, ideally
// on its own VPC subnet, with its own KMS client and HTTP connection
// pool. It re-verifies every freshly minted verdict PDF by calling ONLY
// the public /verify endpoint (attest.idcd.com/verify), guaranteeing
// that the path third-party auditors hit is exercised continuously by
// our own infrastructure.
//
// Wiring:
//
//   - Postgres pool        → repo.New(pool); a thin pendingReportsLister
//     adapter wraps a SELECT over idcd_attest.verdict_report to materialise
//     the selfverify.PendingReport shape (the repo package intentionally
//     doesn't expose ListPendingSelfVerify yet).
//   - VerdictReportsRepo   → selfverify.ReportUpdater directly.
//   - AttestationRecordsRepo → satisfies attestrec.Repository directly.
//   - PDF fetcher          → urlFetcher dispatches file:// + http(s):// to
//     httpFetcher, and s3:// to an aws-sdk-go-v2 backed s3Fetcher built
//     lazily on first use (so boot doesn't require S3 env vars when no
//     archive URL is in flight).
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kite365/idcd/apps/attest/internal/config"
	"github.com/kite365/idcd/apps/attest/internal/repo"
	"github.com/kite365/idcd/apps/attest/internal/selfverify"
)

// defaultVerifyEndpoint is the production /verify URL. D6 requires the
// worker to talk to the public path; we default to production so a
// misconfigured deploy fails OPEN (correct target) rather than silently
// hitting a stub.
const defaultVerifyEndpoint = "https://attest.idcd.com/verify"

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "attest-verifier: load config: %v\n", err)
		os.Exit(1)
	}
	if strings.TrimSpace(cfg.VerifyEndpoint) == "" {
		cfg.VerifyEndpoint = defaultVerifyEndpoint
	}
	if strings.TrimSpace(cfg.DatabaseDSN) == "" {
		fmt.Fprintln(os.Stderr, "attest-verifier: ATTEST_DB_DSN is required")
		os.Exit(1)
	}

	log := newLogger(cfg.LogLevel)
	log.Info("attest-verifier starting",
		"endpoint", cfg.VerifyEndpoint,
		"env", cfg.Env,
	)

	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// --- Postgres -----------------------------------------------------
	pool, err := pgxpool.New(ctx, cfg.DatabaseDSN)
	if err != nil {
		log.Error("attest-verifier: pgxpool init failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	repos := repo.New(pool)

	// D6: brand-new http.Client. Do NOT share Transport with the
	// generator — the whole point of running a separate worker is to
	// have an independent connection pool.
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        16,
			MaxIdleConnsPerHost: 4,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	fetcher := &urlFetcher{
		log:    log,
		http:   &httpFetcher{client: httpClient},
		s3Once: &sync.Once{},
	}

	w := selfverify.New(selfverify.Config{
		Lister:             &pendingReportsLister{pool: pool, log: log},
		Updater:            repos.Reports,
		AttestationRecords: repos.AttestationRecords,
		Fetcher:            fetcher,
		VerifyEndpoint:     cfg.VerifyEndpoint,
		HTTPClient:         httpClient,
		PollInterval:       30 * time.Second,
		BatchSize:          50,
		Logger:             log,
	})

	if err := w.Run(ctx); err != nil {
		log.Error("attest-verifier stopped with error", "err", err)
		os.Exit(1)
	}
	log.Info("attest-verifier exited cleanly")
}

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

// ---------------------------------------------------------------------
// pendingReportsLister: adapter that materialises selfverify.PendingReport
// rows from idcd_attest.verdict_report. Lives at the cmd boundary because
// the repo package intentionally exposes only narrow CRUD + state-machine
// helpers; the projection shape belongs to the consumer.
// ---------------------------------------------------------------------

type pendingReportsLister struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

const listPendingSelfVerifySQL = `
	SELECT id, pdf_url, content_hash
	FROM idcd_attest.verdict_report
	WHERE self_verify_status = 'pending'
	ORDER BY created_at ASC
	LIMIT $1
`

func (l *pendingReportsLister) ListPendingSelfVerify(ctx context.Context, limit int) ([]*selfverify.PendingReport, error) {
	rows, err := l.pool.Query(ctx, listPendingSelfVerifySQL, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending self_verify: %w", err)
	}
	defer rows.Close()

	out := make([]*selfverify.PendingReport, 0, limit)
	for rows.Next() {
		var r selfverify.PendingReport
		if err := rows.Scan(&r.ID, &r.PDFURL, &r.ContentHash); err != nil {
			return nil, fmt.Errorf("scan pending self_verify: %w", err)
		}
		out = append(out, &r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending self_verify: %w", err)
	}
	return out, nil
}

// ---------------------------------------------------------------------
// urlFetcher: dispatches by scheme. s3Fetcher is lazily built on first
// use so boot succeeds without S3 env vars when no archive URL is in
// flight (per task constraint).
// ---------------------------------------------------------------------

type urlFetcher struct {
	log    *slog.Logger
	http   *httpFetcher
	s3Once *sync.Once
	s3     *s3Fetcher
	s3Err  error
}

func (f *urlFetcher) Fetch(ctx context.Context, pdfURL string) ([]byte, error) {
	switch {
	case strings.HasPrefix(pdfURL, "s3://"):
		f.s3Once.Do(func() {
			s, err := newS3FetcherFromEnv(ctx)
			if err != nil {
				f.s3Err = err
				return
			}
			f.s3 = s
		})
		if f.s3Err != nil {
			return nil, f.s3Err
		}
		return f.s3.Fetch(ctx, pdfURL)
	default:
		return f.http.Fetch(ctx, pdfURL)
	}
}

// httpFetcher handles file://, http://, https://.
type httpFetcher struct {
	client *http.Client
}

func (f *httpFetcher) Fetch(ctx context.Context, pdfURL string) ([]byte, error) {
	switch {
	case strings.HasPrefix(pdfURL, "file://"):
		path := strings.TrimPrefix(pdfURL, "file://")
		return os.ReadFile(path)
	case strings.HasPrefix(pdfURL, "http://"), strings.HasPrefix(pdfURL, "https://"):
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pdfURL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := f.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetch %s: status %d", pdfURL, resp.StatusCode)
		}
		// Cap at 64 MiB; v2 verdict PDFs are typically <10 MiB.
		return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	default:
		return nil, fmt.Errorf("unsupported URL scheme: %s", pdfURL)
	}
}
