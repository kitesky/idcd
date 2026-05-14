package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
)

// NodeEnrollmentPool is the minimal DB interface needed by NodeEnrollmentHandler.
type NodeEnrollmentPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
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

// CreateEnrollmentToken issues a one-time enrollment token.
// POST /internal/admin/nodes/enrollment-tokens
func (h *NodeEnrollmentHandler) CreateEnrollmentToken(w http.ResponseWriter, r *http.Request) {
	// Simple token-based admin auth (same as other admin endpoints)
	if r.Header.Get("X-Admin-Token") != h.adminToken {
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
	rawToken, err := generateSecureToken()
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to generate token", err))
		return
	}
	token := "ent_" + rawToken
	tokenHash := hashToken(token)
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

	tokenHash := hashToken(req.Token)

	// Look up token, check validity, mark as used atomically
	var (
		tokenID   string
		expiresAt time.Time
		usedAt    *time.Time
	)
	err := h.pool.QueryRow(r.Context(), `
		SELECT id, expires_at, used_at
		FROM node_enrollment_tokens
		WHERE token_hash = $1
	`, tokenHash).Scan(&tokenID, &expiresAt, &usedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			response.Error(w, r, apperr.Unauthorized("invalid or expired enrollment token"))
		} else {
			response.Error(w, r, apperr.Internal("token lookup failed", err))
		}
		return
	}
	if usedAt != nil {
		response.Error(w, r, apperr.Unauthorized("enrollment token has already been used"))
		return
	}
	if time.Now().After(expiresAt) {
		response.Error(w, r, apperr.Unauthorized("enrollment token has expired"))
		return
	}

	// Generate node credentials
	nodeID := idgen.New("nd_")
	secretKey, err := generateSecureToken()
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to generate credentials", err))
		return
	}
	secretHash := hashToken(secretKey)

	// Capture client IP
	clientIP := extractClientIP(r)

	// Insert enrolled node
	nodeRecordID := idgen.New("en_")
	_, err = h.pool.Exec(r.Context(), `
		INSERT INTO enrolled_nodes
		  (id, node_id, secret_hash, hostname, arch, os, kernel, ip_address, agent_version, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'pending')
	`, nodeRecordID, nodeID, secretHash, req.Hostname, req.Arch, req.OS, req.Kernel, clientIP, req.Version)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to register node", err))
		return
	}

	// Mark token as used
	_, err = h.pool.Exec(r.Context(), `
		UPDATE node_enrollment_tokens
		SET used_at = now(), used_by = $1
		WHERE id = $2
	`, nodeID, tokenID)
	if err != nil {
		// Non-fatal — node is registered; log and continue
		_ = err
	}

	response.JSON(w, r, http.StatusCreated, enrollResponse{
		NodeID:     nodeID,
		SecretKey:  secretKey,
		GatewayURL: h.gatewayURL,
	})
}

// --- helpers ---

func generateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// hashToken returns the lowercase hex SHA-256 of s.
func hashToken(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first (leftmost) address — closest to the real client
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	return r.RemoteAddr
}
