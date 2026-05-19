package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// mockPublicPool implements statusPagePublicPool for tests.
type mockPublicPool struct {
	queryRow func(ctx context.Context, sql string, args ...any) pgx.Row
	query    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (m *mockPublicPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRow != nil {
		return m.queryRow(ctx, sql, args...)
	}
	return &errRow{err: errors.New("not implemented")}
}

func (m *mockPublicPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.query != nil {
		return m.query(ctx, sql, args...)
	}
	return &emptyRows{}, nil
}

func newPublicRouter(pool statusPagePublicPool) http.Handler {
	h := NewStatusPagePublicHandler(pool, nil)
	r := chi.NewRouter()
	r.Get("/v1/status-pages/{slug}/public", h.Get)
	return r
}

func TestStatusPagePublic_NotFound(t *testing.T) {
	pool := &mockPublicPool{
		queryRow: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &errRow{err: errors.New("no rows")}
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/status-pages/missing/public", nil)
	rr := httptest.NewRecorder()
	newPublicRouter(pool).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestStatusPagePublic_Success_NoMonitors(t *testing.T) {
	pool := &mockPublicPool{
		queryRow: func(ctx context.Context, sql string, args ...any) pgx.Row {
			// Scan returns: id, user_id, slug, name, description, custom_domain,
			//               custom_domain_verified_at, custom_domain_cert_expires_at, branding, created_at, updated_at
			// scanRow (from anchor_test.go, same package) handles *string fields only;
			// pgtype and *bool fields are silently left at zero value which is fine for this test.
			return &scanRow{values: []any{
				"sp1", "user1", "demo", "Demo Status",
			}}
		},
		query: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &emptyRows{}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/status-pages/demo/public", nil)
	rr := httptest.NewRecorder()
	newPublicRouter(pool).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := decodeBody(t, rr.Body.Bytes())
	data, _ := body["data"].(map[string]any)

	if data["slug"] != "demo" {
		t.Errorf("expected slug=demo, got %v", data["slug"])
	}
	if data["overall_status"] != "operational" {
		t.Errorf("expected overall_status=operational, got %v", data["overall_status"])
	}
	groups, _ := data["groups"].([]any)
	if len(groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(groups))
	}
}

func TestOverallStatus_Operational(t *testing.T) {
	monitors := []publicMonitor{
		{Status: "operational"},
		{Status: "operational"},
	}
	if got := overallStatus(monitors); got != "operational" {
		t.Errorf("expected operational, got %s", got)
	}
}

func TestOverallStatus_Degraded(t *testing.T) {
	monitors := []publicMonitor{
		{Status: "operational"},
		{Status: "degraded"},
	}
	if got := overallStatus(monitors); got != "degraded" {
		t.Errorf("expected degraded, got %s", got)
	}
}

func TestOverallStatus_Outage(t *testing.T) {
	monitors := []publicMonitor{
		{Status: "degraded"},
		{Status: "outage"},
	}
	if got := overallStatus(monitors); got != "outage" {
		t.Errorf("expected outage, got %s", got)
	}
}

func TestDayStatus(t *testing.T) {
	cases := []struct {
		total, success int64
		want           string
	}{
		{0, 0, "operational"},
		{10, 10, "operational"},
		{10, 8, "degraded"},
		{10, 4, "outage"},
		{3, 0, "outage"},
	}
	for _, c := range cases {
		got := dayStatus(c.total, c.success)
		if got != c.want {
			t.Errorf("dayStatus(%d,%d): want %s, got %s", c.total, c.success, c.want, got)
		}
	}
}

func TestComputeUptimePercent(t *testing.T) {
	rows := []dayCheckRow{
		{date: "2026-04-15", total: 100, success: 99},
		{date: "2026-04-16", total: 100, success: 100},
	}
	got := computeUptimePercent(rows)
	want := 99.5
	if got != want {
		t.Errorf("expected %.2f, got %.2f", want, got)
	}
}

func TestComputeUptimePercent_Empty(t *testing.T) {
	got := computeUptimePercent(nil)
	if got != 100.0 {
		t.Errorf("expected 100.0 for empty, got %.2f", got)
	}
}

// TestBatchCurrentStatuses_FailClosed verifies that batchCurrentStatuses
// surfaces a DB query error to its caller rather than silently defaulting all
// monitors to "operational". The previous fail-open behaviour could mask a
// real outage to the audience the status page exists for.
func TestBatchCurrentStatuses_FailClosed(t *testing.T) {
	pool := &mockPublicPool{
		query: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("db unavailable")
		},
	}
	h := NewStatusPagePublicHandler(pool, nil)

	_, err := h.batchCurrentStatuses(context.Background(), []string{"mon_a", "mon_b"})
	if err == nil {
		t.Fatal("expected error when pool.Query fails, got nil")
	}
}

// TestBatchCurrentStatuses_EmptyInput documents the short-circuit: zero IDs
// yields an empty map without hitting the DB.
func TestBatchCurrentStatuses_EmptyInput(t *testing.T) {
	pool := &mockPublicPool{
		query: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			t.Fatal("Query should not be called for empty input")
			return nil, nil
		},
	}
	h := NewStatusPagePublicHandler(pool, nil)

	got, err := h.batchCurrentStatuses(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %d entries", len(got))
	}
}
