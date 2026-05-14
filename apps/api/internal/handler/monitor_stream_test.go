package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/kite365/idcd/lib/db/gen/idcdmain"
)

// mockStreamPool implements MonitorStreamPool for tests.
type mockStreamPool struct {
	row pgx.Row
}

func (m *mockStreamPool) QueryRow(_ context.Context, _ string, _ ...interface{}) pgx.Row {
	return m.row
}

func newStreamHandler(mq MonitorQuerier, pool MonitorStreamPool) http.Handler {
	h := NewMonitorStreamHandler(mq, pool)
	r := chi.NewRouter()
	r.Get("/{id}/stream", h.Stream)
	return r
}

func TestMonitorStream_noAuth(t *testing.T) {
	pool := &mockStreamPool{row: &errRow{err: errors.New("no rows")}}
	h := newStreamHandler(&mockMonitorQuerier{}, pool)

	req := httptest.NewRequest(http.MethodGet, "/mon_001/stream", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestMonitorStream_monitorNotFound(t *testing.T) {
	mq := &mockMonitorQuerier{
		getByID: func(_ context.Context, _ string) (idcdmain.Monitor, error) {
			return idcdmain.Monitor{}, errors.New("not found")
		},
	}
	pool := &mockStreamPool{row: &errRow{err: errors.New("no rows")}}
	h := newStreamHandler(mq, pool)

	req := httptest.NewRequest(http.MethodGet, "/mon_nonexistent/stream", nil)
	req = injectUserID(req, "u_alice")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestMonitorStream_wrongOwner(t *testing.T) {
	mq := &mockMonitorQuerier{
		getByID: func(_ context.Context, _ string) (idcdmain.Monitor, error) {
			return fakeMonitor("mon_001", "u_bob"), nil
		},
	}
	pool := &mockStreamPool{row: &errRow{err: errors.New("no rows")}}
	h := newStreamHandler(mq, pool)

	req := httptest.NewRequest(http.MethodGet, "/mon_001/stream", nil)
	req = injectUserID(req, "u_alice")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestMonitorStream_success_headers(t *testing.T) {
	mq := &mockMonitorQuerier{
		getByID: func(_ context.Context, _ string) (idcdmain.Monitor, error) {
			return fakeMonitor("mon_001", "u_alice"), nil
		},
	}
	pool := &mockStreamPool{row: &errRow{err: errors.New("no rows")}}

	h := NewMonitorStreamHandler(mq, pool)

	r := chi.NewRouter()
	r.Get("/{id}/stream", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithCancel(req.Context())
		cancel()
		h.Stream(w, req.WithContext(ctx))
	})

	req := httptest.NewRequest(http.MethodGet, "/mon_001/stream", nil)
	req = injectUserID(req, "u_alice")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected Content-Type text/event-stream, got %q", ct)
	}
}
