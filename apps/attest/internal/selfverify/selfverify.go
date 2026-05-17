// Package selfverify implements the Self-Verify Worker described in
// docs/prd/18-evidence-and-attestation.md §3.5 and DECISIONS.md D6.
//
// Design intent (D6 — independence boundary):
//
//   - The Self-Verify Worker runs as a SEPARATE PROCESS (cmd/verifier),
//     ideally on a SEPARATE VPC subnet, with its own KMS client and its
//     own HTTP connection pool. It MUST NOT share internal state, code
//     paths or caches with the Generator.
//
//   - The worker re-verifies each freshly minted verdict PDF by calling
//     ONLY the public /verify endpoint (attest.idcd.com/verify). This
//     guarantees we exercise the exact same code path a third-party
//     auditor or end-user would, so a regression in the verify handler
//     surfaces here before it surfaces in production complaints.
//
//   - The verdict_report row’s self_verify_status field flips to
//     pass / fail; every attempt also writes a step-level WAL row
//     (attestation_record, action=self_verified) per D4 so retries are
//     observable.
//
// Failure handling: a failure flips self_verify_status to fail and
// records the reason. If a RefundEnqueuer is wired (Config.RefundEnqueuer
// non-nil), the worker also enqueues a refund-initiate job for the
// separate Refund Worker binary to consume (D5). The Refund Worker calls
// Paddle's refund API; if that call itself fails, the Paddle webhook
// flow's refund_retry_queue takes over the retry cadence.
package selfverify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"time"

	attestrec "github.com/kite365/idcd/lib/attest/record"
)

// PendingReport is the minimal view of a verdict_report row the Worker
// needs. The repo layer materialises this shape from the v2 table.
type PendingReport struct {
	// ID is the verdict_report PK (vr_*).
	ID string
	// PDFURL is a fetchable location of the signed verdict PDF. The
	// httpFetcher supports file://, http://, https://; s3:// is left for
	// the WORM archiver milestone.
	PDFURL string
	// ContentHash is the hex sha-256 of the PDF as recorded at generation
	// time. The /verify endpoint echoes the hash it computed; when both
	// are present we cross-check them so a swapped archive object is
	// detected even if /verify still calls it valid.
	ContentHash string
}

// PendingReportLister is the consumer-side interface for fetching pending
// reports. Defined here (not in the repo package) so the worker tests
// can stub it without pulling a DB driver.
type PendingReportLister interface {
	ListPendingSelfVerify(ctx context.Context, limit int) ([]*PendingReport, error)
}

// ReportUpdater flips verdict_report.self_verify_status (and the
// accompanying timestamp) after each verification round.
type ReportUpdater interface {
	UpdateSelfVerify(ctx context.Context, reportID string, status string, at time.Time) error
}

// PDFFetcher abstracts where the PDF bytes live (S3 / local file / HTTP).
type PDFFetcher interface {
	Fetch(ctx context.Context, pdfURL string) ([]byte, error)
}

// RefundEnqueuer is the optional hand-off to the Refund Worker (D5).
// recordFailure calls EnqueueRefund after self_verify_status flips to
// "fail" so the user gets refunded. Implementations should be best-effort
// — an enqueue error must not regress the recordFailure return value
// (the report is still marked failed; an operator alert is the recovery
// path).
type RefundEnqueuer interface {
	EnqueueRefund(ctx context.Context, reportID, reason string) error
}

// Self-verify result strings persisted on verdict_report. Kept in this
// package so wiring code shares the same constants.
const (
	StatusPass = "pass"
	StatusFail = "fail"
)

// Defaults applied when Config leaves a field zero.
const (
	defaultPollInterval = 30 * time.Second
	defaultBatchSize    = 50
	defaultHTTPTimeout  = 30 * time.Second
)

// Config bundles the dependencies the Worker needs. All fields except
// Logger are required; New panics if a required field is nil.
type Config struct {
	Lister             PendingReportLister
	Updater            ReportUpdater
	AttestationRecords attestrec.Repository
	Fetcher            PDFFetcher

	// RefundEnqueuer is optional. nil disables refund-on-failure
	// (pre-prod / smoke environments don't need to charge real users).
	RefundEnqueuer RefundEnqueuer

	// VerifyEndpoint is the FULL public URL of /verify, e.g.
	//   https://attest.idcd.com/verify
	// D6 mandates this be the public-facing URL — never an internal
	// loopback — so the worker exercises identical code to third-party
	// callers.
	VerifyEndpoint string

	// HTTPClient is dedicated to the worker. Callers MUST instantiate a
	// fresh client (do not reuse Generator's pool — D6 independence).
	HTTPClient *http.Client

	PollInterval time.Duration
	BatchSize    int

	Logger *slog.Logger

	// now is overridable for tests; nil means time.Now.
	now func() time.Time
}

// Worker periodically scans for verdict_report rows with
// self_verify_status='pending' and exercises the public /verify endpoint
// for each.
type Worker struct {
	cfg Config
}

// New returns a Worker with defaults filled in. Panics if any required
// dependency is nil — misconfiguration here is a programming error.
func New(cfg Config) *Worker {
	if cfg.Lister == nil {
		panic("selfverify: Lister is required")
	}
	if cfg.Updater == nil {
		panic("selfverify: Updater is required")
	}
	if cfg.AttestationRecords == nil {
		panic("selfverify: AttestationRecords is required")
	}
	if cfg.Fetcher == nil {
		panic("selfverify: Fetcher is required")
	}
	if cfg.VerifyEndpoint == "" {
		panic("selfverify: VerifyEndpoint is required (D6: must be public URL)")
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultPollInterval
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultBatchSize
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.now == nil {
		cfg.now = func() time.Time { return time.Now().UTC() }
	}
	return &Worker{cfg: cfg}
}

// Run blocks until ctx is cancelled. On each tick it fetches up to
// BatchSize pending reports and verifies them sequentially.
//
// Errors from individual reports are logged but do not stop the loop —
// the WAL row + status flip already record the failure, and the next
// tick will pick up anything left in pending.
func (w *Worker) Run(ctx context.Context) error {
	w.cfg.Logger.Info("selfverify worker starting",
		"endpoint", w.cfg.VerifyEndpoint,
		"poll_interval", w.cfg.PollInterval,
		"batch_size", w.cfg.BatchSize,
	)

	// Run one tick immediately so a freshly started worker is not idle
	// for PollInterval before doing useful work.
	w.tick(ctx)

	t := time.NewTicker(w.cfg.PollInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			w.cfg.Logger.Info("selfverify worker stopping", "reason", ctx.Err())
			return nil
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	reports, err := w.cfg.Lister.ListPendingSelfVerify(ctx, w.cfg.BatchSize)
	if err != nil {
		w.cfg.Logger.Error("selfverify: list pending failed", "err", err)
		return
	}
	if len(reports) == 0 {
		return
	}
	w.cfg.Logger.Info("selfverify: tick", "count", len(reports))
	for _, r := range reports {
		if ctx.Err() != nil {
			return
		}
		if err := w.VerifyOne(ctx, r); err != nil {
			// VerifyOne already persisted a failure WAL row + flipped
			// self_verify_status, so we only log here.
			w.cfg.Logger.Warn("selfverify: report failed",
				"report_id", r.ID, "err", err)
		}
	}
}

// VerifyOne runs a single end-to-end re-verification cycle for one
// report. It is exported so the CLI / tests can drive it directly.
//
// On any failure the WAL row + verdict_report.self_verify_status are
// updated before returning the error.
func (w *Worker) VerifyOne(ctx context.Context, rep *PendingReport) error {
	if rep == nil {
		return errors.New("selfverify: nil report")
	}

	pdfBytes, err := w.cfg.Fetcher.Fetch(ctx, rep.PDFURL)
	if err != nil {
		return w.recordFailure(ctx, rep.ID, "fetch: "+err.Error())
	}

	body, contentType, err := buildMultipart(rep.ID, pdfBytes)
	if err != nil {
		return w.recordFailure(ctx, rep.ID, "multipart: "+err.Error())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.cfg.VerifyEndpoint, body)
	if err != nil {
		return w.recordFailure(ctx, rep.ID, "newrequest: "+err.Error())
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := w.cfg.HTTPClient.Do(req)
	if err != nil {
		return w.recordFailure(ctx, rep.ID, "http: "+err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Cap the snippet so we don't log an unbounded body.
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return w.recordFailure(ctx, rep.ID,
			fmt.Sprintf("status %d: %s", resp.StatusCode, string(snippet)))
	}

	var verifyResp struct {
		Valid         bool   `json:"valid"`
		Reason        string `json:"reason"`
		ContentSHA256 string `json:"content_sha256"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&verifyResp); err != nil {
		return w.recordFailure(ctx, rep.ID, "decode: "+err.Error())
	}

	if !verifyResp.Valid {
		return w.recordFailure(ctx, rep.ID, "verify rejected: "+verifyResp.Reason)
	}

	// Cross-check: if the generator recorded a hash, /verify's reported
	// hash MUST match it; otherwise the archived object differs from
	// what we signed even though signature math still passes.
	if rep.ContentHash != "" && verifyResp.ContentSHA256 != "" &&
		verifyResp.ContentSHA256 != rep.ContentHash {
		return w.recordFailure(ctx, rep.ID,
			fmt.Sprintf("hash mismatch: expected %s got %s",
				rep.ContentHash, verifyResp.ContentSHA256))
	}

	return w.recordSuccess(ctx, rep.ID)
}

func (w *Worker) recordSuccess(ctx context.Context, reportID string) error {
	now := w.cfg.now()
	completed := now
	rec := &attestrec.Record{
		ID:          attestrec.NewRecordID(),
		ReportID:    reportID,
		Action:      attestrec.ActionSelfVerified,
		Status:      attestrec.StatusSuccess,
		Result:      attestrec.ResultSuccess,
		CreatedAt:   now,
		CompletedAt: &completed,
	}
	if err := w.cfg.AttestationRecords.Insert(ctx, rec); err != nil &&
		!errors.Is(err, attestrec.ErrDuplicateAction) {
		// Duplicate is benign (retry after we already wrote success); any
		// other error we surface in logs but still flip status so the
		// verdict_report row leaves pending. The next tick re-attempts
		// only if status was not flipped.
		w.cfg.Logger.Error("selfverify: insert success WAL failed",
			"report_id", reportID, "err", err)
	}
	if err := w.cfg.Updater.UpdateSelfVerify(ctx, reportID, StatusPass, now); err != nil {
		w.cfg.Logger.Error("selfverify: update self_verify_status=pass failed",
			"report_id", reportID, "err", err)
		return fmt.Errorf("update self_verify_status: %w", err)
	}
	return nil
}

func (w *Worker) recordFailure(ctx context.Context, reportID, reason string) error {
	now := w.cfg.now()
	completed := now
	rec := &attestrec.Record{
		ID:          attestrec.NewRecordID(),
		ReportID:    reportID,
		Action:      attestrec.ActionSelfVerified,
		Status:      attestrec.StatusFailure,
		Result:      attestrec.ResultFailure,
		ErrorDetail: reason,
		CreatedAt:   now,
		CompletedAt: &completed,
	}
	if err := w.cfg.AttestationRecords.Insert(ctx, rec); err != nil &&
		!errors.Is(err, attestrec.ErrDuplicateAction) {
		w.cfg.Logger.Error("selfverify: insert failure WAL failed",
			"report_id", reportID, "err", err)
	}
	if err := w.cfg.Updater.UpdateSelfVerify(ctx, reportID, StatusFail, now); err != nil {
		w.cfg.Logger.Error("selfverify: update self_verify_status=fail failed",
			"report_id", reportID, "err", err)
	}
	if w.cfg.RefundEnqueuer != nil {
		if err := w.cfg.RefundEnqueuer.EnqueueRefund(ctx, reportID, reason); err != nil {
			w.cfg.Logger.Error("selfverify: enqueue refund job failed",
				"report_id", reportID, "err", err)
		}
	}
	return fmt.Errorf("self-verify failed: %s", reason)
}

// buildMultipart constructs the multipart/form-data body the public
// /verify handler expects. Extracted so tests can validate the wire
// format without exercising HTTP.
func buildMultipart(reportID string, pdfBytes []byte) (*bytes.Buffer, string, error) {
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, err := mw.CreateFormFile("file", reportID+".pdf")
	if err != nil {
		return nil, "", err
	}
	if _, err := fw.Write(pdfBytes); err != nil {
		return nil, "", err
	}
	if err := mw.Close(); err != nil {
		return nil, "", err
	}
	return body, mw.FormDataContentType(), nil
}
