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

// ─────────────────────────────────────────────
// Mock implementations
// ─────────────────────────────────────────────

// mockAlertRow simulates a pgx.Row with configurable scan values.
type mockAlertRow struct {
	values []any
	err    error
}

func (m *mockAlertRow) Scan(dest ...any) error {
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

// mockAlertRows simulates pgx.Rows with a slice of row data.
type mockAlertRows struct {
	rows [][]any
	idx  int
	err  error
}

func (m *mockAlertRows) Next() bool {
	m.idx++
	return m.idx <= len(m.rows)
}

func (m *mockAlertRows) Scan(dest ...any) error {
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

func (m *mockAlertRows) Close()                               {}
func (m *mockAlertRows) Err() error                           { return m.err }
func (m *mockAlertRows) CommandTag() pgconn.CommandTag        { return pgconn.CommandTag{} }
func (m *mockAlertRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *mockAlertRows) Values() ([]any, error)               { return nil, nil }
func (m *mockAlertRows) RawValues() [][]byte                  { return nil }
func (m *mockAlertRows) Conn() *pgx.Conn                      { return nil }

// mockAlertPool simulates the AlertPool interface.
type mockAlertPool struct {
	execResult  pgconn.CommandTag
	execErr     error
	queryRowVal *mockAlertRow
	queryRows   *mockAlertRows
	queryErr    error
}

func (m *mockAlertPool) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return m.execResult, m.execErr
}

func (m *mockAlertPool) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if m.queryRowVal != nil {
		return m.queryRowVal
	}
	// Return pgx.ErrNoRows so callers can distinguish "not found" from real DB errors.
	return &mockAlertRow{err: pgx.ErrNoRows}
}

func (m *mockAlertPool) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	if m.queryRows != nil {
		return m.queryRows, nil
	}
	return &mockAlertRows{}, nil
}

// copyValue is a minimal helper to assign scalar values to dest pointers.
func copyValue(dest, src any) {
	switch d := dest.(type) {
	case *string:
		if v, ok := src.(string); ok {
			*d = v
		}
	case **string:
		if src == nil {
			*d = nil
		} else if v, ok := src.(string); ok {
			*d = &v
		}
	case *bool:
		if v, ok := src.(bool); ok {
			*d = v
		}
	case *int:
		if v, ok := src.(int); ok {
			*d = v
		}
	case *int64:
		if v, ok := src.(int64); ok {
			*d = v
		}
	case *time.Time:
		if v, ok := src.(time.Time); ok {
			*d = v
		}
	case **time.Time:
		if src == nil {
			*d = nil
		} else if v, ok := src.(time.Time); ok {
			*d = &v
		}
	case *[]byte:
		if v, ok := src.([]byte); ok {
			*d = v
		}
	}
}

// alertWithUserID injects a user ID into the request context.
func alertWithUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDContextKey(), userID)
	return r.WithContext(ctx)
}

// alertWithChiParam injects a chi URL param into the request context.
func alertWithChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	return r.WithContext(ctx)
}

// ─────────────────────────────────────────────
// Alert Channel tests
// ─────────────────────────────────────────────

func TestAlertHandler_CreateChannel_Success(t *testing.T) {
	pool := &mockAlertPool{execResult: pgconn.NewCommandTag("INSERT 0 1")}
	h := NewAlertHandler(pool)

	body := map[string]any{
		"name":   "My Webhook",
		"type":   "webhook",
		"config": map[string]string{"url": "https://example.com/hook"},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/alert-channels", bytes.NewReader(b))
	req = alertWithUserID(req, "usr_test")
	rr := httptest.NewRecorder()

	h.CreateChannel(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, "My Webhook", data["name"])
	assert.Equal(t, "webhook", data["type"])
	assert.Equal(t, false, data["verified"])
}

func TestAlertHandler_CreateChannel_Unauthenticated(t *testing.T) {
	pool := &mockAlertPool{}
	h := NewAlertHandler(pool)

	req := httptest.NewRequest(http.MethodPost, "/v1/alert-channels", bytes.NewReader([]byte(`{}`)))
	rr := httptest.NewRecorder()

	h.CreateChannel(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAlertHandler_CreateChannel_InvalidType(t *testing.T) {
	pool := &mockAlertPool{}
	h := NewAlertHandler(pool)

	body := `{"name":"Test","type":"unknown","config":{}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/alert-channels", bytes.NewReader([]byte(body)))
	req = alertWithUserID(req, "usr_test")
	rr := httptest.NewRecorder()

	h.CreateChannel(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAlertHandler_CreateChannel_MissingName(t *testing.T) {
	pool := &mockAlertPool{}
	h := NewAlertHandler(pool)

	body := `{"name":"","type":"webhook","config":{}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/alert-channels", bytes.NewReader([]byte(body)))
	req = alertWithUserID(req, "usr_test")
	rr := httptest.NewRecorder()

	h.CreateChannel(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAlertHandler_ListChannels_Empty(t *testing.T) {
	pool := &mockAlertPool{queryRows: &mockAlertRows{}}
	h := NewAlertHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/alert-channels", nil)
	req = alertWithUserID(req, "usr_test")
	rr := httptest.NewRecorder()

	h.ListChannels(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	data := resp["data"].(map[string]any)
	items := data["items"].([]any)
	assert.Len(t, items, 0)
}

func TestAlertHandler_ListChannels_Unauthenticated(t *testing.T) {
	pool := &mockAlertPool{}
	h := NewAlertHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/alert-channels", nil)
	rr := httptest.NewRecorder()

	h.ListChannels(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAlertHandler_DeleteChannel_NotFound(t *testing.T) {
	pool := &mockAlertPool{execResult: pgconn.NewCommandTag("DELETE 0")}
	h := NewAlertHandler(pool)

	req := httptest.NewRequest(http.MethodDelete, "/v1/alert-channels/ch_xxx", nil)
	req = alertWithUserID(req, "usr_test")
	req = alertWithChiParam(req, "id", "ch_xxx")
	rr := httptest.NewRecorder()

	h.DeleteChannel(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestAlertHandler_DeleteChannel_Success(t *testing.T) {
	pool := &mockAlertPool{execResult: pgconn.NewCommandTag("DELETE 1")}
	h := NewAlertHandler(pool)

	req := httptest.NewRequest(http.MethodDelete, "/v1/alert-channels/ch_test", nil)
	req = alertWithUserID(req, "usr_test")
	req = alertWithChiParam(req, "id", "ch_test")
	rr := httptest.NewRecorder()

	h.DeleteChannel(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestAlertHandler_TestChannel_Success(t *testing.T) {
	pool := &mockAlertPool{
		queryRowVal: &mockAlertRow{
			values: []any{"webhook", []byte(`{"url":"https://example.com"}`)},
		},
	}
	h := NewAlertHandler(pool)

	req := httptest.NewRequest(http.MethodPost, "/v1/alert-channels/ch_test/test", nil)
	req = alertWithUserID(req, "usr_test")
	req = alertWithChiParam(req, "id", "ch_test")
	rr := httptest.NewRecorder()

	h.TestChannel(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAlertHandler_TestChannel_NotFound(t *testing.T) {
	pool := &mockAlertPool{}
	h := NewAlertHandler(pool)

	req := httptest.NewRequest(http.MethodPost, "/v1/alert-channels/ch_nope/test", nil)
	req = alertWithUserID(req, "usr_test")
	req = alertWithChiParam(req, "id", "ch_nope")
	rr := httptest.NewRecorder()

	h.TestChannel(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ─────────────────────────────────────────────
// Alert Policy tests
// ─────────────────────────────────────────────

func TestAlertHandler_CreatePolicy_Success(t *testing.T) {
	// QueryRow now serves the ownership EXISTS check — return true.
	pool := &mockAlertPool{
		execResult:  pgconn.NewCommandTag("INSERT 0 1"),
		queryRowVal: &mockAlertRow{values: []any{true}},
	}
	h := NewAlertHandler(pool)

	body := map[string]any{
		"name":        "My Policy",
		"monitor_id":  "mon_test123",
		"channel_ids": []string{"ch_abc"},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/alert-policies", bytes.NewReader(b))
	req = alertWithUserID(req, "usr_test")
	rr := httptest.NewRecorder()

	h.CreatePolicy(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, "My Policy", data["name"])
	assert.Equal(t, "mon_test123", data["monitor_id"])
}

func TestAlertHandler_CreatePolicy_MissingMonitorID(t *testing.T) {
	pool := &mockAlertPool{}
	h := NewAlertHandler(pool)

	body := `{"name":"Test"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/alert-policies", bytes.NewReader([]byte(body)))
	req = alertWithUserID(req, "usr_test")
	rr := httptest.NewRecorder()

	h.CreatePolicy(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAlertHandler_CreatePolicy_MissingName(t *testing.T) {
	pool := &mockAlertPool{}
	h := NewAlertHandler(pool)

	body := `{"monitor_id":"mon_test"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/alert-policies", bytes.NewReader([]byte(body)))
	req = alertWithUserID(req, "usr_test")
	rr := httptest.NewRecorder()

	h.CreatePolicy(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAlertHandler_ListPolicies_Empty(t *testing.T) {
	pool := &mockAlertPool{queryRows: &mockAlertRows{}}
	h := NewAlertHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/alert-policies", nil)
	req = alertWithUserID(req, "usr_test")
	rr := httptest.NewRecorder()

	h.ListPolicies(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	data := resp["data"].(map[string]any)
	items := data["items"].([]any)
	assert.Len(t, items, 0)
}

func TestAlertHandler_ListPolicies_WithMonitorFilter(t *testing.T) {
	pool := &mockAlertPool{queryRows: &mockAlertRows{}}
	h := NewAlertHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/alert-policies?monitor_id=mon_test", nil)
	req = alertWithUserID(req, "usr_test")
	rr := httptest.NewRecorder()

	h.ListPolicies(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAlertHandler_UpdatePolicy_NotFound(t *testing.T) {
	pool := &mockAlertPool{}
	h := NewAlertHandler(pool)

	body := `{"name":"New Name"}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/alert-policies/pol_nope", bytes.NewReader([]byte(body)))
	req = alertWithUserID(req, "usr_test")
	req = alertWithChiParam(req, "id", "pol_nope")
	rr := httptest.NewRecorder()

	h.UpdatePolicy(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestAlertHandler_DeletePolicy_Success(t *testing.T) {
	pool := &mockAlertPool{execResult: pgconn.NewCommandTag("DELETE 1")}
	h := NewAlertHandler(pool)

	req := httptest.NewRequest(http.MethodDelete, "/v1/alert-policies/pol_test", nil)
	req = alertWithUserID(req, "usr_test")
	req = alertWithChiParam(req, "id", "pol_test")
	rr := httptest.NewRecorder()

	h.DeletePolicy(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestAlertHandler_DeletePolicy_NotFound(t *testing.T) {
	pool := &mockAlertPool{execResult: pgconn.NewCommandTag("DELETE 0")}
	h := NewAlertHandler(pool)

	req := httptest.NewRequest(http.MethodDelete, "/v1/alert-policies/pol_nope", nil)
	req = alertWithUserID(req, "usr_test")
	req = alertWithChiParam(req, "id", "pol_nope")
	rr := httptest.NewRecorder()

	h.DeletePolicy(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ─────────────────────────────────────────────
// Alert Event tests
// ─────────────────────────────────────────────

func TestAlertHandler_ListEvents_Empty(t *testing.T) {
	pool := &mockAlertPool{queryRows: &mockAlertRows{}}
	h := NewAlertHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/alert-events", nil)
	req = alertWithUserID(req, "usr_test")
	rr := httptest.NewRecorder()

	h.ListEvents(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	data := resp["data"].(map[string]any)
	items := data["items"].([]any)
	assert.Len(t, items, 0)
}

func TestAlertHandler_ListEvents_WithFilters(t *testing.T) {
	pool := &mockAlertPool{queryRows: &mockAlertRows{}}
	h := NewAlertHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/alert-events?monitor_id=mon_x&status=firing&limit=10", nil)
	req = alertWithUserID(req, "usr_test")
	rr := httptest.NewRecorder()

	h.ListEvents(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAlertHandler_ListEvents_Unauthenticated(t *testing.T) {
	pool := &mockAlertPool{}
	h := NewAlertHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/alert-events", nil)
	rr := httptest.NewRecorder()

	h.ListEvents(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAlertHandler_AcknowledgeEvent_NotFound(t *testing.T) {
	pool := &mockAlertPool{execResult: pgconn.NewCommandTag("UPDATE 0")}
	h := NewAlertHandler(pool)

	req := httptest.NewRequest(http.MethodPost, "/v1/alert-events/evt_nope/ack", nil)
	req = alertWithUserID(req, "usr_test")
	req = alertWithChiParam(req, "id", "evt_nope")
	rr := httptest.NewRecorder()

	h.AcknowledgeEvent(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestAlertHandler_AcknowledgeEvent_Success(t *testing.T) {
	pool := &mockAlertPool{execResult: pgconn.NewCommandTag("UPDATE 1")}
	h := NewAlertHandler(pool)

	req := httptest.NewRequest(http.MethodPost, "/v1/alert-events/evt_test/ack", nil)
	req = alertWithUserID(req, "usr_test")
	req = alertWithChiParam(req, "id", "evt_test")
	rr := httptest.NewRecorder()

	h.AcknowledgeEvent(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, "event acknowledged", data["message"])
}

func TestAlertHandler_AcknowledgeEvent_Unauthenticated(t *testing.T) {
	pool := &mockAlertPool{}
	h := NewAlertHandler(pool)

	req := httptest.NewRequest(http.MethodPost, "/v1/alert-events/evt_test/ack", nil)
	req = alertWithChiParam(req, "id", "evt_test")
	rr := httptest.NewRecorder()

	h.AcknowledgeEvent(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// ─────────────────────────────────────────────
// Helper function tests
