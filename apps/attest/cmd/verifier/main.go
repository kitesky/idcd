// attest-verifier is the Self-Verify Worker entry point.
//
// D6 (DECISIONS.md): this binary runs as an INDEPENDENT process, ideally
// on its own VPC subnet, with its own KMS client and HTTP connection
// pool. It re-verifies every freshly minted verdict PDF by calling ONLY
// the public /verify endpoint (attest.idcd.com/verify), guaranteeing
// that the path third-party auditors hit is exercised continuously by
// our own infrastructure.
//
// Wiring note: the v2 verdict_report Repository + WORM archive fetcher
// land in apps/attest/internal/repo via a separate agent. To keep this
// binary independently buildable today, main wires stub implementations
// that produce no work. The wiring agent will replace the stubs in a
// follow-up commit.
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

	"github.com/kite365/idcd/apps/attest/internal/config"
	"github.com/kite365/idcd/apps/attest/internal/selfverify"
	attestrec "github.com/kite365/idcd/lib/attest/record"
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

	log := newLogger(cfg.LogLevel)
	log.Info("attest-verifier starting",
		"endpoint", cfg.VerifyEndpoint,
		"env", cfg.Env,
	)

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

	w := selfverify.New(selfverify.Config{
		Lister:             &stubLister{log: log},
		Updater:            &stubUpdater{log: log},
		AttestationRecords: &stubRecords{log: log},
		Fetcher:            &httpFetcher{client: httpClient},
		VerifyEndpoint:     cfg.VerifyEndpoint,
		HTTPClient:         httpClient,
		PollInterval:       30 * time.Second,
		BatchSize:          50,
		Logger:             log,
	})

	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer cancel()

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
// httpFetcher: supports file://, http://, https://.
// s3:// is intentionally unimplemented — the WORM archiver is also a
// stub in S2 MVP; once it lands, swap this for an s3.Client-backed
// fetcher in the wiring commit.
// ---------------------------------------------------------------------

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
	case strings.HasPrefix(pdfURL, "s3://"):
		return nil, errors.New("s3:// fetch not implemented (S2 MVP); wiring agent replaces this")
	default:
		return nil, fmt.Errorf("unsupported URL scheme: %s", pdfURL)
	}
}

// ---------------------------------------------------------------------
// Stub repos. Replaced by the wiring agent once
// apps/attest/internal/repo exposes verdict_report + attestation_record
// implementations.
// ---------------------------------------------------------------------

type stubLister struct{ log *slog.Logger }

func (s *stubLister) ListPendingSelfVerify(_ context.Context, _ int) ([]*selfverify.PendingReport, error) {
	s.log.Debug("stubLister: no work (repo wiring pending)")
	return nil, nil
}

type stubUpdater struct{ log *slog.Logger }

func (s *stubUpdater) UpdateSelfVerify(_ context.Context, reportID, status string, _ time.Time) error {
	s.log.Warn("stubUpdater: dropping update (repo wiring pending)",
		"report_id", reportID, "status", status)
	return nil
}

type stubRecords struct{ log *slog.Logger }

func (s *stubRecords) Insert(_ context.Context, r *attestrec.Record) error {
	s.log.Warn("stubRecords: dropping WAL row (repo wiring pending)",
		"report_id", r.ReportID, "action", r.Action, "status", r.Status)
	return nil
}
func (s *stubRecords) Get(context.Context, string, attestrec.Action) (*attestrec.Record, error) {
	return nil, attestrec.ErrNotFound
}
func (s *stubRecords) Update(context.Context, *attestrec.Record) error { return nil }
func (s *stubRecords) ListByReport(context.Context, string) ([]*attestrec.Record, error) {
	return nil, nil
}
