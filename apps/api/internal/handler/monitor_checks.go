package handler

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// MonitorChecksPool is the minimal pgx interface needed by MonitorChecksHandler.
// *pgxpool.Pool satisfies this.
type MonitorChecksPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// MonitorChecksHandler handles the checks history endpoint for a monitor.
type MonitorChecksHandler struct {
	monitorQ MonitorQuerier
	pool     MonitorChecksPool
}

// NewMonitorChecksHandler creates a MonitorChecksHandler.
func NewMonitorChecksHandler(mq MonitorQuerier, pool MonitorChecksPool) *MonitorChecksHandler {
	return &MonitorChecksHandler{monitorQ: mq, pool: pool}
}

// CheckBucket represents one time-bucket in the history response.
type CheckBucket struct {
	BucketStart      string  `json:"bucket_start"`
	Total            int64   `json:"total"`
	Success          int64   `json:"success"`
	Failure          int64   `json:"failure"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
	Status           string  `json:"status"`
}

// MonitorChecksResponse is the JSON response for GET /v1/monitors/{id}/checks.
type MonitorChecksResponse struct {
	MonitorID         string        `json:"monitor_id"`
	Hours             int           `json:"hours"`
	ResolutionMinutes int           `json:"resolution_minutes"`
	Buckets           []CheckBucket `json:"buckets"`
}

// List handles GET /v1/monitors/{id}/checks.
func (h *MonitorChecksHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	id := chi.URLParam(r, "id")
	m, err := h.monitorQ.GetMonitorByID(ctx, id)
	if err != nil {
		response.Error(w, r, apperr.NotFound("monitor not found"))
		return
	}
	if m.UserID != userID {
		response.Error(w, r, apperr.Forbidden("forbidden"))
		return
	}

	hours := 24
	if hParam := r.URL.Query().Get("hours"); hParam != "" {
		if v, err := strconv.Atoi(hParam); err == nil && v > 0 {
			hours = v
		}
	}
	if hours > 168 {
		hours = 168
	}

	resolutionMinutes := 30
	if rParam := r.URL.Query().Get("resolution"); rParam != "" {
		switch rParam {
		case "30m":
			resolutionMinutes = 30
		case "60m":
			resolutionMinutes = 60
		}
	}

	interval := strconv.Itoa(resolutionMinutes) + " minutes"

	rows, err := h.pool.Query(ctx,
		`SELECT
			time_bucket($1::INTERVAL, check_at) AS bucket_start,
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE status = 'up') AS success,
			COUNT(*) FILTER (WHERE status != 'up') AS failure,
			COALESCE(AVG(latency_ms), 0) AS avg_latency_ms
		FROM monitor_checks
		WHERE monitor_id = $2 AND check_at > NOW() - ($3 || ' hours')::INTERVAL
		GROUP BY bucket_start
		ORDER BY bucket_start`,
		interval,
		id,
		strconv.Itoa(hours),
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query checks", err))
		return
	}
	defer rows.Close()

	buckets := make([]CheckBucket, 0)
	for rows.Next() {
		var bucketStart pgtype.Timestamptz
		var total, success, failure int64
		var avgLatency float64

		if err := rows.Scan(&bucketStart, &total, &success, &failure, &avgLatency); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan check row", err))
			return
		}

		status := bucketStatus(total, success, failure)
		bs := ""
		if bucketStart.Valid {
			bs = bucketStart.Time.UTC().Format(time.RFC3339)
		}

		buckets = append(buckets, CheckBucket{
			BucketStart:  bs,
			Total:        total,
			Success:      success,
			Failure:      failure,
			AvgLatencyMs: math.Round(avgLatency*10) / 10,
			Status:       status,
		})
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to iterate check rows", err))
		return
	}

	response.JSON(w, r, http.StatusOK, MonitorChecksResponse{
		MonitorID:         id,
		Hours:             hours,
		ResolutionMinutes: resolutionMinutes,
		Buckets:           buckets,
	})
}

func bucketStatus(total, success, failure int64) string {
	if total == 0 {
		return "empty"
	}
	if failure > 0 && success == 0 {
		return "down"
	}
	if failure > 0 {
		return "degraded"
	}
	return "up"
}
