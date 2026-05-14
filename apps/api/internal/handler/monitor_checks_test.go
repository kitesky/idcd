package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/lib/db/gen/idcdmain"
)

// mockChecksPool implements MonitorChecksPool for tests.
type mockChecksPool struct {
	queryRow func(ctx context.Context, sql string, args ...interface{}) pgx.Row
	query    func(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
}

func (m *mockChecksPool) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	if m.queryRow != nil {
		return m.queryRow(ctx, sql, args...)
	}
	return &errRow{err: errors.New("not implemented")}
}

func (m *mockChecksPool) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	if m.query != nil {
		return m.query(ctx, sql, args...)
	}
	return &emptyRows{}, nil
}

// emptyRows is a pgx.Rows that returns no rows.
type emptyRows struct{}

func (e *emptyRows) Close()                                    {}
func (e *emptyRows) Err() error                                { return nil }
func (e *emptyRows) CommandTag() pgconn.CommandTag             { return pgconn.CommandTag{} }
func (e *emptyRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (e *emptyRows) Next() bool                                { return false }
func (e *emptyRows) Scan(_ ...interface{}) error               { return nil }
func (e *emptyRows) Values() ([]interface{}, error)            { return nil, nil }
func (e *emptyRows) RawValues() [][]byte                       { return nil }
func (e *emptyRows) Conn() *pgx.Conn                           { return nil }

func newChecksHandler(mq MonitorQuerier, pool MonitorChecksPool) http.Handler {
	h := NewMonitorChecksHandler(mq, pool)
	r := chi.NewRouter()
	r.Get("/{id}/checks", h.List)
	return r
}

func TestMonitorChecks_noAuth(t *testing.T) {
	pool := &mockChecksPool{}
	h := newChecksHandler(&mockMonitorQuerier{}, pool)

	req := httptest.NewRequest(http.MethodGet, "/mon_001/checks", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestMonitorChecks_monitorNotFound(t *testing.T) {
	mq := &mockMonitorQuerier{
		getByID: func(_ context.Context, _ string) (idcdmain.Monitor, error) {
			return idcdmain.Monitor{}, errors.New("not found")
		},
	}
	pool := &mockChecksPool{}
	h := newChecksHandler(mq, pool)

	req := httptest.NewRequest(http.MethodGet, "/mon_nonexistent/checks", nil)
	req = injectUserID(req, "u_alice")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestMonitorChecks_wrongOwner(t *testing.T) {
	mq := &mockMonitorQuerier{
		getByID: func(_ context.Context, _ string) (idcdmain.Monitor, error) {
			return fakeMonitor("mon_001", "u_bob"), nil
		},
	}
	pool := &mockChecksPool{}
	h := newChecksHandler(mq, pool)

	req := httptest.NewRequest(http.MethodGet, "/mon_001/checks", nil)
	req = injectUserID(req, "u_alice")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestMonitorChecks_success_emptyBuckets(t *testing.T) {
	mq := &mockMonitorQuerier{
		getByID: func(_ context.Context, id string) (idcdmain.Monitor, error) {
			return fakeMonitor(id, "u_alice"), nil
		},
	}
	pool := &mockChecksPool{}
	h := newChecksHandler(mq, pool)

	req := httptest.NewRequest(http.MethodGet, "/mon_001/checks?hours=24", nil)
	req = injectUserID(req, "u_alice")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data struct {
			MonitorID         string        `json:"monitor_id"`
			Hours             int           `json:"hours"`
			ResolutionMinutes int           `json:"resolution_minutes"`
			Buckets           []interface{} `json:"buckets"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data.MonitorID != "mon_001" {
		t.Errorf("expected monitor_id=mon_001, got %s", resp.Data.MonitorID)
	}
	if resp.Data.Hours != 24 {
		t.Errorf("expected hours=24, got %d", resp.Data.Hours)
	}
	if resp.Data.ResolutionMinutes != 30 {
		t.Errorf("expected resolution_minutes=30, got %d", resp.Data.ResolutionMinutes)
	}
	if resp.Data.Buckets == nil {
		t.Errorf("expected non-nil buckets array")
	}
}

func TestMonitorChecks_hoursClamp(t *testing.T) {
	mq := &mockMonitorQuerier{
		getByID: func(_ context.Context, id string) (idcdmain.Monitor, error) {
			return fakeMonitor(id, "u_alice"), nil
		},
	}
	pool := &mockChecksPool{}
	h := newChecksHandler(mq, pool)

	req := httptest.NewRequest(http.MethodGet, "/mon_001/checks?hours=9999", nil)
	req = injectUserID(req, "u_alice")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Data struct {
			Hours int `json:"hours"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data.Hours != 168 {
		t.Errorf("expected hours clamped to 168, got %d", resp.Data.Hours)
	}
}

func TestBucketStatus(t *testing.T) {
	tests := []struct {
		total, success, failure int64
		want                    string
	}{
		{0, 0, 0, "empty"},
		{2, 2, 0, "up"},
		{2, 0, 2, "down"},
		{3, 2, 1, "degraded"},
	}
	for _, tc := range tests {
		got := bucketStatus(tc.total, tc.success, tc.failure)
		if got != tc.want {
			t.Errorf("bucketStatus(%d,%d,%d) = %q, want %q", tc.total, tc.success, tc.failure, got, tc.want)
		}
	}
}
