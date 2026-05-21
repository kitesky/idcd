// attest-verify is the independent Self-Verify Service entry point.
//
// D6 (DECISIONS.md): this binary is a SEPARATE MODULE from apps/attest.
// It runs as an independent process, ideally on a separate VPC subnet,
// with its own HTTP connection pool. It MUST NOT import code from
// apps/attest and MUST NOT share in-process state with the Generator.
//
// The service periodically polls idcd_attest.attestation_record for
// reports that have been fully archived (action=s3_archived) but not yet
// verified by this service, then calls the PUBLIC POST /verify endpoint
// (attest.idcd.com/verify) for each one and logs results to
// idcd_attest.self_verify_log.
//
// Routes:
//
//	GET /healthz  — liveness probe (always 200 ok)
//	GET /readyz   — readiness probe (200 when DB is reachable)
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kite365/idcd/apps/attest-verify/internal/config"
	"github.com/kite365/idcd/apps/attest-verify/internal/poller"
)

const httpShutdownGrace = 5 * time.Second

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "attest-verify: load config: %v\n", err)
		os.Exit(1)
	}

	log := newLogger(cfg.LogLevel)
	log.Info("attest-verify starting",
		"addr", cfg.BindAddr,
		"env", cfg.Env,
		"verify_endpoint", cfg.VerifyEndpoint,
		"poll_interval", cfg.PollInterval,
	)

	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseDSN)
	if err != nil {
		log.Error("attest-verify: pgxpool init failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// D6: independent HTTP client — MUST NOT share Transport with any
	// apps/attest binary. The independence of the connection pool is part
	// of the audit trail demonstrating separate code paths.
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        16,
			MaxIdleConnsPerHost: 4,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	p := poller.New(poller.Config{
		Lister:         &dbLister{pool: pool},
		Writer:         &dbWriter{pool: pool},
		Fetcher:        &httpFetcher{client: httpClient},
		VerifyEndpoint: cfg.VerifyEndpoint,
		HTTPClient:     httpClient,
		PollInterval:   cfg.PollInterval,
		BatchSize:      cfg.BatchSize,
		Logger:         log,
	})

	go func() {
		if err := p.Run(ctx); err != nil {
			log.Error("attest-verify: poller stopped with error", "err", err)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			log.Warn("attest-verify: readyz DB ping failed", "err", err)
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              cfg.BindAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("attest-verify: listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case <-ctx.Done():
		log.Info("attest-verify: shutdown signal received")
	case err, ok := <-serverErr:
		if ok && err != nil {
			log.Error("attest-verify: ListenAndServe failed", "err", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), httpShutdownGrace)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Warn("attest-verify: graceful shutdown failed", "err", err)
	}
	log.Info("attest-verify: exited cleanly")
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

// dbLister queries idcd_attest for attestation_record rows that were
// successfully archived (action=s3_archived) but not yet verified by
// this service (no matching row in self_verify_log).
type dbLister struct {
	pool *pgxpool.Pool
}

// listPendingSQL finds s3_archived attestation records that lack a
// self_verify_log entry. LEFT JOIN NULL check avoids a correlated
// subquery and is index-friendly.
//
// D1: no cross-schema FK — record_id is a weak reference string.
const listPendingSQL = `
    SELECT ar.id, ar.report_id,
           COALESCE(vr.pdf_url, ''),
           COALESCE(vr.content_hash, '')
    FROM   idcd_attest.attestation_record ar
    JOIN   idcd_attest.verdict_report vr ON vr.id = ar.report_id
    LEFT JOIN idcd_attest.self_verify_log svl ON svl.record_id = ar.id
    WHERE  ar.action  = 's3_archived'
      AND  ar.status  = 'success'
      AND  svl.id IS NULL
    ORDER BY ar.created_at DESC
    LIMIT $1
`

func (l *dbLister) ListPending(ctx context.Context, limit int) ([]*poller.PendingRecord, error) {
	rows, err := l.pool.Query(ctx, listPendingSQL, limit)
	if err != nil {
		return nil, fmt.Errorf("dbLister.ListPending: %w", err)
	}
	defer rows.Close()

	var out []*poller.PendingRecord
	for rows.Next() {
		var r poller.PendingRecord
		if err := rows.Scan(&r.RecordID, &r.ReportID, &r.PDFURL, &r.ContentHash); err != nil {
			return nil, fmt.Errorf("dbLister.ListPending scan: %w", err)
		}
		out = append(out, &r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dbLister.ListPending rows: %w", err)
	}
	return out, nil
}

// dbWriter inserts a row into idcd_attest.self_verify_log.
// NULLIF($6, '') converts empty string to NULL for the error column.
type dbWriter struct {
	pool *pgxpool.Pool
}

const insertLogSQL = `
    INSERT INTO idcd_attest.self_verify_log
        (id, record_id, verified_at, status, latency_ms, error)
    VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''))
    ON CONFLICT (id) DO NOTHING
`

func (w *dbWriter) WriteLog(ctx context.Context, e *poller.LogEntry) error {
	_, err := w.pool.Exec(ctx, insertLogSQL,
		e.ID, e.RecordID, e.VerifiedAt, e.Status, e.LatencyMS, e.Err)
	if err != nil {
		return fmt.Errorf("dbWriter.WriteLog: %w", err)
	}
	return nil
}

// httpFetcher retrieves PDF bytes via HTTP/HTTPS or the local file:// scheme
// (file:// is useful in development and CI for injecting test PDFs).
type httpFetcher struct {
	client *http.Client
}

func (f *httpFetcher) Fetch(ctx context.Context, pdfURL string) ([]byte, error) {
	if strings.HasPrefix(pdfURL, "file://") {
		path := strings.TrimPrefix(pdfURL, "file://")
		return os.ReadFile(path)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pdfURL, nil)
	if err != nil {
		return nil, fmt.Errorf("httpFetcher.Fetch newrequest: %w", err)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpFetcher.Fetch do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("httpFetcher.Fetch: status %d for %s", resp.StatusCode, pdfURL)
	}
	// Cap at 64 MiB — same as apps/attest cmd/verifier.
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}
