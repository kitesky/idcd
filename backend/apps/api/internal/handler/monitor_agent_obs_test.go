package handler

import (
	"bytes"
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

// mockAgentObsPool implements AgentObsPool for tests.
type mockAgentObsPool struct {
	queryRow func(ctx context.Context, sql string, args ...any) pgx.Row
	query    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	exec     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (m *mockAgentObsPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRow != nil {
		return m.queryRow(ctx, sql, args...)
	}
	return &errRow{err: errors.New("not found")}
}

func (m *mockAgentObsPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.query != nil {
		return m.query(ctx, sql, args...)
	}
	return &emptyRows{}, nil
}

func (m *mockAgentObsPool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if m.exec != nil {
		return m.exec(ctx, sql, args...)
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func newAgentObsRouter(mq MonitorQuerier, pool AgentObsPool) http.Handler {
	h := NewAgentObsHandler(mq, pool)
	r := chi.NewRouter()
	r.Post("/{id}/agent-obs", h.CreateConfig)
	r.Get("/{id}/agent-obs", h.GetConfig)
	r.Patch("/{id}/agent-obs", h.UpdateConfig)
	r.Delete("/{id}/agent-obs", h.DeleteConfig)
	r.Get("/{id}/agent-obs/checks", h.ListChecks)
	return r
}

func ownerMQ(_, userID string) *mockMonitorQuerier {
	return &mockMonitorQuerier{
		getByID: func(_ context.Context, mid string) (idcdmain.Monitor, error) {
			return fakeMonitor(mid, userID), nil
		},
	}
}

// --- POST /agent-obs ---

func TestAgentObs_PostConfig_noAuth(t *testing.T) {
	h := newAgentObsRouter(&mockMonitorQuerier{}, &mockAgentObsPool{})
	body, _ := json.Marshal(map[string]any{
		"obs_type":     "llm_endpoint",
		"endpoint_url": "https://api.openai.com/v1/chat",
	})
	req := httptest.NewRequest(http.MethodPost, "/mon_001/agent-obs", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAgentObs_PostConfig_success(t *testing.T) {
	mq := ownerMQ("mon_001", "u_alice")

	configRow := &mockRow{
		scanFn: func(dest ...any) error {
			*(dest[0].(*string)) = "llm_endpoint"
			*(dest[1].(*string)) = "https://api.openai.com/v1/chat"
			return nil
		},
	}
	pool := &mockAgentObsPool{
		exec: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
		queryRow: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return configRow
		},
	}

	h := newAgentObsRouter(mq, pool)
	body, _ := json.Marshal(map[string]any{
		"obs_type":     "llm_endpoint",
		"endpoint_url": "https://api.openai.com/v1/chat",
	})
	req := httptest.NewRequest(http.MethodPost, "/mon_001/agent-obs", bytes.NewReader(body))
	req = injectUserID(req, "u_alice")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- GET /agent-obs ---

func TestAgentObs_GetConfig_notFound(t *testing.T) {
	mq := ownerMQ("mon_001", "u_alice")
	pool := &mockAgentObsPool{
		queryRow: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &errRow{err: errors.New("no rows")}
		},
	}
	h := newAgentObsRouter(mq, pool)
	req := httptest.NewRequest(http.MethodGet, "/mon_001/agent-obs", nil)
	req = injectUserID(req, "u_alice")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestAgentObs_GetConfig_success(t *testing.T) {
	mq := ownerMQ("mon_001", "u_alice")
	pool := &mockAgentObsPool{
		queryRow: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*(dest[0].(*string)) = "tool_api"
					*(dest[1].(*string)) = "https://tool.example.com"
					return nil
				},
			}
		},
	}
	h := newAgentObsRouter(mq, pool)
	req := httptest.NewRequest(http.MethodGet, "/mon_001/agent-obs", nil)
	req = injectUserID(req, "u_alice")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- PATCH /agent-obs ---

func TestAgentObs_PatchConfig_success(t *testing.T) {
	mq := ownerMQ("mon_001", "u_alice")
	pool := &mockAgentObsPool{
		exec: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
		queryRow: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*(dest[0].(*string)) = "llm_endpoint"
					*(dest[1].(*string)) = "https://new.example.com"
					return nil
				},
			}
		},
	}
	h := newAgentObsRouter(mq, pool)
	body, _ := json.Marshal(map[string]any{
		"endpoint_url": "https://new.example.com",
	})
	req := httptest.NewRequest(http.MethodPatch, "/mon_001/agent-obs", bytes.NewReader(body))
	req = injectUserID(req, "u_alice")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- DELETE /agent-obs ---

func TestAgentObs_DeleteConfig_success(t *testing.T) {
	mq := ownerMQ("mon_001", "u_alice")
	pool := &mockAgentObsPool{
		exec: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	h := newAgentObsRouter(mq, pool)
	req := httptest.NewRequest(http.MethodDelete, "/mon_001/agent-obs", nil)
	req = injectUserID(req, "u_alice")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- GET /agent-obs/checks ---

func TestAgentObs_ListChecks_success(t *testing.T) {
	mq := ownerMQ("mon_001", "u_alice")
	pool := &mockAgentObsPool{
		query: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &emptyRows{}, nil
		},
	}
	h := newAgentObsRouter(mq, pool)
	req := httptest.NewRequest(http.MethodGet, "/mon_001/agent-obs/checks", nil)
	req = injectUserID(req, "u_alice")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data AgentObsChecksListResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data.Items == nil {
		t.Error("expected non-nil items array")
	}
}

// mockRow is a pgx.Row backed by a scan function.
type mockRow struct {
	scanFn func(dest ...any) error
}

func (m *mockRow) Scan(dest ...any) error {
	if m.scanFn != nil {
		return m.scanFn(dest...)
	}
	return nil
}
