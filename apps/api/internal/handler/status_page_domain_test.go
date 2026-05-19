package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
)

// --- mock querier ---

type mockStatusPageDomainQuerier struct {
	getByID              func(ctx context.Context, id string) (idcdmain.StatusPage, error)
	getByCustomDomain    func(ctx context.Context, domain *string) (idcdmain.StatusPage, error)
	setCustomDomain      func(ctx context.Context, arg idcdmain.SetStatusPageCustomDomainParams) (idcdmain.StatusPage, error)
	markVerified         func(ctx context.Context, id string) error
}

func (m *mockStatusPageDomainQuerier) GetStatusPageByID(ctx context.Context, id string) (idcdmain.StatusPage, error) {
	if m.getByID != nil {
		return m.getByID(ctx, id)
	}
	return idcdmain.StatusPage{}, errors.New("not found")
}

func (m *mockStatusPageDomainQuerier) GetStatusPageByCustomDomain(ctx context.Context, domain *string) (idcdmain.StatusPage, error) {
	if m.getByCustomDomain != nil {
		return m.getByCustomDomain(ctx, domain)
	}
	return idcdmain.StatusPage{}, errors.New("not found")
}

func (m *mockStatusPageDomainQuerier) SetStatusPageCustomDomain(ctx context.Context, arg idcdmain.SetStatusPageCustomDomainParams) (idcdmain.StatusPage, error) {
	if m.setCustomDomain != nil {
		return m.setCustomDomain(ctx, arg)
	}
	return idcdmain.StatusPage{}, errors.New("not implemented")
}

func (m *mockStatusPageDomainQuerier) MarkCustomDomainVerified(ctx context.Context, id string) error {
	if m.markVerified != nil {
		return m.markVerified(ctx, id)
	}
	return nil
}

// --- helpers ---

func fakeStatusPage(id, userID, slug string, customDomain *string, verified bool) idcdmain.StatusPage {
	sp := idcdmain.StatusPage{
		ID:           id,
		UserID:       userID,
		Slug:         slug,
		Name:         "Test Status Page",
		CustomDomain: customDomain,
	}
	if verified {
		sp.CustomDomainVerifiedAt = pgtype.Timestamptz{Valid: true}
	}
	return sp
}

func newDomainRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func withDomainUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDContextKey(), userID)
	return r.WithContext(ctx)
}

func withChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	return r.WithContext(ctx)
}

func decodeBody(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return m
}

// --- SetStatusPageDomain tests ---

func TestSetStatusPageDomain_Success(t *testing.T) {
	domain := "status.example.com"
	q := &mockStatusPageDomainQuerier{
		getByID: func(ctx context.Context, id string) (idcdmain.StatusPage, error) {
			return fakeStatusPage(id, "user1", "my-page", nil, false), nil
		},
		setCustomDomain: func(ctx context.Context, arg idcdmain.SetStatusPageCustomDomainParams) (idcdmain.StatusPage, error) {
			return fakeStatusPage(arg.ID, arg.UserID, "my-page", arg.CustomDomain, false), nil
		},
	}
	h := NewStatusPageDomainHandler(q, slog.Default())
	// Override CNAME lookup so the async goroutine doesn't attempt real DNS.
	h = h.withLookupCNAME(func(_ context.Context, d string) (string, error) {
		return "status.idcd.com.", nil
	})

	req := newDomainRequest(t, http.MethodPatch, "/v1/status-pages/sp1/domain", map[string]string{"custom_domain": domain})
	req = withDomainUserID(req, "user1")
	req = withChiParam(req, "id", "sp1")

	rr := httptest.NewRecorder()
	h.SetStatusPageDomain(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := decodeBody(t, rr.Body.Bytes())
	data, _ := body["data"].(map[string]any)
	if data == nil {
		t.Fatal("expected data field in response")
	}
	if data["custom_domain"] != domain {
		t.Errorf("expected custom_domain=%q, got %q", domain, data["custom_domain"])
	}
}

func TestSetStatusPageDomain_UnbindWithEmptyString(t *testing.T) {
	q := &mockStatusPageDomainQuerier{
		getByID: func(ctx context.Context, id string) (idcdmain.StatusPage, error) {
			d := "status.example.com"
			return fakeStatusPage(id, "user1", "my-page", &d, true), nil
		},
		setCustomDomain: func(ctx context.Context, arg idcdmain.SetStatusPageCustomDomainParams) (idcdmain.StatusPage, error) {
			return fakeStatusPage(arg.ID, arg.UserID, "my-page", nil, false), nil
		},
	}
	h := NewStatusPageDomainHandler(q, slog.Default())

	req := newDomainRequest(t, http.MethodPatch, "/v1/status-pages/sp1/domain", map[string]string{"custom_domain": ""})
	req = withDomainUserID(req, "user1")
	req = withChiParam(req, "id", "sp1")

	rr := httptest.NewRecorder()
	h.SetStatusPageDomain(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSetStatusPageDomain_InvalidDomainFormat(t *testing.T) {
	q := &mockStatusPageDomainQuerier{
		getByID: func(ctx context.Context, id string) (idcdmain.StatusPage, error) {
			return fakeStatusPage(id, "user1", "my-page", nil, false), nil
		},
	}
	h := NewStatusPageDomainHandler(q, slog.Default())

	req := newDomainRequest(t, http.MethodPatch, "/v1/status-pages/sp1/domain", map[string]string{"custom_domain": "not a domain!!!"})
	req = withDomainUserID(req, "user1")
	req = withChiParam(req, "id", "sp1")

	rr := httptest.NewRecorder()
	h.SetStatusPageDomain(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestSetStatusPageDomain_RejectsIdcdSubdomain(t *testing.T) {
	q := &mockStatusPageDomainQuerier{
		getByID: func(ctx context.Context, id string) (idcdmain.StatusPage, error) {
			return fakeStatusPage(id, "user1", "my-page", nil, false), nil
		},
	}
	h := NewStatusPageDomainHandler(q, slog.Default())

	req := newDomainRequest(t, http.MethodPatch, "/v1/status-pages/sp1/domain", map[string]string{"custom_domain": "hacker.idcd.com"})
	req = withDomainUserID(req, "user1")
	req = withChiParam(req, "id", "sp1")

	rr := httptest.NewRecorder()
	h.SetStatusPageDomain(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestSetStatusPageDomain_OwnershipCheckFails(t *testing.T) {
	q := &mockStatusPageDomainQuerier{
		getByID: func(ctx context.Context, id string) (idcdmain.StatusPage, error) {
			// Belongs to user2, not user1.
			return fakeStatusPage(id, "user2", "their-page", nil, false), nil
		},
	}
	h := NewStatusPageDomainHandler(q, slog.Default())

	req := newDomainRequest(t, http.MethodPatch, "/v1/status-pages/sp1/domain", map[string]string{"custom_domain": "status.example.com"})
	req = withDomainUserID(req, "user1")
	req = withChiParam(req, "id", "sp1")

	rr := httptest.NewRecorder()
	h.SetStatusPageDomain(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestSetStatusPageDomain_Unauthenticated(t *testing.T) {
	h := NewStatusPageDomainHandler(&mockStatusPageDomainQuerier{}, slog.Default())

	req := newDomainRequest(t, http.MethodPatch, "/v1/status-pages/sp1/domain", map[string]string{"custom_domain": "status.example.com"})
	// No userID set.
	req = withChiParam(req, "id", "sp1")

	rr := httptest.NewRecorder()
	h.SetStatusPageDomain(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// --- VerifyStatusPageDomain tests ---

func TestVerifyStatusPageDomain_CNAMECorrect(t *testing.T) {
	d := "status.example.com"
	q := &mockStatusPageDomainQuerier{
		getByID: func(ctx context.Context, id string) (idcdmain.StatusPage, error) {
			return fakeStatusPage(id, "user1", "my-page", &d, false), nil
		},
		markVerified: func(ctx context.Context, id string) error {
			return nil
		},
	}
	h := NewStatusPageDomainHandler(q, slog.Default())
	h = h.withLookupCNAME(func(_ context.Context, domain string) (string, error) {
		return "status.idcd.com.", nil
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/status-pages/sp1/domain/verify", nil)
	req = withDomainUserID(req, "user1")
	req = withChiParam(req, "id", "sp1")

	rr := httptest.NewRecorder()
	h.VerifyStatusPageDomain(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr.Body.Bytes())
	data, _ := body["data"].(map[string]any)
	if data["verified"] != true {
		t.Errorf("expected verified=true, got %v", data["verified"])
	}
}

func TestVerifyStatusPageDomain_CNAMEWrong(t *testing.T) {
	d := "status.example.com"
	q := &mockStatusPageDomainQuerier{
		getByID: func(ctx context.Context, id string) (idcdmain.StatusPage, error) {
			return fakeStatusPage(id, "user1", "my-page", &d, false), nil
		},
	}
	h := NewStatusPageDomainHandler(q, slog.Default())
	h = h.withLookupCNAME(func(_ context.Context, domain string) (string, error) {
		return "other-host.example.com.", nil // wrong CNAME
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/status-pages/sp1/domain/verify", nil)
	req = withDomainUserID(req, "user1")
	req = withChiParam(req, "id", "sp1")

	rr := httptest.NewRecorder()
	h.VerifyStatusPageDomain(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	body := decodeBody(t, rr.Body.Bytes())
	data, _ := body["data"].(map[string]any)
	if data["verified"] == true {
		t.Error("expected verified=false for wrong CNAME")
	}
}

func TestVerifyStatusPageDomain_NoDomainConfigured(t *testing.T) {
	q := &mockStatusPageDomainQuerier{
		getByID: func(ctx context.Context, id string) (idcdmain.StatusPage, error) {
			return fakeStatusPage(id, "user1", "my-page", nil, false), nil
		},
	}
	h := NewStatusPageDomainHandler(q, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/v1/status-pages/sp1/domain/verify", nil)
	req = withDomainUserID(req, "user1")
	req = withChiParam(req, "id", "sp1")

	rr := httptest.NewRecorder()
	h.VerifyStatusPageDomain(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for no domain configured, got %d", rr.Code)
	}
}

func TestVerifyStatusPageDomain_OwnershipCheckFails(t *testing.T) {
	d := "status.example.com"
	q := &mockStatusPageDomainQuerier{
		getByID: func(ctx context.Context, id string) (idcdmain.StatusPage, error) {
			return fakeStatusPage(id, "user2", "their-page", &d, false), nil
		},
	}
	h := NewStatusPageDomainHandler(q, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/v1/status-pages/sp1/domain/verify", nil)
	req = withDomainUserID(req, "user1")
	req = withChiParam(req, "id", "sp1")

	rr := httptest.NewRecorder()
	h.VerifyStatusPageDomain(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}
