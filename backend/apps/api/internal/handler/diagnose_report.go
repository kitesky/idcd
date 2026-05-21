// Package handler provides HTTP handlers for the idcd API.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

const diagnoseTTL = 7 * 24 * time.Hour // 7 days

// DiagnoseReportHandler handles saving and retrieving diagnose reports via Redis.
type DiagnoseReportHandler struct {
	redis redis.UniversalClient
}

// NewDiagnoseReportHandler creates a new DiagnoseReportHandler.
func NewDiagnoseReportHandler(rdb redis.UniversalClient) *DiagnoseReportHandler {
	return &DiagnoseReportHandler{redis: rdb}
}

// SaveReport handles POST /v1/diagnose/reports
func (h *DiagnoseReportHandler) SaveReport(w http.ResponseWriter, r *http.Request) {
	var body json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, r, apperr.Validation("invalid JSON", ""))
		return
	}

	// Extract id from body
	var meta struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &meta); err != nil || meta.ID == "" {
		response.Error(w, r, apperr.Validation("missing id field", ""))
		return
	}

	key := "diagnose:report:" + meta.ID
	if err := h.redis.Set(r.Context(), key, []byte(body), diagnoseTTL).Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to save report", err))
		return
	}

	response.JSON(w, r, http.StatusCreated, map[string]string{"id": meta.ID})
}

// GetReport handles GET /v1/diagnose/reports/{id}
func (h *DiagnoseReportHandler) GetReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, r, apperr.Validation("missing id", ""))
		return
	}

	key := "diagnose:report:" + id
	data, err := h.redis.Get(r.Context(), key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			response.Error(w, r, apperr.NotFound("report not found"))
			return
		}
		response.Error(w, r, apperr.Internal("failed to get report", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
