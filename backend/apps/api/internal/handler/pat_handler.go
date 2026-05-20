package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
)

const (
	patTokenPrefix    = "idcd_pat_"
	patRawBytes       = 32
	patDisplayPrefixN = 8
)

// allowedPATScopes is the closed set of scopes a Personal Access Token may
// declare. Anything else is rejected at creation time so a user cannot mint
// a token bearing privileges that the rest of the system doesn't recognise
// (the auth middleware silently ignores unknown scopes, which would let an
// attacker fabricate "admin"/"billing:refund" labels on their own tokens).
//
// Keep this list in sync with the scopes accepted by downstream authorization
// checks. New scopes must be added here and to the relevant scope-checking
// middleware in the same change.
var allowedPATScopes = map[string]struct{}{
	"read":            {},
	"write":           {},
	"read:monitors":   {},
	"write:monitors":  {},
	"read:probes":     {},
	"write:probes":    {},
	"read:certs":      {},
	"write:certs":     {},
	"read:verdicts":   {},
	"write:verdicts":  {},
	"read:billing":    {},
	"read:teams":      {},
	"write:teams":     {},
	"read:status":     {},
	"write:status":    {},
	"read:alerts":     {},
}

// validatePATScopes returns the first invalid scope encountered, or "" if all
// scopes are in the allowlist. An empty slice is valid (no extra grants).
func validatePATScopes(scopes []string) string {
	for _, s := range scopes {
		if _, ok := allowedPATScopes[s]; !ok {
			return s
		}
	}
	return ""
}

// PATPool is the minimal pgx interface needed by PATHandler.
type PATPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// PATHandler implements /v1/account/tokens endpoints.
type PATHandler struct {
	pool PATPool
}

// NewPATHandler creates a PATHandler backed by the given pool.
func NewPATHandler(pool PATPool) *PATHandler {
	return &PATHandler{pool: pool}
}

// patResponse is the public representation of a PAT (no token value).
type patResponse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	TokenPrefix string   `json:"token_prefix"`
	Scopes      []string `json:"scopes"`
	ExpiresAt   *string  `json:"expires_at"`
	CreatedAt   string   `json:"created_at"`
}

// patCreateResponse extends patResponse with the one-time token value.
type patCreateResponse struct {
	patResponse
	Token string `json:"token"`
}

// createPATRequest is the body for POST /v1/account/tokens.
type createPATRequest struct {
	Name        string   `json:"name"`
	Scopes      []string `json:"scopes"`
	ExpiresDays *int     `json:"expires_days"`
}

// Create handles POST /v1/account/tokens.
func (h *PATHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	var req createPATRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.Name == "" {
		response.Error(w, r, apperr.Validation("name is required", ""))
		return
	}
	if len(req.Scopes) == 0 {
		req.Scopes = []string{}
	}
	if bad := validatePATScopes(req.Scopes); bad != "" {
		response.Error(w, r, apperr.Validation("unknown scope: "+bad, "scopes"))
		return
	}

	rawToken, tokenPrefix, tokenHash, err := generatePAT()
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to generate token", err))
		return
	}

	patID := idgen.New("pat_")

	var expiresAt *time.Time
	if req.ExpiresDays != nil && *req.ExpiresDays > 0 {
		t := time.Now().UTC().Add(time.Duration(*req.ExpiresDays) * 24 * time.Hour)
		expiresAt = &t
	}

	now := time.Now().UTC()

	row := h.pool.QueryRow(r.Context(), `
		INSERT INTO personal_access_tokens
			(id, user_id, name, token_hash, token_prefix, scopes, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
		RETURNING id, name, token_prefix, scopes, expires_at, created_at`,
		patID, userID, req.Name, tokenHash, tokenPrefix, req.Scopes, expiresAt, now,
	)

	var result patRow
	if err := scanPATRow(row, &result); err != nil {
		response.Error(w, r, apperr.Internal("failed to create token", err))
		return
	}

	resp := patCreateResponse{
		patResponse: toPATResponse(result),
		Token:       rawToken,
	}
	response.JSON(w, r, http.StatusCreated, resp)
}

// List handles GET /v1/account/tokens.
func (h *PATHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT id, name, token_prefix, scopes, expires_at, created_at
		FROM personal_access_tokens
		WHERE user_id = $1
		ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list tokens", err))
		return
	}
	defer rows.Close()

	items := make([]patResponse, 0)
	for rows.Next() {
		var pr patRow
		if err := rows.Scan(&pr.id, &pr.name, &pr.tokenPrefix, &pr.scopes, &pr.expiresAt, &pr.createdAt); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan token row", err))
			return
		}
		items = append(items, toPATResponse(pr))
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to read tokens", err))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]any{"tokens": items})
}

// Delete handles DELETE /v1/account/tokens/{id}.
func (h *PATHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	patID := chi.URLParam(r, "id")
	if patID == "" {
		response.Error(w, r, apperr.Validation("token id is required", ""))
		return
	}

	var ownerID string
	err := h.pool.QueryRow(r.Context(),
		`SELECT user_id FROM personal_access_tokens WHERE id = $1`,
		patID,
	).Scan(&ownerID)
	if err != nil {
		if err == pgx.ErrNoRows {
			response.Error(w, r, apperr.NotFound("token not found"))
			return
		}
		response.Error(w, r, apperr.Internal("failed to fetch token", err))
		return
	}

	if ownerID != userID {
		response.Error(w, r, apperr.Forbidden("not your token"))
		return
	}

	tag, err := h.pool.Exec(r.Context(),
		`DELETE FROM personal_access_tokens WHERE id = $1`,
		patID,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to delete token", err))
		return
	}
	if tag.RowsAffected() == 0 {
		response.Error(w, r, apperr.NotFound("token not found"))
		return
	}

	response.JSON(w, r, http.StatusNoContent, nil)
}

// ─────────────────────────────────────────────
// Internal types & helpers
// ─────────────────────────────────────────────

type patRow struct {
	id          string
	name        string
	tokenPrefix string
	scopes      []string
	expiresAt   *time.Time
	createdAt   time.Time
}

func scanPATRow(row pgx.Row, pr *patRow) error {
	return row.Scan(&pr.id, &pr.name, &pr.tokenPrefix, &pr.scopes, &pr.expiresAt, &pr.createdAt)
}

func toPATResponse(pr patRow) patResponse {
	var expiresAt *string
	if pr.expiresAt != nil {
		s := pr.expiresAt.UTC().Format(time.RFC3339)
		expiresAt = &s
	}
	scopes := pr.scopes
	if scopes == nil {
		scopes = []string{}
	}
	return patResponse{
		ID:          pr.id,
		Name:        pr.name,
		TokenPrefix: pr.tokenPrefix,
		Scopes:      scopes,
		ExpiresAt:   expiresAt,
		CreatedAt:   pr.createdAt.UTC().Format(time.RFC3339),
	}
}

// generatePAT creates a cryptographically random PAT.
// Returns the full token (idcd_pat_<hex>), its display prefix, and a SHA-256 hash.
func generatePAT() (fullToken, tokenPrefix, tokenHash string, err error) {
	b := make([]byte, patRawBytes)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", err
	}
	rawHex := hex.EncodeToString(b)
	fullToken = patTokenPrefix + rawHex
	tokenPrefix = patTokenPrefix + rawHex[:patDisplayPrefixN]
	sum := sha256.Sum256([]byte(fullToken))
	tokenHash = hex.EncodeToString(sum[:])
	return fullToken, tokenPrefix, tokenHash, nil
}
