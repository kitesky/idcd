package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/lib/db/gen/idcdmain"
)

// mockAnchorPool implements AnchorPool for tests.
type mockAnchorPool struct {
	queryRow func(ctx context.Context, sql string, args ...any) pgx.Row
	query    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (m *mockAnchorPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRow != nil {
		return m.queryRow(ctx, sql, args...)
	}
	return &errRow{err: errors.New("not implemented")}
}

func (m *mockAnchorPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.query != nil {
		return m.query(ctx, sql, args...)
	}
	return &emptyRows{}, nil
}

// errRow is a pgx.Row that always returns a configurable error.
type errRow struct {
	err error
}

func (e *errRow) Scan(_ ...any) error {
	return e.err
}

func newAnchorRouter(mq MonitorQuerier, pool AnchorPool) http.Handler {
	h := NewAnchorHandler(mq, pool)
	r := chi.NewRouter()
	r.Get("/{id}/baseline", h.GetBaseline)
	r.Get("/{id}/deviations", h.ListDeviations)
	return r
}

// --- GetBaseline tests ---

func TestAnchorHandler_GetBaseline_noAuth_401(t *testing.T) {
	h := newAnchorRouter(&mockMonitorQuerier{}, &mockAnchorPool{})
	req := httptest.NewRequest(http.MethodGet, "/mon_001/baseline", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAnchorHandler_GetBaseline_monitorNotFound_404(t *testing.T) {
	mq := &mockMonitorQuerier{
		getByID: func(_ context.Context, _ string) (idcdmain.Monitor, error) {
			return idcdmain.Monitor{}, errors.New("not found")
		},
	}
	h := newAnchorRouter(mq, &mockAnchorPool{})
	req := httptest.NewRequest(http.MethodGet, "/mon_999/baseline", nil)
	req = injectUserID(req, "u_alice")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAnchorHandler_GetBaseline_noBaseline_404(t *testing.T) {
	mq := &mockMonitorQuerier{
		getByID: func(_ context.Context, id string) (idcdmain.Monitor, error) {
			return fakeMonitor(id, "u_alice"), nil
		},
	}
	pool := &mockAnchorPool{
		queryRow: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &errRow{err: pgx.ErrNoRows}
		},
	}
	h := newAnchorRouter(mq, pool)
	req := httptest.NewRequest(http.MethodGet, "/mon_001/baseline", nil)
	req = injectUserID(req, "u_alice")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAnchorHandler_GetBaseline_success_200(t *testing.T) {
	mq := &mockMonitorQuerier{
		getByID: func(_ context.Context, id string) (idcdmain.Monitor, error) {
			return fakeMonitor(id, "u_alice"), nil
		},
	}
	p50 := 50.0
	p95 := 100.0
	p99 := 200.0
	sr := 0.99
	pool := &mockAnchorPool{
		queryRow: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanRow{
				values: []any{
					"bln_xyz", "mon_001", &p50, &p95, &p99, &sr, 500, time.Now(), 168,
				},
			}
		},
	}
	h := newAnchorRouter(mq, pool)
	req := httptest.NewRequest(http.MethodGet, "/mon_001/baseline", nil)
	req = injectUserID(req, "u_alice")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- ListDeviations tests ---

func TestAnchorHandler_ListDeviations_noAuth_401(t *testing.T) {
	h := newAnchorRouter(&mockMonitorQuerier{}, &mockAnchorPool{})
	req := httptest.NewRequest(http.MethodGet, "/mon_001/deviations", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAnchorHandler_ListDeviations_success_empty_200(t *testing.T) {
	mq := &mockMonitorQuerier{
		getByID: func(_ context.Context, id string) (idcdmain.Monitor, error) {
			return fakeMonitor(id, "u_alice"), nil
		},
	}
	pool := &mockAnchorPool{
		query: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &emptyRows{}, nil
		},
	}
	h := newAnchorRouter(mq, pool)
	req := httptest.NewRequest(http.MethodGet, "/mon_001/deviations", nil)
	req = injectUserID(req, "u_alice")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAnchorHandler_ListDeviations_wrongOwner_403(t *testing.T) {
	mq := &mockMonitorQuerier{
		getByID: func(_ context.Context, id string) (idcdmain.Monitor, error) {
			return fakeMonitor(id, "u_bob"), nil
		},
	}
	h := newAnchorRouter(mq, &mockAnchorPool{})
	req := httptest.NewRequest(http.MethodGet, "/mon_001/deviations", nil)
	req = injectUserID(req, "u_alice")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

// --- scanRow helper ---

type scanRow struct {
	values []any
	err    error
}

func (s *scanRow) Scan(dest ...any) error {
	if s.err != nil {
		return s.err
	}
	for i, d := range dest {
		if i >= len(s.values) {
			break
		}
		switch v := d.(type) {
		case *string:
			if sv, ok := s.values[i].(string); ok {
				*v = sv
			}
		case **float64:
			if fv, ok := s.values[i].(*float64); ok {
				*v = fv
			}
		case *float64:
			if fv, ok := s.values[i].(float64); ok {
				*v = fv
			}
		case *int:
			if iv, ok := s.values[i].(int); ok {
				*v = iv
			}
		case *time.Time:
			if tv, ok := s.values[i].(time.Time); ok {
				*v = tv
			}
		}
	}
	return nil
}

// mockQueryRows wraps a slice of scan values for Query mock.
type mockQueryRows struct {
	rows [][]any
	pos  int
	err  error
}

func (m *mockQueryRows) Close() {}
func (m *mockQueryRows) Err() error { return m.err }
func (m *mockQueryRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (m *mockQueryRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *mockQueryRows) Next() bool {
	if m.pos < len(m.rows) {
		m.pos++
		return true
	}
	return false
}
func (m *mockQueryRows) Scan(dest ...any) error {
	row := m.rows[m.pos-1]
	for i, d := range dest {
		if i >= len(row) {
			break
		}
		switch v := d.(type) {
		case *string:
			if sv, ok := row[i].(string); ok {
				*v = sv
			}
		case *float64:
			if fv, ok := row[i].(float64); ok {
				*v = fv
			}
		case *int:
			if iv, ok := row[i].(int); ok {
				*v = iv
			}
		case *time.Time:
			if tv, ok := row[i].(time.Time); ok {
				*v = tv
			}
		case **time.Time:
			// optional resolved_at
		}
	}
	return nil
}
func (m *mockQueryRows) Values() ([]any, error) { return nil, nil }
func (m *mockQueryRows) RawValues() [][]byte            { return nil }
func (m *mockQueryRows) Conn() *pgx.Conn                { return nil }
