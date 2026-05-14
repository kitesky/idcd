package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/api/internal/middleware"
)

func setupDashboardHandler(t *testing.T) (*DashboardHandler, pgxmock.PgxPoolIface, *miniredis.Miniredis) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	mr := miniredis.NewMiniRedis()
	if err := mr.Start(); err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return NewDashboardHandler(mockPool, rdb), mockPool, mr
}

func injectDashboardUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDContextKey(), userID)
	return r.WithContext(ctx)
}

func TestDashboardSummary_RealDB(t *testing.T) {
	h, mockPool, mr := setupDashboardHandler(t)
	defer mockPool.Close()
	defer mr.Close()

	uid := "u_test"

	mockPool.ExpectQuery(`SELECT`).
		WithArgs(uid).
		WillReturnRows(pgxmock.NewRows([]string{"active", "paused", "down", "total"}).
			AddRow(3, 1, 1, 5))

	mockPool.ExpectQuery(`SELECT COUNT`).
		WithArgs(uid).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1440))

	mockPool.ExpectQuery(`SELECT COUNT`).
		WithArgs(uid).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))

	mockPool.ExpectQuery(`SELECT COUNT`).
		WithArgs(uid).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(7))

	mockPool.ExpectQuery(`SELECT COUNT`).
		WithArgs(uid).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(3))

	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/summary", nil)
	req = injectDashboardUserID(req, uid)
	req = withReqID(req, "test-summary-1")
	rr := httptest.NewRecorder()

	h.Summary(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body struct {
		Data DashboardSummaryResponse `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	d := body.Data
	if d.Monitors.Total != 5 {
		t.Errorf("monitors.total: want 5, got %d", d.Monitors.Total)
	}
	if d.Monitors.Up != 3 {
		t.Errorf("monitors.up: want 3, got %d", d.Monitors.Up)
	}
	if d.ChecksToday != 1440 {
		t.Errorf("checks_today: want 1440, got %d", d.ChecksToday)
	}
	if d.IncidentsOpen != 2 {
		t.Errorf("incidents_open: want 2, got %d", d.IncidentsOpen)
	}
	if d.AlertsFired7d != 7 {
		t.Errorf("alerts_fired_7d: want 7, got %d", d.AlertsFired7d)
	}
	if d.StatusPages != 3 {
		t.Errorf("status_pages: want 3, got %d", d.StatusPages)
	}
}

func TestDashboardSummary_NilPool(t *testing.T) {
	h := NewDashboardHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/summary", nil)
	req = injectDashboardUserID(req, "u_test")
	req = withReqID(req, "test-summary-nil")
	rr := httptest.NewRecorder()

	h.Summary(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var body struct {
		Data DashboardSummaryResponse `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.Monitors.Total != 0 {
		t.Errorf("expected 0, got %d", body.Data.Monitors.Total)
	}
}

func TestDashboardHandler_Summary_unauthenticated(t *testing.T) {
	h := NewDashboardHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/summary", nil)
	req = withReqID(req, "test-summary-unauth")
	rr := httptest.NewRecorder()

	h.Summary(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestDashboardPins_GetEmpty(t *testing.T) {
	h, mockPool, mr := setupDashboardHandler(t)
	defer mockPool.Close()
	defer mr.Close()

	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/pins", nil)
	req = injectDashboardUserID(req, "u_pins_test")
	req = withReqID(req, "test-pins-empty")
	rr := httptest.NewRecorder()

	h.GetPins(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body struct {
		Data DashboardPinsResponse `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Data.MonitorIDs) != 0 {
		t.Errorf("expected empty monitor_ids, got %v", body.Data.MonitorIDs)
	}
}

func TestDashboardPins_PutAndGet(t *testing.T) {
	h, mockPool, mr := setupDashboardHandler(t)
	defer mockPool.Close()
	defer mr.Close()

	uid := "u_pins_roundtrip"
	ids := []string{"mon_aaa", "mon_bbb", "mon_ccc"}

	putBody, _ := json.Marshal(UpdatePinsRequest{MonitorIDs: ids})
	req := httptest.NewRequest(http.MethodPut, "/v1/dashboard/pins", bytes.NewReader(putBody))
	req.Header.Set("Content-Type", "application/json")
	req = injectDashboardUserID(req, uid)
	req = withReqID(req, "test-pins-put")
	rr := httptest.NewRecorder()

	h.UpdatePins(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("PUT pins: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/v1/dashboard/pins", nil)
	req2 = injectDashboardUserID(req2, uid)
	req2 = withReqID(req2, "test-pins-get")
	rr2 := httptest.NewRecorder()

	h.GetPins(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("GET pins: expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}

	var body struct {
		Data DashboardPinsResponse `json:"data"`
	}
	if err := json.NewDecoder(rr2.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Data.MonitorIDs) != 3 {
		t.Errorf("expected 3 pins, got %d", len(body.Data.MonitorIDs))
	}
	for i, id := range ids {
		if body.Data.MonitorIDs[i] != id {
			t.Errorf("pin[%d]: want %s, got %s", i, id, body.Data.MonitorIDs[i])
		}
	}
}

func TestDashboardPins_ExceedsMax(t *testing.T) {
	h, mockPool, mr := setupDashboardHandler(t)
	defer mockPool.Close()
	defer mr.Close()

	ids := []string{"a", "b", "c", "d", "e", "f", "g"}
	putBody, _ := json.Marshal(UpdatePinsRequest{MonitorIDs: ids})
	req := httptest.NewRequest(http.MethodPut, "/v1/dashboard/pins", bytes.NewReader(putBody))
	req.Header.Set("Content-Type", "application/json")
	req = injectDashboardUserID(req, "u_toomany")
	req = withReqID(req, "test-pins-toomany")
	rr := httptest.NewRecorder()

	h.UpdatePins(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestDashboardPins_Unauthenticated(t *testing.T) {
	h := NewDashboardHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/pins", nil)
	req = withReqID(req, "test-pins-unauth")
	rr := httptest.NewRecorder()

	h.GetPins(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}
