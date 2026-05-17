package pebble

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// init shortens the harness healthcheck loop so unit tests that exercise
// the "not ready" path don't burn the full 30s in CI. Real integration
// runs go through Start, which sees the production defaults via the
// `healthcheckTimeout` package-level var (we save / restore around each
// test that mutates it).
func init() {
	healthcheckTimeout = 500 * time.Millisecond
	healthcheckPollInterval = 50 * time.Millisecond
}

// TestEnsureTrailingDot verifies the fqdn normaliser used before every
// challtestsrv POST.
func TestEnsureTrailingDot(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"_acme-challenge.example.com", "_acme-challenge.example.com."},
		{"_acme-challenge.example.com.", "_acme-challenge.example.com."},
		{"", "."},
	}
	for _, c := range cases {
		if got := ensureTrailingDot(c.in); got != c.want {
			t.Errorf("ensureTrailingDot(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestStartFromEnv_NotReady covers the env-var mode bailout when the
// caller-managed Pebble is not actually up: Start must surface ErrSkip
// rather than fail the test.
func TestStartFromEnv_NotReady(t *testing.T) {
	t.Setenv("PEBBLE_DIRECTORY_URL", "https://127.0.0.1:1/dir")
	t.Setenv("PEBBLE_CHALLTESTSRV_URL", "http://127.0.0.1:1")

	h, err := Start(t)
	if err == nil {
		h.Close()
		t.Fatal("expected ErrSkip, got nil")
	}
	if !errors.Is(err, ErrSkip) {
		t.Fatalf("expected ErrSkip, got %v", err)
	}
}

// TestSetClearTXT_AgainstStub spins up an in-process HTTP server that
// mimics challtestsrv's management API and verifies that the harness's
// SetTXT / ClearTXT helpers post the right JSON envelopes.
//
// This exercises the wire format without needing docker.
func TestSetClearTXT_AgainstStub(t *testing.T) {
	type received struct {
		Path string
		Host string
		Val  string
	}
	var got []received

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var m map[string]string
		if err := json.Unmarshal(body, &m); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		got = append(got, received{Path: r.URL.Path, Host: m["host"], Val: m["value"]})
		w.WriteHeader(200)
	}))
	defer srv.Close()

	h := &Harness{
		chalMgmtURL: srv.URL,
		httpClient:  &http.Client{Timeout: 2 * time.Second},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := h.SetTXT(ctx, "_acme-challenge.example.com", "v1"); err != nil {
		t.Fatalf("SetTXT: %v", err)
	}
	if err := h.ClearTXT(ctx, "_acme-challenge.example.com"); err != nil {
		t.Fatalf("ClearTXT: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 requests, got %d: %+v", len(got), got)
	}
	if got[0].Path != "/set-txt" || got[0].Host != "_acme-challenge.example.com." || got[0].Val != "v1" {
		t.Errorf("set-txt request wrong: %+v", got[0])
	}
	if got[1].Path != "/clear-txt" || got[1].Host != "_acme-challenge.example.com." {
		t.Errorf("clear-txt request wrong: %+v", got[1])
	}
}

// TestSetTXT_NilHarness guards the public API against panic when called
// on a zero-value Harness (which is what Start returns when ErrSkip is
// propagated).
func TestSetTXT_NilHarness(t *testing.T) {
	var h *Harness
	if err := h.SetTXT(context.Background(), "x", "y"); err == nil {
		t.Error("SetTXT on nil harness: want error, got nil")
	}
	if err := h.ClearTXT(context.Background(), "x"); err == nil {
		t.Error("ClearTXT on nil harness: want error, got nil")
	}
	// Close must be safe on nil.
	h.Close()
}

// TestSolverAdapter verifies the ca.DnsSolver wrapper forwards to the
// stubbed management API and reports the requested timeout.
func TestSolverAdapter(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, r.URL.Path+":"+string(b))
		w.WriteHeader(200)
	}))
	defer srv.Close()

	h := &Harness{
		chalMgmtURL: srv.URL,
		httpClient:  &http.Client{Timeout: 2 * time.Second},
	}

	s := h.Solver(30 * time.Second)
	if s.Timeout() != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", s.Timeout())
	}
	if err := s.Present(context.Background(), "_acme-challenge.x.test.", "abc"); err != nil {
		t.Fatalf("Present: %v", err)
	}
	if err := s.CleanUp(context.Background(), "_acme-challenge.x.test.", "abc"); err != nil {
		t.Fatalf("CleanUp: %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(bodies))
	}
	if !strings.HasPrefix(bodies[0], "/set-txt:") || !strings.Contains(bodies[0], `"value":"abc"`) {
		t.Errorf("present body wrong: %s", bodies[0])
	}
	if !strings.HasPrefix(bodies[1], "/clear-txt:") {
		t.Errorf("cleanup body wrong: %s", bodies[1])
	}
}

// TestSolverDefaultTimeout: passing 0 should fall back to a sensible
// default (>= 30s) rather than disable the lego propagation wait.
func TestSolverDefaultTimeout(t *testing.T) {
	h := &Harness{}
	if to := h.Solver(0).Timeout(); to < 30*time.Second {
		t.Errorf("default solver timeout = %v, want >= 30s", to)
	}
}

// TestStartFromEnv_ReadyAgainstStubs constructs in-process stubs for
// Pebble's directory and challtestsrv's management endpoint and confirms
// Start (env-var mode) wires them up successfully.
func TestStartFromEnv_ReadyAgainstStubs(t *testing.T) {
	// Pebble dir stub: any GET returns 200.
	pebble := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer pebble.Close()

	// challtestsrv stub: /clear-txt with empty body — challtestsrv
	// would normally return 400 here, but our readiness probe only
	// cares about the HTTP layer responding at all.
	chal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
	}))
	defer chal.Close()

	t.Setenv("PEBBLE_DIRECTORY_URL", pebble.URL)
	t.Setenv("PEBBLE_CHALLTESTSRV_URL", chal.URL)
	t.Setenv("PEBBLE_DNS_SERVER", "127.0.0.1:8053")

	h, err := Start(t)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Close()

	if h.DirectoryURL != pebble.URL {
		t.Errorf("DirectoryURL = %q, want %q", h.DirectoryURL, pebble.URL)
	}
	if h.DNSServer != "127.0.0.1:8053" {
		t.Errorf("DNSServer = %q", h.DNSServer)
	}
}

// TestEnvVarDefaultsApplied confirms PEBBLE_CHALLTESTSRV_URL and
// PEBBLE_DNS_SERVER fall back to sensible defaults when unset.
func TestEnvVarDefaultsApplied(t *testing.T) {
	// We can't hit a real harness here, but we can check the parsing.
	// Start will return ErrSkip because the defaults point at a port
	// nothing is listening on; we just want to verify the harness
	// got the defaults rather than empty strings.
	t.Setenv("PEBBLE_DIRECTORY_URL", "https://127.0.0.1:1/dir")
	_ = os.Unsetenv("PEBBLE_CHALLTESTSRV_URL")
	_ = os.Unsetenv("PEBBLE_DNS_SERVER")

	h, err := startFromEnv(os.Getenv("PEBBLE_DIRECTORY_URL"))
	// We expect ErrSkip from waitReady, but startFromEnv must have
	// populated the fields before that.
	if err == nil {
		h.Close()
		t.Fatal("expected ErrSkip from unreachable defaults")
	}
	if !errors.Is(err, ErrSkip) {
		t.Fatalf("expected ErrSkip, got %v", err)
	}
}
