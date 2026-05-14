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

type mockOncallRows struct {
	rows [][]interface{}
	idx  int
	err  error
}

func (m *mockOncallRows) Next() bool {
	m.idx++
	return m.idx <= len(m.rows)
}

func (m *mockOncallRows) Scan(dest ...interface{}) error {
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

func (m *mockOncallRows) Close()                                        {}
func (m *mockOncallRows) Err() error                                    { return m.err }
func (m *mockOncallRows) CommandTag() pgconn.CommandTag                 { return pgconn.CommandTag{} }
func (m *mockOncallRows) FieldDescriptions() []pgconn.FieldDescription  { return nil }
func (m *mockOncallRows) Values() ([]any, error)                        { return nil, nil }
func (m *mockOncallRows) RawValues() [][]byte                           { return nil }
func (m *mockOncallRows) Conn() *pgx.Conn                               { return nil }

type mockOncallRow struct {
	values []interface{}
	err    error
}

func (m *mockOncallRow) Scan(dest ...interface{}) error {
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

type mockOncallPool struct {
	execResult    pgconn.CommandTag
	execErr       error
	queryRowQueue []*mockOncallRow
	queryRowIdx   int
	queryRowsData *mockOncallRows
	queryErr      error
}

func (m *mockOncallPool) Exec(_ context.Context, _ string, _ ...interface{}) (pgconn.CommandTag, error) {
	return m.execResult, m.execErr
}

func (m *mockOncallPool) QueryRow(_ context.Context, _ string, _ ...interface{}) pgx.Row {
	if m.queryRowIdx < len(m.queryRowQueue) {
		row := m.queryRowQueue[m.queryRowIdx]
		m.queryRowIdx++
		return row
	}
	return &mockOncallRow{err: pgx.ErrNoRows}
}

func (m *mockOncallPool) Query(_ context.Context, _ string, _ ...interface{}) (pgx.Rows, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	if m.queryRowsData != nil {
		return m.queryRowsData, nil
	}
	return &mockOncallRows{}, nil
}

func oncallWithUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDContextKey(), userID)
	return r.WithContext(ctx)
}

func oncallWithChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	return r.WithContext(ctx)
}

func TestOncallHandler_CreateSchedule_Unauthorized(t *testing.T) {
	pool := &mockOncallPool{}
	h := NewOncallHandler(pool)

	body := map[string]interface{}{
		"team_id":       "t_test",
		"name":          "工程师值班",
		"rotation_type": "weekly",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/oncall/schedules", bytes.NewReader(b))
	rr := httptest.NewRecorder()

	h.CreateSchedule(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestOncallHandler_CreateSchedule_Success(t *testing.T) {
	pool := &mockOncallPool{
		execResult: pgconn.NewCommandTag("INSERT 0 1"),
	}
	h := NewOncallHandler(pool)

	body := map[string]interface{}{
		"team_id":       "t_test",
		"name":          "工程师值班",
		"rotation_type": "weekly",
		"handoff_hour":  9,
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/oncall/schedules", bytes.NewReader(b))
	req = oncallWithUserID(req, "u_creator")
	rr := httptest.NewRecorder()

	h.CreateSchedule(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "工程师值班", data["name"])
	assert.Equal(t, "t_test", data["team_id"])
	assert.Equal(t, "weekly", data["rotation_type"])
}

func TestOncallHandler_GetCurrentOnCall_NoParticipants(t *testing.T) {
	now := time.Now().UTC()
	pool := &mockOncallPool{
		queryRowQueue: []*mockOncallRow{
			{err: pgx.ErrNoRows},
			{values: []interface{}{"sch_test", "weekly", 9}},
		},
		queryRowsData: &mockOncallRows{rows: nil},
	}
	h := NewOncallHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/oncall/schedules/sch_test/current", nil)
	req = oncallWithUserID(req, "u_test")
	req = oncallWithChiParam(req, "id", "sch_test")
	_ = now
	rr := httptest.NewRecorder()

	h.GetCurrentOnCall(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "", data["user_id"])
}

func TestOncallHandler_GetCurrentOnCall_WithParticipants(t *testing.T) {
	pool := &mockOncallPool{
		queryRowQueue: []*mockOncallRow{
			{err: pgx.ErrNoRows},
			{values: []interface{}{"sch_test", "weekly", 9}},
		},
		queryRowsData: &mockOncallRows{rows: [][]interface{}{
			{"par_1", "sch_test", "u_alice", 0},
		}},
	}
	h := NewOncallHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/oncall/schedules/sch_test/current", nil)
	req = oncallWithUserID(req, "u_test")
	req = oncallWithChiParam(req, "id", "sch_test")
	rr := httptest.NewRecorder()

	h.GetCurrentOnCall(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "u_alice", data["user_id"])
}

func TestOncallHandler_CreateSchedule_InvalidRotationType(t *testing.T) {
	pool := &mockOncallPool{}
	h := NewOncallHandler(pool)

	body := map[string]interface{}{
		"team_id":       "t_test",
		"name":          "Bad Schedule",
		"rotation_type": "monthly",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/oncall/schedules", bytes.NewReader(b))
	req = oncallWithUserID(req, "u_creator")
	rr := httptest.NewRecorder()

	h.CreateSchedule(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
