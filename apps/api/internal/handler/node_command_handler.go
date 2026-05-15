package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
)

// NodeCommandPool is the minimal DB interface for NodeCommandHandler.
type NodeCommandPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// NodeCommandHandler queues OTA upgrade and config-reload commands for agent nodes.
type NodeCommandHandler struct {
	pool       NodeCommandPool
	adminToken string
}

// NewNodeCommandHandler creates the handler.
func NewNodeCommandHandler(pool *pgxpool.Pool, adminToken string) *NodeCommandHandler {
	return &NodeCommandHandler{pool: pool, adminToken: adminToken}
}

// UpgradeRequest is the body for POST /internal/admin/nodes/{id}/upgrade.
type upgradeRequest struct {
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	Checksum    string `json:"checksum"` // optional SHA-256
}

// QueueUpgrade enqueues an OTA upgrade command for the specified node.
// POST /internal/admin/nodes/{node_id}/upgrade
func (h *NodeCommandHandler) QueueUpgrade(w http.ResponseWriter, r *http.Request) {
	if !h.isAdmin(r) {
		response.Error(w, r, apperr.Unauthorized("invalid admin token"))
		return
	}

	nodeID := chi.URLParam(r, "node_id")
	if !h.nodeExists(r.Context(), nodeID) {
		response.Error(w, r, apperr.NotFound("node not found"))
		return
	}

	var req upgradeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.DownloadURL == "" {
		response.Error(w, r, apperr.Validation("download_url is required", ""))
		return
	}
	if req.Version == "" {
		response.Error(w, r, apperr.Validation("version is required", ""))
		return
	}
	if req.Checksum == "" {
		response.Error(w, r, apperr.Validation("checksum is required — unsigned upgrades are not permitted", ""))
		return
	}

	payload, _ := json.Marshal(req)
	cmdID, err := h.insertCommand(r.Context(), nodeID, "upgrade", string(payload))
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to queue command", err))
		return
	}

	response.JSON(w, r, http.StatusCreated, map[string]string{
		"cmd_id":  cmdID,
		"status":  "queued",
		"node_id": nodeID,
		"command": "upgrade",
		"version": req.Version,
	})
}

// QueueReloadConfig enqueues a config hot-reload command for the specified node.
// POST /internal/admin/nodes/{node_id}/reload-config
func (h *NodeCommandHandler) QueueReloadConfig(w http.ResponseWriter, r *http.Request) {
	if !h.isAdmin(r) {
		response.Error(w, r, apperr.Unauthorized("invalid admin token"))
		return
	}

	nodeID := chi.URLParam(r, "node_id")
	if !h.nodeExists(r.Context(), nodeID) {
		response.Error(w, r, apperr.NotFound("node not found"))
		return
	}

	cmdID, err := h.insertCommand(r.Context(), nodeID, "reload_config", "{}")
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to queue command", err))
		return
	}

	response.JSON(w, r, http.StatusCreated, map[string]string{
		"cmd_id":  cmdID,
		"status":  "queued",
		"node_id": nodeID,
		"command": "reload_config",
	})
}

// ListNodes returns enrolled nodes with their current status and fingerprint.
// GET /internal/admin/nodes
func (h *NodeCommandHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
	if !h.isAdmin(r) {
		response.Error(w, r, apperr.Unauthorized("invalid admin token"))
		return
	}

	var raw []byte
	err := h.pool.QueryRow(r.Context(), `
		SELECT COALESCE(json_agg(row_to_json(t)), '[]'::json)
		FROM (
			SELECT
				node_id,
				hostname,
				arch,
				os,
				ip_address,
				agent_version,
				status,
				enrolled_at,
				last_seen_at,
				fingerprint
			FROM enrolled_nodes
			ORDER BY enrolled_at DESC
		) t
	`).Scan(&raw)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list nodes", err))
		return
	}

	response.JSON(w, r, http.StatusOK, json.RawMessage(raw))
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (h *NodeCommandHandler) isAdmin(r *http.Request) bool {
	provided := r.Header.Get("X-Admin-Token")
	return subtle.ConstantTimeCompare([]byte(provided), []byte(h.adminToken)) == 1
}

func (h *NodeCommandHandler) nodeExists(ctx context.Context, nodeID string) bool {
	var dummy int
	err := h.pool.QueryRow(ctx,
		`SELECT 1 FROM enrolled_nodes WHERE node_id = $1`, nodeID,
	).Scan(&dummy)
	return err == nil
}

func (h *NodeCommandHandler) insertCommand(ctx context.Context, nodeID, command, payload string) (string, error) {
	cmdID := idgen.New("cmd_")
	_, err := h.pool.Exec(ctx, `
		INSERT INTO node_commands (id, node_id, command, payload)
		VALUES ($1, $2, $3, $4::jsonb)
	`, cmdID, nodeID, command, payload)
	if err != nil {
		return "", err
	}
	return cmdID, nil
}
