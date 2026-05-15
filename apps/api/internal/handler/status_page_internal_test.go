package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kite365/idcd/lib/db/gen/idcdmain"
)

// mockStatusPageInternalQuerier implements statusPageInternalQuerier.
type mockStatusPageInternalQuerier struct {
	getByCustomDomain func(ctx context.Context, domain string) (idcdmain.StatusPage, error)
}

func (m *mockStatusPageInternalQuerier) GetStatusPageByCustomDomain(ctx context.Context, domain string) (idcdmain.StatusPage, error) {
	if m.getByCustomDomain != nil {
		return m.getByCustomDomain(ctx, domain)
	}
	return idcdmain.StatusPage{}, errors.New("not found")
}

func TestByDomain_VerifiedDomain(t *testing.T) {
	q := &mockStatusPageInternalQuerier{
		getByCustomDomain: func(ctx context.Context, domain string) (idcdmain.StatusPage, error) {
			return idcdmain.StatusPage{
				ID:                     "sp1",
				UserID:                 "user1",
				Slug:                   "my-status",
				Name:                   "My Status",
				CustomDomain:           &domain,
				CustomDomainVerifiedAt: pgtype.Timestamptz{Valid: true},
			}, nil
		},
	}
	h := NewStatusPageInternalHandler(q)

	req := httptest.NewRequest(http.MethodGet, "/internal/status-pages/by-domain?domain=status.example.com", nil)
	rr := httptest.NewRecorder()
	h.ByDomain(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr.Body.Bytes())
	data, _ := body["data"].(map[string]any)
	if data["slug"] != "my-status" {
		t.Errorf("expected slug=my-status, got %v", data["slug"])
	}
}

func TestByDomain_UnverifiedDomain(t *testing.T) {
	domain := "status.example.com"
	q := &mockStatusPageInternalQuerier{
		getByCustomDomain: func(ctx context.Context, d string) (idcdmain.StatusPage, error) {
			return idcdmain.StatusPage{
				ID:                     "sp1",
				Slug:                   "my-status",
				CustomDomain:           &domain,
				CustomDomainVerifiedAt: pgtype.Timestamptz{Valid: false}, // not verified
			}, nil
		},
	}
	h := NewStatusPageInternalHandler(q)

	req := httptest.NewRequest(http.MethodGet, "/internal/status-pages/by-domain?domain=status.example.com", nil)
	rr := httptest.NewRecorder()
	h.ByDomain(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unverified domain, got %d", rr.Code)
	}
}

func TestByDomain_MissingDomainParam(t *testing.T) {
	h := NewStatusPageInternalHandler(&mockStatusPageInternalQuerier{})

	req := httptest.NewRequest(http.MethodGet, "/internal/status-pages/by-domain", nil)
	rr := httptest.NewRecorder()
	h.ByDomain(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing domain param, got %d", rr.Code)
	}
}

func TestByDomain_NotFound(t *testing.T) {
	q := &mockStatusPageInternalQuerier{
		getByCustomDomain: func(ctx context.Context, domain string) (idcdmain.StatusPage, error) {
			return idcdmain.StatusPage{}, errors.New("not found")
		},
	}
	h := NewStatusPageInternalHandler(q)

	req := httptest.NewRequest(http.MethodGet, "/internal/status-pages/by-domain?domain=unknown.example.com", nil)
	rr := httptest.NewRecorder()
	h.ByDomain(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}
