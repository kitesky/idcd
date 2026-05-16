// Package handler implements HTTP handlers for the API server.
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/apps/api/internal/denylist"
	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/quota"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
)

// MonitorQuerier is the subset of DB operations required by MonitorHandler.
type MonitorQuerier interface {
	GetMonitorByID(ctx context.Context, id string) (idcdmain.Monitor, error)
	ListMonitorsByUser(ctx context.Context, arg idcdmain.ListMonitorsByUserParams) ([]idcdmain.Monitor, error)
	CreateMonitor(ctx context.Context, arg idcdmain.CreateMonitorParams) (idcdmain.Monitor, error)
	UpdateMonitorStatus(ctx context.Context, arg idcdmain.UpdateMonitorStatusParams) (idcdmain.Monitor, error)
	UpdateMonitorFields(ctx context.Context, arg idcdmain.UpdateMonitorFieldsParams) (idcdmain.Monitor, error)
	DeleteMonitor(ctx context.Context, id string) error
}

// QuotaPool is the minimal pgx interface needed to perform quota-related DB
// lookups (subscription plan + monitor count).
// *pgxpool.Pool satisfies this.
type QuotaPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// BulkPool extends QuotaPool with Exec and Query needed for bulk operations.
// *pgxpool.Pool satisfies this.
type BulkPool interface {
	QuotaPool
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// MonitorHandler handles monitor CRUD endpoints.
type MonitorHandler struct {
	q        MonitorQuerier
	pool     QuotaPool // optional; nil disables DB-backed quota checks
	bulkPool BulkPool  // optional; nil disables bulk operations
}

// NewMonitorHandler creates a MonitorHandler wired to the given querier.
// No quota DB pool is set; quota checks are skipped unless WithQuotaPool is called.
func NewMonitorHandler(q MonitorQuerier) *MonitorHandler {
	return &MonitorHandler{q: q}
}

// WithQuotaPool returns a copy of the handler with the quota pool configured.
// Call this in server setup to enable per-plan quota enforcement.
func (h *MonitorHandler) WithQuotaPool(pool QuotaPool) *MonitorHandler {
	return &MonitorHandler{q: h.q, pool: pool, bulkPool: h.bulkPool}
}

// WithBulkPool returns a copy of the handler with the bulk pool configured.
// Call this in server setup to enable bulk operations.
func (h *MonitorHandler) WithBulkPool(pool BulkPool) *MonitorHandler {
	return &MonitorHandler{q: h.q, pool: h.pool, bulkPool: pool}
}

// userPlan fetches the subscription plan for a user.
// Returns "free" when no active subscription exists (free tier by default).
func (h *MonitorHandler) userPlan(ctx context.Context, userID string) string {
	if h.pool == nil {
		return "free"
	}
	var plan string
	err := h.pool.QueryRow(ctx,
		`SELECT plan FROM subscriptions WHERE user_id = $1 AND status = 'active' LIMIT 1`,
		userID,
	).Scan(&plan)
	if err != nil {
		return "free"
	}
	return plan
}

// monitorCount returns the number of non-archived monitors owned by a user.
func (h *MonitorHandler) monitorCount(ctx context.Context, userID string) int {
	if h.pool == nil {
		return 0
	}
	var count int
	err := h.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM monitors WHERE user_id = $1 AND status != 'archived'`,
		userID,
	).Scan(&count)
	if err != nil {
		return 0
	}
	return count
}

// quotaError writes a 402 Payment Required response for quota exceeded errors.
// The response body includes an upgrade_url hint for the frontend.
func quotaError(w http.ResponseWriter, _ *http.Request, msg string) {
	type quotaBody struct {
		Error      string `json:"error"`
		Message    string `json:"message"`
		UpgradeURL string `json:"upgrade_url"`
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusPaymentRequired)
	_ = json.NewEncoder(w).Encode(quotaBody{
		Error:      "quota_exceeded",
		Message:    msg,
		UpgradeURL: "/app/billing",
	})
}

// checkQuotaErr returns true if the error is a quota exceeded error and writes
// the 402 response. Returns false (no action taken) when err is nil or another
// error type.
func checkQuotaErr(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	if appErr := apperr.AsError(err); appErr != nil && appErr.Code == quota.CodeQuotaExceeded {
		quotaError(w, r, appErr.Message)
		return true
	}
	return false
}

// --- Request / Response types ---

// CreateMonitorRequest is the body for POST /v1/monitors.
type CreateMonitorRequest struct {
	Name       string          `json:"name"`
	Type       string          `json:"type"`
	Target     string          `json:"target"`
	Config     json.RawMessage `json:"config"`
	IntervalS  int32           `json:"interval_s"`
	NodeCount  int32           `json:"node_count"`
}

// UpdateMonitorRequest is the body for PATCH /v1/monitors/:id.
type UpdateMonitorRequest struct {
	Name      *string          `json:"name"`
	Config    *json.RawMessage `json:"config"`
	IntervalS *int32           `json:"interval_s"`
	Status    *string          `json:"status"`
}

// MonitorResponse is the JSON representation of a monitor.
type MonitorResponse struct {
	ID             string          `json:"id"`
	UserID         string          `json:"user_id"`
	Name           string          `json:"name"`
	Type           string          `json:"type"`
	Target         string          `json:"target"`
	Config         json.RawMessage `json:"config"`
	IntervalS      int32           `json:"interval_s"`
	NodeCount      int32           `json:"node_count"`
	Status         string          `json:"status"`
	LastCheckAt    *string         `json:"last_check_at,omitempty"`
	NextCheckAt    *string         `json:"next_check_at,omitempty"`
	CreatedAt      string          `json:"created_at"`
	UpdatedAt      string          `json:"updated_at"`
	UptimePercent  float64         `json:"uptime_percent"`
}

// MonitorListResponse is the paginated list response.
type MonitorListResponse struct {
	Items  []MonitorResponse `json:"items"`
	Total  int               `json:"total"`
	Page   int               `json:"page"`
	Limit  int               `json:"limit"`
}

// BulkActionRequest is the body for POST /v1/monitors/bulk.
type BulkActionRequest struct {
	IDs    []string `json:"ids"`
	Action string   `json:"action"`
}

// BulkActionResponse is returned by POST /v1/monitors/bulk.
type BulkActionResponse struct {
	Succeeded []string `json:"succeeded"`
	Failed    []string `json:"failed"`
	Total     int      `json:"total"`
}

// validMonitorTypes is the allowed set for monitor type field.
var validMonitorTypes = map[string]bool{
	"http": true, "https": true, "ping": true, "tcp": true,
	"dns": true, "ssl_expiry": true, "domain_expiry": true,
	"icp_change": true, "keyword": true,
}

// validIntervals is the allowed set for interval_s.
var validIntervals = map[int32]bool{
	60: true, 300: true, 1800: true,
}

// --- Handlers ---

// Create handles POST /v1/monitors.
func (h *MonitorHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	var req CreateMonitorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid JSON request body", ""))
		return
	}

	// Validate type
	if !validMonitorTypes[req.Type] {
		response.Error(w, r, apperr.Validation("invalid monitor type", "type"))
		return
	}

	// Validate target (SSRF protection via denylist)
	if _, err := denylist.CheckTarget(req.Target); err != nil {
		response.Error(w, r, apperr.Validation(err.Error(), "target"))
		return
	}

	// Validate interval_s
	if req.IntervalS == 0 {
		req.IntervalS = 300
	}
	if !validIntervals[req.IntervalS] {
		response.Error(w, r, apperr.Validation("interval_s must be 60, 300, or 1800", "interval_s"))
		return
	}

	// ── Quota enforcement ────────────────────────────────────────────────────
	plan := h.userPlan(ctx, userID)
	current := h.monitorCount(ctx, userID)

	if checkQuotaErr(w, r, quota.CheckMonitorCount(plan, current)) {
		return
	}
	if checkQuotaErr(w, r, quota.CheckMonitorInterval(plan, int(req.IntervalS))) {
		return
	}
	// Only enforce node count quota when the user explicitly requested nodes.
	if req.NodeCount > 0 {
		if checkQuotaErr(w, r, quota.CheckNodeCount(plan, int(req.NodeCount))) {
			return
		}
	}
	// ── End quota enforcement ─────────────────────────────────────────────────

	// Default node_count (applied after quota check so default doesn't trigger quota errors).
	if req.NodeCount == 0 {
		limits := quota.Limits(plan)
		if limits.MaxNodes > 0 {
			req.NodeCount = 1 // safe default: minimum valid value
		} else {
			req.NodeCount = 3 // unlimited plan: use the original default
		}
	}

	// Default config
	if req.Config == nil {
		req.Config = json.RawMessage("{}")
	}

	monitorID := idgen.New("mon_")

	m, err := h.q.CreateMonitor(ctx, idcdmain.CreateMonitorParams{
		ID:        monitorID,
		UserID:    userID,
		Name:      req.Name,
		Type:      req.Type,
		Target:    req.Target,
		Config:    []byte(req.Config),
		IntervalS: req.IntervalS,
		NodeCount: req.NodeCount,
	})
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to create monitor", err))
		return
	}

	response.JSON(w, r, http.StatusCreated, monitorToResponse(m))
}

// List handles GET /v1/monitors?page=1&limit=20&search=xxx&status=UP.
func (h *MonitorHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	page, limit := parsePagination(r)
	search := r.URL.Query().Get("search")
	statusFilter := r.URL.Query().Get("status")

	// Normalise frontend status labels (UP/DOWN/PAUSED) → DB values.
	switch statusFilter {
	case "UP":
		statusFilter = "active"
	case "DOWN":
		statusFilter = "down"
	case "PAUSED":
		statusFilter = "paused"
	}

	// If search or status filter is present and bulkPool is available, use raw SQL
	// so we can apply dynamic WHERE clauses that sqlc-generated code doesn't support.
	if (search != "" || statusFilter != "") && h.bulkPool != nil {
		rawSQL := `SELECT id, user_id, name, type, target, config, interval_s, node_count, status,
		                   last_check_at, next_check_at, created_at, updated_at
		            FROM monitors
		            WHERE user_id = $1 AND status != 'archived'`
		args := []any{userID}
		argIdx := 2
		if search != "" {
			rawSQL += fmt.Sprintf(" AND name ILIKE $%d", argIdx)
			args = append(args, "%"+search+"%")
			argIdx++
		}
		if statusFilter != "" {
			rawSQL += fmt.Sprintf(" AND status = $%d", argIdx)
			args = append(args, statusFilter)
			argIdx++
		}

		// Count total matching rows first.
		countSQL := `SELECT COUNT(*) FROM monitors WHERE user_id = $1 AND status != 'archived'`
		countArgs := []any{userID}
		countArgIdx := 2
		if search != "" {
			countSQL += fmt.Sprintf(" AND name ILIKE $%d", countArgIdx)
			countArgs = append(countArgs, "%"+search+"%")
			countArgIdx++
		}
		if statusFilter != "" {
			countSQL += fmt.Sprintf(" AND status = $%d", countArgIdx)
			countArgs = append(countArgs, statusFilter)
		}
		var total int
		_ = h.bulkPool.QueryRow(ctx, countSQL, countArgs...).Scan(&total)

		rawSQL += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
		args = append(args, int32(limit), int32((page-1)*limit))

		rows, err := h.bulkPool.Query(ctx, rawSQL, args...)
		if err != nil {
			response.Error(w, r, apperr.Internal("failed to list monitors", err))
			return
		}
		defer rows.Close()

		var items []MonitorResponse
		for rows.Next() {
			var m idcdmain.Monitor
			if err := rows.Scan(
				&m.ID, &m.UserID, &m.Name, &m.Type, &m.Target, &m.Config,
				&m.IntervalS, &m.NodeCount, &m.Status,
				&m.LastCheckAt, &m.NextCheckAt, &m.CreatedAt, &m.UpdatedAt,
			); err != nil {
				response.Error(w, r, apperr.Internal("failed to scan monitor", err))
				return
			}
			items = append(items, monitorToResponse(m))
		}
		if err := rows.Err(); err != nil {
			response.Error(w, r, apperr.Internal("failed to iterate monitors", err))
			return
		}
		if items == nil {
			items = []MonitorResponse{}
		}

		// Batch-fetch 24h uptime for all returned monitors.
		if len(items) > 0 {
			ids := make([]string, len(items))
			for i, item := range items {
				ids[i] = item.ID
			}
			if uptimeMap := h.fetchUptimeMap(ctx, ids); uptimeMap != nil {
				for i := range items {
					if pct, ok := uptimeMap[items[i].ID]; ok {
						items[i].UptimePercent = pct
					}
				}
			}
		}

		response.JSON(w, r, http.StatusOK, MonitorListResponse{
			Items: items,
			Total: total,
			Page:  page,
			Limit: limit,
		})
		return
	}

	// No filters — use the fast sqlc-generated path.
	ms, err := h.q.ListMonitorsByUser(ctx, idcdmain.ListMonitorsByUserParams{
		UserID: userID,
		Limit:  int32(limit),
		Offset: int32((page - 1) * limit),
	})
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list monitors", err))
		return
	}

	items := make([]MonitorResponse, len(ms))
	for i, m := range ms {
		items[i] = monitorToResponse(m)
	}

	// Batch-fetch 24h uptime for all returned monitors.
	if len(items) > 0 {
		ids := make([]string, len(items))
		for i, item := range items {
			ids[i] = item.ID
		}
		if uptimeMap := h.fetchUptimeMap(ctx, ids); uptimeMap != nil {
			for i := range items {
				if pct, ok := uptimeMap[items[i].ID]; ok {
					items[i].UptimePercent = pct
				}
			}
		}
	}

	// Count total for accurate pagination.
	var total int
	if h.bulkPool != nil {
		_ = h.bulkPool.QueryRow(ctx,
			`SELECT COUNT(*) FROM monitors WHERE user_id = $1 AND status != 'archived'`,
			userID,
		).Scan(&total)
	} else {
		total = len(items)
	}

	response.JSON(w, r, http.StatusOK, MonitorListResponse{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// Get handles GET /v1/monitors/:id.
func (h *MonitorHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	id := chi.URLParam(r, "id")
	m, err := h.q.GetMonitorByID(ctx, id)
	if err != nil {
		response.Error(w, r, apperr.NotFound("monitor not found"))
		return
	}

	// Ownership check
	if m.UserID != userID {
		response.Error(w, r, apperr.NotFound("monitor not found"))
		return
	}

	resp := monitorToResponse(m)
	if uptimeMap := h.fetchUptimeMap(ctx, []string{m.ID}); uptimeMap != nil {
		if pct, ok := uptimeMap[m.ID]; ok {
			resp.UptimePercent = pct
		}
	}
	response.JSON(w, r, http.StatusOK, resp)
}

// Update handles PATCH /v1/monitors/:id.
func (h *MonitorHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	id := chi.URLParam(r, "id")
	m, err := h.q.GetMonitorByID(ctx, id)
	if err != nil {
		response.Error(w, r, apperr.NotFound("monitor not found"))
		return
	}
	if m.UserID != userID {
		response.Error(w, r, apperr.NotFound("monitor not found"))
		return
	}

	var req UpdateMonitorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid JSON request body", ""))
		return
	}

	// Validate status if provided
	if req.Status != nil {
		allowed := map[string]bool{"active": true, "paused": true, "maintenance": true}
		if !allowed[*req.Status] {
			response.Error(w, r, apperr.Validation("status must be active, paused, or maintenance", "status"))
			return
		}
	}

	// Validate interval_s if provided
	if req.IntervalS != nil && !validIntervals[*req.IntervalS] {
		response.Error(w, r, apperr.Validation("interval_s must be 60, 300, or 1800", "interval_s"))
		return
	}

	updated, err := h.updateMonitor(ctx, m, req)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to update monitor", err))
		return
	}

	response.JSON(w, r, http.StatusOK, monitorToResponse(updated))
}

// updateMonitor applies partial updates to a monitor.
// Both status changes and field changes (name, config, interval_s) go through
// dedicated sqlc-generated queries — no raw SQL in the handler.
func (h *MonitorHandler) updateMonitor(ctx context.Context, m idcdmain.Monitor, req UpdateMonitorRequest) (idcdmain.Monitor, error) {
	updated := m

	// Only call UpdateMonitorStatus when the caller explicitly changes status;
	// an unconditional call would write a no-op UPDATE and corrupt updated_at.
	if req.Status != nil {
		var err error
		updated, err = h.q.UpdateMonitorStatus(ctx, idcdmain.UpdateMonitorStatusParams{
			ID:     m.ID,
			Status: *req.Status,
		})
		if err != nil {
			return idcdmain.Monitor{}, fmt.Errorf("updateMonitor: update status: %w", err)
		}
	}

	// Persist field changes (name / config / interval_s) via the sqlc-generated query.
	if req.Name != nil || req.Config != nil || req.IntervalS != nil {
		name := updated.Name
		if req.Name != nil {
			name = *req.Name
		}
		config := updated.Config
		if req.Config != nil {
			config = []byte(*req.Config)
		}
		intervalS := updated.IntervalS
		if req.IntervalS != nil {
			intervalS = *req.IntervalS
		}
		var err error
		updated, err = h.q.UpdateMonitorFields(ctx, idcdmain.UpdateMonitorFieldsParams{
			ID:        m.ID,
			Name:      name,
			Config:    config,
			IntervalS: intervalS,
		})
		if err != nil {
			return idcdmain.Monitor{}, fmt.Errorf("updateMonitor: persist field changes: %w", err)
		}
	}

	return updated, nil
}

// Delete handles DELETE /v1/monitors/:id (soft-delete: status='archived').
func (h *MonitorHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	id := chi.URLParam(r, "id")
	m, err := h.q.GetMonitorByID(ctx, id)
	if err != nil {
		response.Error(w, r, apperr.NotFound("monitor not found"))
		return
	}
	if m.UserID != userID {
		response.Error(w, r, apperr.NotFound("monitor not found"))
		return
	}

	if err := h.q.DeleteMonitor(ctx, id); err != nil {
		response.Error(w, r, apperr.Internal("failed to delete monitor", err))
		return
	}

	response.JSON(w, r, http.StatusNoContent, nil)
}

// Pause handles POST /v1/monitors/:id/pause.
func (h *MonitorHandler) Pause(w http.ResponseWriter, r *http.Request) {
	h.setStatus(w, r, "paused")
}

// Resume handles POST /v1/monitors/:id/resume.
func (h *MonitorHandler) Resume(w http.ResponseWriter, r *http.Request) {
	h.setStatus(w, r, "active")
}

// BulkAction handles POST /v1/monitors/bulk.
func (h *MonitorHandler) BulkAction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	var req BulkActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid JSON request body", ""))
		return
	}

	if len(req.IDs) == 0 {
		response.Error(w, r, apperr.Validation("ids must not be empty", "ids"))
		return
	}
	if len(req.IDs) > 50 {
		response.Error(w, r, apperr.Validation("ids must not exceed 50", "ids"))
		return
	}

	validActions := map[string]bool{"pause": true, "resume": true, "delete": true}
	if !validActions[req.Action] {
		response.Error(w, r, apperr.Validation("action must be pause, resume, or delete", "action"))
		return
	}

	if h.bulkPool == nil {
		response.Error(w, r, apperr.Internal("bulk operations not available", nil))
		return
	}

	rows, err := h.bulkPool.Query(ctx,
		`SELECT id FROM monitors WHERE id = ANY($1) AND user_id = $2 AND status != 'archived'`,
		req.IDs, userID,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to verify monitor ownership", err))
		return
	}
	owned := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			response.Error(w, r, apperr.Internal("failed to scan monitor ids", err))
			return
		}
		owned[id] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to query monitor ownership", err))
		return
	}

	succeeded := make([]string, 0, len(req.IDs))
	failed := make([]string, 0)
	for _, id := range req.IDs {
		if !owned[id] {
			failed = append(failed, id)
		} else {
			succeeded = append(succeeded, id)
		}
	}

	if len(succeeded) > 0 {
		var sql string
		switch req.Action {
		case "pause":
			sql = `UPDATE monitors SET status='paused', updated_at=NOW() WHERE id = ANY($1) AND user_id = $2`
		case "resume":
			sql = `UPDATE monitors SET status='active', updated_at=NOW() WHERE id = ANY($1) AND user_id = $2`
		case "delete":
			sql = `UPDATE monitors SET status='archived', updated_at=NOW() WHERE id = ANY($1) AND user_id = $2`
		}
		if _, err := h.bulkPool.Exec(ctx, sql, succeeded, userID); err != nil {
			response.Error(w, r, apperr.Internal("failed to execute bulk action", err))
			return
		}
	}

	response.JSON(w, r, http.StatusOK, BulkActionResponse{
		Succeeded: succeeded,
		Failed:    failed,
		Total:     len(req.IDs),
	})
}

func (h *MonitorHandler) setStatus(w http.ResponseWriter, r *http.Request, status string) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	id := chi.URLParam(r, "id")
	m, err := h.q.GetMonitorByID(ctx, id)
	if err != nil {
		response.Error(w, r, apperr.NotFound("monitor not found"))
		return
	}
	if m.UserID != userID {
		response.Error(w, r, apperr.NotFound("monitor not found"))
		return
	}

	updated, err := h.q.UpdateMonitorStatus(ctx, idcdmain.UpdateMonitorStatusParams{
		ID:     id,
		Status: status,
	})
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to update monitor status", err))
		return
	}

	response.JSON(w, r, http.StatusOK, monitorToResponse(updated))
}

// --- helpers ---

func monitorToResponse(m idcdmain.Monitor) MonitorResponse {
	resp := MonitorResponse{
		ID:        m.ID,
		UserID:    m.UserID,
		Name:      m.Name,
		Type:      m.Type,
		Target:    m.Target,
		Config:    json.RawMessage(m.Config),
		IntervalS: m.IntervalS,
		NodeCount: m.NodeCount,
		Status:    m.Status,
	}
	if m.CreatedAt.Valid {
		t := m.CreatedAt.Time.UTC().Format("2006-01-02T15:04:05Z")
		resp.CreatedAt = t
	}
	if m.UpdatedAt.Valid {
		t := m.UpdatedAt.Time.UTC().Format("2006-01-02T15:04:05Z")
		resp.UpdatedAt = t
	}
	if m.LastCheckAt.Valid {
		t := m.LastCheckAt.Time.UTC().Format("2006-01-02T15:04:05Z")
		resp.LastCheckAt = &t
	}
	if m.NextCheckAt.Valid {
		t := m.NextCheckAt.Time.UTC().Format("2006-01-02T15:04:05Z")
		resp.NextCheckAt = &t
	}
	return resp
}

// fetchUptimeMap queries monitor_checks to compute 24-hour uptime percentages
// for the provided set of monitor IDs. Returns a map[monitorID]uptimePct.
// Returns nil (no error surfaced) when bulkPool is nil or the query fails, so
// callers simply get 0.0 defaults.
func (h *MonitorHandler) fetchUptimeMap(ctx context.Context, monitorIDs []string) map[string]float64 {
	if h.bulkPool == nil || len(monitorIDs) == 0 {
		return nil
	}
	const uptimeSQL = `
		SELECT
			monitor_id,
			ROUND(
				COUNT(*) FILTER (WHERE status = 'up')::numeric / NULLIF(COUNT(*), 0) * 100,
				2
			) AS uptime_pct
		FROM monitor_checks
		WHERE monitor_id = ANY($1)
		  AND checked_at > NOW() - INTERVAL '24 hours'
		GROUP BY monitor_id`

	rows, err := h.bulkPool.Query(ctx, uptimeSQL, monitorIDs)
	if err != nil {
		return nil
	}
	defer rows.Close()

	result := make(map[string]float64, len(monitorIDs))
	for rows.Next() {
		var mid string
		var pct float64
		if err := rows.Scan(&mid, &pct); err == nil {
			result[mid] = pct
		}
	}
	return result
}

// parsePagination extracts page/limit from query params with sensible defaults.
func parsePagination(r *http.Request) (page, limit int) {
	page = 1
	limit = 20

	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	return
}

