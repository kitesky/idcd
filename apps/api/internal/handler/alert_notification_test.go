package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// multiQueryAlertPool allows different QueryRow responses for successive calls.
type multiQueryAlertPool struct {
	queryRowCalls int
	rowValues     [][]interface{}
	rowErrors     []error
	queryRows     *mockAlertRows
	queryErr      error
}

func (m *multiQueryAlertPool) Exec(_ context.Context, _ string, _ ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (m *multiQueryAlertPool) QueryRow(_ context.Context, _ string, _ ...interface{}) pgx.Row {
	idx := m.queryRowCalls
	m.queryRowCalls++
	if idx < len(m.rowErrors) && m.rowErrors[idx] != nil {
		return &mockAlertRow{err: m.rowErrors[idx]}
	}
	if idx < len(m.rowValues) {
		return &mockAlertRow{values: m.rowValues[idx]}
	}
	return &mockAlertRow{err: pgx.ErrNoRows}
}

func (m *multiQueryAlertPool) Query(_ context.Context, _ string, _ ...interface{}) (pgx.Rows, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	if m.queryRows != nil {
		return m.queryRows, nil
	}
	return &mockAlertRows{}, nil
}

func TestAlertNotificationHandler_List_Unauthenticated(t *testing.T) {
	pool := &mockAlertPool{}
	h := NewAlertNotificationHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/alert-channels/ch-001/notifications", nil)
	req = alertWithChiParam(req, "id", "ch-001")
	rr := httptest.NewRecorder()

	h.List(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAlertNotificationHandler_List_ChannelNotFound(t *testing.T) {
	pool := &mockAlertPool{
		queryRowVal: &mockAlertRow{err: pgx.ErrNoRows},
	}
	h := NewAlertNotificationHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/alert-channels/ch-missing/notifications", nil)
	req = alertWithUserID(req, "usr_test")
	req = alertWithChiParam(req, "id", "ch-missing")
	rr := httptest.NewRecorder()

	h.List(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestAlertNotificationHandler_List_OtherUserChannel(t *testing.T) {
	pool := &mockAlertPool{
		queryRowVal: &mockAlertRow{values: []interface{}{"usr_other"}},
	}
	h := NewAlertNotificationHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/alert-channels/ch-001/notifications", nil)
	req = alertWithUserID(req, "usr_current")
	req = alertWithChiParam(req, "id", "ch-001")
	rr := httptest.NewRecorder()

	h.List(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAlertNotificationHandler_List_Success_Empty(t *testing.T) {
	pool := &multiQueryAlertPool{
		rowValues: [][]interface{}{
			{"usr_test"},
			{int64(0)},
		},
		queryRows: &mockAlertRows{},
	}
	h := NewAlertNotificationHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/alert-channels/ch-001/notifications", nil)
	req = alertWithUserID(req, "usr_test")
	req = alertWithChiParam(req, "id", "ch-001")
	rr := httptest.NewRecorder()

	h.List(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	data := resp["data"].(map[string]interface{})
	notifications := data["notifications"].([]interface{})
	assert.Len(t, notifications, 0)
	assert.Equal(t, float64(0), data["total"])
}

func TestAlertNotificationHandler_List_LimitOffset(t *testing.T) {
	pool := &multiQueryAlertPool{
		rowValues: [][]interface{}{
			{"usr_test"},
			{int64(42)},
		},
		queryRows: &mockAlertRows{},
	}
	h := NewAlertNotificationHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/alert-channels/ch-001/notifications?limit=10&offset=20", nil)
	req = alertWithUserID(req, "usr_test")
	req = alertWithChiParam(req, "id", "ch-001")
	rr := httptest.NewRecorder()

	h.List(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(42), data["total"])
}
