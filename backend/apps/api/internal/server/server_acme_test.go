// server_acme_test.go — smoke tests for ACME HTTP-01 route wiring.
//
// We don't drive autocert end-to-end (that would require a live ACME
// server).  Instead we mount the manager onto a server, hit the
// /.well-known/acme-challenge/{token} path, and verify the response
// comes from autocert (404 with the autocert body, NOT chi's default
// 404).  This is enough to prove the route is wired and that swapping
// the production manager in will be served correctly.

package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	acmemgr "github.com/kite365/idcd/apps/api/internal/acme"
)

// stubChecker satisfies acmemgr.DomainChecker without touching a real DB.
type stubChecker struct{}

func (stubChecker) IsVerifiedDomain(ctx context.Context, host string) (bool, error) {
	return false, nil
}

// newACMETestServer builds a Server with only the bits required for the
// ACME route — no DB, no Redis.  We bypass setupRouter and inject the
// chi router directly so the smoke test stays hermetic.
func newACMETestServer(t *testing.T) *Server {
	t.Helper()
	r := chi.NewRouter()
	return &Server{
		router: r,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestMountACME_ChallengeRouteRegistered(t *testing.T) {
	srv := newACMETestServer(t)
	mgr := acmemgr.New(acmemgr.Config{CacheDir: t.TempDir()}, stubChecker{})

	srv.MountACME(mgr)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/acme-challenge/abc123token", nil)
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	// autocert returns 404 with body "acme/autocert: ..." when no
	// challenge is pending — proves the request reached the autocert
	// handler rather than chi's default 404.
	body := rr.Body.String()
	if rr.Code == http.StatusNotFound && !strings.Contains(body, "autocert") {
		t.Fatalf("expected autocert-served 404, got chi 404 body=%q", body)
	}
	// Any non-405 / non-pure-chi response means the route is wired.
	if rr.Code == http.StatusMethodNotAllowed {
		t.Fatalf("ACME challenge route not registered as GET: status=%d", rr.Code)
	}
}

func TestMountACME_NilManagerIsNoop(t *testing.T) {
	srv := newACMETestServer(t)
	srv.MountACME(nil) // must not panic

	req := httptest.NewRequest(http.MethodGet, "/.well-known/acme-challenge/x", nil)
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (no route), got %d", rr.Code)
	}
}

func TestStartACMEHTTPListener_NilManagerIsNoop(t *testing.T) {
	srv := newACMETestServer(t)
	// Should return immediately without starting any goroutine.
	srv.StartACMEHTTPListener(nil, ":0")
	srv.StartACMEHTTPListener(&acmemgr.Manager{}, "")
}

func TestACMETLSConfig_ReturnsConfiguredTLS(t *testing.T) {
	if cfg := ACMETLSConfig(nil); cfg != nil {
		t.Fatalf("expected nil tls.Config for nil manager, got %#v", cfg)
	}
	mgr := acmemgr.New(acmemgr.Config{CacheDir: t.TempDir()}, stubChecker{})
	cfg := ACMETLSConfig(mgr)
	if cfg == nil || cfg.GetCertificate == nil {
		t.Fatal("expected tls.Config with GetCertificate wired")
	}
	foundALPN := false
	for _, proto := range cfg.NextProtos {
		if proto == "acme-tls/1" {
			foundALPN = true
			break
		}
	}
	if !foundALPN {
		t.Errorf("expected acme-tls/1 in NextProtos, got %v", cfg.NextProtos)
	}
}
