package handler

// Node enrollment & activation state machine
// ------------------------------------------
//
//   1. Admin calls POST /internal/admin/nodes/enrollment-tokens
//      → stores a SHA-256 hash of a single-use token in
//        node_enrollment_tokens with a default 24h TTL.
//
//   2. Operator runs the agent on a fresh box. Agent calls
//        POST /v1/agent/enroll  { token, hostname, arch, os, kernel }
//      → API atomically (a) marks the token as used and
//        (b) inserts a row into enrolled_nodes with status='pending'.
//        API returns { node_id, secret_key, gateway_url }.
//
//   3. status='pending' → status='active' happens via ONE of:
//        (a) Gateway auto-activate: when the agent opens a WebSocket
//            to the gateway, apps/gateway/internal/handler/ws.go runs
//            `UPDATE enrolled_nodes SET status='active', last_seen_at=now()`.
//            This is the production path — no admin action required.
//        (b) Admin manual activate: POST /v1/admin/nodes/{id}/activate
//            (handled by NodeActivate below) lets ops force a pending
//            node into 'active' without waiting for a gateway connect.
//            Useful for staging, broken egress, or pre-warming a node.
//
//   4. status='active' ↔ status='offline': gateway flips to 'offline'
//      when the WS read pump exits (see ws.go readPump defer).
//      A cleanup job in apps/gateway/internal/scheduler/cleanup.go
//      also stale-times nodes that haven't sent heartbeats.
//
//   5. status='disabled' / 'drained' are admin-only terminal states
//      (see migration 00030 for the full enum).
//
// Why no gateway → API "node-online" HTTP webhook? Gateway already shares
// the postgres pool, so it writes status directly (one round-trip, no
// inter-service auth surface). If we ever split gateway out into its own
// DB tenant we'll need a `POST /internal/nodes/{id}/heartbeat-active`
// endpoint here — leaving that TODO for the gateway team.

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
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

// activateResponse is the response payload for NodeActivate.
type activateResponse struct {
	NodeID         string `json:"node_id"`
	Status         string `json:"status"`           // always "active" on success
	PreviousStatus string `json:"previous_status"`  // pending|offline|drained|...
}

// NodeActivate flips an enrolled node from any non-disabled state to 'active'.
//
// Use case: an admin in the dashboard explicitly approves a freshly enrolled
// node ("status=pending") before the agent has connected to the gateway, OR
// recovers a node that the cleanup job marked 'offline' after a transient
// network blip.
//
// Refuses to act on 'disabled' nodes — those are intentional bans and must
// be re-enabled through a separate (audited) path, not this convenience
// endpoint.
//
// POST /v1/admin/nodes/{node_id}/activate
//
// TODO(server.go): mount POST /v1/admin/nodes/{node_id}/activate -> NodeActivate
func (h *NodeEnrollmentHandler) NodeActivate(w http.ResponseWriter, r *http.Request) {
	if !h.isAdmin(r) {
		response.Error(w, r, apperr.Unauthorized("invalid admin token"))
		return
	}

	nodeID := chi.URLParam(r, "node_id")
	if nodeID == "" {
		response.Error(w, r, apperr.Validation("node_id is required", ""))
		return
	}

	// Atomic state-machine guard: only transition if the node exists and
	// isn't disabled. RETURNING the prior status gives the admin UI a
	// useful audit trail without a second roundtrip.
	var previousStatus string
	err := h.pool.QueryRow(r.Context(), `
		WITH prev AS (
			SELECT status FROM enrolled_nodes WHERE node_id = $1
		)
		UPDATE enrolled_nodes
		SET status = 'active', last_seen_at = NOW()
		WHERE node_id = $1
		  AND status != 'disabled'
		RETURNING (SELECT status FROM prev)
	`, nodeID).Scan(&previousStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Two possible causes: node doesn't exist, or it's disabled.
			// Disambiguate with one more query so the admin gets a clear
			// error instead of a generic 404.
			var status string
			lookupErr := h.pool.QueryRow(r.Context(),
				`SELECT status FROM enrolled_nodes WHERE node_id = $1`,
				nodeID,
			).Scan(&status)
			if errors.Is(lookupErr, pgx.ErrNoRows) {
				response.Error(w, r, apperr.NotFound("node not found"))
				return
			}
			if lookupErr != nil {
				response.Error(w, r, apperr.Internal("failed to look up node status", lookupErr))
				return
			}
			if status == "disabled" {
				response.Error(w, r, apperr.Conflict(
					"node is disabled; re-enable through admin console first",
				))
				return
			}
			// Some other race — surface generically.
			response.Error(w, r, apperr.Internal("activation update affected 0 rows", nil))
			return
		}
		response.Error(w, r, apperr.Internal("failed to activate node", err))
		return
	}

	response.JSON(w, r, http.StatusOK, activateResponse{
		NodeID:         nodeID,
		Status:         "active",
		PreviousStatus: previousStatus,
	})
}
