package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kite365/idcd/apps/api/internal/middleware"
)

func TestDashboardHandler_Summary_authenticated(t *testing.T) {
	h := NewDashboardHandler()

	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/summary", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDContextKey(), "u_testuser")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.Summary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body struct {
		Data struct {
			Monitors struct {
				Total  int `json:"total"`
				Up     int `json:"up"`
				Down   int `json:"down"`
				Paused int `json:"paused"`
			} `json:"monitors"`
			ChecksToday   int     `json:"checks_today"`
			AvgUptime7d   float64 `json:"avg_uptime_7d"`
			IncidentsOpen int     `json:"incidents_open"`
			AlertsFired7d int     `json:"alerts_fired_7d"`
			StatusPages   int     `json:"status_pages"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	d := body.Data
	if d.Monitors.Total == 0 {
		t.Error("monitors.total should be non-zero")
	}
	if d.ChecksToday == 0 {
		t.Error("checks_today should be non-zero")
	}
	if d.AvgUptime7d == 0 {
		t.Error("avg_uptime_7d should be non-zero")
	}
}

func TestDashboardHandler_Summary_unauthenticated(t *testing.T) {
	h := NewDashboardHandler()

	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/summary", nil)
	rec := httptest.NewRecorder()

	h.Summary(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
