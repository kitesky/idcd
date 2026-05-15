package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// AnchorPool is the minimal pgx interface needed by AnchorHandler.
type AnchorPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// AnchorHandler handles anchor baseline and deviation endpoints.
type AnchorHandler struct {
	monitorQ MonitorQuerier
	pool     AnchorPool
}

// NewAnchorHandler creates an AnchorHandler.
func NewAnchorHandler(mq MonitorQuerier, pool AnchorPool) *AnchorHandler {
	return &AnchorHandler{monitorQ: mq, pool: pool}
}

// BaselineResponse is the JSON body for GET /v1/monitors/{id}/baseline.
type BaselineResponse struct {
	ID          string   `json:"id"`
	MonitorID   string   `json:"monitor_id"`
	P50Latency  *float64 `json:"p50_latency_ms"`
	P95Latency  *float64 `json:"p95_latency_ms"`
	P99Latency  *float64 `json:"p99_latency_ms"`
	SuccessRate *float64 `json:"success_rate"`
	SampleCount int      `json:"sample_count"`
	ComputedAt  string   `json:"computed_at"`
	WindowHours int      `json:"window_hours"`
}

// DeviationItem represents one anchor deviation record.
type DeviationItem struct {
	ID            string  `json:"id"`
	MonitorID     string  `json:"monitor_id"`
	BaselineID    string  `json:"baseline_id"`
	DeviationType string  `json:"deviation_type"`
	CurrentValue  float64 `json:"current_value"`
	BaselineValue float64 `json:"baseline_value"`
	DeviationPct  float64 `json:"deviation_pct"`
	Severity      string  `json:"severity"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"created_at"`
	ResolvedAt    *string `json:"resolved_at,omitempty"`
}

// DeviationsResponse is the JSON body for GET /v1/monitors/{id}/deviations.
type DeviationsResponse struct {
	MonitorID  string          `json:"monitor_id"`
	Deviations []DeviationItem `json:"deviations"`
}

func (h *AnchorHandler) ownerCheck(ctx context.Context, w http.ResponseWriter, r *http.Request, monitorID string) (userID string, ok bool) {
	userID = middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return "", false
	}
	m, err := h.monitorQ.GetMonitorByID(ctx, monitorID)
	if err != nil {
		response.Error(w, r, apperr.NotFound("monitor not found"))
		return "", false
	}
	if m.UserID != userID {
		response.Error(w, r, apperr.Forbidden("forbidden"))
		return "", false
	}
	return userID, true
}

// GetBaseline handles GET /v1/monitors/{id}/baseline.
func (h *AnchorHandler) GetBaseline(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if _, ok := h.ownerCheck(ctx, w, r, id); !ok {
		return
	}

	var bl BaselineResponse
	var computedAt time.Time
	err := h.pool.QueryRow(ctx, `
		SELECT id, monitor_id, p50_latency, p95_latency, p99_latency,
		       success_rate, sample_count, computed_at, window_hours
		FROM monitor_baselines
		WHERE monitor_id = $1
	`, id).Scan(
		&bl.ID, &bl.MonitorID, &bl.P50Latency, &bl.P95Latency, &bl.P99Latency,
		&bl.SuccessRate, &bl.SampleCount, &computedAt, &bl.WindowHours,
	)
	if err == pgx.ErrNoRows {
		response.Error(w, r, apperr.NotFound("no baseline found for this monitor"))
		return
	}
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query baseline", err))
		return
	}
	bl.ComputedAt = computedAt.UTC().Format(time.RFC3339)
	response.JSON(w, r, http.StatusOK, bl)
}

// ListDeviations handles GET /v1/monitors/{id}/deviations.
func (h *AnchorHandler) ListDeviations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if _, ok := h.ownerCheck(ctx, w, r, id); !ok {
		return
	}

	rows, err := h.pool.Query(ctx, `
		SELECT id, monitor_id, baseline_id, deviation_type,
		       current_value, baseline_value, deviation_pct,
		       severity, status, created_at, resolved_at
		FROM anchor_deviations
		WHERE monitor_id = $1
		ORDER BY created_at DESC
		LIMIT 20
	`, id)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query deviations", err))
		return
	}
	defer rows.Close()

	deviations := make([]DeviationItem, 0)
	for rows.Next() {
		var item DeviationItem
		var createdAt time.Time
		var resolvedAt *time.Time

		if err := rows.Scan(
			&item.ID, &item.MonitorID, &item.BaselineID, &item.DeviationType,
			&item.CurrentValue, &item.BaselineValue, &item.DeviationPct,
			&item.Severity, &item.Status, &createdAt, &resolvedAt,
		); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan deviation row", err))
			return
		}
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		if resolvedAt != nil {
			s := resolvedAt.UTC().Format(time.RFC3339)
			item.ResolvedAt = &s
		}
		deviations = append(deviations, item)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to iterate deviation rows", err))
		return
	}

	response.JSON(w, r, http.StatusOK, DeviationsResponse{
		MonitorID:  id,
		Deviations: deviations,
	})
}
