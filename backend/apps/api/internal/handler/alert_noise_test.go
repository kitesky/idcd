package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

func noiseWithUserID(r *http.Request, userID string) *http.Request {
	return alertWithUserID(r, userID)
}

func noiseWithChiParam(r *http.Request, key, value string) *http.Request {
	return alertWithChiParam(r, key, value)
}

// ─────────────────────────────────────────────
// POST /v1/alert-silences
// ─────────────────────────────────────────────

func TestAlertNoiseHandler_CreateSilence_Unauthorized(t *testing.T) {
	pool := &mockAlertPool{}
	h := NewAlertNoiseHandler(pool)

	body := map[string]any{
		"reason":     "maintenance",
		"starts_at":  time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		"ends_at":    time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/alert-silences", bytes.NewReader(b))
	w := httptest.NewRecorder()

	h.CreateSilence(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAlertNoiseHandler_CreateSilence_EndsBeforeStarts(t *testing.T) {
	pool := &mockAlertPool{}
	h := NewAlertNoiseHandler(pool)

	now := time.Now().UTC()
	body := map[string]any{
		"reason":    "maintenance",
		"starts_at": now.Add(2 * time.Hour).Format(time.RFC3339),
		"ends_at":   now.Add(time.Hour).Format(time.RFC3339),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/alert-silences", bytes.NewReader(b))
	req = noiseWithUserID(req, "u_test")
	w := httptest.NewRecorder()

	h.CreateSilence(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAlertNoiseHandler_CreateSilence_Success(t *testing.T) {
	pool := &mockAlertPool{execResult: pgconn.NewCommandTag("INSERT 0 1")}
	h := NewAlertNoiseHandler(pool)

	now := time.Now().UTC()
	body := map[string]any{
		"reason":    "scheduled maintenance",
		"starts_at": now.Add(time.Hour).Format(time.RFC3339),
		"ends_at":   now.Add(2 * time.Hour).Format(time.RFC3339),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/alert-silences", bytes.NewReader(b))
	req = noiseWithUserID(req, "u_test")
	w := httptest.NewRecorder()

	h.CreateSilence(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	data := resp["data"].(map[string]any)
	assert.NotEmpty(t, data["id"])
	assert.Equal(t, "u_test", data["user_id"])
	assert.Equal(t, "scheduled maintenance", data["reason"])
	assert.Equal(t, "upcoming", data["status"])
}

// ─────────────────────────────────────────────
// GET /v1/alert-silences
// ─────────────────────────────────────────────

func TestAlertNoiseHandler_ListSilences_OK(t *testing.T) {
	now := time.Now().UTC()
	startsAt := now.Add(-10 * time.Minute)
	endsAt := now.Add(50 * time.Minute)

	pool := &mockAlertPool{
		queryRows: &mockAlertRows{
			rows: [][]any{
				{
					"sil_abc", "u_test", (*string)(nil),
					"routine check", startsAt, endsAt, now,
				},
			},
		},
	}
	h := NewAlertNoiseHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/alert-silences", nil)
	req = noiseWithUserID(req, "u_test")
	w := httptest.NewRecorder()

	h.ListSilences(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	data := resp["data"].(map[string]any)
	items := data["items"].([]any)
	assert.Len(t, items, 1)
}

// ─────────────────────────────────────────────
// DELETE /v1/alert-silences/{id}
// ─────────────────────────────────────────────

func TestAlertNoiseHandler_DeleteSilence_OK(t *testing.T) {
	pool := &mockAlertPool{execResult: pgconn.NewCommandTag("UPDATE 1")}
	h := NewAlertNoiseHandler(pool)

	req := httptest.NewRequest(http.MethodDelete, "/v1/alert-silences/sil_abc", nil)
	req = noiseWithUserID(req, "u_test")
	req = noiseWithChiParam(req, "id", "sil_abc")
	w := httptest.NewRecorder()

	h.DeleteSilence(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestAlertNoiseHandler_DeleteSilence_NotFound(t *testing.T) {
	pool := &mockAlertPool{execResult: pgconn.NewCommandTag("UPDATE 0")}
	h := NewAlertNoiseHandler(pool)

	req := httptest.NewRequest(http.MethodDelete, "/v1/alert-silences/sil_missing", nil)
	req = noiseWithUserID(req, "u_test")
	req = noiseWithChiParam(req, "id", "sil_missing")
	w := httptest.NewRecorder()

	h.DeleteSilence(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ─────────────────────────────────────────────
// GET /v1/reports/alert-noise
// ─────────────────────────────────────────────

func TestAlertNoiseHandler_NoiseReport_OK(t *testing.T) {
	pool := &mockAlertPool{
		queryRows: &mockAlertRows{rows: [][]any{}},
	}
	h := NewAlertNoiseHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/reports/alert-noise?days=7", nil)
	req = noiseWithUserID(req, "u_test")
	w := httptest.NewRecorder()

	h.NoiseReport(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var outer map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&outer))
	data := outer["data"].(map[string]any)
	period := data["period"].(map[string]any)
	assert.NotEmpty(t, period["from"])
	assert.NotEmpty(t, period["to"])
}

func TestAlertNoiseHandler_NoiseReport_Unauthorized(t *testing.T) {
	pool := &mockAlertPool{}
	h := NewAlertNoiseHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/reports/alert-noise", nil)
	w := httptest.NewRecorder()

	h.NoiseReport(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ─────────────────────────────────────────────
// POST /v1/alert-groups
// ─────────────────────────────────────────────

func TestAlertNoiseHandler_CreateGroup_Success(t *testing.T) {
	pool := &mockAlertPool{execResult: pgconn.NewCommandTag("INSERT 0 1")}
	h := NewAlertNoiseHandler(pool)

	body := map[string]any{
		"name":        "Production APIs",
		"group_by":    "monitor_prefix",
		"group_value": "api-",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/alert-groups", bytes.NewReader(b))
	req = noiseWithUserID(req, "u_test")
	w := httptest.NewRecorder()

	h.CreateGroup(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var outer map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&outer))
	data := outer["data"].(map[string]any)
	assert.NotEmpty(t, data["id"])
	assert.Equal(t, float64(60), data["wait_seconds"])
}

// ─────────────────────────────────────────────
// GET /v1/alert-groups
// ─────────────────────────────────────────────

func TestAlertNoiseHandler_ListGroups_OK(t *testing.T) {
	now := time.Now().UTC()
	pool := &mockAlertPool{
		queryRows: &mockAlertRows{
			rows: [][]any{
				{"agrp_1", "u_test", "Prod", "monitor_prefix", "api-", 60, now},
			},
		},
	}
	h := NewAlertNoiseHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/alert-groups", nil)
	req = noiseWithUserID(req, "u_test")
	w := httptest.NewRecorder()

	h.ListGroups(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	data := resp["data"].(map[string]any)
	items := data["items"].([]any)
	assert.Len(t, items, 1)
}

// ─────────────────────────────────────────────
// mockAlertPool multi-query support for NoiseReport
// ─────────────────────────────────────────────

// multiQueryPool returns different rows for the first and second Query calls.
type multiQueryPool struct {
	mockAlertPool
	calls     int
	rowSets   []*mockAlertRows
}

func (m *multiQueryPool) Query(_ any, _ string, _ ...any) (AlertRows, error) {
	idx := m.calls
	m.calls++
	if idx < len(m.rowSets) {
		return m.rowSets[idx], nil
	}
	return &mockAlertRows{}, nil
}

// ─────────────────────────────────────────────
// DELETE /v1/alert-groups/{id}
// ─────────────────────────────────────────────

func TestAlertNoiseHandler_DeleteGroup_Success(t *testing.T) {
	pool := &mockAlertPool{execResult: pgconn.NewCommandTag("DELETE 1")}
	h := NewAlertNoiseHandler(pool)

	req := httptest.NewRequest(http.MethodDelete, "/v1/alert-groups/agrp_1", nil)
	req = noiseWithUserID(req, "u_test")
	req = noiseWithChiParam(req, "id", "agrp_1")
	w := httptest.NewRecorder()

	h.DeleteGroup(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestAlertNoiseHandler_DeleteGroup_NotFound(t *testing.T) {
	pool := &mockAlertPool{execResult: pgconn.NewCommandTag("DELETE 0")}
	h := NewAlertNoiseHandler(pool)

	req := httptest.NewRequest(http.MethodDelete, "/v1/alert-groups/agrp_missing", nil)
	req = noiseWithUserID(req, "u_test")
	req = noiseWithChiParam(req, "id", "agrp_missing")
	w := httptest.NewRecorder()

	h.DeleteGroup(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAlertNoiseHandler_NoiseReport_WithData(t *testing.T) {
	r := chi.NewRouter()
	pool := &mockAlertPool{
		queryRows: &mockAlertRows{rows: [][]any{}},
	}
	h := NewAlertNoiseHandler(pool)
	r.Get("/reports/alert-noise", h.NoiseReport)

	req := httptest.NewRequest(http.MethodGet, "/reports/alert-noise", nil)
	req = noiseWithUserID(req, "u_test")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
