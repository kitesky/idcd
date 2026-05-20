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
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newUpgradeTestHandler(t *testing.T) (*NodeUpgradeHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	return NewNodeUpgradeHandler(mockPool, ""), mockPool
}

func upgradeColumns() []string {
	return []string{"id", "version", "download_url", "checksum", "rollout_pct", "status", "created_at", "updated_at"}
}

func upgradeRow(id, version, downloadURL, checksum string, pct int, status string) []any {
	now := time.Now().UTC().Truncate(time.Second)
	return []any{id, version, downloadURL, checksum, pct, status, now, now}
}

// injectChiParam sets a chi URL parameter in the request context.
func injectChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// TestCreateRollout_Success verifies that a valid creation request inserts a row and returns 201.
func TestCreateRollout_Success(t *testing.T) {
	h, mockPool := newUpgradeTestHandler(t)
	defer mockPool.Close()

	mockPool.ExpectQuery(`INSERT INTO node_upgrade_rollouts`).
		WithArgs(pgxmock.AnyArg(), "v1.2.0", "https://releases.idcd.com/agent-v1.2.0", "sha256:abc123", 10).
		WillReturnRows(pgxmock.NewRows(upgradeColumns()).AddRow(
			upgradeRow("oru_test01", "v1.2.0", "https://releases.idcd.com/agent-v1.2.0", "sha256:abc123", 10, "active")...,
		))

	body, _ := json.Marshal(createRolloutRequest{
		Version:     "v1.2.0",
		DownloadURL: "https://releases.idcd.com/agent-v1.2.0",
		Checksum:    "sha256:abc123",
		RolloutPct:  10,
	})
	req := httptest.NewRequest(http.MethodPost, "/internal/admin/upgrade-rollouts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, "v1.2.0", data["version"])
	assert.Equal(t, float64(10), data["rollout_pct"])
}

// TestCreateRollout_ValidationError verifies that missing required fields return 400.
func TestCreateRollout_ValidationError(t *testing.T) {
	h, mockPool := newUpgradeTestHandler(t)
	defer mockPool.Close()

	body, _ := json.Marshal(map[string]string{"version": "v1.2.0"})
	req := httptest.NewRequest(http.MethodPost, "/internal/admin/upgrade-rollouts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	require.NoError(t, mockPool.ExpectationsWereMet())
}

// TestListRollouts_Success verifies that the list endpoint returns all rollout rows.
func TestListRollouts_Success(t *testing.T) {
	h, mockPool := newUpgradeTestHandler(t)
	defer mockPool.Close()

	mockPool.ExpectQuery(`SELECT id, version, download_url, checksum, rollout_pct, status, created_at, updated_at`).
		WillReturnRows(pgxmock.NewRows(upgradeColumns()).
			AddRow(upgradeRow("oru_aaa", "v1.1.0", "https://example.com/a", "sha256:111", 1, "completed")...).
			AddRow(upgradeRow("oru_bbb", "v1.2.0", "https://example.com/b", "sha256:222", 10, "active")...),
		)

	req := httptest.NewRequest(http.MethodGet, "/internal/admin/upgrade-rollouts", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	data := resp["data"].([]any)
	assert.Len(t, data, 2)
}

// TestUpdateRolloutPct_Success verifies that rollout_pct can be updated.
func TestUpdateRolloutPct_Success(t *testing.T) {
	h, mockPool := newUpgradeTestHandler(t)
	defer mockPool.Close()

	rolloutID := "oru_test_xyz"

	mockPool.ExpectExec(`UPDATE node_upgrade_rollouts`).
		WithArgs(50, rolloutID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mockPool.ExpectQuery(`SELECT id, version`).
		WithArgs(rolloutID).
		WillReturnRows(pgxmock.NewRows(upgradeColumns()).AddRow(
			upgradeRow(rolloutID, "v1.2.0", "https://example.com/b", "sha256:222", 50, "active")...,
		))

	pct := 50
	body, _ := json.Marshal(updateRolloutRequest{RolloutPct: &pct})
	req := httptest.NewRequest(http.MethodPatch, "/internal/admin/upgrade-rollouts/"+rolloutID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = injectChiParam(req, "id", rolloutID)
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, float64(50), data["rollout_pct"])
}

// TestPauseRollout_Success verifies that a rollout can be paused.
func TestPauseRollout_Success(t *testing.T) {
	h, mockPool := newUpgradeTestHandler(t)
	defer mockPool.Close()

	rolloutID := "oru_pause_test"
	status := "paused"

	mockPool.ExpectExec(`UPDATE node_upgrade_rollouts`).
		WithArgs(status, rolloutID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mockPool.ExpectQuery(`SELECT id, version`).
		WithArgs(rolloutID).
		WillReturnRows(pgxmock.NewRows(upgradeColumns()).AddRow(
			upgradeRow(rolloutID, "v1.2.0", "https://example.com/b", "sha256:222", 10, "paused")...,
		))

	body, _ := json.Marshal(updateRolloutRequest{Status: &status})
	req := httptest.NewRequest(http.MethodPatch, "/internal/admin/upgrade-rollouts/"+rolloutID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = injectChiParam(req, "id", rolloutID)
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, "paused", data["status"])
}
