package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
)

// NodeEnrollmentPool is the minimal DB interface needed by NodeEnrollmentHandler.
type NodeEnrollmentPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Begin(ctx context.Context) (pgx.Tx, error)
}

// NodeEnrollmentHandler handles agent enrollment token issuance and node registration.
type NodeEnrollmentHandler struct {
	pool       NodeEnrollmentPool
	gatewayURL string // returned to newly enrolled agents
	adminToken string // internal admin token for token creation
}

// NewNodeEnrollmentHandler creates the handler.
func NewNodeEnrollmentHandler(pool *pgxpool.Pool, gatewayURL, adminToken string) *NodeEnrollmentHandler {
	return &NodeEnrollmentHandler{pool: pool, gatewayURL: gatewayURL, adminToken: adminToken}
}

// --- request / response types ---

type createEnrollmentTokenRequest struct {
	Label     string `json:"label"`      // human label, e.g. "jp-tokyo-01"
	ExpiresIn string `json:"expires_in"` // duration string, default "24h"
}

type createEnrollmentTokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type enrollRequest struct {
	Token    string `json:"token"`
	Hostname string `json:"hostname"`
	Arch     string `json:"arch"`
	OS       string `json:"os"`
	Kernel   string `json:"kernel"`
	Version  string `json:"version"` // agent binary version, optional
}

type enrollResponse struct {
	NodeID     string `json:"node_id"`
	SecretKey  string `json:"secret_key"`
	GatewayURL string `json:"gateway_url"`
}

// isAdmin returns true if the request carries a valid admin token.
func (h *NodeEnrollmentHandler) isAdmin(r *http.Request) bool {
	return subtle.ConstantTimeCompare(
		[]byte(r.Header.Get("X-Admin-Token")),
		[]byte(h.adminToken),
	) == 1
}

// CreateEnrollmentToken issues a one-time enrollment token.
// POST /internal/admin/nodes/enrollment-tokens
func (h *NodeEnrollmentHandler) CreateEnrollmentToken(w http.ResponseWriter, r *http.Request) {
	if !h.isAdmin(r) {
		response.Error(w, r, apperr.Unauthorized("invalid admin token"))
		return
	}

	var req createEnrollmentTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Accept empty body — all fields are optional
		req = createEnrollmentTokenRequest{}
	}

	expiresIn := 24 * time.Hour
	if req.ExpiresIn != "" {
		d, err := time.ParseDuration(req.ExpiresIn)
		if err != nil {
			response.Error(w, r, apperr.Validation("invalid expires_in", err.Error()))
			return
		}
		if d < time.Minute || d > 7*24*time.Hour {
			response.Error(w, r, apperr.Validation("expires_in must be between 1m and 7d", ""))
			return
		}
		expiresIn = d
	}

	// Generate token: "ent_" + 32 random hex bytes = 68 chars total
	rawToken, err := idgen.RawSecret()
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to generate token", err))
		return
	}
	token := "ent_" + rawToken
	tokenHash := idgen.SHA256Hex(token)
	expiresAt := time.Now().Add(expiresIn)

	id := idgen.New("et_")
	_, err = h.pool.Exec(r.Context(), `
		INSERT INTO node_enrollment_tokens (id, token_hash, label, expires_at)
		VALUES ($1, $2, $3, $4)
	`, id, tokenHash, req.Label, expiresAt)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to store token", err))
		return
	}

	response.JSON(w, r, http.StatusCreated, createEnrollmentTokenResponse{
		Token:     token,
		ExpiresAt: expiresAt,
	})
}

// Enroll exchanges a one-time enrollment token for node credentials.
// POST /v1/agent/enroll
func (h *NodeEnrollmentHandler) Enroll(w http.ResponseWriter, r *http.Request) {
	var req enrollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.Token == "" {
		response.Error(w, r, apperr.Validation("token is required", ""))
		return
	}

	tokenHash := idgen.SHA256Hex(req.Token)

	// Generate node credentials before the transaction so they are ready.
	nodeID := idgen.New("nd_")
	secretKey, err := idgen.RawSecret()
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to generate credentials", err))
		return
	}
	secretHash := idgen.SHA256Hex(secretKey)
	clientIP := middleware.ClientIP(r)
	nodeRecordID := idgen.New("en_")

	// Execute token claim + node registration atomically so a transient INSERT
	// failure cannot permanently consume the token without creating a node record.
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to begin transaction", err))
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	var tokenID string
	err = tx.QueryRow(r.Context(), `
		UPDATE node_enrollment_tokens
		SET used_at = now()
		WHERE token_hash = $1
		  AND used_at IS NULL
		  AND expires_at > now()
		RETURNING id
	`, tokenHash).Scan(&tokenID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(w, r, apperr.Unauthorized("invalid or expired enrollment token"))
		} else {
			response.Error(w, r, apperr.Internal("token claim failed", err))
		}
		return
	}

	_, err = tx.Exec(r.Context(), `
		INSERT INTO enrolled_nodes
		  (id, node_id, secret_hash, hostname, arch, os, kernel, ip_address, agent_version, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'pending')
	`, nodeRecordID, nodeID, secretHash, req.Hostname, req.Arch, req.OS, req.Kernel, clientIP, req.Version)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to register node", err))
		return
	}

	_, _ = tx.Exec(r.Context(), `
		UPDATE node_enrollment_tokens SET used_by = $1 WHERE id = $2
	`, nodeID, tokenID)

	if err := tx.Commit(r.Context()); err != nil {
		response.Error(w, r, apperr.Internal("failed to commit enrollment", err))
		return
	}

	response.JSON(w, r, http.StatusCreated, enrollResponse{
		NodeID:     nodeID,
		SecretKey:  secretKey,
		GatewayURL: h.gatewayURL,
	})
}
