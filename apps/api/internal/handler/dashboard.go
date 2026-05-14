package handler

import (
	"net/http"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// DashboardHandler handles the dashboard summary endpoint.
type DashboardHandler struct{}

// NewDashboardHandler creates a DashboardHandler.
func NewDashboardHandler() *DashboardHandler {
	return &DashboardHandler{}
}

// MonitorSummary holds per-status counts for monitors.
type MonitorSummary struct {
	Total  int `json:"total"`
	Up     int `json:"up"`
	Down   int `json:"down"`
	Paused int `json:"paused"`
}

// DashboardSummaryResponse is the response body for GET /v1/dashboard/summary.
type DashboardSummaryResponse struct {
	Monitors      MonitorSummary `json:"monitors"`
	ChecksToday   int            `json:"checks_today"`
	AvgUptime7d   float64        `json:"avg_uptime_7d"`
	IncidentsOpen int            `json:"incidents_open"`
	AlertsFired7d int            `json:"alerts_fired_7d"`
	StatusPages   int            `json:"status_pages"`
}

// Summary handles GET /v1/dashboard/summary.
// Requires authentication. Returns stub data; real SQL queries are wired in S3.
func (h *DashboardHandler) Summary(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	data := DashboardSummaryResponse{
		Monitors: MonitorSummary{
			Total:  5,
			Up:     4,
			Down:   1,
			Paused: 0,
		},
		ChecksToday:   1440,
		AvgUptime7d:   99.7,
		IncidentsOpen: 1,
		AlertsFired7d: 3,
		StatusPages:   2,
	}

	response.JSON(w, r, http.StatusOK, data)
}
