package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Mock types
// ---------------------------------------------------------------------------

type mockStatusPageRows struct {
	rows [][]any
	idx  int
	err  error
}

func (m *mockStatusPageRows) Next() bool {
	m.idx++
	return m.idx <= len(m.rows)
}

func (m *mockStatusPageRows) Scan(dest ...any) error {
	if m.err != nil {
		return m.err
	}
	row := m.rows[m.idx-1]
	for i, d := range dest {
		if i >= len(row) {
			break
		}
		copyValue(d, row[i])
	}
	return nil
}

func (m *mockStatusPageRows) Close()                                       {}
func (m *mockStatusPageRows) Err() error                                   { return m.err }
func (m *mockStatusPageRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (m *mockStatusPageRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *mockStatusPageRows) Values() ([]any, error)                       { return nil, nil }
func (m *mockStatusPageRows) RawValues() [][]byte                          { return nil }
func (m *mockStatusPageRows) Conn() *pgx.Conn                              { return nil }

type mockStatusPageRow struct {
	values []any
	err    error
}

func (m *mockStatusPageRow) Scan(dest ...any) error {
	if m.err != nil {
		return m.err
	}
	for i, d := range dest {
		if i >= len(m.values) {
			break
		}
		copyValue(d, m.values[i])
	}
	return nil
}

type mockStatusPagePool struct {
	execResult    pgconn.CommandTag
	execErr       error
	queryRowQueue []*mockStatusPageRow
	queryRowIdx   int
	queryRowsData *mockStatusPageRows
	queryErr      error
}

func (m *mockStatusPagePool) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return m.execResult, m.execErr
}

func (m *mockStatusPagePool) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if m.queryRowIdx < len(m.queryRowQueue) {
		row := m.queryRowQueue[m.queryRowIdx]
		m.queryRowIdx++
		return row
	}
	return &mockStatusPageRow{err: pgx.ErrNoRows}
}

func (m *mockStatusPagePool) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	if m.queryRowsData != nil {
		return m.queryRowsData, nil
	}
	return &mockStatusPageRows{}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func statusPageWithUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDContextKey(), userID)
	return r.WithContext(ctx)
}

func statusPageWithChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	return r.WithContext(ctx)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestStatusPageUserHandler_List_Success(t *testing.T) {
	now := time.Now().UTC()
	pool := &mockStatusPagePool{
		queryRowsData: &mockStatusPageRows{
			rows: [][]any{
				{"sp_001", "First Page", "first-page", now},
				{"sp_002", "Second Page", "second-page", now},
			},
		},
	}
	h := NewStatusPageUserHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/status-pages", nil)
	req = statusPageWithUserID(req, "u_test")
	rr := httptest.NewRecorder()

	h.List(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))

	data, ok := resp["data"].(map[string]any)
	require.True(t, ok, "expected data object")

	items, ok := data["status_pages"].([]any)
	require.True(t, ok, "expected status_pages array")
	assert.Len(t, items, 2)

	first := items[0].(map[string]any)
	assert.Equal(t, "sp_001", first["id"])
	assert.Equal(t, "First Page", first["name"])
	assert.Equal(t, "first-page", first["slug"])
}

func TestStatusPageUserHandler_List_Unauthorized(t *testing.T) {
	pool := &mockStatusPagePool{}
	h := NewStatusPageUserHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/status-pages", nil)
	// No userID set in context
	rr := httptest.NewRecorder()

	h.List(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestStatusPageUserHandler_Create_Success(t *testing.T) {
	now := time.Now().UTC()
	pool := &mockStatusPagePool{
		queryRowQueue: []*mockStatusPageRow{
			{values: []any{now}},
		},
	}
	h := NewStatusPageUserHandler(pool)

	body := map[string]any{
		"name": "Test Page",
		"slug": "test-page",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/status-pages", bytes.NewReader(b))
	req = statusPageWithUserID(req, "u_creator")
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))

	data, ok := resp["data"].(map[string]any)
	require.True(t, ok, "expected data object")

	sp, ok := data["status_page"].(map[string]any)
	require.True(t, ok, "expected status_page object")

	id, ok := sp["id"].(string)
	require.True(t, ok, "expected id string")
	assert.NotEmpty(t, id)
	assert.Equal(t, "Test Page", sp["name"])
	assert.Equal(t, "test-page", sp["slug"])
}

func TestStatusPageUserHandler_Create_Unauthorized(t *testing.T) {
	pool := &mockStatusPagePool{}
	h := NewStatusPageUserHandler(pool)

	body := map[string]any{
		"name": "Test Page",
		"slug": "test-page",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/status-pages", bytes.NewReader(b))
	// No userID in context
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestStatusPageUserHandler_Delete_Success(t *testing.T) {
	pool := &mockStatusPagePool{
		execResult: pgconn.NewCommandTag("DELETE 1"),
	}
	h := NewStatusPageUserHandler(pool)

	req := httptest.NewRequest(http.MethodDelete, "/v1/status-pages/sp_001", nil)
	req = statusPageWithUserID(req, "u_owner")
	req = statusPageWithChiParam(req, "id", "sp_001")
	rr := httptest.NewRecorder()

	h.Delete(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestStatusPageUserHandler_Delete_NotFound(t *testing.T) {
	pool := &mockStatusPagePool{
		execResult: pgconn.NewCommandTag("DELETE 0"),
	}
	h := NewStatusPageUserHandler(pool)

	req := httptest.NewRequest(http.MethodDelete, "/v1/status-pages/sp_missing", nil)
	req = statusPageWithUserID(req, "u_owner")
	req = statusPageWithChiParam(req, "id", "sp_missing")
	rr := httptest.NewRecorder()

	h.Delete(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestStatusPageUserHandler_Get_Success(t *testing.T) {
	now := time.Now().UTC()
	pool := &mockStatusPagePool{
		queryRowQueue: []*mockStatusPageRow{
			{values: []any{"sp_001", "My Page", "my-page", now}},
		},
	}
	h := NewStatusPageUserHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/status-pages/sp_001", nil)
	req = statusPageWithUserID(req, "u_owner")
	req = statusPageWithChiParam(req, "id", "sp_001")
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))

	data, ok := resp["data"].(map[string]any)
	require.True(t, ok, "expected data object")

	sp, ok := data["status_page"].(map[string]any)
	require.True(t, ok, "expected status_page object")

	assert.Equal(t, "sp_001", sp["id"])
	assert.Equal(t, "My Page", sp["name"])
	assert.Equal(t, "my-page", sp["slug"])
	assert.Equal(t, true, sp["is_public"])
	assert.Equal(t, "operational", sp["overall_status"])
}

func TestStatusPageUserHandler_Get_NotFound(t *testing.T) {
	pool := &mockStatusPagePool{
		queryRowQueue: []*mockStatusPageRow{
			{err: pgx.ErrNoRows},
		},
	}
	h := NewStatusPageUserHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/status-pages/sp_missing", nil)
	req = statusPageWithUserID(req, "u_owner")
	req = statusPageWithChiParam(req, "id", "sp_missing")
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}
