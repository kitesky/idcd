package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/quota"
)

// ─────────────────────────────────────────────
// Mocks for QuotaHandler
// ─────────────────────────────────────────────

// mockQuotaHandlerPool provides configurable QueryRow results for the
// QuotaHandler's plan and count queries.
// Call ordering: plan → monitors → channels → status_pages.
type mockQuotaHandlerPool struct {
	responses []*mockQuotaRow
	call      int
}

func (m *mockQuotaHandlerPool) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if m.call < len(m.responses) {
		r := m.responses[m.call]
		m.call++
		return r
	}
	return &mockQuotaRow{err: pgx.ErrNoRows}
}

func (m *mockQuotaHandlerPool) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}

// mockQuotaRateLimiter simulates the CurrentUsage and DailyTrend calls.
type mockQuotaRateLimiter struct {
	used int
	err  error
}

func (m *mockQuotaRateLimiter) CurrentUsage(_ context.Context, _ string) (int, error) {
	return m.used, m.err
}

func (m *mockQuotaRateLimiter) DailyTrend(_ context.Context, _ string) ([]quota.DayCount, error) {
	return nil, m.err
}

// quotaHandlerInjectUserID adds a user ID to the request context.
func quotaHandlerInjectUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDContextKey(), userID)
	return r.WithContext(ctx)
}

// ─────────────────────────────────────────────
// Tests — GetQuota
// ─────────────────────────────────────────────

func TestQuotaHandler_GetQuota_NoAuth(t *testing.T) {
	h := NewQuotaHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/account/quota", nil)
	rr := httptest.NewRecorder()
	h.GetQuota(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestQuotaHandler_GetQuota_NilPool(t *testing.T) {
	// With nil pool all counts default to 0 and plan defaults to "free".
	h := NewQuotaHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/account/quota", nil)
	req = quotaHandlerInjectUserID(req, "u_test")
	rr := httptest.NewRecorder()
	h.GetQuota(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestQuotaHandler_GetQuota_FreePlan(t *testing.T) {
	pool := &mockQuotaHandlerPool{
		responses: []*mockQuotaRow{
			{val: "free"}, // plan lookup
			{val: 2},      // monitor count
			{val: 1},      // channel count
			{val: 0},      // status page count
		},
	}
	rl := &mockQuotaRateLimiter{used: 42}
	h := NewQuotaHandler(pool, rl)

	req := httptest.NewRequest(http.MethodGet, "/v1/account/quota", nil)
	req = quotaHandlerInjectUserID(req, "u_free")
	rr := httptest.NewRecorder()
	h.GetQuota(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestQuotaHandler_GetQuota_ProPlan(t *testing.T) {
	pool := &mockQuotaHandlerPool{
		responses: []*mockQuotaRow{
			{val: "pro"},
			{val: 10},
			{val: 3},
			{val: 1},
		},
	}
	rl := &mockQuotaRateLimiter{used: 500}
	h := NewQuotaHandler(pool, rl)

	req := httptest.NewRequest(http.MethodGet, "/v1/account/quota", nil)
	req = quotaHandlerInjectUserID(req, "u_pro")
	rr := httptest.NewRecorder()
	h.GetQuota(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestQuotaHandler_GetQuota_RateLimiterError(t *testing.T) {
	// When CurrentUsage returns an error, API daily usage is reported as 0.
	pool := &mockQuotaHandlerPool{
		responses: []*mockQuotaRow{
			{val: "pro"},
			{val: 0},
			{val: 0},
			{val: 0},
		},
	}
	rl := &mockQuotaRateLimiter{err: errors.New("redis unavailable")}
	h := NewQuotaHandler(pool, rl)

	req := httptest.NewRequest(http.MethodGet, "/v1/account/quota", nil)
	req = quotaHandlerInjectUserID(req, "u_pro")
	rr := httptest.NewRecorder()
	h.GetQuota(rr, req)

	// Should still return 200 — rate limiter errors are non-fatal.
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 even when rate limiter errors, got %d", rr.Code)
	}
}

func TestQuotaHandler_GetQuota_PlanLookupError(t *testing.T) {
	// When the plan DB lookup fails, should fall back to "free".
	pool := &mockQuotaHandlerPool{
		responses: []*mockQuotaRow{
			{err: pgx.ErrNoRows}, // plan lookup fails → default free
			{val: 0},
			{val: 0},
			{val: 0},
		},
	}
	h := NewQuotaHandler(pool, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/account/quota", nil)
	req = quotaHandlerInjectUserID(req, "u_noDB")
	rr := httptest.NewRecorder()
	h.GetQuota(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 fallback to free, got %d", rr.Code)
	}
}

func TestQuotaHandler_GetQuota_BusinessPlan(t *testing.T) {
	pool := &mockQuotaHandlerPool{
		responses: []*mockQuotaRow{
			{val: "business"},
			{val: 500},
			{val: 100},
			{val: 20},
		},
	}
	rl := &mockQuotaRateLimiter{used: 0}
	h := NewQuotaHandler(pool, rl)

	req := httptest.NewRequest(http.MethodGet, "/v1/account/quota", nil)
	req = quotaHandlerInjectUserID(req, "u_business")
	rr := httptest.NewRecorder()
	h.GetQuota(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
