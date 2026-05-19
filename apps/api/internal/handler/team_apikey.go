package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
)

type TeamAPIKeyPool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type TeamAPIKeyHandler struct {
	pool TeamAPIKeyPool
}

func NewTeamAPIKeyHandler(pool TeamAPIKeyPool) *TeamAPIKeyHandler {
	return &TeamAPIKeyHandler{pool: pool}
}

type teamAPIKeyResponse struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Prefix    string   `json:"prefix"`
	Scopes    []string `json:"scopes"`
	Status    string   `json:"status"`
	KeyType   string   `json:"key_type"`
	CreatedAt string   `json:"created_at"`
}

type createTeamAPIKeyRequest struct {
	Name    string `json:"name"`
	KeyType string `json:"key_type"`
}

func (h *TeamAPIKeyHandler) requireAdminRole(ctx context.Context, teamID, userID string) error {
	var role string
	err := h.pool.QueryRow(ctx,
		`SELECT role FROM team_memberships WHERE team_id = $1 AND user_id = $2`,
		teamID, userID,
	).Scan(&role)
	if err != nil {
		return err
	}
	if role != "owner" && role != "admin" {
		return pgx.ErrNoRows
	}
	return nil
}

func (h *TeamAPIKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	teamID := chi.URLParam(r, "id")

	if err := h.requireAdminRole(r.Context(), teamID, userID); err != nil {
		response.Error(w, r, apperr.Forbidden("owner or admin required"))
		return
	}

	var req createTeamAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.Name == "" {
		response.Error(w, r, apperr.Validation("name is required", ""))
		return
	}

	ktype := keyTypeProduction
	if req.KeyType == keyTypeTest {
		ktype = keyTypeTest
	}

	rawKey, prefix, hash, err := generateAPIKey(ktype)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to generate key", err))
		return
	}

	keyID := idgen.New("key")

	var id, name, keyPfx, status, kt string
	var scopes []string
	var createdAt pgtype.Timestamptz

	err = h.pool.QueryRow(r.Context(),
		`INSERT INTO api_key
			(id, owner_type, owner_id, team_id, name, prefix, secret_hash, scopes, created_by, key_type)
		 VALUES ($1, 'team', $2, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, name, prefix, scopes, status, key_type, created_at`,
		keyID, teamID, req.Name, prefix, hash,
		[]string{"read", "write"}, userID, ktype,
	).Scan(&id, &name, &keyPfx, &scopes, &status, &kt, &createdAt)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to create team api key", err))
		return
	}

	pfx := apiKeyLivePrefix
	if ktype == keyTypeTest {
		pfx = apiKeyTestPrefix
	}

	type createResp struct {
		teamAPIKeyResponse
		Key string `json:"key"`
	}
	response.JSON(w, r, http.StatusCreated, map[string]any{
		"api_key": createResp{
			teamAPIKeyResponse: teamAPIKeyResponse{
				ID:        id,
				Name:      name,
				Prefix:    pfx + keyPfx + "...",
				Scopes:    scopes,
				Status:    status,
				KeyType:   kt,
				CreatedAt: createdAt.Time.UTC().Format(time.RFC3339),
			},
			Key: rawKey,
		},
	})
}

func (h *TeamAPIKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	teamID := chi.URLParam(r, "id")

	var isMember bool
	if err := h.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM team_memberships WHERE team_id = $1 AND user_id = $2)`,
		teamID, userID,
	).Scan(&isMember); err != nil || !isMember {
		response.Error(w, r, apperr.Forbidden("not a team member"))
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT id, name, prefix, scopes, status, key_type, created_at
		 FROM api_key
		 WHERE owner_type = 'team' AND owner_id = $1 AND status = 'active'
		 ORDER BY created_at DESC`,
		teamID,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list team api keys", err))
		return
	}
	defer rows.Close()

	items := make([]teamAPIKeyResponse, 0)
	for rows.Next() {
		var id, name, prefix, status, ktype string
		var scopes []string
		var createdAt pgtype.Timestamptz
		if err := rows.Scan(&id, &name, &prefix, &scopes, &status, &ktype, &createdAt); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan api key", err))
			return
		}
		pfx := apiKeyLivePrefix
		if ktype == keyTypeTest {
			pfx = apiKeyTestPrefix
		}
		items = append(items, teamAPIKeyResponse{
			ID:        id,
			Name:      name,
			Prefix:    pfx + prefix + "...",
			Scopes:    scopes,
			Status:    status,
			KeyType:   ktype,
			CreatedAt: createdAt.Time.UTC().Format(time.RFC3339),
		})
	}

	response.JSON(w, r, http.StatusOK, map[string]any{"api_keys": items})
}

func (h *TeamAPIKeyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	teamID := chi.URLParam(r, "id")
	keyID := chi.URLParam(r, "key_id")

	if err := h.requireAdminRole(r.Context(), teamID, userID); err != nil {
		response.Error(w, r, apperr.Forbidden("owner or admin required"))
		return
	}

	var ownerID string
	err := h.pool.QueryRow(r.Context(),
		`SELECT owner_id FROM api_key WHERE id = $1 AND owner_type = 'team' AND status = 'active'`,
		keyID,
	).Scan(&ownerID)
	if err != nil {
		response.Error(w, r, apperr.NotFound("api key not found"))
		return
	}
	if ownerID != teamID {
		response.Error(w, r, apperr.Forbidden("key does not belong to this team"))
		return
	}

	if _, err := h.pool.Exec(r.Context(),
		`UPDATE api_key SET status = 'revoked', revoked_at = now() WHERE id = $1`,
		keyID,
	); err != nil {
		response.Error(w, r, apperr.Internal("failed to revoke api key", err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
