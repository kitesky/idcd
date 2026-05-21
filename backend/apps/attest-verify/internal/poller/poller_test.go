package poller_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kite365/idcd/apps/attest-verify/internal/poller"
)

// --- test stubs ---

type stubLister struct {
	records []*poller.PendingRecord
}

func (s *stubLister) ListPending(_ context.Context, _ int) ([]*poller.PendingRecord, error) {
	return s.records, nil
}

type stubFetcher struct {
	data []byte
}

func (s *stubFetcher) Fetch(_ context.Context, _ string) ([]byte, error) {
	return s.data, nil
}

type stubWriter struct {
	entries []*poller.LogEntry
}

func (s *stubWriter) WriteLog(_ context.Context, e *poller.LogEntry) error {
	s.entries = append(s.entries, e)
	return nil
}

func makePoller(t *testing.T, endpoint string) (*poller.Poller, *stubWriter) {
	t.Helper()
	w := &stubWriter{}
	p := poller.New(poller.Config{
		Lister:         &stubLister{},
		Writer:         w,
		Fetcher:        &stubFetcher{data: []byte("%PDF-1.4 fake content")},
		VerifyEndpoint: endpoint,
		PollInterval:   time.Minute,
	})
	return p, w
}

func makeRecord(id, hash string) *poller.PendingRecord {
	return &poller.PendingRecord{
		RecordID:    id,
		ReportID:    "vr_" + id,
		PDFURL:      "file:///dev/null",
		ContentHash: hash,
	}
}

func TestVerifyOne_Pass(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "expect POST", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, "bad multipart", http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(poller.VerifyResponse{
			Valid:         true,
			ContentSHA256: "abc123",
		})
	}))
	defer srv.Close()

	p, w := makePoller(t, srv.URL)
	rec := makeRecord("att_test01", "abc123")

	if err := p.VerifyOne(context.Background(), rec); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(w.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(w.entries))
	}
	got := w.entries[0]
	if got.Status != poller.StatusPass {
		t.Errorf("expected status=%q, got %q", poller.StatusPass, got.Status)
	}
	if got.RecordID != rec.RecordID {
		t.Errorf("expected record_id=%q, got %q", rec.RecordID, got.RecordID)
	}
	if !strings.HasPrefix(got.ID, "svl_") {
		t.Errorf("log ID should start with svl_, got %q", got.ID)
	}
}

func TestVerifyOne_Fail_InvalidSignature(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(poller.VerifyResponse{
			Valid:   false,
			Reason:  "signer key does not match idcd kms key",
		})
	}))
	defer srv.Close()

	p, w := makePoller(t, srv.URL)
	rec := makeRecord("att_test02", "")

	err := p.VerifyOne(context.Background(), rec)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(w.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(w.entries))
	}
	if w.entries[0].Status != poller.StatusFail {
		t.Errorf("expected status=%q, got %q", poller.StatusFail, w.entries[0].Status)
	}
}

func TestVerifyOne_Error_HTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	p, w := makePoller(t, srv.URL)
	rec := makeRecord("att_test03", "")

	err := p.VerifyOne(context.Background(), rec)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(w.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(w.entries))
	}
	if w.entries[0].Status != poller.StatusError {
		t.Errorf("expected status=%q, got %q", poller.StatusError, w.entries[0].Status)
	}
}

func TestVerifyOne_Fail_HashMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(poller.VerifyResponse{
			Valid:         true,
			ContentSHA256: "different_hash",
		})
	}))
	defer srv.Close()

	p, w := makePoller(t, srv.URL)
	rec := makeRecord("att_test04", "expected_hash")

	err := p.VerifyOne(context.Background(), rec)
	if err == nil {
		t.Fatal("expected error for hash mismatch, got nil")
	}
	if len(w.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(w.entries))
	}
	if w.entries[0].Status != poller.StatusFail {
		t.Errorf("expected status=%q, got %q", poller.StatusFail, w.entries[0].Status)
	}
	if !strings.Contains(w.entries[0].Err, "hash mismatch") {
		t.Errorf("expected hash mismatch in error, got: %q", w.entries[0].Err)
	}
}

func TestVerifyOne_Pass_NoHashCheck(t *testing.T) {
	// When ContentHash is empty, skip the hash cross-check.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(poller.VerifyResponse{
			Valid:         true,
			ContentSHA256: "some_hash",
		})
	}))
	defer srv.Close()

	p, w := makePoller(t, srv.URL)
	rec := makeRecord("att_test05", "") // empty ContentHash

	if err := p.VerifyOne(context.Background(), rec); err != nil {
		t.Fatalf("expected pass when ContentHash is empty, got: %v", err)
	}
	if w.entries[0].Status != poller.StatusPass {
		t.Errorf("expected pass, got %q", w.entries[0].Status)
	}
}

func TestVerifyOne_NilRecord(t *testing.T) {
	p, _ := makePoller(t, "http://localhost")
	err := p.VerifyOne(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil record")
	}
}

// errFetcher always returns an error — exercises the pre-/verify
// fetch-failure branch in VerifyOne.
type errFetcher struct{ err error }

func (e *errFetcher) Fetch(_ context.Context, _ string) ([]byte, error) {
	return nil, e.err
}

// errLister always returns an error — exercises the tick() degraded path
// where the DB is briefly unavailable.
type errLister struct{ err error }

func (e *errLister) ListPending(_ context.Context, _ int) ([]*poller.PendingRecord, error) {
	return nil, e.err
}

func TestVerifyOne_Error_FetchFail(t *testing.T) {
	w := &stubWriter{}
	p := poller.New(poller.Config{
		Lister:         &stubLister{},
		Writer:         w,
		Fetcher:        &errFetcher{err: errors.New("connection refused")},
		VerifyEndpoint: "http://unused",
		PollInterval:   time.Minute,
	})
	rec := makeRecord("att_fetchfail", "expected_hash")

	if err := p.VerifyOne(context.Background(), rec); err == nil {
		t.Fatal("expected error from fetch failure")
	}
	if len(w.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(w.entries))
	}
	if got := w.entries[0]; got.Status != poller.StatusError {
		t.Errorf("expected status=%q, got %q", poller.StatusError, got.Status)
	}
	if !strings.HasPrefix(w.entries[0].Err, "fetch:") {
		t.Errorf("expected err to start with 'fetch:', got %q", w.entries[0].Err)
	}
}

func TestVerifyOne_Error_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json at all"))
	}))
	defer srv.Close()

	p, w := makePoller(t, srv.URL)
	rec := makeRecord("att_malformed", "")

	if err := p.VerifyOne(context.Background(), rec); err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if len(w.entries) != 1 || w.entries[0].Status != poller.StatusError {
		t.Fatalf("expected one StatusError entry, got %+v", w.entries)
	}
	if !strings.Contains(w.entries[0].Err, "decode:") {
		t.Errorf("expected err to contain 'decode:', got %q", w.entries[0].Err)
	}
}

// TestVerifyOne_Fail_ServerOmitsHash documents the security-critical
// behaviour: a /verify response with valid=true but no content_sha256
// MUST NOT pass when the caller has a recorded hash to cross-check.
// Accepting it would defeat the D6 audit purpose — the server could be
// compromised or replaced by a stub.
func TestVerifyOne_Fail_ServerOmitsHash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(poller.VerifyResponse{
			Valid: true,
			// ContentSHA256 intentionally absent.
		})
	}))
	defer srv.Close()

	p, w := makePoller(t, srv.URL)
	rec := makeRecord("att_serveromit", "expected_hash")

	if err := p.VerifyOne(context.Background(), rec); err == nil {
		t.Fatal("expected error when server omits content_sha256 but record has hash")
	}
	if got := w.entries[0]; got.Status != poller.StatusFail {
		t.Errorf("expected status=%q, got %q", poller.StatusFail, got.Status)
	}
	if !strings.Contains(w.entries[0].Err, "omitted content_sha256") {
		t.Errorf("expected err to mention omitted hash, got %q", w.entries[0].Err)
	}
}

func TestTick_ListerError(t *testing.T) {
	// Drive one tick where the lister fails. The poller should log the
	// error and continue (no panic, no log entry written) so the next
	// tick can retry. This is the failure mode during a brief DB blip.
	w := &stubWriter{}
	p := poller.New(poller.Config{
		Lister:         &errLister{err: errors.New("db unreachable")},
		Writer:         w,
		Fetcher:        &stubFetcher{},
		VerifyEndpoint: "http://unused",
		PollInterval:   10 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := p.Run(ctx); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if len(w.entries) != 0 {
		t.Errorf("expected 0 log entries on lister failure, got %d", len(w.entries))
	}
}

func TestLogEntry_LatencyRecorded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(poller.VerifyResponse{Valid: true})
	}))
	defer srv.Close()

	p, w := makePoller(t, srv.URL)
	rec := makeRecord("att_test06", "")

	if err := p.VerifyOne(context.Background(), rec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.entries[0].LatencyMS < 0 {
		t.Errorf("expected non-negative latency, got %d", w.entries[0].LatencyMS)
	}
}
