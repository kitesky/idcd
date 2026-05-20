// Package handler implements HTTP handlers for the API server.
package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// LeaderboardPool is the subset of pgxpool.Pool used by LeaderboardHandler.
// It is satisfied by both *pgxpool.Pool and pgxmock.PgxPoolIface.
type LeaderboardPool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// LeaderboardHandler handles the public CDN leaderboard endpoints.
type LeaderboardHandler struct {
	pool LeaderboardPool
}

// NewLeaderboardHandler creates a new LeaderboardHandler.
func NewLeaderboardHandler(pool LeaderboardPool) *LeaderboardHandler {
	return &LeaderboardHandler{pool: pool}
}

// LeaderboardEntry represents a single CDN provider in the leaderboard.
type LeaderboardEntry struct {
	Rank       int     `json:"rank"`
	MonitorID  string  `json:"monitor_id"`
	Name       string  `json:"name"`
	Target     string  `json:"target"`
	AvgLatency float64 `json:"avg_latency_ms"`
	P50Latency float64 `json:"p50_latency_ms"`
	P95Latency float64 `json:"p95_latency_ms"`
	Uptime     float64 `json:"uptime_pct"`
	CheckCount int64   `json:"check_count"`
}

// LeaderboardResponse is the JSON response for GET /v1/leaderboard/cdn.
type LeaderboardResponse struct {
	Entries     []LeaderboardEntry `json:"entries"`
	Total       int                `json:"total"`
	WindowHours int                `json:"window_hours"`
	GeneratedAt string             `json:"generated_at"`
}

// CDNLeaderboard handles GET /v1/leaderboard/cdn.
// Returns CDN provider latency ranking based on real monitor_checks data.
// Only includes monitors owned by systemUserID (idcd_system).
func (h *LeaderboardHandler) CDNLeaderboard(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	windowHours := 24

	// Query aggregated latency stats from monitor_checks for system CDN monitors.
	// We compute p50 manually using percentile_disc (available in standard PostgreSQL).
	// Uptime is the percentage of checks with status='up'.
	rows, err := h.pool.Query(ctx, `
		SELECT
			m.id,
			m.name,
			m.target,
			COUNT(c.latency_ms)                                         AS check_count,
			COALESCE(AVG(c.latency_ms), 0)                             AS avg_latency_ms,
			COALESCE(PERCENTILE_DISC(0.5) WITHIN GROUP (ORDER BY c.latency_ms), 0) AS p50_latency_ms,
			COALESCE(PERCENTILE_DISC(0.95) WITHIN GROUP (ORDER BY c.latency_ms), 0) AS p95_latency_ms,
			COALESCE(
				100.0 * SUM(CASE WHEN c.status = 'up' THEN 1 ELSE 0 END)::float / NULLIF(COUNT(*), 0),
				0
			) AS uptime_pct
		FROM monitors m
		LEFT JOIN monitor_checks c
			ON c.monitor_id = m.id
			AND c.check_at >= NOW() - ($1::int * INTERVAL '1 hour')
		WHERE m.user_id = $2
		  AND m.status  = 'active'
		GROUP BY m.id, m.name, m.target
		ORDER BY avg_latency_ms ASC
	`, windowHours, systemUserID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query leaderboard", err))
		return
	}
	defer rows.Close()

	var entries []LeaderboardEntry
	rank := 1
	for rows.Next() {
		var e LeaderboardEntry
		if err := rows.Scan(
			&e.MonitorID,
			&e.Name,
			&e.Target,
			&e.CheckCount,
			&e.AvgLatency,
			&e.P50Latency,
			&e.P95Latency,
			&e.Uptime,
		); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan leaderboard row", err))
			return
		}
		e.Rank = rank
		rank++
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("row iteration error", err))
		return
	}

	if entries == nil {
		entries = []LeaderboardEntry{}
	}

	response.JSON(w, r, http.StatusOK, LeaderboardResponse{
		Entries:     entries,
		Total:       len(entries),
		WindowHours: windowHours,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	})
}
