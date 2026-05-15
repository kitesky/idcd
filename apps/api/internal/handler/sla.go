package handler

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// SLAPool is the minimal pgx interface needed by SLAHandler.
// *pgxpool.Pool satisfies this.
type SLAPool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// SLAHandler handles SLA report endpoints.
type SLAHandler struct {
	pool SLAPool
}

// NewSLAHandler creates an SLAHandler wired to the given pool.
func NewSLAHandler(pool SLAPool) *SLAHandler {
	return &SLAHandler{pool: pool}
}

// SLAMonthEntry holds per-month uptime statistics.
type SLAMonthEntry struct {
	Month        string  `json:"month"`
	UptimePct    float64 `json:"uptime_pct"`
	TotalChecks  int64   `json:"total_checks"`
	FailedChecks int64   `json:"failed_checks"`
}

// SLAMonitorEntry holds the full SLA data for one monitor.
type SLAMonitorEntry struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Type         string          `json:"type"`
	Months       []SLAMonthEntry `json:"months"`
	AvgUptimePct float64         `json:"avg_uptime_pct"`
}

// SLAPeriod is the date range covered by the report.
type SLAPeriod struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// SLAReportResponse is the response body for GET /v1/reports/sla.
type SLAReportResponse struct {
	Period   SLAPeriod         `json:"period"`
	Monitors []SLAMonitorEntry `json:"monitors"`
}

// GetSLA handles GET /v1/reports/sla.
// Query params:
//   - months: number of months to include (default 3, max 12)
func (h *SLAHandler) GetSLA(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	months := 3
	if m := r.URL.Query().Get("months"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v > 0 {
			months = v
		}
	}
	if months > 12 {
		months = 12
	}

	now := time.Now().UTC()
	from := time.Date(now.Year(), now.Month()-time.Month(months-1), 1, 0, 0, 0, 0, time.UTC)
	toEnd := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC)

	period := SLAPeriod{
		From: from.Format("2006-01-02"),
		To:   toEnd.Format("2006-01-02"),
	}

	rows, err := h.pool.Query(ctx, `
		SELECT
			monitors.id,
			monitors.name,
			monitors.type,
			DATE_TRUNC('month', monitor_checks.check_at) AS month,
			COUNT(*) AS total_checks,
			COUNT(*) FILTER (WHERE monitor_checks.status != 'up') AS failed_checks
		FROM monitors
		JOIN monitor_checks ON monitors.id = monitor_checks.monitor_id
		WHERE monitors.user_id = $1
		  AND monitor_checks.check_at >= $2
		GROUP BY monitors.id, monitors.name, monitors.type, month
		ORDER BY monitors.id, month
	`, userID, from)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query SLA data", err))
		return
	}
	defer rows.Close()

	type rowKey struct {
		id   string
		name string
		typ  string
	}

	monitorOrder := make([]rowKey, 0)
	monitorSeen := make(map[string]bool)
	monthsByMonitor := make(map[string][]SLAMonthEntry)
	monitorMeta := make(map[string]rowKey)

	for rows.Next() {
		var (
			id           string
			name         string
			typ          string
			monthTime    time.Time
			totalChecks  int64
			failedChecks int64
		)
		if err := rows.Scan(&id, &name, &typ, &monthTime, &totalChecks, &failedChecks); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan SLA row", err))
			return
		}

		uptimePct := 100.0
		if totalChecks > 0 {
			uptimePct = math.Round(float64(totalChecks-failedChecks)/float64(totalChecks)*10000) / 100
		}

		entry := SLAMonthEntry{
			Month:        monthTime.Format("2006-01"),
			UptimePct:    uptimePct,
			TotalChecks:  totalChecks,
			FailedChecks: failedChecks,
		}

		if !monitorSeen[id] {
			monitorSeen[id] = true
			key := rowKey{id: id, name: name, typ: typ}
			monitorOrder = append(monitorOrder, key)
			monitorMeta[id] = key
		}
		monthsByMonitor[id] = append(monthsByMonitor[id], entry)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to iterate SLA rows", err))
		return
	}

	monitors := make([]SLAMonitorEntry, 0, len(monitorOrder))
	for _, key := range monitorOrder {
		monthEntries := monthsByMonitor[key.id]
		avgUptimePct := 0.0
		if len(monthEntries) > 0 {
			sum := 0.0
			for _, me := range monthEntries {
				sum += me.UptimePct
			}
			avgUptimePct = math.Round(sum/float64(len(monthEntries))*100) / 100
		}
		monitors = append(monitors, SLAMonitorEntry{
			ID:           key.id,
			Name:         key.name,
			Type:         key.typ,
			Months:       monthEntries,
			AvgUptimePct: avgUptimePct,
		})
	}

	response.JSON(w, r, http.StatusOK, SLAReportResponse{
		Period:   period,
		Monitors: monitors,
	})
}
