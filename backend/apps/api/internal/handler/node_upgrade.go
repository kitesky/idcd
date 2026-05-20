package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
)

// NodeUpgradePool is the minimal DB interface for NodeUpgradeHandler.
type NodeUpgradePool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// NodeUpgradeHandler handles upgrade rollout management endpoints.
type NodeUpgradeHandler struct {
	pool       NodeUpgradePool
	gatewayURL string
}

// NewNodeUpgradeHandler creates a new NodeUpgradeHandler.
// gatewayURL is the internal base URL of the gateway service (e.g. "http://gateway:8081").
func NewNodeUpgradeHandler(pool NodeUpgradePool, gatewayURL string) *NodeUpgradeHandler {
	return &NodeUpgradeHandler{pool: pool, gatewayURL: gatewayURL}
}

// UpgradeRollout is the API representation of a rollout record.
type UpgradeRollout struct {
	ID          string    `json:"id"`
	Version     string    `json:"version"`
	DownloadURL string    `json:"download_url"`
	Checksum    string    `json:"checksum"`
	RolloutPct  int       `json:"rollout_pct"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type createRolloutRequest struct {
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	Checksum    string `json:"checksum"`
	RolloutPct  int    `json:"rollout_pct"`
}

type updateRolloutRequest struct {
	RolloutPct *int    `json:"rollout_pct,omitempty"`
	Status     *string `json:"status,omitempty"`
}

// Create handles POST /internal/admin/upgrade-rollouts.
func (h *NodeUpgradeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRolloutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.Version == "" {
		response.Error(w, r, apperr.Validation("version is required", "version"))
		return
	}
	if req.DownloadURL == "" {
		response.Error(w, r, apperr.Validation("download_url is required", "download_url"))
		return
	}
	if req.Checksum == "" {
		response.Error(w, r, apperr.Validation("checksum is required", "checksum"))
		return
	}
	if req.RolloutPct < 1 || req.RolloutPct > 100 {
		req.RolloutPct = 1
	}

	ctx := r.Context()
	id := idgen.NodeUpgradeRollout()

	rollout := &UpgradeRollout{}
	err := h.pool.QueryRow(ctx, `
		INSERT INTO node_upgrade_rollouts (id, version, download_url, checksum, rollout_pct)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, version, download_url, checksum, rollout_pct, status, created_at, updated_at
	`, id, req.Version, req.DownloadURL, req.Checksum, req.RolloutPct).Scan(
		&rollout.ID, &rollout.Version, &rollout.DownloadURL, &rollout.Checksum,
		&rollout.RolloutPct, &rollout.Status, &rollout.CreatedAt, &rollout.UpdatedAt,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to create rollout", err))
		return
	}

	h.triggerBroadcast(ctx, rollout)

	response.JSON(w, r, http.StatusCreated, rollout)
}

// List handles GET /internal/admin/upgrade-rollouts.
func (h *NodeUpgradeHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.pool.Query(ctx, `
		SELECT id, version, download_url, checksum, rollout_pct, status, created_at, updated_at
		FROM node_upgrade_rollouts
		ORDER BY created_at DESC
		LIMIT 100
	`)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list rollouts", err))
		return
	}
	defer rows.Close()

	rollouts := make([]UpgradeRollout, 0)
	for rows.Next() {
		var ro UpgradeRollout
		if err := rows.Scan(
			&ro.ID, &ro.Version, &ro.DownloadURL, &ro.Checksum,
			&ro.RolloutPct, &ro.Status, &ro.CreatedAt, &ro.UpdatedAt,
		); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan rollout row", err))
			return
		}
		rollouts = append(rollouts, ro)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to iterate rollouts", err))
		return
	}

	response.JSON(w, r, http.StatusOK, rollouts)
}

// Update handles PATCH /internal/admin/upgrade-rollouts/{id}.
func (h *NodeUpgradeHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, r, apperr.Validation("rollout id is required", "id"))
		return
	}

	var req updateRolloutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}

	ctx := r.Context()

	if req.RolloutPct != nil {
		if *req.RolloutPct < 1 || *req.RolloutPct > 100 {
			response.Error(w, r, apperr.Validation("rollout_pct must be between 1 and 100", "rollout_pct"))
			return
		}
		_, err := h.pool.Exec(ctx, `
			UPDATE node_upgrade_rollouts
			SET rollout_pct = $1, updated_at = NOW()
			WHERE id = $2
		`, *req.RolloutPct, id)
		if err != nil {
			response.Error(w, r, apperr.Internal("failed to update rollout_pct", err))
			return
		}
	}

	if req.Status != nil {
		switch *req.Status {
		case "active", "paused", "completed":
		default:
			response.Error(w, r, apperr.Validation("status must be active, paused, or completed", "status"))
			return
		}
		_, err := h.pool.Exec(ctx, `
			UPDATE node_upgrade_rollouts
			SET status = $1, updated_at = NOW()
			WHERE id = $2
		`, *req.Status, id)
		if err != nil {
			response.Error(w, r, apperr.Internal("failed to update status", err))
			return
		}
	}

	rollout := &UpgradeRollout{}
	err := h.pool.QueryRow(ctx, `
		SELECT id, version, download_url, checksum, rollout_pct, status, created_at, updated_at
		FROM node_upgrade_rollouts
		WHERE id = $1
	`, id).Scan(
		&rollout.ID, &rollout.Version, &rollout.DownloadURL, &rollout.Checksum,
		&rollout.RolloutPct, &rollout.Status, &rollout.CreatedAt, &rollout.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(w, r, apperr.NotFound("rollout not found"))
		} else {
			response.Error(w, r, apperr.Internal("failed to fetch rollout", err))
		}
		return
	}

	if rollout.Status == "active" {
		h.triggerBroadcast(ctx, rollout)
	}

	response.JSON(w, r, http.StatusOK, rollout)
}

// triggerBroadcast calls the gateway's internal broadcast endpoint.
// Errors are logged but do not fail the API response.
func (h *NodeUpgradeHandler) triggerBroadcast(_ context.Context, ro *UpgradeRollout) {
	if h.gatewayURL == "" {
		return
	}

	body, err := json.Marshal(map[string]any{
		"version":      ro.Version,
		"download_url": ro.DownloadURL,
		"checksum":     ro.Checksum,
		"rollout_pct":  ro.RolloutPct,
	})
	if err != nil {
		return
	}

	client := &http.Client{Timeout: 5 * time.Second}
	//nolint:gosec — URL is configured by operator, not user input
	resp, err := client.Post(h.gatewayURL+"/internal/broadcast-upgrade", "application/json", bytes.NewReader(body))
	if err != nil || resp == nil {
		return
	}
	resp.Body.Close()
}
