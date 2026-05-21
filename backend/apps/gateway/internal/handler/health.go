package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/kite365/idcd/apps/gateway/internal/hub"
	"github.com/redis/go-redis/v9"
)

// HealthHandler handles health check endpoints.
type HealthHandler struct {
	hub    *hub.Hub
	redis  redis.UniversalClient
	logger *slog.Logger
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(h *hub.Hub, rdb redis.UniversalClient, logger *slog.Logger) *HealthHandler {
	return &HealthHandler{
		hub:    h,
		redis:  rdb,
		logger: logger,
	}
}

// HealthResponse is the response format for health checks.
type HealthResponse struct {
	Status  string         `json:"status"`
	Version string         `json:"version"`
	Checks  map[string]any `json:"checks,omitempty"`
}

// Health handles GET /health requests.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	resp := HealthResponse{
		Status:  "ok",
		Version: "0.1.0",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// DeepHealth handles GET /health/deep requests.
// Checks Redis and connection count.
func (h *HealthHandler) DeepHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	checks := make(map[string]any)
	allOK := true

	// Check Redis
	if h.redis != nil {
		if err := h.redis.Ping(ctx).Err(); err != nil {
			checks["redis"] = map[string]string{
				"status": "error",
				"error":  err.Error(),
			}
			allOK = false
		} else {
			checks["redis"] = map[string]string{
				"status": "ok",
			}
		}
	} else {
		checks["redis"] = map[string]string{
			"status": "error",
			"error":  "redis client not configured",
		}
		allOK = false
	}

	// Check Hub connections
	if h.hub != nil {
		checks["connections"] = map[string]any{
			"status": "ok",
			"count":  h.hub.Count(),
		}
	}

	status := "ok"
	statusCode := http.StatusOK
	if !allOK {
		status = "degraded"
		statusCode = http.StatusServiceUnavailable
	}

	resp := HealthResponse{
		Status:  status,
		Version: "0.1.0",
		Checks:  checks,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(resp)
}
