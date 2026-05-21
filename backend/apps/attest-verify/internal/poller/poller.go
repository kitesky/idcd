// Package poller implements the Self-Verify polling loop for the
// independent attest-verify service.
//
// D6 independence boundary: this package MUST NOT import anything from
// apps/attest. It communicates with the attestation layer exclusively via
// the public POST /verify HTTP endpoint and writes its own self_verify_log
// table — never touching the generator's internal state, DB connections,
// or KMS client.
//
// The independence is the point: a bug shared between the generator and
// the verifier would make self-verification meaningless. Running this as
// a separate Go module (not just a separate goroutine) makes the dependency
// boundary machine-enforceable.
package poller

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"time"
)

// PendingRecord is a row returned by RecordLister: an attestation_record
// that has been fully archived and needs independent re-verification by
// the public /verify endpoint.
type PendingRecord struct {
	RecordID    string // attestation_record.id (used as identifier in self_verify_log)
	ReportID    string // verdict_report.id
	PDFURL      string // fetchable URL of the signed verdict PDF
	ContentHash string // hex sha256 recorded at generation time; "" means skip cross-check
}

// RecordLister lists attestation records ready for self-verification.
// The concrete implementation in cmd/verifier queries idcd_attest via pgx.
// Using an interface here keeps the poller package free of DB dependencies.
type RecordLister interface {
	ListPending(ctx context.Context, limit int) ([]*PendingRecord, error)
}

// LogWriter persists the result of each verification attempt to
// idcd_attest.self_verify_log. Separate from verdict_report.self_verify_status
// which is owned by apps/attest.
type LogWriter interface {
	WriteLog(ctx context.Context, entry *LogEntry) error
}

// PDFFetcher retrieves raw PDF bytes from a URL (http, https, or file://).
// The concrete implementation in cmd/verifier supports http and file schemes.
type PDFFetcher interface {
	Fetch(ctx context.Context, url string) ([]byte, error)
}

// LogEntry is one row written to idcd_attest.self_verify_log.
type LogEntry struct {
	ID         string
	RecordID   string
	VerifiedAt time.Time
	Status     string // pass | fail | error
	LatencyMS  int64
	Err        string // empty on pass
}

// VerifyResponse is the JSON body returned by POST /verify (attest.idcd.com).
// Only the fields the poller needs are included; additional fields are ignored.
type VerifyResponse struct {
	Valid         bool   `json:"valid"`
	Reason        string `json:"reason"`
	ContentSHA256 string `json:"content_sha256"`
}

const (
	StatusPass  = "pass"
	StatusFail  = "fail"
	StatusError = "error"
)

const (
	DefaultPollInterval = 5 * time.Minute
	DefaultBatchSize    = 20
	DefaultHTTPTimeout  = 30 * time.Second
)

// Config holds all dependencies for the Poller.
//
// Note: no sign.Verifier, KMS client, or cryptographic material here.
// D6: this service must not hold any key material — verification must go
// through the public HTTP endpoint.
type Config struct {
	Lister         RecordLister
	Writer         LogWriter
	Fetcher        PDFFetcher
	VerifyEndpoint string
	HTTPClient     *http.Client
	PollInterval   time.Duration
	BatchSize      int
	Logger         *slog.Logger
	// now is injectable for tests; nil → time.Now().UTC()
	now func() time.Time
}

// Poller is the attest-verify service main loop.
type Poller struct {
	cfg Config
}

// New returns a Poller with defaults filled in. Panics if required fields are nil.
func New(cfg Config) *Poller {
	if cfg.Lister == nil {
		panic("poller: Lister is required")
	}
	if cfg.Writer == nil {
		panic("poller: Writer is required")
	}
	if cfg.Fetcher == nil {
		panic("poller: Fetcher is required")
	}
	if cfg.VerifyEndpoint == "" {
		panic("poller: VerifyEndpoint is required (D6: must be public URL)")
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: DefaultHTTPTimeout}
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = DefaultPollInterval
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = DefaultBatchSize
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.now == nil {
		cfg.now = func() time.Time { return time.Now().UTC() }
	}
	return &Poller{cfg: cfg}
}

// Run blocks until ctx is cancelled, running one poll tick per interval.
// Individual record errors are logged but do not stop the loop.
func (p *Poller) Run(ctx context.Context) error {
	p.cfg.Logger.Info("attest-verifier poller starting",
		"endpoint", p.cfg.VerifyEndpoint,
		"poll_interval", p.cfg.PollInterval,
		"batch_size", p.cfg.BatchSize,
	)

	// Run one tick immediately on startup.
	p.tick(ctx)

	t := time.NewTicker(p.cfg.PollInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			p.cfg.Logger.Info("attest-verifier poller stopping", "reason", ctx.Err())
			return nil
		case <-t.C:
			p.tick(ctx)
		}
	}
}

func (p *Poller) tick(ctx context.Context) {
	records, err := p.cfg.Lister.ListPending(ctx, p.cfg.BatchSize)
	if err != nil {
		p.cfg.Logger.Error("attest-verifier: list pending failed", "err", err)
		return
	}
	if len(records) == 0 {
		return
	}
	p.cfg.Logger.Info("attest-verifier: tick", "count", len(records))
	for _, r := range records {
		if ctx.Err() != nil {
			return
		}
		if err := p.VerifyOne(ctx, r); err != nil {
			p.cfg.Logger.Warn("attest-verifier: record verification failed",
				"record_id", r.RecordID, "err", err)
		}
	}
}

// VerifyOne runs a single end-to-end re-verification cycle for one record.
// It is exported so tests and diagnostics can drive it directly without
// starting the full poll loop.
//
// On any outcome (pass, fail, or error) a row is written to self_verify_log.
func (p *Poller) VerifyOne(ctx context.Context, rec *PendingRecord) error {
	if rec == nil {
		return fmt.Errorf("poller: nil record")
	}
	start := p.cfg.now()

	pdfBytes, err := p.cfg.Fetcher.Fetch(ctx, rec.PDFURL)
	if err != nil {
		return p.writeLog(ctx, rec.RecordID, start, StatusError, 0,
			"fetch: "+err.Error())
	}

	body, contentType, err := buildMultipart(rec.RecordID, pdfBytes)
	if err != nil {
		return p.writeLog(ctx, rec.RecordID, start, StatusError, 0,
			"multipart: "+err.Error())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.cfg.VerifyEndpoint, body)
	if err != nil {
		return p.writeLog(ctx, rec.RecordID, start, StatusError, 0,
			"newrequest: "+err.Error())
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := p.cfg.HTTPClient.Do(req)
	latencyMS := p.cfg.now().Sub(start).Milliseconds()
	if err != nil {
		return p.writeLog(ctx, rec.RecordID, start, StatusError, latencyMS,
			"http: "+err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return p.writeLog(ctx, rec.RecordID, start, StatusError, latencyMS,
			fmt.Sprintf("status %d: %s", resp.StatusCode, string(snippet)))
	}

	var vr VerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&vr); err != nil {
		return p.writeLog(ctx, rec.RecordID, start, StatusError, latencyMS,
			"decode: "+err.Error())
	}

	if !vr.Valid {
		return p.writeLog(ctx, rec.RecordID, start, StatusFail, latencyMS,
			"verify rejected: "+vr.Reason)
	}

	// Cross-check: if both sides recorded a hash, they must agree.
	// A mismatch means the archived PDF differs from what was signed.
	if rec.ContentHash != "" && vr.ContentSHA256 != "" &&
		vr.ContentSHA256 != rec.ContentHash {
		return p.writeLog(ctx, rec.RecordID, start, StatusFail, latencyMS,
			fmt.Sprintf("hash mismatch: expected %s got %s",
				rec.ContentHash, vr.ContentSHA256))
	}

	return p.writeLog(ctx, rec.RecordID, start, StatusPass, latencyMS, "")
}

func (p *Poller) writeLog(ctx context.Context, recordID string, at time.Time,
	status string, latencyMS int64, errMsg string) error {
	entry := &LogEntry{
		ID:         newLogID(),
		RecordID:   recordID,
		VerifiedAt: at,
		Status:     status,
		LatencyMS:  latencyMS,
		Err:        errMsg,
	}
	if werr := p.cfg.Writer.WriteLog(ctx, entry); werr != nil {
		p.cfg.Logger.Error("attest-verifier: write log failed",
			"record_id", recordID, "status", status, "err", werr)
	}
	if status == StatusPass {
		return nil
	}
	return fmt.Errorf("self-verify %s: %s", status, errMsg)
}

// buildMultipart constructs the multipart/form-data body for POST /verify.
// The "file" field name matches what apps/attest/internal/handler/verify expects.
func buildMultipart(recordID string, pdfBytes []byte) (*bytes.Buffer, string, error) {
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, err := mw.CreateFormFile("file", recordID+".pdf")
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

// logIDEncoding is unpadded lowercase base32.
var logIDEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// newLogID returns "svl_" + 24 lowercase base32 chars sourced from crypto/rand.
func newLogID() string {
	var b [15]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Errorf("poller: crypto/rand failed: %w", err))
	}
	raw := logIDEncoding.EncodeToString(b[:])
	out := make([]byte, len(raw))
	for i := range raw {
		c := raw[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return "svl_" + string(out)
}
