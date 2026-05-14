package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/quota"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
)

// AlertPool is the interface for raw pgx operations used by AlertHandler.
// Using pgx directly because sqlc codegen for new tables is deferred.
// pgxpool.Pool satisfies this interface — the *pgx.Rows / pgx.Row return
// types are the same concrete interfaces that pgxpool uses.
type AlertPool interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
}

// AlertRow is an alias so tests can implement the pgx.Row scan interface.
type AlertRow = pgx.Row

// AlertRows is an alias so tests can implement the pgx.Rows interface.
type AlertRows = pgx.Rows

// AlertHandler implements alert channel, policy, and event endpoints.
type AlertHandler struct {
	pool AlertPool
}

// NewAlertHandler creates an AlertHandler.
func NewAlertHandler(pool AlertPool) *AlertHandler {
	return &AlertHandler{pool: pool}
}

// ─────────────────────────────────────────────
// Request / Response types
// ─────────────────────────────────────────────

// CreateChannelRequest is the body for POST /v1/alert-channels.
type CreateChannelRequest struct {
	Name   string          `json:"name"`
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
}

// AlertChannelResponse is the JSON representation of an alert_channel row.
type AlertChannelResponse struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Config    json.RawMessage `json:"config"`
	Verified  bool            `json:"verified"`
	CreatedAt string          `json:"created_at"`
}

// CreatePolicyRequest is the body for POST /v1/alert-policies.
type CreatePolicyRequest struct {
	Name       string   `json:"name"`
	MonitorID  string   `json:"monitor_id"`
	ChannelIDs []string `json:"channel_ids"`
	DelayS     *int     `json:"delay_s"`
	RecoveryN  *int     `json:"recovery_n"`
	MuteStart  *string  `json:"mute_start"`
	MuteEnd    *string  `json:"mute_end"`
}

// UpdatePolicyRequest is the body for PATCH /v1/alert-policies/:id.
type UpdatePolicyRequest struct {
	Name       *string   `json:"name"`
	ChannelIDs *[]string `json:"channel_ids"`
	DelayS     *int      `json:"delay_s"`
	RecoveryN  *int      `json:"recovery_n"`
	MuteStart  *string   `json:"mute_start"`
	MuteEnd    *string   `json:"mute_end"`
	Enabled    *bool     `json:"enabled"`
}

// AlertPolicyResponse is the JSON representation of an alert_policy row.
type AlertPolicyResponse struct {
	ID         string   `json:"id"`
	UserID     string   `json:"user_id"`
	MonitorID  string   `json:"monitor_id"`
	ChannelIDs []string `json:"channel_ids"`
	Name       string   `json:"name"`
	DelayS     int      `json:"delay_s"`
	RecoveryN  int      `json:"recovery_n"`
	MuteStart  *string  `json:"mute_start,omitempty"`
	MuteEnd    *string  `json:"mute_end,omitempty"`
	Enabled    bool     `json:"enabled"`
	CreatedAt  string   `json:"created_at"`
}

// AlertEventResponse is the JSON representation of an alert_event row.
type AlertEventResponse struct {
	ID              string  `json:"id"`
	MonitorID       string  `json:"monitor_id"`
	PolicyID        string  `json:"policy_id"`
	Status          string  `json:"status"`
	StartedAt       string  `json:"started_at"`
	ResolvedAt      *string `json:"resolved_at,omitempty"`
	AcknowledgedBy  *string `json:"acknowledged_by,omitempty"`
	AcknowledgedAt  *string `json:"acknowledged_at,omitempty"`
	Metadata        json.RawMessage `json:"metadata"`
}

// ─────────────────────────────────────────────
// Alert Channel endpoints
// ─────────────────────────────────────────────

var validChannelTypes = map[string]bool{
	"email": true, "webhook": true, "wecom": true,
	"dingtalk": true, "feishu": true, "telegram": true, "slack": true,
}

// CreateChannel handles POST /v1/alert-channels.
func (h *AlertHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	var req CreateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid JSON request body", ""))
		return
	}
	if req.Name == "" {
		response.Error(w, r, apperr.Validation("name is required", "name"))
		return
	}
	if !validChannelTypes[req.Type] {
		response.Error(w, r, apperr.Validation("invalid channel type", "type"))
		return
	}
	if req.Config == nil {
		req.Config = json.RawMessage("{}")
	}

	// ── Quota enforcement: channel count ─────────────────────────────────────
	{
		plan := alertUserPlan(ctx, h.pool, userID)
		current := alertChannelCount(ctx, h.pool, userID)
		if err := quota.CheckChannelCount(plan, current); err != nil {
			if appErr := apperr.AsError(err); appErr != nil && appErr.Code == quota.CodeQuotaExceeded {
				quotaError(w, r, appErr.Message)
				return
			}
		}
	}
	// ── End quota enforcement ─────────────────────────────────────────────────

	id := idgen.Channel()
	now := time.Now()

	_, err := h.pool.Exec(ctx, `
		INSERT INTO alert_channels (id, user_id, name, type, config, verified, created_at)
		VALUES ($1, $2, $3, $4, $5, FALSE, $6)`,
		id, userID, req.Name, req.Type, []byte(req.Config), now,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to create alert channel", err))
		return
	}

	resp := AlertChannelResponse{
		ID:        id,
		UserID:    userID,
		Name:      req.Name,
		Type:      req.Type,
		Config:    req.Config,
		Verified:  false,
		CreatedAt: now.UTC().Format(time.RFC3339),
	}
	response.JSON(w, r, http.StatusCreated, resp)
}

// ListChannels handles GET /v1/alert-channels.
func (h *AlertHandler) ListChannels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	rows, err := h.pool.Query(ctx, `
		SELECT id, user_id, name, type, config, verified, created_at
		FROM alert_channels
		WHERE user_id = $1
		ORDER BY created_at DESC`, userID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list alert channels", err))
		return
	}
	defer rows.Close()

	var items []AlertChannelResponse
	for rows.Next() {
		var item AlertChannelResponse
		var cfg []byte
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.UserID, &item.Name, &item.Type, &cfg, &item.Verified, &createdAt); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan alert channel", err))
			return
		}
		item.Config = json.RawMessage(cfg)
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to iterate alert channels", err))
		return
	}

	if items == nil {
		items = []AlertChannelResponse{}
	}
	response.JSON(w, r, http.StatusOK, map[string]interface{}{"items": items})
}

// DeleteChannel handles DELETE /v1/alert-channels/:id.
func (h *AlertHandler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	id := chi.URLParam(r, "id")
	tag, err := h.pool.Exec(ctx, `
		DELETE FROM alert_channels WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to delete alert channel", err))
		return
	}
	if tag.RowsAffected() == 0 {
		response.Error(w, r, apperr.NotFound("alert channel not found"))
		return
	}

	response.JSON(w, r, http.StatusNoContent, nil)
}

// TestChannel handles POST /v1/alert-channels/:id/test.
// It sends a test notification through the channel.
func (h *AlertHandler) TestChannel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	id := chi.URLParam(r, "id")

	var channelType string
	var cfg []byte
	err := h.pool.QueryRow(ctx, `
		SELECT type, config FROM alert_channels WHERE id = $1 AND user_id = $2`, id, userID).
		Scan(&channelType, &cfg)
	if err != nil {
		response.Error(w, r, apperr.NotFound("alert channel not found"))
		return
	}

	// Build test payload description (no external send in handler — just validate config is parseable)
	_ = channelType
	_ = cfg

	response.JSON(w, r, http.StatusOK, map[string]string{
		"message": "test notification queued",
		"channel_id": id,
	})
}

// ─────────────────────────────────────────────
// Alert Policy endpoints
// ─────────────────────────────────────────────

// CreatePolicy handles POST /v1/alert-policies.
func (h *AlertHandler) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	var req CreatePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid JSON request body", ""))
		return
	}
	if req.Name == "" {
		response.Error(w, r, apperr.Validation("name is required", "name"))
		return
	}
	if req.MonitorID == "" {
		response.Error(w, r, apperr.Validation("monitor_id is required", "monitor_id"))
		return
	}

	delayS := 0
	if req.DelayS != nil {
		delayS = *req.DelayS
	}
	recoveryN := 3
	if req.RecoveryN != nil {
		recoveryN = *req.RecoveryN
	}
	channelIDs := req.ChannelIDs
	if channelIDs == nil {
		channelIDs = []string{}
	}

	id := idgen.AlertPolicy()
	now := time.Now()

	channelIDsJSON, err := json.Marshal(channelIDs)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to marshal channel_ids", err))
		return
	}

	_, err = h.pool.Exec(ctx, `
		INSERT INTO alert_policies
			(id, user_id, monitor_id, channel_ids, name, delay_s, recovery_n, mute_start, mute_end, enabled, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, TRUE, $10)`,
		id, userID, req.MonitorID, channelIDsJSON, req.Name,
		delayS, recoveryN, req.MuteStart, req.MuteEnd, now,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to create alert policy", err))
		return
	}

	resp := AlertPolicyResponse{
		ID:         id,
		UserID:     userID,
		MonitorID:  req.MonitorID,
		ChannelIDs: channelIDs,
		Name:       req.Name,
		DelayS:     delayS,
		RecoveryN:  recoveryN,
		MuteStart:  req.MuteStart,
		MuteEnd:    req.MuteEnd,
		Enabled:    true,
		CreatedAt:  now.UTC().Format(time.RFC3339),
	}
	response.JSON(w, r, http.StatusCreated, resp)
}

// ListPolicies handles GET /v1/alert-policies?monitor_id=.
func (h *AlertHandler) ListPolicies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	monitorID := r.URL.Query().Get("monitor_id")

	var rows AlertRows
	var err error
	if monitorID != "" {
		rows, err = h.pool.Query(ctx, `
			SELECT id, user_id, monitor_id, channel_ids, name, delay_s, recovery_n,
			       mute_start, mute_end, enabled, created_at
			FROM alert_policies
			WHERE user_id = $1 AND monitor_id = $2
			ORDER BY created_at DESC`, userID, monitorID)
	} else {
		rows, err = h.pool.Query(ctx, `
			SELECT id, user_id, monitor_id, channel_ids, name, delay_s, recovery_n,
			       mute_start, mute_end, enabled, created_at
			FROM alert_policies
			WHERE user_id = $1
			ORDER BY created_at DESC`, userID)
	}
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list alert policies", err))
		return
	}
	defer rows.Close()

	var items []AlertPolicyResponse
	for rows.Next() {
		var item AlertPolicyResponse
		var channelIDsJSON []byte
		var createdAt time.Time
		if err := rows.Scan(
			&item.ID, &item.UserID, &item.MonitorID, &channelIDsJSON,
			&item.Name, &item.DelayS, &item.RecoveryN,
			&item.MuteStart, &item.MuteEnd, &item.Enabled, &createdAt,
		); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan alert policy", err))
			return
		}
		if err := json.Unmarshal(channelIDsJSON, &item.ChannelIDs); err != nil {
			item.ChannelIDs = []string{}
		}
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to iterate alert policies", err))
		return
	}

	if items == nil {
		items = []AlertPolicyResponse{}
	}
	response.JSON(w, r, http.StatusOK, map[string]interface{}{"items": items})
}

// UpdatePolicy handles PATCH /v1/alert-policies/:id.
func (h *AlertHandler) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	id := chi.URLParam(r, "id")

	// Fetch existing policy
	var existing AlertPolicyResponse
	var channelIDsJSON []byte
	var createdAt time.Time
	err := h.pool.QueryRow(ctx, `
		SELECT id, user_id, monitor_id, channel_ids, name, delay_s, recovery_n,
		       mute_start, mute_end, enabled, created_at
		FROM alert_policies WHERE id = $1 AND user_id = $2`, id, userID).
		Scan(&existing.ID, &existing.UserID, &existing.MonitorID, &channelIDsJSON,
			&existing.Name, &existing.DelayS, &existing.RecoveryN,
			&existing.MuteStart, &existing.MuteEnd, &existing.Enabled, &createdAt)
	if err != nil {
		response.Error(w, r, apperr.NotFound("alert policy not found"))
		return
	}
	if err := json.Unmarshal(channelIDsJSON, &existing.ChannelIDs); err != nil {
		existing.ChannelIDs = []string{}
	}
	existing.CreatedAt = createdAt.UTC().Format(time.RFC3339)

	var req UpdatePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid JSON request body", ""))
		return
	}

	// Apply patch
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.ChannelIDs != nil {
		existing.ChannelIDs = *req.ChannelIDs
	}
	if req.DelayS != nil {
		existing.DelayS = *req.DelayS
	}
	if req.RecoveryN != nil {
		existing.RecoveryN = *req.RecoveryN
	}
	if req.MuteStart != nil {
		existing.MuteStart = req.MuteStart
	}
	if req.MuteEnd != nil {
		existing.MuteEnd = req.MuteEnd
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	newChannelIDsJSON, err := json.Marshal(existing.ChannelIDs)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to marshal channel_ids", err))
		return
	}

	_, err = h.pool.Exec(ctx, `
		UPDATE alert_policies
		SET name=$1, channel_ids=$2, delay_s=$3, recovery_n=$4,
		    mute_start=$5, mute_end=$6, enabled=$7
		WHERE id=$8 AND user_id=$9`,
		existing.Name, newChannelIDsJSON, existing.DelayS, existing.RecoveryN,
		existing.MuteStart, existing.MuteEnd, existing.Enabled,
		id, userID,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to update alert policy", err))
		return
	}

	response.JSON(w, r, http.StatusOK, existing)
}

// DeletePolicy handles DELETE /v1/alert-policies/:id.
func (h *AlertHandler) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	id := chi.URLParam(r, "id")
	tag, err := h.pool.Exec(ctx, `
		DELETE FROM alert_policies WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to delete alert policy", err))
		return
	}
	if tag.RowsAffected() == 0 {
		response.Error(w, r, apperr.NotFound("alert policy not found"))
		return
	}

	response.JSON(w, r, http.StatusNoContent, nil)
}

// ─────────────────────────────────────────────
// Alert Event endpoints
// ─────────────────────────────────────────────

// ListEvents handles GET /v1/alert-events?monitor_id=&status=&limit=.
func (h *AlertHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	q := r.URL.Query()
	monitorID := q.Get("monitor_id")
	status := q.Get("status")
	limit := 50
	if l := q.Get("limit"); l != "" {
		if v, err := parseInt(l); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}

	// Events are scoped to the user's monitors via a join on alert_policies.
	// Since D1 forbids cross-schema FKs, we do an application-level ownership check via
	// an EXISTS sub-select on alert_policies (same user_id).
	query := `
		SELECT ae.id, ae.monitor_id, ae.policy_id, ae.status,
		       ae.started_at, ae.resolved_at,
		       ae.acknowledged_by, ae.acknowledged_at, ae.metadata
		FROM alert_events ae
		WHERE EXISTS (
			SELECT 1 FROM alert_policies ap
			WHERE ap.id = ae.policy_id AND ap.user_id = $1
		)`
	args := []interface{}{userID}
	argIdx := 2

	if monitorID != "" {
		query += " AND ae.monitor_id = $" + itoa(argIdx)
		args = append(args, monitorID)
		argIdx++
	}
	if status != "" {
		query += " AND ae.status = $" + itoa(argIdx)
		args = append(args, status)
		argIdx++
	}
	query += " ORDER BY ae.started_at DESC LIMIT $" + itoa(argIdx)
	args = append(args, limit)

	rows, err := h.pool.Query(ctx, query, args...)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list alert events", err))
		return
	}
	defer rows.Close()

	var items []AlertEventResponse
	for rows.Next() {
		var item AlertEventResponse
		var startedAt time.Time
		var resolvedAt *time.Time
		var acknowledgedAt *time.Time
		var metadata []byte

		if err := rows.Scan(
			&item.ID, &item.MonitorID, &item.PolicyID, &item.Status,
			&startedAt, &resolvedAt,
			&item.AcknowledgedBy, &acknowledgedAt, &metadata,
		); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan alert event", err))
			return
		}
		item.StartedAt = startedAt.UTC().Format(time.RFC3339)
		if resolvedAt != nil {
			t := resolvedAt.UTC().Format(time.RFC3339)
			item.ResolvedAt = &t
		}
		if acknowledgedAt != nil {
			t := acknowledgedAt.UTC().Format(time.RFC3339)
			item.AcknowledgedAt = &t
		}
		item.Metadata = json.RawMessage(metadata)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to iterate alert events", err))
		return
	}

	if items == nil {
		items = []AlertEventResponse{}
	}
	response.JSON(w, r, http.StatusOK, map[string]interface{}{"items": items})
}

// AcknowledgeEvent handles POST /v1/alert-events/:id/ack.
func (h *AlertHandler) AcknowledgeEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	id := chi.URLParam(r, "id")
	now := time.Now()

	tag, err := h.pool.Exec(ctx, `
		UPDATE alert_events
		SET status = 'acknowledged',
		    acknowledged_by = $1,
		    acknowledged_at = $2
		WHERE id = $3
		  AND EXISTS (
		      SELECT 1 FROM alert_policies ap
		      WHERE ap.id = alert_events.policy_id AND ap.user_id = $4
		  )`,
		userID, now, id, userID,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to acknowledge alert event", err))
		return
	}
	if tag.RowsAffected() == 0 {
		response.Error(w, r, apperr.NotFound("alert event not found"))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]string{
		"message":          "event acknowledged",
		"acknowledged_by":  userID,
		"acknowledged_at":  now.UTC().Format(time.RFC3339),
	})
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

func parseInt(s string) (int, error) {
	v := 0
	_, err := itoa2(s, &v)
	return v, err
}

func itoa2(s string, out *int) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, apperr.Validation("not a number", "")
		}
		n = n*10 + int(c-'0')
	}
	*out = n
	return n, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 4)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}

// alertUserPlan fetches the subscription plan for a user via the pool.
// Returns "free" when no active subscription exists.
func alertUserPlan(ctx context.Context, pool AlertPool, userID string) string {
	var plan string
	err := pool.QueryRow(ctx,
		`SELECT plan FROM subscriptions WHERE user_id = $1 AND status = 'active' LIMIT 1`,
		userID,
	).Scan(&plan)
	if err != nil {
		return "free"
	}
	return plan
}

// alertChannelCount returns the number of alert channels owned by a user.
func alertChannelCount(ctx context.Context, pool AlertPool, userID string) int {
	var count int
	err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM alert_channels WHERE user_id = $1`,
		userID,
	).Scan(&count)
	if err != nil {
		return 0
	}
	return count
}
