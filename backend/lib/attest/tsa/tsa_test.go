package tsa_test

import (
	"bytes"
	"context"
	"crypto"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kite365/idcd/lib/attest/tsa"
)

// fakeProvider is a Provider implementation for unit-testing Multi.
type fakeProvider struct {
	name    string
	calls   atomic.Int32
	err     error // when non-nil, Stamp returns this error
	token   []byte
	issued  time.Time
	hookCtx func(context.Context) // optional — called with the ctx the Multi handed us
}

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) Stamp(ctx context.Context, _ crypto.Hash, _ []byte) ([]byte, time.Time, error) {
	f.calls.Add(1)
	if f.hookCtx != nil {
		f.hookCtx(ctx)
	}
	if f.err != nil {
		return nil, time.Time{}, f.err
	}
	return f.token, f.issued, nil
}

func newOK(name string) *fakeProvider {
	return &fakeProvider{
		name:   name,
		token:  []byte("token-" + name),
		issued: time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC),
	}
}

func TestNewMulti_CopiesSlice(t *testing.T) {
	a := newOK("a")
	b := newOK("b")
	providers := []tsa.Provider{a, b}
	m := tsa.NewMulti(providers...)
	// Mutating the caller's slice must not affect Multi's view.
	providers[0] = newOK("hacked")
	got := m.Names()
	if got[0] != "a" || got[1] != "b" {
		t.Fatalf("NewMulti did not copy providers: %v", got)
	}
}

func TestMulti_NamesEmpty(t *testing.T) {
	m := tsa.NewMulti()
	if len(m.Names()) != 0 {
		t.Fatalf("expected empty names, got %v", m.Names())
	}
}

func TestMulti_NoProviders(t *testing.T) {
	m := tsa.NewMulti()
	_, _, name, err := m.Stamp(context.Background(), crypto.SHA256, make([]byte, 32))
	if !errors.Is(err, tsa.ErrUpstreamUnavailable) {
		t.Fatalf("want ErrUpstreamUnavailable, got %v", err)
	}
	if name != "" {
		t.Fatalf("want empty provider name, got %q", name)
	}
}

func TestMulti_FirstProviderSuccess(t *testing.T) {
	a := newOK("primary")
	b := newOK("backup")
	m := tsa.NewMulti(a, b)

	tok, issued, name, err := m.Stamp(context.Background(), crypto.SHA256, make([]byte, 32))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "primary" {
		t.Fatalf("want primary, got %s", name)
	}
	if !bytes.Equal(tok, a.token) {
		t.Fatalf("token mismatch: %x vs %x", tok, a.token)
	}
	if !issued.Equal(a.issued) {
		t.Fatalf("issued time mismatch")
	}
	if b.calls.Load() != 0 {
		t.Fatalf("backup must not have been called")
	}
}

func TestMulti_FalloverOnUpstreamUnavailable(t *testing.T) {
	a := &fakeProvider{name: "primary", err: fmt.Errorf("%w: dial tcp", tsa.ErrUpstreamUnavailable)}
	b := newOK("backup")
	m := tsa.NewMulti(a, b)

	_, _, name, err := m.Stamp(context.Background(), crypto.SHA256, make([]byte, 32))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "backup" {
		t.Fatalf("want backup, got %s", name)
	}
	if a.calls.Load() != 1 || b.calls.Load() != 1 {
		t.Fatalf("expected both providers called once, got a=%d b=%d", a.calls.Load(), b.calls.Load())
	}
}

func TestMulti_FalloverOnInvalidResponse(t *testing.T) {
	a := &fakeProvider{name: "primary", err: fmt.Errorf("%w: bad ASN.1", tsa.ErrInvalidResponse)}
	b := newOK("backup")
	m := tsa.NewMulti(a, b)

	_, _, name, err := m.Stamp(context.Background(), crypto.SHA256, make([]byte, 32))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "backup" {
		t.Fatalf("want backup, got %s", name)
	}
}

func TestMulti_AllUpstreamFail(t *testing.T) {
	a := &fakeProvider{name: "primary", err: fmt.Errorf("%w: a", tsa.ErrUpstreamUnavailable)}
	b := &fakeProvider{name: "backup", err: fmt.Errorf("%w: b", tsa.ErrUpstreamUnavailable)}
	m := tsa.NewMulti(a, b)

	_, _, name, err := m.Stamp(context.Background(), crypto.SHA256, make([]byte, 32))
	if !errors.Is(err, tsa.ErrUpstreamUnavailable) {
		t.Fatalf("want wrapped ErrUpstreamUnavailable, got %v", err)
	}
	if name != "" {
		t.Fatalf("expected empty providerName on all-fail, got %q", name)
	}
	if a.calls.Load() != 1 || b.calls.Load() != 1 {
		t.Fatalf("each provider must be tried exactly once")
	}
}

func TestMulti_AbortOnAuthFailed(t *testing.T) {
	a := &fakeProvider{name: "primary", err: fmt.Errorf("%w: HTTP 401", tsa.ErrAuthFailed)}
	b := newOK("backup")
	m := tsa.NewMulti(a, b)

	_, _, name, err := m.Stamp(context.Background(), crypto.SHA256, make([]byte, 32))
	if !errors.Is(err, tsa.ErrAuthFailed) {
		t.Fatalf("want ErrAuthFailed, got %v", err)
	}
	if name != "primary" {
		t.Fatalf("want primary, got %s", name)
	}
	if b.calls.Load() != 0 {
		t.Fatalf("backup MUST NOT be called after fatal auth failure")
	}
}

func TestMulti_AbortOnInvalidInput(t *testing.T) {
	a := &fakeProvider{name: "primary", err: fmt.Errorf("%w: bad digest", tsa.ErrInvalidInput)}
	b := newOK("backup")
	m := tsa.NewMulti(a, b)

	_, _, _, err := m.Stamp(context.Background(), crypto.SHA256, make([]byte, 32))
	if !errors.Is(err, tsa.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
	if b.calls.Load() != 0 {
		t.Fatalf("backup MUST NOT be called after invalid input")
	}
}

func TestMulti_UnknownErrorTreatedAsTransient(t *testing.T) {
	// A provider that returns a non-sentinel error should still trigger
	// fall-over. After all providers fail, the final error is wrapped
	// with ErrUpstreamUnavailable for the caller.
	a := &fakeProvider{name: "primary", err: errors.New("totally random")}
	b := &fakeProvider{name: "backup", err: errors.New("also random")}
	m := tsa.NewMulti(a, b)

	_, _, _, err := m.Stamp(context.Background(), crypto.SHA256, make([]byte, 32))
	if !errors.Is(err, tsa.ErrUpstreamUnavailable) {
		t.Fatalf("want wrapped ErrUpstreamUnavailable, got %v", err)
	}
}

func TestMulti_PerCallTimeoutApplied(t *testing.T) {
	// The Multi must give each provider its own deadline; verify by
	// inspecting the deadline visible to the provider.
	var seenDeadlines []time.Time
	hook := func(ctx context.Context) {
		dl, ok := ctx.Deadline()
		if ok {
			seenDeadlines = append(seenDeadlines, dl)
		}
	}
	a := &fakeProvider{name: "a", err: fmt.Errorf("%w: x", tsa.ErrUpstreamUnavailable), hookCtx: hook}
	b := &fakeProvider{name: "b", token: []byte("ok"), hookCtx: hook}
	m := tsa.NewMulti(a, b)
	m.PerCallTimeout = 250 * time.Millisecond

	_, _, _, err := m.Stamp(context.Background(), crypto.SHA256, make([]byte, 32))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(seenDeadlines) != 2 {
		t.Fatalf("expected 2 deadlines, got %d", len(seenDeadlines))
	}
}

func TestMulti_PerCallTimeoutDisabledWhenNonPositive(t *testing.T) {
	var hadDeadline bool
	hook := func(ctx context.Context) {
		_, hadDeadline = ctx.Deadline()
	}
	a := &fakeProvider{name: "a", token: []byte("ok"), hookCtx: hook}
	m := tsa.NewMulti(a)
	m.PerCallTimeout = 0

	_, _, _, err := m.Stamp(context.Background(), crypto.SHA256, make([]byte, 32))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hadDeadline {
		t.Fatalf("expected no deadline when PerCallTimeout <= 0")
	}
}

func TestMulti_LoggerNilSafe(t *testing.T) {
	a := &fakeProvider{name: "a", err: fmt.Errorf("%w: x", tsa.ErrUpstreamUnavailable)}
	b := newOK("b")
	m := tsa.NewMulti(a, b)
	m.Logger = nil // explicit
	if _, _, _, err := m.Stamp(context.Background(), crypto.SHA256, make([]byte, 32)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMulti_LoggerCalledOnFallover(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	a := &fakeProvider{name: "primary", err: fmt.Errorf("%w: x", tsa.ErrUpstreamUnavailable)}
	b := newOK("backup")
	m := tsa.NewMulti(a, b)
	m.Logger = logger

	if _, _, _, err := m.Stamp(context.Background(), crypto.SHA256, make([]byte, 32)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("primary")) {
		t.Fatalf("logger output missing primary: %s", buf.String())
	}
}

func TestMulti_LoggerCalledOnFatalAbort(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	a := &fakeProvider{name: "primary", err: fmt.Errorf("%w: 401", tsa.ErrAuthFailed)}
	m := tsa.NewMulti(a)
	m.Logger = logger

	_, _, _, err := m.Stamp(context.Background(), crypto.SHA256, make([]byte, 32))
	if !errors.Is(err, tsa.ErrAuthFailed) {
		t.Fatalf("want ErrAuthFailed, got %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("abort")) {
		t.Fatalf("logger output missing abort marker: %s", buf.String())
	}
}

func TestValidateDigest_OK(t *testing.T) {
	sum := sha256.Sum256([]byte("hello"))
	if err := tsa.ValidateDigest(crypto.SHA256, sum[:]); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateDigest_WrongLength(t *testing.T) {
	err := tsa.ValidateDigest(crypto.SHA256, []byte("too short"))
	if !errors.Is(err, tsa.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestValidateDigest_UnavailableHash(t *testing.T) {
	// crypto.Hash(0) reports !Available.
	err := tsa.ValidateDigest(crypto.Hash(0), []byte{})
	if !errors.Is(err, tsa.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestSentinelErrors_AreDistinct(t *testing.T) {
	// Belt-and-braces: assert the sentinel errors are pairwise distinct
	// so a misuse of errors.Is in production code surfaces in tests.
	all := []error{tsa.ErrUpstreamUnavailable, tsa.ErrInvalidResponse, tsa.ErrAuthFailed, tsa.ErrInvalidInput}
	for i := 0; i < len(all); i++ {
		for j := 0; j < len(all); j++ {
			if i == j {
				continue
			}
			if errors.Is(all[i], all[j]) {
				t.Fatalf("sentinel %d collides with %d", i, j)
			}
		}
	}
}
