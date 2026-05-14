package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
)

// AlertNoiseHandler implements silence, group, and noise report endpoints.
type AlertNoiseHandler struct {
	pool AlertPool
}

// NewAlertNoiseHandler creates an AlertNoiseHandler.
func NewAlertNoiseHandler(pool AlertPool) *AlertNoiseHandler {
	return &AlertNoiseHandler{pool: pool}
}

// ─────────────────────────────────────────────
// Request / Response types
// ─────────────────────────────────────────────

// CreateSilenceRequest is the body for POST /v1/alert-silences.
type CreateSilenceRequest struct {
	MonitorID *string `json:"monitor_id"`
	Reason    string  `json:"reason"`
	StartsAt  string  `json:"starts_at"`
	EndsAt    string  `json:"ends_at"`
}

// SilenceResponse is the JSON representation of an alert_silences row.
type SilenceResponse struct {
	ID        string  `json:"id"`
	UserID    string  `json:"user_id"`
	MonitorID *string `json:"monitor_id,omitempty"`
	Reason    string  `json:"reason"`
	StartsAt  string  `json:"starts_at"`
	EndsAt    string  `json:"ends_at"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"created_at"`
}

// CreateGroupRequest is the body for POST /v1/alert-groups.
type CreateGroupRequest struct {
	Name        string `json:"name"`
	GroupBy     string `json:"group_by"`
	GroupValue  string `json:"group_value"`
	WaitSeconds *int   `json:"wait_seconds"`
}

// AlertGroupResponse is the JSON representation of an alert_groups row.
type AlertGroupResponse struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	GroupBy     string `json:"group_by"`
	GroupValue  string `json:"group_value"`
	WaitSeconds int    `json:"wait_seconds"`
	CreatedAt   string `json:"created_at"`
}

// NoiseReportResponse is the noise analysis report.
type NoiseReportResponse struct {
	Period           NoisePeriod     `json:"period"`
	TotalFirings     int             `json:"total_firings"`
	TotalFlaps       int             `json:"total_flaps"`
	NoisestMonitors  []NoisyMonitor  `json:"noisiest_monitors"`
	DailyTrend       []NoiseDayEntry `json:"daily_trend"`
}

// NoisePeriod holds the report date range.
type NoisePeriod struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// NoisyMonitor is a single monitor's noise summary.
type NoisyMonitor struct {
	MonitorID string `json:"monitor_id"`
	Firings   int    `json:"firings"`
	Flaps     int    `json:"flaps"`
}

// NoiseDayEntry is a single day's noise summary.
type NoiseDayEntry struct {
	Date     string `json:"date"`
	Firings  int    `json:"firings"`
	Flaps    int    `json:"flaps"`
}

// ─────────────────────────────────────────────
// Silence endpoints
// ─────────────────────────────────────────────

// CreateSilence handles POST /v1/alert-silences.
func (h *AlertNoiseHandler) CreateSilence(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	var req CreateSilenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid JSON request body", ""))
		return
	}
	if req.Reason == "" {
		response.Error(w, r, apperr.Validation("reason is required", "reason"))
		return
	}

	startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		response.Error(w, r, apperr.Validation("starts_at must be RFC3339", "starts_at"))
		return
	}
	endsAt, err := time.Parse(time.RFC3339, req.EndsAt)
	if err != nil {
		response.Error(w, r, apperr.Validation("ends_at must be RFC3339", "ends_at"))
		return
	}
	if !endsAt.After(startsAt) {
		response.Error(w, r, apperr.Validation("ends_at must be after starts_at", "ends_at"))
		return
	}

	id := idgen.New("sil_")
	now := time.Now()

	_, err = h.pool.Exec(ctx, `
		INSERT INTO alert_silences (id, user_id, monitor_id, reason, starts_at, ends_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, userID, req.MonitorID, req.Reason, startsAt, endsAt, now,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to create silence", err))
		return
	}

	resp := SilenceResponse{
		ID:        id,
		UserID:    userID,
		MonitorID: req.MonitorID,
		Reason:    req.Reason,
		StartsAt:  startsAt.UTC().Format(time.RFC3339),
		EndsAt:    endsAt.UTC().Format(time.RFC3339),
		Status:    silenceStatus(startsAt, endsAt),
		CreatedAt: now.UTC().Format(time.RFC3339),
	}
	response.JSON(w, r, http.StatusCreated, resp)
}

// ListSilences handles GET /v1/alert-silences.
func (h *AlertNoiseHandler) ListSilences(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	rows, err := h.pool.Query(ctx, `
		SELECT id, user_id, monitor_id, reason, starts_at, ends_at, created_at
		FROM alert_silences
		WHERE user_id = $1
		  AND ends_at > NOW()
		ORDER BY starts_at ASC`, userID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list silences", err))
		return
	}
	defer rows.Close()

	var items []SilenceResponse
	for rows.Next() {
		var item SilenceResponse
		var startsAt, endsAt, createdAt time.Time
		if err := rows.Scan(
			&item.ID, &item.UserID, &item.MonitorID,
			&item.Reason, &startsAt, &endsAt, &createdAt,
		); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan silence", err))
			return
		}
		item.StartsAt = startsAt.UTC().Format(time.RFC3339)
		item.EndsAt = endsAt.UTC().Format(time.RFC3339)
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		item.Status = silenceStatus(startsAt, endsAt)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to iterate silences", err))
		return
	}

	if items == nil {
		items = []SilenceResponse{}
	}
	response.JSON(w, r, http.StatusOK, map[string]interface{}{"items": items})
}

// DeleteSilence handles DELETE /v1/alert-silences/{id}.
func (h *AlertNoiseHandler) DeleteSilence(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	id := chi.URLParam(r, "id")
	now := time.Now()

	tag, err := h.pool.Exec(ctx, `
		UPDATE alert_silences SET ends_at = $1
		WHERE id = $2 AND user_id = $3 AND ends_at > NOW()`,
		now, id, userID,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to delete silence", err))
		return
	}
	if tag.RowsAffected() == 0 {
		response.Error(w, r, apperr.NotFound("silence not found"))
		return
	}

	response.JSON(w, r, http.StatusNoContent, nil)
}

// ─────────────────────────────────────────────
// Noise report endpoint
// ─────────────────────────────────────────────

// NoiseReport handles GET /v1/reports/alert-noise.
func (h *AlertNoiseHandler) NoiseReport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 && v <= 365 {
			days = v
		}
	}

	toDate := time.Now().UTC()
	fromDate := toDate.AddDate(0, 0, -days)

	rows, err := h.pool.Query(ctx, `
		SELECT monitor_id, SUM(total_firings), SUM(flap_count)
		FROM alert_noise_stats
		WHERE user_id = $1 AND date >= $2 AND date <= $3
		GROUP BY monitor_id
		ORDER BY SUM(total_firings) DESC
		LIMIT 20`,
		userID, fromDate.Format("2006-01-02"), toDate.Format("2006-01-02"),
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query noise stats", err))
		return
	}
	defer rows.Close()

	var totalFirings, totalFlaps int
	var noisiest []NoisyMonitor
	for rows.Next() {
		var m NoisyMonitor
		if err := rows.Scan(&m.MonitorID, &m.Firings, &m.Flaps); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan noise row", err))
			return
		}
		totalFirings += m.Firings
		totalFlaps += m.Flaps
		noisiest = append(noisiest, m)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to iterate noise rows", err))
		return
	}

	dailyRows, err := h.pool.Query(ctx, `
		SELECT date, SUM(total_firings), SUM(flap_count)
		FROM alert_noise_stats
		WHERE user_id = $1 AND date >= $2 AND date <= $3
		GROUP BY date
		ORDER BY date ASC`,
		userID, fromDate.Format("2006-01-02"), toDate.Format("2006-01-02"),
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query daily noise", err))
		return
	}
	defer dailyRows.Close()

	var trend []NoiseDayEntry
	for dailyRows.Next() {
		var entry NoiseDayEntry
		var d time.Time
		if err := dailyRows.Scan(&d, &entry.Firings, &entry.Flaps); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan daily noise row", err))
			return
		}
		entry.Date = d.Format("2006-01-02")
		trend = append(trend, entry)
	}
	if err := dailyRows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to iterate daily noise rows", err))
		return
	}

	if noisiest == nil {
		noisiest = []NoisyMonitor{}
	}
	if trend == nil {
		trend = []NoiseDayEntry{}
	}

	resp := NoiseReportResponse{
		Period: NoisePeriod{
			From: fromDate.Format("2006-01-02"),
			To:   toDate.Format("2006-01-02"),
		},
		TotalFirings:    totalFirings,
		TotalFlaps:      totalFlaps,
		NoisestMonitors: noisiest,
		DailyTrend:      trend,
	}
	response.JSON(w, r, http.StatusOK, resp)
}

// ─────────────────────────────────────────────
// Alert Group endpoints
// ─────────────────────────────────────────────

var validGroupByValues = map[string]bool{
	"monitor_prefix": true,
	"tag":            true,
	"type":           true,
}

// CreateGroup handles POST /v1/alert-groups.
func (h *AlertNoiseHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	var req CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid JSON request body", ""))
		return
	}
	if req.Name == "" {
		response.Error(w, r, apperr.Validation("name is required", "name"))
		return
	}
	if !validGroupByValues[req.GroupBy] {
		response.Error(w, r, apperr.Validation("invalid group_by value", "group_by"))
		return
	}
	if req.GroupValue == "" {
		response.Error(w, r, apperr.Validation("group_value is required", "group_value"))
		return
	}

	waitSeconds := 60
	if req.WaitSeconds != nil {
		waitSeconds = *req.WaitSeconds
	}

	id := idgen.New("agrp_")
	now := time.Now()

	_, err := h.pool.Exec(ctx, `
		INSERT INTO alert_groups (id, user_id, name, group_by, group_value, wait_seconds, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, userID, req.Name, req.GroupBy, req.GroupValue, waitSeconds, now,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to create alert group", err))
		return
	}

	resp := AlertGroupResponse{
		ID:          id,
		UserID:      userID,
		Name:        req.Name,
		GroupBy:     req.GroupBy,
		GroupValue:  req.GroupValue,
		WaitSeconds: waitSeconds,
		CreatedAt:   now.UTC().Format(time.RFC3339),
	}
	response.JSON(w, r, http.StatusCreated, resp)
}

// ListGroups handles GET /v1/alert-groups.
func (h *AlertNoiseHandler) ListGroups(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	rows, err := h.pool.Query(ctx, `
		SELECT id, user_id, name, group_by, group_value, wait_seconds, created_at
		FROM alert_groups
		WHERE user_id = $1
		ORDER BY created_at DESC`, userID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list alert groups", err))
		return
	}
	defer rows.Close()

	var items []AlertGroupResponse
	for rows.Next() {
		var item AlertGroupResponse
		var createdAt time.Time
		if err := rows.Scan(
			&item.ID, &item.UserID, &item.Name,
			&item.GroupBy, &item.GroupValue, &item.WaitSeconds, &createdAt,
		); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan alert group", err))
			return
		}
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to iterate alert groups", err))
		return
	}

	if items == nil {
		items = []AlertGroupResponse{}
	}
	response.JSON(w, r, http.StatusOK, map[string]interface{}{"items": items})
}

// ─────────────────────────────────────────────
// Helper
// ─────────────────────────────────────────────

func silenceStatus(startsAt, endsAt time.Time) string {
	now := time.Now()
	if now.Before(startsAt) {
		return "upcoming"
	}
	if now.After(endsAt) {
		return "expired"
	}
	return "active"
}

