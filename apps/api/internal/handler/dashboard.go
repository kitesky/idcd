package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// DashboardPool is the minimal pgx interface needed by DashboardHandler.
// *pgxpool.Pool satisfies this.
type DashboardPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// DashboardHandler handles dashboard endpoints.
type DashboardHandler struct {
	pool  DashboardPool
	redis *redis.Client
}

// NewDashboardHandler creates a DashboardHandler.
func NewDashboardHandler(pool DashboardPool, rdb *redis.Client) *DashboardHandler {
	return &DashboardHandler{pool: pool, redis: rdb}
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

// DashboardPinsResponse is the response body for GET /v1/dashboard/pins.
type DashboardPinsResponse struct {
	MonitorIDs []string `json:"monitor_ids"`
}

// UpdatePinsRequest is the body for PUT /v1/dashboard/pins.
type UpdatePinsRequest struct {
	MonitorIDs []string `json:"monitor_ids"`
}

const maxPins = 6

func dashboardPinsKey(userID string) string {
	return "dashboard:pins:" + userID
}

// Summary handles GET /v1/dashboard/summary.
func (h *DashboardHandler) Summary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	var data DashboardSummaryResponse

	if h.pool != nil {
		var active, paused, down, total int
		_ = h.pool.QueryRow(ctx, `
			SELECT
				COUNT(*) FILTER (WHERE status = 'active'),
				COUNT(*) FILTER (WHERE status = 'paused'),
				COUNT(*) FILTER (WHERE status = 'down'),
				COUNT(*)
			FROM monitors WHERE user_id = $1
		`, userID).Scan(&active, &paused, &down, &total)

		data.Monitors = MonitorSummary{
			Total:  total,
			Up:     active,
			Down:   down,
			Paused: paused,
		}

		var checksToday int
		_ = h.pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM monitor_checks mc
			JOIN monitors m ON m.id = mc.monitor_id
			WHERE m.user_id = $1 AND mc.checked_at > NOW() - INTERVAL '24 hours'
		`, userID).Scan(&checksToday)
		data.ChecksToday = checksToday

		var incidentsOpen int
		_ = h.pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM alert_events ae
			JOIN monitors m ON m.id = ae.monitor_id
			WHERE m.user_id = $1 AND ae.status = 'firing'
		`, userID).Scan(&incidentsOpen)
		data.IncidentsOpen = incidentsOpen

		var alertsFired7d int
		_ = h.pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM alert_events ae
			JOIN monitors m ON m.id = ae.monitor_id
			WHERE m.user_id = $1 AND ae.created_at > NOW() - INTERVAL '7 days'
		`, userID).Scan(&alertsFired7d)
		data.AlertsFired7d = alertsFired7d

		var statusPages int
		_ = h.pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM status_pages WHERE user_id = $1
		`, userID).Scan(&statusPages)
		data.StatusPages = statusPages
	}

	response.JSON(w, r, http.StatusOK, data)
}

// GetPins handles GET /v1/dashboard/pins.
func (h *DashboardHandler) GetPins(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	ids := []string{}
	if h.redis != nil {
		val, err := h.redis.Get(ctx, dashboardPinsKey(userID)).Result()
		if err == nil {
			_ = json.Unmarshal([]byte(val), &ids)
		}
	}

	response.JSON(w, r, http.StatusOK, DashboardPinsResponse{MonitorIDs: ids})
}

// UpdatePins handles PUT /v1/dashboard/pins.
func (h *DashboardHandler) UpdatePins(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	var req UpdatePinsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid JSON request body", ""))
		return
	}

	if len(req.MonitorIDs) > maxPins {
		response.Error(w, r, apperr.Validation("monitor_ids cannot exceed 6 items", "monitor_ids"))
		return
	}

	if req.MonitorIDs == nil {
		req.MonitorIDs = []string{}
	}

	if h.redis != nil {
		raw, err := json.Marshal(req.MonitorIDs)
		if err != nil {
			response.Error(w, r, apperr.Internal("failed to serialize pins", err))
			return
		}
		if err := h.redis.Set(ctx, dashboardPinsKey(userID), raw, 0).Err(); err != nil {
			response.Error(w, r, apperr.Internal("failed to save pins", err))
			return
		}
	}

	response.JSON(w, r, http.StatusOK, DashboardPinsResponse{MonitorIDs: req.MonitorIDs})
}
