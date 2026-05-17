package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
)

func TestAdminBearerAuth_Accepts(t *testing.T) {
	mw := adminBearerAuth("sekret")
	called := false
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer sekret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !called {
		t.Fatalf("next handler not invoked")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestAdminBearerAuth_Rejects(t *testing.T) {
	cases := map[string]string{
		"missing":    "",
		"no_prefix":  "sekret",
		"wrong_token": "Bearer nope",
		"empty_token": "Bearer ",
	}
	mw := adminBearerAuth("sekret")
	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				t.Fatalf("next must not be invoked")
			})
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			if header != "" {
				req.Header.Set("Authorization", header)
			}
			rec := httptest.NewRecorder()
			mw(next).ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("code = %d, want 401", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), "CERT_UNAUTHORIZED") {
				t.Fatalf("body should expose CERT_UNAUTHORIZED, got %q", rec.Body.String())
			}
		})
	}
}

// stubAuditPool implements repo.Pool well enough that AuditLogsRepo can
// Append. The Pool interface only needs Exec/Query/QueryRow; we proxy
// QueryRow to a fake scanner so RETURNING id, occurred_at completes.
//
// (We deliberately avoid pgxmock here — main_test stays a unit test of
// the audit gate wiring, not of repo SQL.)

func TestAuditAbuseGate_NilRepo(t *testing.T) {
	g := newAuditAbuseGate(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := g.Ban(context.Background(), 7, "spam"); err == nil {
		t.Fatalf("nil repo should error")
	}
}

func TestAuditAbuseGate_NilReceiver(t *testing.T) {
	var g *auditAbuseGate
	if err := g.Ban(context.Background(), 7, "spam"); err == nil {
		t.Fatalf("nil receiver should error")
	}
}

// Compile-time check: auditAbuseGate satisfies the contract we expose
// to the handler. We do not import the handler package here (would be a
// circular dep at link time for cmd/), but verify the method set
// directly.
type abuseGateAPI interface {
	Ban(ctx context.Context, accountID int64, reason string) error
}

var _ abuseGateAPI = (*auditAbuseGate)(nil)

// Touch the unused error path from repo to keep linters happy without a
// real DB. The integration path is covered by repo/audit_logs_test.go.
var _ = errors.New
var _ = (*repo.AuditLog)(nil)
