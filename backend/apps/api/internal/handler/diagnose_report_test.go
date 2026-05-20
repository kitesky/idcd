package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDiagnoseTestEnv(t *testing.T) (*DiagnoseReportHandler, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	h := NewDiagnoseReportHandler(rdb)
	return h, rdb
}

// TestDiagnoseReportHandler_SaveAndGet_miniredis tests the full save+get round-trip.
func TestDiagnoseReportHandler_SaveAndGet_miniredis(t *testing.T) {
	h, rdb := newDiagnoseTestEnv(t)

	report := map[string]any{
		"id":         "rpt_abc123",
		"domain":     "example.com",
		"createdAt":  "2026-05-15T00:00:00Z",
		"checks":     []any{},
		"doneCount":  0,
		"errorCount": 0,
	}
	body, err := json.Marshal(report)
	require.NoError(t, err)

	// Save the report
	req := httptest.NewRequest(http.MethodPost, "/v1/diagnose/reports", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withRequestID(req, "req-save-1")
	rr := httptest.NewRecorder()
	h.SaveReport(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code, "save: %s", rr.Body.String())

	var saveResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &saveResp))
	assert.Equal(t, "rpt_abc123", saveResp.Data.ID)

	// Verify it was stored in Redis
	stored, err := rdb.Get(context.Background(), "diagnose:report:rpt_abc123").Bytes()
	require.NoError(t, err)
	assert.Contains(t, string(stored), "example.com")

	// Get the report
	getReq := httptest.NewRequest(http.MethodGet, "/v1/diagnose/reports/rpt_abc123", nil)
	getReq = withRequestID(getReq, "req-get-1")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "rpt_abc123")
	getReq = getReq.WithContext(context.WithValue(getReq.Context(), chi.RouteCtxKey, rctx))

	getRR := httptest.NewRecorder()
	h.GetReport(getRR, getReq)

	require.Equal(t, http.StatusOK, getRR.Code, "get: %s", getRR.Body.String())

	var gotReport map[string]any
	require.NoError(t, json.Unmarshal(getRR.Body.Bytes(), &gotReport))
	assert.Equal(t, "example.com", gotReport["domain"])
	assert.Equal(t, "rpt_abc123", gotReport["id"])
}

// TestDiagnoseReportHandler_GetNotFound verifies 404 when report does not exist.
func TestDiagnoseReportHandler_GetNotFound(t *testing.T) {
	h, _ := newDiagnoseTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/diagnose/reports/nonexistent", nil)
	req = withRequestID(req, "req-notfound-1")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.GetReport(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)

	var errResp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &errResp))
	assert.Equal(t, "NOT_FOUND", errResp.Error.Code)
}

// TestDiagnoseReportHandler_SaveInvalidJSON verifies 400 on invalid JSON body.
func TestDiagnoseReportHandler_SaveInvalidJSON(t *testing.T) {
	h, _ := newDiagnoseTestEnv(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/diagnose/reports", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	req = withRequestID(req, "req-invalid-1")
	rr := httptest.NewRecorder()
	h.SaveReport(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestDiagnoseReportHandler_SaveMissingID verifies 400 when id field is absent.
func TestDiagnoseReportHandler_SaveMissingID(t *testing.T) {
	h, _ := newDiagnoseTestEnv(t)

	body, _ := json.Marshal(map[string]string{"domain": "example.com"})
	req := httptest.NewRequest(http.MethodPost, "/v1/diagnose/reports", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withRequestID(req, "req-missing-id-1")
	rr := httptest.NewRecorder()
	h.SaveReport(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
