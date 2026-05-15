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

type mockPMRow struct {
	values []any
	err    error
}

func (m *mockPMRow) Scan(dest ...any) error {
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

type mockPMRows struct {
	rows [][]any
	idx  int
	err  error
}

func (m *mockPMRows) Next() bool {
	m.idx++
	return m.idx <= len(m.rows)
}
func (m *mockPMRows) Scan(dest ...any) error {
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
func (m *mockPMRows) Close()                                       {}
func (m *mockPMRows) Err() error                                   { return m.err }
func (m *mockPMRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (m *mockPMRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *mockPMRows) Values() ([]any, error)                       { return nil, nil }
func (m *mockPMRows) RawValues() [][]byte                          { return nil }
func (m *mockPMRows) Conn() *pgx.Conn                              { return nil }

type mockPMPool struct {
	execResult    pgconn.CommandTag
	execErr       error
	queryRowQueue []*mockPMRow
	queryRowIdx   int
	queryRowsData *mockPMRows
	queryErr      error
}

func (m *mockPMPool) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return m.execResult, m.execErr
}

func (m *mockPMPool) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if m.queryRowIdx < len(m.queryRowQueue) {
		row := m.queryRowQueue[m.queryRowIdx]
		m.queryRowIdx++
		return row
	}
	return &mockPMRow{err: pgx.ErrNoRows}
}

func (m *mockPMPool) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	if m.queryRowsData != nil {
		return m.queryRowsData, nil
	}
	return &mockPMRows{}, nil
}

func pmWithUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDContextKey(), userID)
	return r.WithContext(ctx)
}

func pmWithEventID(r *http.Request, eventID string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("event_id", eventID)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	return r.WithContext(ctx)
}

func TestPostmortemHandler_Draft_Unauthenticated(t *testing.T) {
	h := NewPostmortemHandler(&mockPMPool{})
	req := httptest.NewRequest(http.MethodPost, "/v1/incidents/ev_1/draft", nil)
	req = pmWithEventID(req, "ev_1")
	rr := httptest.NewRecorder()
	h.Draft(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestPostmortemHandler_Draft_Success(t *testing.T) {
	startedAt := time.Now().Add(-45 * time.Minute)
	resolvedAt := time.Now()

	pool := &mockPMPool{
		execResult: pgconn.NewCommandTag("INSERT 0 1"),
		queryRowQueue: []*mockPMRow{
			{values: []any{"mon_1", startedAt, &resolvedAt, []byte("{}")}},
			{values: []any{"API Gateway", "http"}},
		},
	}
	h := NewPostmortemHandler(pool)

	req := httptest.NewRequest(http.MethodPost, "/v1/incidents/ev_1/draft", nil)
	req = pmWithUserID(req, "usr_test")
	req = pmWithEventID(req, "ev_1")
	rr := httptest.NewRecorder()

	h.Draft(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	data := resp["data"].(map[string]any)
	assert.NotEmpty(t, data["title"])
	assert.NotEmpty(t, data["severity"])
	assert.Equal(t, "high", data["severity"])
}

func TestPostmortemHandler_Get_NotFound(t *testing.T) {
	h := NewPostmortemHandler(&mockPMPool{})
	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/ev_1/postmortem", nil)
	req = pmWithUserID(req, "usr_test")
	req = pmWithEventID(req, "ev_1")
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestPostmortemHandler_Update_Success(t *testing.T) {
	now := time.Now()
	timelineJSON := `[{"time":"2024-01-01T00:00:00Z","event":"故障开始"}]`
	actionItemsJSON := `[{"item":"检查服务器负载","owner":"待指定","due_date":"2026-05-21"}]`

	pool := &mockPMPool{
		execResult: pgconn.NewCommandTag("UPDATE 1"),
		queryRowQueue: []*mockPMRow{
			{values: []any{
				"pm_abc", "ev_1", "mon_1", "usr_test",
				"Old Title", "draft", "low", "old impact",
				[]byte(timelineJSON), "root", "resolution",
				[]byte(actionItemsJSON), now, now,
			}},
		},
	}
	h := NewPostmortemHandler(pool)

	newTitle := "New Title"
	body, _ := json.Marshal(map[string]any{"title": newTitle})
	req := httptest.NewRequest(http.MethodPatch, "/v1/incidents/ev_1/postmortem", bytes.NewReader(body))
	req = pmWithUserID(req, "usr_test")
	req = pmWithEventID(req, "ev_1")
	rr := httptest.NewRecorder()

	h.Update(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, newTitle, data["title"])
}

func TestPostmortemHandler_List_Unauthenticated(t *testing.T) {
	h := NewPostmortemHandler(&mockPMPool{})
	req := httptest.NewRequest(http.MethodGet, "/v1/incidents", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestPostmortemHandler_List_Empty(t *testing.T) {
	h := NewPostmortemHandler(&mockPMPool{})
	req := httptest.NewRequest(http.MethodGet, "/v1/incidents", nil)
	req = pmWithUserID(req, "usr_test")
	rr := httptest.NewRecorder()
	h.List(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	data := resp["data"].(map[string]any)
	items := data["items"].([]any)
	assert.Len(t, items, 0)
}
