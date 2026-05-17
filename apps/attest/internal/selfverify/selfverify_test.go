package selfverify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	attestrec "github.com/kite365/idcd/lib/attest/record"
)

// ---- fakes -----------------------------------------------------------

type fakeLister struct {
	mu      sync.Mutex
	batches [][]*PendingReport
	calls   int
	err     error
}

func (f *fakeLister) ListPendingSelfVerify(_ context.Context, _ int) ([]*PendingReport, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if len(f.batches) == 0 {
		return nil, nil
	}
	b := f.batches[0]
	f.batches = f.batches[1:]
	return b, nil
}

type updaterCall struct {
	ReportID string
	Status   string
	At       time.Time
}

type fakeUpdater struct {
	mu    sync.Mutex
	calls []updaterCall
	err   error
}

func (f *fakeUpdater) UpdateSelfVerify(_ context.Context, reportID, status string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, updaterCall{reportID, status, at})
	return f.err
}

func (f *fakeUpdater) Calls() []updaterCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]updaterCall, len(f.calls))
	copy(out, f.calls)
	return out
}

type fakeFetcher struct {
	bytesByURL map[string][]byte
	errByURL   map[string]error
}

func (f *fakeFetcher) Fetch(_ context.Context, url string) ([]byte, error) {
	if err, ok := f.errByURL[url]; ok {
		return nil, err
	}
	b, ok := f.bytesByURL[url]
	if !ok {
		return nil, fmt.Errorf("no bytes for %s", url)
	}
	return b, nil
}

type fakeRecords struct {
	mu      sync.Mutex
	rows    []*attestrec.Record
	insErr  error
	dupOnce bool
}

func (f *fakeRecords) Insert(_ context.Context, r *attestrec.Record) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.dupOnce {
		f.dupOnce = false
		return attestrec.ErrDuplicateAction
	}
	if f.insErr != nil {
		return f.insErr
	}
	f.rows = append(f.rows, r)
	return nil
}
func (f *fakeRecords) Get(context.Context, string, attestrec.Action) (*attestrec.Record, error) {
	return nil, attestrec.ErrNotFound
}
func (f *fakeRecords) Update(context.Context, *attestrec.Record) error { return nil }
func (f *fakeRecords) ListByReport(context.Context, string) ([]*attestrec.Record, error) {
	return nil, nil
}
func (f *fakeRecords) Rows() []*attestrec.Record {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*attestrec.Record, len(f.rows))
	copy(out, f.rows)
	return out
}

// ---- helpers ---------------------------------------------------------

func sha256hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type verifyResponse struct {
	Valid         bool   `json:"valid"`
	Reason        string `json:"reason,omitempty"`
	ContentSHA256 string `json:"content_sha256,omitempty"`
}

// newVerifyServer wraps httptest.Server with a handler that asserts the
// incoming multipart shape and replies with `resp` (or `status`).
func newVerifyServer(t *testing.T, status int, resp *verifyResponse) (*httptest.Server, *int) {
	t.Helper()
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != http.MethodPost {
			t.Errorf("verify server: method=%s want POST", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "multipart/form-data") {
			t.Errorf("verify server: content-type=%q want multipart/form-data", ct)
		}
		// Parse the multipart so we know the worker actually sent the
		// file field — protects against silent regressions in
		// buildMultipart.
		if err := r.ParseMultipartForm(4 << 20); err != nil {
			t.Errorf("verify server: parse multipart: %v", err)
		}
		if r.MultipartForm == nil || len(r.MultipartForm.File["file"]) != 1 {
			t.Errorf("verify server: missing file field")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if resp != nil {
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func newWorker(t *testing.T, cfg Config) *Worker {
	t.Helper()
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	}
	// Pin the clock for stable assertions.
	fixed := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	cfg.now = func() time.Time { return fixed }
	return New(cfg)
}

// ---- tests -----------------------------------------------------------

func TestVerifyOne_HappyPath(t *testing.T) {
	pdf := []byte("%PDF-fake")
	hash := sha256hex(pdf)
	srv, hits := newVerifyServer(t, 200, &verifyResponse{
		Valid:         true,
		ContentSHA256: hash,
	})

	upd := &fakeUpdater{}
	recs := &fakeRecords{}
	w := newWorker(t, Config{
		Lister:             &fakeLister{},
		Updater:            upd,
		AttestationRecords: recs,
		Fetcher:            &fakeFetcher{bytesByURL: map[string][]byte{"file:///x.pdf": pdf}},
		VerifyEndpoint:     srv.URL,
	})

	err := w.VerifyOne(context.Background(), &PendingReport{
		ID: "vr_abc", PDFURL: "file:///x.pdf", ContentHash: hash,
	})
	if err != nil {
		t.Fatalf("VerifyOne: %v", err)
	}
	if *hits != 1 {
		t.Fatalf("verify hits=%d want 1", *hits)
	}
	calls := upd.Calls()
	if len(calls) != 1 || calls[0].Status != StatusPass || calls[0].ReportID != "vr_abc" {
		t.Fatalf("updater calls=%+v want one pass for vr_abc", calls)
	}
	rows := recs.Rows()
	if len(rows) != 1 ||
		rows[0].Action != attestrec.ActionSelfVerified ||
		rows[0].Status != attestrec.StatusSuccess ||
		rows[0].Result != attestrec.ResultSuccess {
		t.Fatalf("WAL rows=%+v want one success row", rows)
	}
}

func TestVerifyOne_VerifyReturnsInvalid(t *testing.T) {
	pdf := []byte("%PDF-fake")
	srv, _ := newVerifyServer(t, 200, &verifyResponse{Valid: false, Reason: "bad sig"})

	upd := &fakeUpdater{}
	recs := &fakeRecords{}
	w := newWorker(t, Config{
		Lister:             &fakeLister{},
		Updater:            upd,
		AttestationRecords: recs,
		Fetcher:            &fakeFetcher{bytesByURL: map[string][]byte{"f": pdf}},
		VerifyEndpoint:     srv.URL,
	})

	err := w.VerifyOne(context.Background(), &PendingReport{ID: "vr_1", PDFURL: "f"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "bad sig") {
		t.Fatalf("error=%v want to contain 'bad sig'", err)
	}
	calls := upd.Calls()
	if len(calls) != 1 || calls[0].Status != StatusFail {
		t.Fatalf("updater calls=%+v want one fail", calls)
	}
	rows := recs.Rows()
	if len(rows) != 1 || rows[0].Status != attestrec.StatusFailure ||
		!strings.Contains(rows[0].ErrorDetail, "bad sig") {
		t.Fatalf("WAL rows=%+v want one failure with reason", rows)
	}
}

func TestVerifyOne_HTTP5xx(t *testing.T) {
	srv, _ := newVerifyServer(t, 503, nil)

	upd := &fakeUpdater{}
	recs := &fakeRecords{}
	w := newWorker(t, Config{
		Lister:             &fakeLister{},
		Updater:            upd,
		AttestationRecords: recs,
		Fetcher:            &fakeFetcher{bytesByURL: map[string][]byte{"f": []byte("x")}},
		VerifyEndpoint:     srv.URL,
	})

	err := w.VerifyOne(context.Background(), &PendingReport{ID: "vr_2", PDFURL: "f"})
	if err == nil || !strings.Contains(err.Error(), "status 503") {
		t.Fatalf("err=%v want status 503", err)
	}
	if upd.Calls()[0].Status != StatusFail {
		t.Fatalf("expected fail status flip")
	}
}

func TestVerifyOne_FetcherFails(t *testing.T) {
	srv, hits := newVerifyServer(t, 200, &verifyResponse{Valid: true})

	upd := &fakeUpdater{}
	recs := &fakeRecords{}
	w := newWorker(t, Config{
		Lister:             &fakeLister{},
		Updater:            upd,
		AttestationRecords: recs,
		Fetcher: &fakeFetcher{
			errByURL: map[string]error{"f": errors.New("disk gone")},
		},
		VerifyEndpoint: srv.URL,
	})

	err := w.VerifyOne(context.Background(), &PendingReport{ID: "vr_3", PDFURL: "f"})
	if err == nil || !strings.Contains(err.Error(), "disk gone") {
		t.Fatalf("err=%v want disk gone", err)
	}
	if *hits != 0 {
		t.Fatalf("verify server should not be called when fetch fails; hits=%d", *hits)
	}
	if upd.Calls()[0].Status != StatusFail {
		t.Fatalf("expected fail status flip")
	}
}

func TestVerifyOne_HashMismatch(t *testing.T) {
	pdf := []byte("%PDF-fake")
	srv, _ := newVerifyServer(t, 200, &verifyResponse{
		Valid:         true,
		ContentSHA256: "deadbeef",
	})

	upd := &fakeUpdater{}
	recs := &fakeRecords{}
	w := newWorker(t, Config{
		Lister:             &fakeLister{},
		Updater:            upd,
		AttestationRecords: recs,
		Fetcher:            &fakeFetcher{bytesByURL: map[string][]byte{"f": pdf}},
		VerifyEndpoint:     srv.URL,
	})

	err := w.VerifyOne(context.Background(), &PendingReport{
		ID: "vr_h", PDFURL: "f", ContentHash: sha256hex(pdf),
	})
	if err == nil || !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("err=%v want hash mismatch", err)
	}
	if upd.Calls()[0].Status != StatusFail {
		t.Fatalf("expected fail status flip")
	}
}

func TestVerifyOne_NilReport(t *testing.T) {
	srv, _ := newVerifyServer(t, 200, &verifyResponse{Valid: true})
	w := newWorker(t, Config{
		Lister: &fakeLister{}, Updater: &fakeUpdater{},
		AttestationRecords: &fakeRecords{},
		Fetcher:            &fakeFetcher{},
		VerifyEndpoint:     srv.URL,
	})
	if err := w.VerifyOne(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil report")
	}
}

func TestVerifyOne_DuplicateWALIsSwallowed(t *testing.T) {
	pdf := []byte("%PDF-fake")
	srv, _ := newVerifyServer(t, 200, &verifyResponse{Valid: true})

	upd := &fakeUpdater{}
	recs := &fakeRecords{dupOnce: true}
	w := newWorker(t, Config{
		Lister:             &fakeLister{},
		Updater:            upd,
		AttestationRecords: recs,
		Fetcher:            &fakeFetcher{bytesByURL: map[string][]byte{"f": pdf}},
		VerifyEndpoint:     srv.URL,
	})

	// Even though Insert returns ErrDuplicateAction, success path still
	// flips status to pass.
	if err := w.VerifyOne(context.Background(), &PendingReport{ID: "vr_d", PDFURL: "f"}); err != nil {
		t.Fatalf("VerifyOne: %v", err)
	}
	if upd.Calls()[0].Status != StatusPass {
		t.Fatalf("expected pass even on duplicate WAL row")
	}
}

func TestRun_ProcessesBatchThenExits(t *testing.T) {
	pdf := []byte("%PDF-fake")
	srv, hits := newVerifyServer(t, 200, &verifyResponse{Valid: true})

	lister := &fakeLister{
		batches: [][]*PendingReport{{
			{ID: "vr_a", PDFURL: "f"},
			{ID: "vr_b", PDFURL: "f"},
			{ID: "vr_c", PDFURL: "f"},
		}},
	}
	upd := &fakeUpdater{}
	recs := &fakeRecords{}

	w := newWorker(t, Config{
		Lister:             lister,
		Updater:            upd,
		AttestationRecords: recs,
		Fetcher:            &fakeFetcher{bytesByURL: map[string][]byte{"f": pdf}},
		VerifyEndpoint:     srv.URL,
		PollInterval:       time.Hour, // only the immediate tick should fire
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	// Give the immediate tick room to drain the batch.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(upd.Calls()) == 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned err=%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after ctx cancel")
	}

	if *hits != 3 {
		t.Fatalf("verify hits=%d want 3", *hits)
	}
	if got := len(upd.Calls()); got != 3 {
		t.Fatalf("updater calls=%d want 3", got)
	}
	for _, c := range upd.Calls() {
		if c.Status != StatusPass {
			t.Fatalf("call=%+v want pass", c)
		}
	}
}

func TestRun_ListerErrorDoesNotKillLoop(t *testing.T) {
	srv, _ := newVerifyServer(t, 200, &verifyResponse{Valid: true})

	lister := &fakeLister{err: errors.New("boom")}
	w := newWorker(t, Config{
		Lister:             lister,
		Updater:            &fakeUpdater{},
		AttestationRecords: &fakeRecords{},
		Fetcher:            &fakeFetcher{},
		VerifyEndpoint:     srv.URL,
		PollInterval:       time.Hour,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run err=%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit")
	}
}

func TestRun_AlreadyCancelledContext(t *testing.T) {
	srv, _ := newVerifyServer(t, 200, &verifyResponse{Valid: true})
	w := newWorker(t, Config{
		Lister:             &fakeLister{},
		Updater:            &fakeUpdater{},
		AttestationRecords: &fakeRecords{},
		Fetcher:            &fakeFetcher{},
		VerifyEndpoint:     srv.URL,
		PollInterval:       time.Hour,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := w.Run(ctx); err != nil {
		t.Fatalf("Run err=%v", err)
	}
}

func TestNew_PanicsOnMissingDeps(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Config)
	}{
		{"no lister", func(c *Config) { c.Lister = nil }},
		{"no updater", func(c *Config) { c.Updater = nil }},
		{"no records", func(c *Config) { c.AttestationRecords = nil }},
		{"no fetcher", func(c *Config) { c.Fetcher = nil }},
		{"no endpoint", func(c *Config) { c.VerifyEndpoint = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{
				Lister:             &fakeLister{},
				Updater:            &fakeUpdater{},
				AttestationRecords: &fakeRecords{},
				Fetcher:            &fakeFetcher{},
				VerifyEndpoint:     "https://example.test/verify",
			}
			tc.mut(&cfg)
			defer func() {
				if r := recover(); r == nil {
					t.Fatal("expected panic")
				}
			}()
			_ = New(cfg)
		})
	}
}

func TestNew_AppliesDefaults(t *testing.T) {
	w := New(Config{
		Lister:             &fakeLister{},
		Updater:            &fakeUpdater{},
		AttestationRecords: &fakeRecords{},
		Fetcher:            &fakeFetcher{},
		VerifyEndpoint:     "https://example.test/verify",
	})
	if w.cfg.PollInterval != defaultPollInterval {
		t.Errorf("PollInterval=%v want %v", w.cfg.PollInterval, defaultPollInterval)
	}
	if w.cfg.BatchSize != defaultBatchSize {
		t.Errorf("BatchSize=%d want %d", w.cfg.BatchSize, defaultBatchSize)
	}
	if w.cfg.HTTPClient == nil {
		t.Error("HTTPClient default not set")
	}
	if w.cfg.Logger == nil {
		t.Error("Logger default not set")
	}
	if w.cfg.now == nil {
		t.Error("now default not set")
	}
}

func TestBuildMultipart_RoundTrip(t *testing.T) {
	body, ct, err := buildMultipart("vr_x", []byte("hello"))
	if err != nil {
		t.Fatalf("buildMultipart: %v", err)
	}
	if !strings.HasPrefix(ct, "multipart/form-data") {
		t.Fatalf("content-type=%q", ct)
	}
	_, params, err := mimeBoundary(ct)
	if err != nil {
		t.Fatalf("parse content-type: %v", err)
	}
	mr := multipart.NewReader(body, params["boundary"])
	part, err := mr.NextPart()
	if err != nil {
		t.Fatalf("NextPart: %v", err)
	}
	if part.FormName() != "file" {
		t.Errorf("form name=%q want file", part.FormName())
	}
	if part.FileName() != "vr_x.pdf" {
		t.Errorf("filename=%q want vr_x.pdf", part.FileName())
	}
	got, _ := io.ReadAll(part)
	if string(got) != "hello" {
		t.Errorf("body=%q want hello", got)
	}
}

// fakeRefundEnqueuer records EnqueueRefund calls so tests can assert
// whether refund hand-off fires on the recordFailure path. The errOnce
// field lets one call return an error so the "best-effort" guarantee is
// covered (a failing enqueue must NOT regress the recordFailure return).
type fakeRefundEnqueuer struct {
	calls   []refundCall
	errOnce error
}

type refundCall struct {
	ReportID string
	Reason   string
}

func (f *fakeRefundEnqueuer) EnqueueRefund(_ context.Context, reportID, reason string) error {
	f.calls = append(f.calls, refundCall{ReportID: reportID, Reason: reason})
	if f.errOnce != nil {
		err := f.errOnce
		f.errOnce = nil
		return err
	}
	return nil
}

func TestRecordFailure_EnqueuesRefundWhenWired(t *testing.T) {
	pdf := []byte("%PDF-fake")
	srv, _ := newVerifyServer(t, 200, &verifyResponse{Valid: false, Reason: "bad sig"})

	enq := &fakeRefundEnqueuer{}
	w := newWorker(t, Config{
		Lister:             &fakeLister{},
		Updater:            &fakeUpdater{},
		AttestationRecords: &fakeRecords{},
		Fetcher:            &fakeFetcher{bytesByURL: map[string][]byte{"f": pdf}},
		VerifyEndpoint:     srv.URL,
		RefundEnqueuer:     enq,
	})

	_ = w.VerifyOne(context.Background(), &PendingReport{ID: "vr_99", PDFURL: "f"})

	if len(enq.calls) != 1 {
		t.Fatalf("enqueue calls=%d want 1", len(enq.calls))
	}
	if enq.calls[0].ReportID != "vr_99" {
		t.Fatalf("report_id=%q want vr_99", enq.calls[0].ReportID)
	}
	if !strings.Contains(enq.calls[0].Reason, "bad sig") {
		t.Fatalf("reason=%q want to contain 'bad sig'", enq.calls[0].Reason)
	}
}

func TestRecordFailure_NilEnqueuerIsNoOp(t *testing.T) {
	pdf := []byte("%PDF-fake")
	srv, _ := newVerifyServer(t, 200, &verifyResponse{Valid: false, Reason: "bad sig"})

	w := newWorker(t, Config{
		Lister:             &fakeLister{},
		Updater:            &fakeUpdater{},
		AttestationRecords: &fakeRecords{},
		Fetcher:            &fakeFetcher{bytesByURL: map[string][]byte{"f": pdf}},
		VerifyEndpoint:     srv.URL,
		// RefundEnqueuer intentionally nil
	})

	if err := w.VerifyOne(context.Background(), &PendingReport{ID: "vr_x", PDFURL: "f"}); err == nil {
		t.Fatalf("expected error from invalid signature")
	}
}

func TestRecordFailure_EnqueueErrorDoesNotMask(t *testing.T) {
	pdf := []byte("%PDF-fake")
	srv, _ := newVerifyServer(t, 200, &verifyResponse{Valid: false, Reason: "bad sig"})

	enq := &fakeRefundEnqueuer{errOnce: fmt.Errorf("redis down")}
	w := newWorker(t, Config{
		Lister:             &fakeLister{},
		Updater:            &fakeUpdater{},
		AttestationRecords: &fakeRecords{},
		Fetcher:            &fakeFetcher{bytesByURL: map[string][]byte{"f": pdf}},
		VerifyEndpoint:     srv.URL,
		RefundEnqueuer:     enq,
	})

	err := w.VerifyOne(context.Background(), &PendingReport{ID: "vr_y", PDFURL: "f"})
	if err == nil || !strings.Contains(err.Error(), "bad sig") {
		t.Fatalf("expected original 'bad sig' error to survive enqueue failure, got %v", err)
	}
	if len(enq.calls) != 1 {
		t.Fatalf("enqueue attempted=%d want 1", len(enq.calls))
	}
}

// TestNew_DefaultNowFunc covers the `cfg.now == nil` defaulting branch
// in New(). The shared newWorker helper always pins now, so this test
// constructs the Worker directly.
func TestNew_DefaultNowFunc(t *testing.T) {
	w := New(Config{
		Lister:             &fakeLister{},
		Updater:            &fakeUpdater{},
		AttestationRecords: &fakeRecords{},
		Fetcher:            &fakeFetcher{},
		VerifyEndpoint:     "https://example.test/verify",
	})
	got := w.cfg.now()
	if got.IsZero() {
		t.Fatal("default now() should return non-zero time")
	}
	if got.Location() != time.UTC {
		t.Errorf("default now() should be UTC, got %v", got.Location())
	}
}

// TestRun_TickerBranchFires covers the `<-t.C` case in Run's select.
// We use a very short PollInterval so the ticker fires before context
// cancellation.
func TestRun_TickerBranchFires(t *testing.T) {
	pdf := []byte("%PDF-fake")
	srv, _ := newVerifyServer(t, 200, &verifyResponse{Valid: true})

	lister := &fakeLister{
		// Two batches: the immediate tick consumes the first, the
		// ticker-driven tick consumes the second.
		batches: [][]*PendingReport{
			{{ID: "vr_1", PDFURL: "f"}},
			{{ID: "vr_2", PDFURL: "f"}},
		},
	}
	upd := &fakeUpdater{}
	w := newWorker(t, Config{
		Lister:             lister,
		Updater:            upd,
		AttestationRecords: &fakeRecords{},
		Fetcher:            &fakeFetcher{bytesByURL: map[string][]byte{"f": pdf}},
		VerifyEndpoint:     srv.URL,
		PollInterval:       10 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	// Wait until both batches have been processed.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(upd.Calls()) >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done

	if got := len(upd.Calls()); got < 2 {
		t.Fatalf("expected ticker-driven tick to consume second batch; calls=%d", got)
	}
}

// TestTick_CtxCancelMidBatch covers the `ctx.Err() != nil` check inside
// the for-loop in tick(). We feed a batch of 3 reports; the first
// VerifyOne call also cancels the ctx, so the loop should bail before
// reaching report #2.
type cancellingLister struct {
	once    bool
	reports []*PendingReport
}

func (c *cancellingLister) ListPendingSelfVerify(_ context.Context, _ int) ([]*PendingReport, error) {
	if c.once {
		return nil, nil
	}
	c.once = true
	return c.reports, nil
}

type cancellingUpdater struct {
	cancel func()
	calls  []updaterCall
}

func (c *cancellingUpdater) UpdateSelfVerify(_ context.Context, id, st string, at time.Time) error {
	c.calls = append(c.calls, updaterCall{id, st, at})
	// Cancel after the first call so the loop's per-iteration ctx.Err()
	// check trips before the next report is processed.
	c.cancel()
	return nil
}

func TestTick_CtxCancelMidBatch(t *testing.T) {
	pdf := []byte("%PDF-fake")
	srv, _ := newVerifyServer(t, 200, &verifyResponse{Valid: true})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	upd := &cancellingUpdater{cancel: cancel}

	w := newWorker(t, Config{
		Lister: &cancellingLister{reports: []*PendingReport{
			{ID: "vr_1", PDFURL: "f"},
			{ID: "vr_2", PDFURL: "f"},
			{ID: "vr_3", PDFURL: "f"},
		}},
		Updater:            upd,
		AttestationRecords: &fakeRecords{},
		Fetcher:            &fakeFetcher{bytesByURL: map[string][]byte{"f": pdf}},
		VerifyEndpoint:     srv.URL,
		PollInterval:       time.Hour,
	})

	w.tick(ctx)
	// Exactly one report should have been processed before ctx-cancel
	// short-circuited the loop.
	if len(upd.calls) != 1 {
		t.Fatalf("expected 1 update before cancel, got %d", len(upd.calls))
	}
}

// TestVerifyOne_HTTPDoError covers the http.Client.Do error branch when
// the endpoint is unreachable.
func TestVerifyOne_HTTPDoError(t *testing.T) {
	pdf := []byte("%PDF-fake")
	upd := &fakeUpdater{}
	w := newWorker(t, Config{
		Lister:             &fakeLister{},
		Updater:            upd,
		AttestationRecords: &fakeRecords{},
		Fetcher:            &fakeFetcher{bytesByURL: map[string][]byte{"f": pdf}},
		// 127.0.0.1:1 is reserved + unbound — Do() will fail with a
		// connection-refused error.
		VerifyEndpoint: "http://127.0.0.1:1/verify",
		HTTPClient:     &http.Client{Timeout: 200 * time.Millisecond},
	})

	err := w.VerifyOne(context.Background(), &PendingReport{ID: "vr_do", PDFURL: "f"})
	if err == nil || !strings.Contains(err.Error(), "self-verify failed") {
		t.Fatalf("err=%v want self-verify failed", err)
	}
	if upd.Calls()[0].Status != StatusFail {
		t.Fatal("expected fail status flip")
	}
}

// TestVerifyOne_JSONDecodeError covers the json.NewDecoder.Decode error
// branch (server returns 200 + non-JSON body).
func TestVerifyOne_JSONDecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("this is not json {{"))
	}))
	t.Cleanup(srv.Close)

	upd := &fakeUpdater{}
	w := newWorker(t, Config{
		Lister:             &fakeLister{},
		Updater:            upd,
		AttestationRecords: &fakeRecords{},
		Fetcher:            &fakeFetcher{bytesByURL: map[string][]byte{"f": []byte("x")}},
		VerifyEndpoint:     srv.URL,
	})

	err := w.VerifyOne(context.Background(), &PendingReport{ID: "vr_j", PDFURL: "f"})
	if err == nil {
		t.Fatal("expected decode error")
	}
	if upd.Calls()[0].Status != StatusFail {
		t.Fatal("expected fail status flip")
	}
}

// TestVerifyOne_NewRequestError covers the http.NewRequestWithContext
// error branch. We trigger it with a URL containing a NUL byte, which
// net/http rejects.
func TestVerifyOne_NewRequestError(t *testing.T) {
	upd := &fakeUpdater{}
	w := newWorker(t, Config{
		Lister:             &fakeLister{},
		Updater:            upd,
		AttestationRecords: &fakeRecords{},
		Fetcher:            &fakeFetcher{bytesByURL: map[string][]byte{"f": []byte("x")}},
		VerifyEndpoint:     "http://example.test/\x00bad",
	})

	err := w.VerifyOne(context.Background(), &PendingReport{ID: "vr_n", PDFURL: "f"})
	if err == nil || !strings.Contains(err.Error(), "self-verify failed") {
		t.Fatalf("err=%v want self-verify failed", err)
	}
	if upd.Calls()[0].Status != StatusFail {
		t.Fatal("expected fail status flip")
	}
}

// erroringRecords always returns a non-duplicate Insert error so the
// "Insert WAL failed (logged + continue)" branch in both recordSuccess
// and recordFailure runs.
type erroringRecords struct{}

func (erroringRecords) Insert(context.Context, *attestrec.Record) error {
	return fmt.Errorf("disk full")
}
func (erroringRecords) Get(context.Context, string, attestrec.Action) (*attestrec.Record, error) {
	return nil, attestrec.ErrNotFound
}
func (erroringRecords) Update(context.Context, *attestrec.Record) error { return nil }
func (erroringRecords) ListByReport(context.Context, string) ([]*attestrec.Record, error) {
	return nil, nil
}

func TestRecordSuccess_WALInsertErrorIsLogged(t *testing.T) {
	pdf := []byte("%PDF-fake")
	srv, _ := newVerifyServer(t, 200, &verifyResponse{Valid: true})

	upd := &fakeUpdater{}
	w := newWorker(t, Config{
		Lister:             &fakeLister{},
		Updater:            upd,
		AttestationRecords: erroringRecords{},
		Fetcher:            &fakeFetcher{bytesByURL: map[string][]byte{"f": pdf}},
		VerifyEndpoint:     srv.URL,
	})

	// Insert errors but recordSuccess proceeds to flip status — return is nil.
	if err := w.VerifyOne(context.Background(), &PendingReport{ID: "vr_w1", PDFURL: "f"}); err != nil {
		t.Fatalf("VerifyOne: %v", err)
	}
	if upd.Calls()[0].Status != StatusPass {
		t.Fatal("expected pass status flip despite WAL insert error")
	}
}

func TestRecordFailure_WALInsertErrorIsLogged(t *testing.T) {
	pdf := []byte("%PDF-fake")
	srv, _ := newVerifyServer(t, 200, &verifyResponse{Valid: false, Reason: "boom"})

	upd := &fakeUpdater{}
	w := newWorker(t, Config{
		Lister:             &fakeLister{},
		Updater:            upd,
		AttestationRecords: erroringRecords{},
		Fetcher:            &fakeFetcher{bytesByURL: map[string][]byte{"f": pdf}},
		VerifyEndpoint:     srv.URL,
	})

	err := w.VerifyOne(context.Background(), &PendingReport{ID: "vr_w2", PDFURL: "f"})
	if err == nil {
		t.Fatal("expected failure error")
	}
	if upd.Calls()[0].Status != StatusFail {
		t.Fatal("expected fail status flip despite WAL insert error")
	}
}

// erroringUpdater returns an error from UpdateSelfVerify so both
// recordSuccess and recordFailure exercise their updater-failure branch.
type erroringUpdater struct {
	mu    sync.Mutex
	calls []updaterCall
}

func (e *erroringUpdater) UpdateSelfVerify(_ context.Context, id, st string, at time.Time) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls = append(e.calls, updaterCall{id, st, at})
	return fmt.Errorf("db down")
}

func TestRecordSuccess_UpdaterErrorPropagates(t *testing.T) {
	pdf := []byte("%PDF-fake")
	srv, _ := newVerifyServer(t, 200, &verifyResponse{Valid: true})

	upd := &erroringUpdater{}
	w := newWorker(t, Config{
		Lister:             &fakeLister{},
		Updater:            upd,
		AttestationRecords: &fakeRecords{},
		Fetcher:            &fakeFetcher{bytesByURL: map[string][]byte{"f": pdf}},
		VerifyEndpoint:     srv.URL,
	})

	err := w.VerifyOne(context.Background(), &PendingReport{ID: "vr_u1", PDFURL: "f"})
	if err == nil || !strings.Contains(err.Error(), "update self_verify_status") {
		t.Fatalf("err=%v want update error", err)
	}
}

func TestRecordFailure_UpdaterErrorIsLoggedNotMasked(t *testing.T) {
	pdf := []byte("%PDF-fake")
	srv, _ := newVerifyServer(t, 200, &verifyResponse{Valid: false, Reason: "rejected"})

	upd := &erroringUpdater{}
	w := newWorker(t, Config{
		Lister:             &fakeLister{},
		Updater:            upd,
		AttestationRecords: &fakeRecords{},
		Fetcher:            &fakeFetcher{bytesByURL: map[string][]byte{"f": pdf}},
		VerifyEndpoint:     srv.URL,
	})

	err := w.VerifyOne(context.Background(), &PendingReport{ID: "vr_u2", PDFURL: "f"})
	// The original failure reason must surface, not the updater error.
	if err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("err=%v want original 'rejected' reason", err)
	}
}

// mimeBoundary parses a Content-Type header without importing mime in
// the production file; mirrors mime.ParseMediaType.
func mimeBoundary(ct string) (string, map[string]string, error) {
	// Cheap parser sufficient for "multipart/form-data; boundary=xxx".
	parts := strings.SplitN(ct, ";", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("no boundary in %q", ct)
	}
	kv := strings.SplitN(strings.TrimSpace(parts[1]), "=", 2)
	if len(kv) != 2 || kv[0] != "boundary" {
		return "", nil, fmt.Errorf("bad boundary param in %q", ct)
	}
	return strings.TrimSpace(parts[0]), map[string]string{"boundary": kv[1]}, nil
}
