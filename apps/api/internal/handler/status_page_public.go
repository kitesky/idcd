package handler

import (
	"context"
	"errors"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
	"github.com/kite365/idcd/lib/shared/apperr"
)

type statusPagePublicPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type StatusPagePublicHandler struct {
	pool statusPagePublicPool
}

func NewStatusPagePublicHandler(pool statusPagePublicPool) *StatusPagePublicHandler {
	return &StatusPagePublicHandler{pool: pool}
}

type publicMonitorHistory struct {
	Date   string  `json:"date"`
	Status string  `json:"status"`
	Uptime float64 `json:"uptime"`
}

type publicMonitor struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Status        string                 `json:"status"`
	UptimePercent float64                `json:"uptime_percent"`
	History       []publicMonitorHistory `json:"history"`
}

type publicGroup struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Monitors []publicMonitor `json:"monitors"`
}

type statusPagePublicResponse struct {
	Slug          string        `json:"slug"`
	Title         string        `json:"title"`
	Description   string        `json:"description"`
	Branding      bool          `json:"branding"`
	OverallStatus string        `json:"overall_status"`
	Groups        []publicGroup `json:"groups"`
	Events        []struct{}    `json:"events"`
}

func (h *StatusPagePublicHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slug := chi.URLParam(r, "slug")

	sp, err := h.getStatusPageBySlug(ctx, slug)
	if err != nil {
		response.Error(w, r, apperr.NotFound("status page not found"))
		return
	}

	monitors, err := h.listMonitorsByUser(ctx, sp.UserID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query monitors", err))
		return
	}

	// Pre-fetch current statuses for all monitors in a single query to avoid N+1.
	monitorIDs := make([]string, len(monitors))
	for i, m := range monitors {
		monitorIDs[i] = m.ID
	}
	currentStatuses, err := h.batchCurrentStatuses(ctx, monitorIDs)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query monitor statuses", err))
		return
	}

	pubMonitors := make([]publicMonitor, 0, len(monitors))
	for _, m := range monitors {
		pm, err := h.buildPublicMonitor(ctx, m, currentStatuses[m.ID])
		if err != nil {
			response.Error(w, r, apperr.Internal("failed to build monitor data", err))
			return
		}
		pubMonitors = append(pubMonitors, pm)
	}

	overall := overallStatus(pubMonitors)

	desc := ""
	if sp.Description != nil {
		desc = *sp.Description
	}

	resp := statusPagePublicResponse{
		Slug:          sp.Slug,
		Title:         sp.Name,
		Description:   desc,
		Branding:      sp.Branding,
		OverallStatus: overall,
		Groups: []publicGroup{
			{ID: "default", Name: "Monitors", Monitors: pubMonitors},
		},
		Events: []struct{}{},
	}

	response.JSON(w, r, http.StatusOK, resp)
}

func (h *StatusPagePublicHandler) getStatusPageBySlug(ctx context.Context, slug string) (idcdmain.StatusPage, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, user_id, slug, name, description, custom_domain,
		        custom_domain_verified_at, custom_domain_cert_expires_at, branding, created_at, updated_at
		 FROM status_pages WHERE slug = $1`,
		slug,
	)
	var sp idcdmain.StatusPage
	err := row.Scan(
		&sp.ID, &sp.UserID, &sp.Slug, &sp.Name, &sp.Description, &sp.CustomDomain,
		&sp.CustomDomainVerifiedAt, &sp.CustomDomainCertExpiresAt, &sp.Branding,
		&sp.CreatedAt, &sp.UpdatedAt,
	)
	if err != nil {
		return idcdmain.StatusPage{}, errors.New("not found")
	}
	return sp, nil
}

func (h *StatusPagePublicHandler) listMonitorsByUser(ctx context.Context, userID string) ([]idcdmain.Monitor, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, user_id, name, type, target, config, interval_s, node_count, status, last_check_at, next_check_at, created_at, updated_at
		 FROM monitors WHERE user_id = $1 AND status != 'archived' ORDER BY created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var monitors []idcdmain.Monitor
	for rows.Next() {
		var m idcdmain.Monitor
		if err := rows.Scan(
			&m.ID, &m.UserID, &m.Name, &m.Type, &m.Target, &m.Config,
			&m.IntervalS, &m.NodeCount, &m.Status, &m.LastCheckAt, &m.NextCheckAt,
			&m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		monitors = append(monitors, m)
	}
	return monitors, rows.Err()
}

type dayCheckRow struct {
	date    string
	total   int64
	success int64
}

// buildPublicMonitor assembles monitor display data using a currentStatus pre-fetched
// by the caller (batchCurrentStatuses) to avoid an N+1 round-trip per monitor.
func (h *StatusPagePublicHandler) buildPublicMonitor(ctx context.Context, m idcdmain.Monitor, currentStatus string) (publicMonitor, error) {
	since := time.Now().UTC().Add(-30 * 24 * time.Hour)

	rows, err := h.pool.Query(ctx,
		`SELECT DATE(check_at AT TIME ZONE 'UTC') AS day,
		        COUNT(*) AS total,
		        COUNT(*) FILTER (WHERE status = 'up') AS success
		 FROM monitor_checks
		 WHERE monitor_id = $1 AND check_at >= $2
		 GROUP BY day
		 ORDER BY day ASC`,
		m.ID, since,
	)
	if err != nil {
		return publicMonitor{}, err
	}
	defer rows.Close()

	var dayRows []dayCheckRow
	for rows.Next() {
		var dr dayCheckRow
		if err := rows.Scan(&dr.date, &dr.total, &dr.success); err != nil {
			return publicMonitor{}, err
		}
		dayRows = append(dayRows, dr)
	}
	if err := rows.Err(); err != nil {
		return publicMonitor{}, err
	}

	history := buildHistory(dayRows, since)
	uptimePct := computeUptimePercent(dayRows)

	return publicMonitor{
		ID:            m.ID,
		Name:          m.Name,
		Status:        currentStatus,
		UptimePercent: uptimePct,
		History:       history,
	}, nil
}

// buildHistory produces a 30-day window oldest-first (since+0 … since+29).
// Days with no check data are treated as 100% uptime / operational.
func buildHistory(dayRows []dayCheckRow, since time.Time) []publicMonitorHistory {
	type totals struct{ total, success int64 }
	byDate := make(map[string]totals, len(dayRows))
	for _, dr := range dayRows {
		byDate[dr.date] = totals{dr.total, dr.success}
	}

	// Build oldest-first directly (i=0 is since, i=29 is ~today).
	history := make([]publicMonitorHistory, 0, 30)
	for i := 0; i < 30; i++ {
		d := since.AddDate(0, 0, i)
		dateStr := d.Format("2006-01-02")
		if row, ok := byDate[dateStr]; ok {
			uptime := 0.0
			if row.total > 0 {
				uptime = math.Round(float64(row.success)/float64(row.total)*10000) / 100
			}
			status := dayStatus(row.total, row.success)
			history = append(history, publicMonitorHistory{Date: dateStr, Status: status, Uptime: uptime})
		} else {
			history = append(history, publicMonitorHistory{Date: dateStr, Status: "operational", Uptime: 100.0})
		}
	}
	return history
}

func dayStatus(total, success int64) string {
	if total == 0 || success == total {
		return "operational"
	}
	if float64(success)/float64(total) >= 0.5 {
		return "degraded"
	}
	return "outage"
}

func computeUptimePercent(dayRows []dayCheckRow) float64 {
	var totalChecks, successChecks int64
	for _, dr := range dayRows {
		totalChecks += dr.total
		successChecks += dr.success
	}
	if totalChecks == 0 {
		return 100.0
	}
	return math.Round(float64(successChecks)/float64(totalChecks)*10000) / 100
}

// batchCurrentStatuses fetches the 3 most-recent check statuses for every
// monitor in a single LATERAL JOIN, eliminating the N+1 round-trip.
func (h *StatusPagePublicHandler) batchCurrentStatuses(ctx context.Context, monitorIDs []string) (map[string]string, error) {
	result := make(map[string]string, len(monitorIDs))
	for _, id := range monitorIDs {
		result[id] = "operational"
	}
	if len(monitorIDs) == 0 {
		return result, nil
	}

	rows, err := h.pool.Query(ctx, `
		SELECT mc.monitor_id, mc.status
		FROM UNNEST($1::text[]) AS ids(monitor_id)
		CROSS JOIN LATERAL (
			SELECT monitor_id, status
			FROM monitor_checks
			WHERE monitor_id = ids.monitor_id
			ORDER BY check_at DESC
			LIMIT 3
		) mc
	`, monitorIDs)
	if err != nil {
		return result, nil // fail open: default to operational
	}
	defer rows.Close()

	statusMap := make(map[string][]string, len(monitorIDs))
	for rows.Next() {
		var monID, st string
		if err := rows.Scan(&monID, &st); err != nil {
			continue
		}
		statusMap[monID] = append(statusMap[monID], st)
	}
	_ = rows.Err() // best-effort; defaults already set

	for id, statuses := range statusMap {
		failures := 0
		for _, s := range statuses {
			if s != "up" {
				failures++
			}
		}
		switch {
		case failures == 0:
			result[id] = "operational"
		case failures == len(statuses):
			result[id] = "outage"
		default:
			result[id] = "degraded"
		}
	}
	return result, nil
}

func overallStatus(monitors []publicMonitor) string {
	worst := "operational"
	for _, m := range monitors {
		if m.Status == "outage" {
			return "outage"
		}
		if m.Status == "degraded" {
			worst = "degraded"
		}
	}
	return worst
}
