package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// MonitorStreamPool is the minimal pgx interface needed by MonitorStreamHandler.
// *pgxpool.Pool satisfies this.
type MonitorStreamPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// MonitorStreamHandler serves the SSE stream for a single monitor.
type MonitorStreamHandler struct {
	monitorQ MonitorQuerier
	pool     MonitorStreamPool
}

// NewMonitorStreamHandler creates a MonitorStreamHandler.
func NewMonitorStreamHandler(mq MonitorQuerier, pool MonitorStreamPool) *MonitorStreamHandler {
	return &MonitorStreamHandler{monitorQ: mq, pool: pool}
}

// monitorCheckSSEEvent is the JSON payload pushed over SSE.
type monitorCheckSSEEvent struct {
	MonitorID string `json:"monitor_id"`
	NodeID    string `json:"node_id"`
	Status    string `json:"status"`
	LatencyMs *int32 `json:"latency_ms"`
	CheckedAt string `json:"checked_at"`
	Error     string `json:"error"`
}

// Stream handles GET /v1/monitors/{id}/stream.
func (h *MonitorStreamHandler) Stream(w http.ResponseWriter, r *http.Request) {
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	lastCheckAt := ""

	sendLatestCheck := func() {
		var checkAt pgtype.Timestamptz
		var monitorID, nodeID, status string
		var latencyMs *int32
		var checkErr *string

		row := h.pool.QueryRow(ctx,
			`SELECT check_at, monitor_id, node_id, status, latency_ms, error
			   FROM monitor_checks
			  WHERE monitor_id = $1
			  ORDER BY check_at DESC
			  LIMIT 1`,
			id,
		)
		if err := row.Scan(&checkAt, &monitorID, &nodeID, &status, &latencyMs, &checkErr); err != nil {
			fmt.Fprintf(w, "event: empty\ndata: {}\n\n")
			flusher.Flush()
			return
		}

		checkedAt := ""
		if checkAt.Valid {
			checkedAt = checkAt.Time.UTC().Format("2006-01-02T15:04:05Z")
		}

		if checkedAt == lastCheckAt {
			return
		}
		lastCheckAt = checkedAt

		errStr := ""
		if checkErr != nil {
			errStr = *checkErr
		}

		evt := monitorCheckSSEEvent{
			MonitorID: monitorID,
			NodeID:    nodeID,
			Status:    status,
			LatencyMs: latencyMs,
			CheckedAt: checkedAt,
			Error:     errStr,
		}
		data, err := json.Marshal(evt)
		if err != nil {
			return
		}
		fmt.Fprintf(w, "event: check\ndata: %s\n\n", data)
		flusher.Flush()
	}

	sendLatestCheck()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	pingTicker := time.NewTicker(15 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sendLatestCheck()
		case <-pingTicker.C:
			fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
			flusher.Flush()
		}
	}
}
