package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// AgentObsPool is the minimal pgx interface required by AgentObsHandler.
// *pgxpool.Pool satisfies this.
type AgentObsPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// AgentObsHandler handles agent observability config and check history endpoints.
type AgentObsHandler struct {
	monitorQ MonitorQuerier
	pool     AgentObsPool
}

// NewAgentObsHandler creates an AgentObsHandler.
func NewAgentObsHandler(mq MonitorQuerier, pool AgentObsPool) *AgentObsHandler {
	return &AgentObsHandler{monitorQ: mq, pool: pool}
}

// validObsTypes is the set of allowed obs_type values.
var validObsTypes = map[string]bool{
	"llm_endpoint": true,
	"tool_api":     true,
	"rag":          true,
}

// --- Request / Response types ---

// AgentObsConfigRequest is the body for POST and PATCH agent-obs config endpoints.
type AgentObsConfigRequest struct {
	ObsType           *string          `json:"obs_type"`
	EndpointURL       *string          `json:"endpoint_url"`
	ModelName         *string          `json:"model_name"`
	ExpectedTokensMax *int             `json:"expected_tokens_max"`
	LatencySLAMs      *int             `json:"latency_sla_ms"`
	PayloadTemplate   *json.RawMessage `json:"payload_template"`
	CheckIntervalS    *int             `json:"check_interval_s"`
}

// AgentObsConfigResponse is the JSON representation of a monitor_agent_obs_configs row.
type AgentObsConfigResponse struct {
	MonitorID         string           `json:"monitor_id"`
	ObsType           string           `json:"obs_type"`
	EndpointURL       string           `json:"endpoint_url"`
	ModelName         *string          `json:"model_name,omitempty"`
	ExpectedTokensMax *int             `json:"expected_tokens_max,omitempty"`
	LatencySLAMs      *int             `json:"latency_sla_ms,omitempty"`
	PayloadTemplate   *json.RawMessage `json:"payload_template,omitempty"`
	CheckIntervalS    int              `json:"check_interval_s"`
	CreatedAt         string           `json:"created_at"`
	UpdatedAt         string           `json:"updated_at"`
}

// AgentObsCheckResponse is the JSON representation of one monitor_agent_obs_checks row.
type AgentObsCheckResponse struct {
	ID              string   `json:"id"`
	MonitorID       string   `json:"monitor_id"`
	ObsType         string   `json:"obs_type"`
	Status          string   `json:"status"`
	LatencyMs       *float64 `json:"latency_ms,omitempty"`
	TokensUsed      *int     `json:"tokens_used,omitempty"`
	ErrorCode       *string  `json:"error_code,omitempty"`
	ResponsePreview *string  `json:"response_preview,omitempty"`
	CheckedAt       string   `json:"checked_at"`
}

// AgentObsChecksListResponse is the paginated list response for checks.
type AgentObsChecksListResponse struct {
	Items  []AgentObsCheckResponse `json:"items"`
	Total  int                     `json:"total"`
	Limit  int                     `json:"limit"`
	Offset int                     `json:"offset"`
}

// ownerCheck verifies the monitor exists and belongs to userID.
// Returns (monitor exists, ok). Writes error response when not ok.
func (h *AgentObsHandler) ownerCheck(w http.ResponseWriter, r *http.Request, monitorID, userID string) bool {
	m, err := h.monitorQ.GetMonitorByID(r.Context(), monitorID)
	if err != nil {
		response.Error(w, r, apperr.NotFound("monitor not found"))
		return false
	}
	if m.UserID != userID {
		response.Error(w, r, apperr.NotFound("monitor not found"))
		return false
	}
	return true
}

// CreateConfig handles POST /v1/monitors/{id}/agent-obs.
func (h *AgentObsHandler) CreateConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	monitorID := chi.URLParam(r, "id")
	if !h.ownerCheck(w, r, monitorID, userID) {
		return
	}

	var req AgentObsConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid JSON request body", ""))
		return
	}

	if req.ObsType == nil || !validObsTypes[*req.ObsType] {
		response.Error(w, r, apperr.Validation("obs_type must be llm_endpoint, tool_api, or rag", "obs_type"))
		return
	}
	if req.EndpointURL == nil || *req.EndpointURL == "" {
		response.Error(w, r, apperr.Validation("endpoint_url is required", "endpoint_url"))
		return
	}

	checkIntervalS := 60
	if req.CheckIntervalS != nil && *req.CheckIntervalS > 0 {
		checkIntervalS = *req.CheckIntervalS
	}

	var payloadJSON []byte
	if req.PayloadTemplate != nil {
		payloadJSON = []byte(*req.PayloadTemplate)
	}

	now := time.Now().UTC()
	_, err := h.pool.Exec(ctx,
		`INSERT INTO monitor_agent_obs_configs
		  (monitor_id, obs_type, endpoint_url, model_name, expected_tokens_max, latency_sla_ms, payload_template, check_interval_s, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		monitorID,
		*req.ObsType,
		*req.EndpointURL,
		req.ModelName,
		req.ExpectedTokensMax,
		req.LatencySLAMs,
		payloadJSON,
		checkIntervalS,
		now,
		now,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to create agent obs config", err))
		return
	}

	cfg, ok := h.fetchConfig(w, r, ctx, monitorID)
	if !ok {
		return
	}
	response.JSON(w, r, http.StatusCreated, cfg)
}

// GetConfig handles GET /v1/monitors/{id}/agent-obs.
func (h *AgentObsHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	monitorID := chi.URLParam(r, "id")
	if !h.ownerCheck(w, r, monitorID, userID) {
		return
	}

	cfg, ok := h.fetchConfig(w, r, ctx, monitorID)
	if !ok {
		return
	}
	response.JSON(w, r, http.StatusOK, cfg)
}

// UpdateConfig handles PATCH /v1/monitors/{id}/agent-obs.
func (h *AgentObsHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	monitorID := chi.URLParam(r, "id")
	if !h.ownerCheck(w, r, monitorID, userID) {
		return
	}

	var req AgentObsConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid JSON request body", ""))
		return
	}

	if req.ObsType != nil && !validObsTypes[*req.ObsType] {
		response.Error(w, r, apperr.Validation("obs_type must be llm_endpoint, tool_api, or rag", "obs_type"))
		return
	}

	var setClauses []string
	var args []any
	argIdx := 1

	if req.ObsType != nil {
		setClauses = append(setClauses, "obs_type=$"+strconv.Itoa(argIdx))
		args = append(args, *req.ObsType)
		argIdx++
	}
	if req.EndpointURL != nil {
		setClauses = append(setClauses, "endpoint_url=$"+strconv.Itoa(argIdx))
		args = append(args, *req.EndpointURL)
		argIdx++
	}
	if req.ModelName != nil {
		setClauses = append(setClauses, "model_name=$"+strconv.Itoa(argIdx))
		args = append(args, *req.ModelName)
		argIdx++
	}
	if req.ExpectedTokensMax != nil {
		setClauses = append(setClauses, "expected_tokens_max=$"+strconv.Itoa(argIdx))
		args = append(args, *req.ExpectedTokensMax)
		argIdx++
	}
	if req.LatencySLAMs != nil {
		setClauses = append(setClauses, "latency_sla_ms=$"+strconv.Itoa(argIdx))
		args = append(args, *req.LatencySLAMs)
		argIdx++
	}
	if req.PayloadTemplate != nil {
		setClauses = append(setClauses, "payload_template=$"+strconv.Itoa(argIdx))
		args = append(args, []byte(*req.PayloadTemplate))
		argIdx++
	}
	if req.CheckIntervalS != nil {
		setClauses = append(setClauses, "check_interval_s=$"+strconv.Itoa(argIdx))
		args = append(args, *req.CheckIntervalS)
		argIdx++
	}

	if len(setClauses) == 0 {
		cfg, ok := h.fetchConfig(w, r, ctx, monitorID)
		if !ok {
			return
		}
		response.JSON(w, r, http.StatusOK, cfg)
		return
	}

	setClauses = append(setClauses, "updated_at=$"+strconv.Itoa(argIdx))
	args = append(args, time.Now().UTC())
	argIdx++
	args = append(args, monitorID)

	sql := "UPDATE monitor_agent_obs_configs SET "
	for i, c := range setClauses {
		if i > 0 {
			sql += ","
		}
		sql += c
	}
	sql += " WHERE monitor_id=$" + strconv.Itoa(argIdx)

	tag, err := h.pool.Exec(ctx, sql, args...)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to update agent obs config", err))
		return
	}
	if tag.RowsAffected() == 0 {
		response.Error(w, r, apperr.NotFound("agent obs config not found"))
		return
	}

	cfg, ok := h.fetchConfig(w, r, ctx, monitorID)
	if !ok {
		return
	}
	response.JSON(w, r, http.StatusOK, cfg)
}

// DeleteConfig handles DELETE /v1/monitors/{id}/agent-obs.
func (h *AgentObsHandler) DeleteConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	monitorID := chi.URLParam(r, "id")
	if !h.ownerCheck(w, r, monitorID, userID) {
		return
	}

	tag, err := h.pool.Exec(ctx,
		`DELETE FROM monitor_agent_obs_configs WHERE monitor_id=$1`,
		monitorID,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to delete agent obs config", err))
		return
	}
	if tag.RowsAffected() == 0 {
		response.Error(w, r, apperr.NotFound("agent obs config not found"))
		return
	}

	response.JSON(w, r, http.StatusNoContent, nil)
}

// ListChecks handles GET /v1/monitors/{id}/agent-obs/checks.
func (h *AgentObsHandler) ListChecks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	monitorID := chi.URLParam(r, "id")
	if !h.ownerCheck(w, r, monitorID, userID) {
		return
	}

	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	rows, err := h.pool.Query(ctx,
		`SELECT id, monitor_id, obs_type, status, latency_ms, tokens_used, error_code, response_preview, checked_at
		 FROM monitor_agent_obs_checks
		 WHERE monitor_id=$1
		 ORDER BY checked_at DESC
		 LIMIT $2 OFFSET $3`,
		monitorID, limit, offset,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query agent obs checks", err))
		return
	}
	defer rows.Close()

	items := make([]AgentObsCheckResponse, 0)
	for rows.Next() {
		var (
			id              string
			mid             string
			obsType         string
			status          string
			latencyMs       pgtype.Float8
			tokensUsed      pgtype.Int4
			errorCode       pgtype.Text
			responsePreview pgtype.Text
			checkedAt       pgtype.Timestamptz
		)
		if err := rows.Scan(&id, &mid, &obsType, &status, &latencyMs, &tokensUsed, &errorCode, &responsePreview, &checkedAt); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan check row", err))
			return
		}
		item := AgentObsCheckResponse{
			ID:        id,
			MonitorID: mid,
			ObsType:   obsType,
			Status:    status,
		}
		if latencyMs.Valid {
			v := latencyMs.Float64
			item.LatencyMs = &v
		}
		if tokensUsed.Valid {
			v := int(tokensUsed.Int32)
			item.TokensUsed = &v
		}
		if errorCode.Valid {
			item.ErrorCode = &errorCode.String
		}
		if responsePreview.Valid {
			item.ResponsePreview = &responsePreview.String
		}
		if checkedAt.Valid {
			item.CheckedAt = checkedAt.Time.UTC().Format(time.RFC3339)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to iterate check rows", err))
		return
	}

	response.JSON(w, r, http.StatusOK, AgentObsChecksListResponse{
		Items:  items,
		Total:  len(items),
		Limit:  limit,
		Offset: offset,
	})
}

// fetchConfig reads a single agent obs config row and converts it to a response.
// Writes error response and returns false if not found or query fails.
func (h *AgentObsHandler) fetchConfig(w http.ResponseWriter, r *http.Request, ctx context.Context, monitorID string) (AgentObsConfigResponse, bool) {
	var (
		obsType           string
		endpointURL       string
		modelName         pgtype.Text
		expectedTokensMax pgtype.Int4
		latencySLAMs      pgtype.Int4
		payloadTemplate   []byte
		checkIntervalS    int
		createdAt         pgtype.Timestamptz
		updatedAt         pgtype.Timestamptz
	)
	err := h.pool.QueryRow(ctx,
		`SELECT obs_type, endpoint_url, model_name, expected_tokens_max, latency_sla_ms,
		        payload_template, check_interval_s, created_at, updated_at
		 FROM monitor_agent_obs_configs
		 WHERE monitor_id=$1`,
		monitorID,
	).Scan(&obsType, &endpointURL, &modelName, &expectedTokensMax, &latencySLAMs,
		&payloadTemplate, &checkIntervalS, &createdAt, &updatedAt)
	if err != nil {
		response.Error(w, r, apperr.NotFound("agent obs config not found"))
		return AgentObsConfigResponse{}, false
	}

	cfg := AgentObsConfigResponse{
		MonitorID:      monitorID,
		ObsType:        obsType,
		EndpointURL:    endpointURL,
		CheckIntervalS: checkIntervalS,
	}
	if modelName.Valid {
		cfg.ModelName = &modelName.String
	}
	if expectedTokensMax.Valid {
		v := int(expectedTokensMax.Int32)
		cfg.ExpectedTokensMax = &v
	}
	if latencySLAMs.Valid {
		v := int(latencySLAMs.Int32)
		cfg.LatencySLAMs = &v
	}
	if len(payloadTemplate) > 0 {
		raw := json.RawMessage(payloadTemplate)
		cfg.PayloadTemplate = &raw
	}
	if createdAt.Valid {
		cfg.CreatedAt = createdAt.Time.UTC().Format(time.RFC3339)
	}
	if updatedAt.Valid {
		cfg.UpdatedAt = updatedAt.Time.UTC().Format(time.RFC3339)
	}

	return cfg, true
}
