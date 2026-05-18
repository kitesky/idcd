// Package handler implements HTTP handlers for the API server.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/apps/api/internal/denylist"
	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
	"github.com/kite365/idcd/lib/shared/stream"
)

// errNoActiveProbeNode is returned by resolveActiveNodeID when no enrolled
// node is currently in the 'active' state. The HTTP handler maps this to a
// 503 Service Unavailable so callers (frontend, agent SDK) can distinguish
// "no capacity right now" from generic server failure.
//
// See `node_enrollment_handler.go` for the full enrollment → pending → active
// state machine. Active status is set by the gateway WS handler when an agent
// connects, or manually by admin via POST /v1/admin/nodes/{id}/activate.
var errNoActiveProbeNode = errors.New("no active probe node available")

// ProbeHandler handles probe task endpoints.
type ProbeHandler struct {
	pool         ProbePool
	streamClient *stream.Client
}

// ProbePool is the interface for database operations.
type ProbePool interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// NewProbeHandler creates a new ProbeHandler.
func NewProbeHandler(pool ProbePool, streamClient *stream.Client) *ProbeHandler {
	return &ProbeHandler{
		pool:         pool,
		streamClient: streamClient,
	}
}

// ProbeRequest is the common request structure for probe endpoints.
type ProbeRequest struct {
	Target string         `json:"target"`
	NodeID string         `json:"node_id,omitempty"` // single node shorthand
	Nodes  []string       `json:"nodes,omitempty"`
	Params map[string]any `json:"params,omitempty"`
}

// ProbeResponse is the response structure for probe endpoints.
type ProbeResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

// DiagnoseResponse is the response structure for diagnose endpoint.
type DiagnoseResponse struct {
	DiagnosisID string   `json:"diagnosis_id"`
	TaskIDs     []string `json:"task_ids"`
	Status      string   `json:"status"`
}

// HTTP handles HTTP/HTTPS probe requests.
// POST /v1/probe/http
func (h *ProbeHandler) HTTP(w http.ResponseWriter, r *http.Request) {
	h.handleProbe(w, r, "http")
}

// Ping handles ICMP Ping probe requests.
// POST /v1/probe/ping
func (h *ProbeHandler) Ping(w http.ResponseWriter, r *http.Request) {
	h.handleProbe(w, r, "ping")
}

// TCP handles TCP connection probe requests.
// POST /v1/probe/tcp
func (h *ProbeHandler) TCP(w http.ResponseWriter, r *http.Request) {
	h.handleProbe(w, r, "tcp")
}

// DNS handles DNS resolution probe requests.
// POST /v1/probe/dns
func (h *ProbeHandler) DNS(w http.ResponseWriter, r *http.Request) {
	h.handleProbe(w, r, "dns")
}

// Traceroute handles traceroute probe requests.
// POST /v1/probe/traceroute
func (h *ProbeHandler) Traceroute(w http.ResponseWriter, r *http.Request) {
	h.handleProbe(w, r, "traceroute")
}

// SMTP handles SMTP connection test requests.
// POST /v1/probe/smtp
func (h *ProbeHandler) SMTP(w http.ResponseWriter, r *http.Request) {
	h.handleProbe(w, r, "smtp")
}

// NTP handles NTP server test requests.
// POST /v1/probe/ntp
func (h *ProbeHandler) NTP(w http.ResponseWriter, r *http.Request) {
	h.handleProbe(w, r, "ntp")
}

// MTR handles MTR (traceroute + per-hop ping) probe requests.
// POST /v1/probe/mtr
func (h *ProbeHandler) MTR(w http.ResponseWriter, r *http.Request) {
	h.handleProbe(w, r, "mtr")
}

// Speedtest handles HTTP bandwidth measurement probe requests.
// POST /v1/probe/speedtest
func (h *ProbeHandler) Speedtest(w http.ResponseWriter, r *http.Request) {
	h.handleProbe(w, r, "speedtest")
}

// Diagnose handles comprehensive diagnostic requests.
// POST /v1/diagnose
func (h *ProbeHandler) Diagnose(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request
	var req ProbeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("Invalid JSON request body", ""))
		return
	}

	// Validate target and get pre-resolved IP to prevent DNS rebinding.
	resolvedTarget, err := denylist.CheckTarget(req.Target)
	if err != nil {
		response.Error(w, r, apperr.Validation(err.Error(), ""))
		return
	}

	// Resolve node for diagnose tasks
	diagNodeID := req.NodeID
	if diagNodeID == "" && len(req.Nodes) > 0 {
		diagNodeID = req.Nodes[0]
	}
	if diagNodeID == "" {
		resolved, resolveErr := h.resolveActiveNodeID(ctx)
		if resolveErr != nil {
			h.writeNodeResolutionError(w, r, resolveErr)
			return
		}
		diagNodeID = resolved
	}

	// Generate diagnosis ID
	diagnosisID := idgen.ProbeTask()

	// Create 5 probe tasks: http, ping, dns, tcp, traceroute
	probeTypes := []string{"http", "ping", "dns", "tcp", "traceroute"}
	taskIDs := make([]string, 0, len(probeTypes))

	for _, probeType := range probeTypes {
		taskID, err := h.createProbeTask(ctx, r, probeType, resolvedTarget, diagNodeID, req.Nodes, req.Params)
		if err != nil {
			response.Error(w, r, apperr.Internal("Failed to create probe task", err))
			return
		}
		taskIDs = append(taskIDs, taskID)
	}

	resp := DiagnoseResponse{
		DiagnosisID: diagnosisID,
		TaskIDs:     taskIDs,
		Status:      "queued",
	}

	response.JSON(w, r, http.StatusOK, resp)
}

// handleProbe is the common handler for all single probe types.
func (h *ProbeHandler) handleProbe(w http.ResponseWriter, r *http.Request, probeType string) {
	ctx := r.Context()

	// Parse request
	var req ProbeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("Invalid JSON request body", ""))
		return
	}

	// Validate target and get pre-resolved IP to prevent DNS rebinding.
	resolvedTarget, checkErr := denylist.CheckTarget(req.Target)
	if checkErr != nil {
		response.Error(w, r, apperr.Validation(checkErr.Error(), ""))
		return
	}

	// Resolve node: prefer explicit node_id, then first item in nodes array
	nodeID := req.NodeID
	if nodeID == "" && len(req.Nodes) > 0 {
		nodeID = req.Nodes[0]
	}
	// If still empty, pick any active node from DB. Returning a clear 503
	// (instead of silently queuing a task that no one will ever pick up) is
	// load-bearing — see REVIEW-FINDINGS-2026-05-16 P0#9.
	if nodeID == "" {
		resolved, resolveErr := h.resolveActiveNodeID(ctx)
		if resolveErr != nil {
			h.writeNodeResolutionError(w, r, resolveErr)
			return
		}
		nodeID = resolved
	}

	// Create probe task
	taskID, err := h.createProbeTask(ctx, r, probeType, resolvedTarget, nodeID, req.Nodes, req.Params)
	if err != nil {
		response.Error(w, r, apperr.Internal("Failed to create probe task", err))
		return
	}

	resp := ProbeResponse{
		TaskID: taskID,
		Status: "queued",
	}

	response.JSON(w, r, http.StatusOK, resp)
}

// createProbeTask creates a probe task in the database and pushes it to Redis Stream.
// nodeID is the resolved target node; nodes is the original selection list.
func (h *ProbeHandler) createProbeTask(
	ctx context.Context,
	r *http.Request,
	probeType string,
	target string,
	nodeID string,
	nodes []string,
	params map[string]any,
) (string, error) {
	taskID := idgen.ProbeTask()

	// Normalize target
	targetNormalized := normalizeTarget(target)

	// User context is set by authn middleware; may be empty for anonymous
	// tool-page requests. Must go through the typed-key accessor.
	var initiatedBy *string
	if userID := middleware.UserIDFromContext(r.Context()); userID != "" {
		initiatedBy = &userID
	}
	// api_key_id is populated when the request was authenticated via an API
	// key — preserves the audit trail back to the issuing key.
	var apiKeyID *string
	if k := middleware.APIKeyIDFromContext(r.Context()); k != "" {
		apiKeyID = &k
	}

	// Get client IP
	clientIP := getClientIP(r)

	// Get user agent
	userAgent := r.UserAgent()

	// Serialize params and node_selection
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("marshal params: %w", err)
	}

	nodeSelectionJSON, err := json.Marshal(nodes)
	if err != nil {
		return "", fmt.Errorf("marshal node_selection: %w", err)
	}

	// Insert into database
	query := `
		INSERT INTO probe_task (
			id, type, target, target_normalized, params,
			initiated_by, api_key_id, client_ip, user_agent,
			node_selection, status, created_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, 'queued', NOW()
		)
	`

	_, err = h.pool.Exec(ctx, query,
		taskID, probeType, target, targetNormalized, paramsJSON,
		initiatedBy, apiKeyID, clientIP, userAgent,
		nodeSelectionJSON,
	)
	if err != nil {
		return "", fmt.Errorf("insert probe_task: %w", err)
	}

	// Push to Redis Stream stream.ProbeTasks ("probe.tasks").
	streamData := map[string]any{
		"task_id":           taskID,
		"type":              probeType,
		"target":            target,
		"target_normalized": targetNormalized,
		"params":            string(paramsJSON),
		"node_id":           nodeID,
		"node_selection":    string(nodeSelectionJSON),
		"created_at":        time.Now().Unix(),
	}

	if _, err := h.streamClient.Add(ctx, stream.ProbeTasks, streamData); err != nil {
		return "", fmt.Errorf("push to stream: %w", err)
	}

	return taskID, nil
}

// resolveActiveNodeID picks any active probe node from the DB.
//
// Returns errNoActiveProbeNode when no row matches (operator must enroll a
// node or wait for an existing one to come online). Any other error is a real
// DB failure and is wrapped, so the caller can map it to 500.
func (h *ProbeHandler) resolveActiveNodeID(ctx context.Context) (string, error) {
	var nodeID string
	err := h.pool.QueryRow(ctx,
		`SELECT node_id FROM enrolled_nodes WHERE status='active' LIMIT 1`,
	).Scan(&nodeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errNoActiveProbeNode
		}
		return "", fmt.Errorf("query active node: %w", err)
	}
	// Defensive: pgx normally returns ErrNoRows when zero rows match, but
	// some Scan implementations may yield an empty string instead.
	if nodeID == "" {
		return "", errNoActiveProbeNode
	}
	return nodeID, nil
}

// writeNodeResolutionError converts a resolveActiveNodeID error into an HTTP
// response: 503 when no node is online, 500 + log for genuine DB failures.
func (h *ProbeHandler) writeNodeResolutionError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errNoActiveProbeNode) {
		response.Error(w, r, apperr.Unavailable(
			"no active probe node available, please contact support",
			err,
		))
		return
	}
	slog.Default().Error("probe: failed to resolve active node",
		"err", err,
		"path", r.URL.Path,
	)
	response.Error(w, r, apperr.Internal("failed to resolve probe node", err))
}

// normalizeTarget normalizes the target for database storage.
func normalizeTarget(target string) string {
	// Convert to lowercase and trim spaces
	normalized := strings.ToLower(strings.TrimSpace(target))

	// Remove http:// or https:// prefix if present
	normalized = strings.TrimPrefix(normalized, "http://")
	normalized = strings.TrimPrefix(normalized, "https://")

	return normalized
}

// getClientIP delegates to the middleware package so proxy-header trust logic is centralised.
func getClientIP(r *http.Request) string {
	return middleware.ClientIP(r)
}

// TaskResult handles GET /v1/probe/tasks/{taskId}.
//
// Anonymous task lookups remain allowed (public tool pages create probe
// tasks without logging in), but tasks that WERE created by an authenticated
// user are scoped to that user. A leaked task_id therefore can't be used by
// another account to read someone else's probe result. The router pairs this
// handler with OptionalAuthnWithTokens so UserID / APIKeyID land in context
// when a Bearer / cookie is present.
func (h *ProbeHandler) TaskResult(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		response.Error(w, r, apperr.Validation("missing task ID", ""))
		return
	}

	ctx := r.Context()
	var status string
	var result *json.RawMessage
	var createdAt time.Time
	var completedAt *time.Time
	var initiatedBy *string
	var taskAPIKeyID *string

	err := h.pool.QueryRow(ctx, `
		SELECT status, result, created_at, completed_at, initiated_by, api_key_id
		FROM probe_task WHERE id = $1
	`, taskID).Scan(&status, &result, &createdAt, &completedAt, &initiatedBy, &taskAPIKeyID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(w, r, apperr.NotFound("task not found"))
			return
		}
		response.Error(w, r, apperr.Internal("query failed", err))
		return
	}

	// Ownership check: if the task was created by an authenticated principal,
	// only that principal can read the result. Return 404 (not 403) so an
	// attacker can't enumerate which task IDs exist on other accounts.
	if initiatedBy != nil && *initiatedBy != "" {
		requesterUserID := middleware.UserIDFromContext(ctx)
		if requesterUserID != *initiatedBy {
			response.Error(w, r, apperr.NotFound("task not found"))
			return
		}
	}

	type TaskResultResponse struct {
		TaskID      string           `json:"task_id"`
		Status      string           `json:"status"`
		Result      *json.RawMessage `json:"result,omitempty"`
		CreatedAt   time.Time        `json:"created_at"`
		CompletedAt *time.Time       `json:"completed_at,omitempty"`
	}

	response.JSON(w, r, http.StatusOK, TaskResultResponse{
		TaskID:      taskID,
		Status:      status,
		Result:      result,
		CreatedAt:   createdAt,
		CompletedAt: completedAt,
	})
}
