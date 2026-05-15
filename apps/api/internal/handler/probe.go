// Package handler implements HTTP handlers for the API server.
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/apps/api/internal/denylist"
	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
	"github.com/kite365/idcd/lib/shared/stream"
)

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
		_ = h.pool.QueryRow(ctx,
			`SELECT node_id FROM enrolled_nodes WHERE status='active' LIMIT 1`,
		).Scan(&diagNodeID)
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
	// If still empty, pick any active node from DB
	if nodeID == "" {
		_ = h.pool.QueryRow(ctx,
			`SELECT node_id FROM enrolled_nodes WHERE status='active' LIMIT 1`,
		).Scan(&nodeID)
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

	// Get user info from context (set by authn middleware, may be nil)
	var initiatedBy *string
	if userID, ok := r.Context().Value("user_id").(string); ok && userID != "" {
		initiatedBy = &userID
	}

	// Get API key ID from context (if available)
	var apiKeyID *string
	if keyID, ok := r.Context().Value("api_key_id").(string); ok && keyID != "" {
		apiKeyID = &keyID
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

	// Push to Redis Stream "probe.tasks"
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

	if _, err := h.streamClient.Add(ctx, "probe.tasks", streamData); err != nil {
		return "", fmt.Errorf("push to stream: %w", err)
	}

	return taskID, nil
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
