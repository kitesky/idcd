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

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
)

const (
	apiKeyLivePrefix = "sk_live_"
	apiKeyTestPrefix = "sk_test_"
	apiKeyRawBytes   = 32
	apiKeyPrefixLen  = 8 // characters from raw key used as prefix (after sk_live_ / sk_test_)
	apiKeyOwnerType  = "user"

	keyTypeProduction = "production"
	keyTypeTest       = "test"
)

// APIKeyQuerier is the subset of sqlc Queries used by APIKeyHandler.
type APIKeyQuerier interface {
	CreateAPIKey(ctx context.Context, arg idcdmain.CreateAPIKeyParams) (idcdmain.ApiKey, error)
	ListAPIKeysByOwner(ctx context.Context, arg idcdmain.ListAPIKeysByOwnerParams) ([]idcdmain.ApiKey, error)
	RevokeAPIKey(ctx context.Context, id string) error
	GetAPIKeyByID(ctx context.Context, id string) (idcdmain.ApiKey, error)
}

// APIKeyHandler implements /v1/account/api-keys endpoints.
type APIKeyHandler struct {
	q APIKeyQuerier
}

// NewAPIKeyHandler creates an APIKeyHandler.
func NewAPIKeyHandler(q APIKeyQuerier) *APIKeyHandler {
	return &APIKeyHandler{q: q}
}

// ─────────────────────────────────────────────
// Response types
// ─────────────────────────────────────────────

// apiKeyResponse is the public representation of an API key.
// The secret_hash is never returned; key_prefix is masked as sk_live_xxx...xxx.
type apiKeyResponse struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	KeyPrefix  string   `json:"key_prefix"`
	Scopes     []string `json:"scopes"`
	Status     string   `json:"status"`
	Type       string   `json:"type"`
	CreatedAt  string   `json:"created_at"`
	LastUsedAt *string  `json:"last_used_at"`
}

// apiKeyCreateResponse extends apiKeyResponse with the one-time full key.
type apiKeyCreateResponse struct {
	apiKeyResponse
	Key string `json:"key"`
}

func toAPIKeyResponse(k idcdmain.ApiKey) apiKeyResponse {
	var lastUsed *string
	if k.LastUsedAt.Valid {
		s := k.LastUsedAt.Time.UTC().Format(time.RFC3339)
		lastUsed = &s
	}
	ktype := k.KeyType
	if ktype == "" {
		ktype = keyTypeProduction
	}
	pfx := apiKeyLivePrefix
	if ktype == keyTypeTest {
		pfx = apiKeyTestPrefix
	}
	return apiKeyResponse{
		ID:         k.ID,
		Name:       k.Name,
		KeyPrefix:  pfx + k.Prefix + "...",
		Scopes:     k.Scopes,
		Status:     k.Status,
		Type:       ktype,
		CreatedAt:  k.CreatedAt.Time.UTC().Format(time.RFC3339),
		LastUsedAt: lastUsed,
	}
}

// ─────────────────────────────────────────────
// Handlers
// ─────────────────────────────────────────────

// createAPIKeyRequest is the body for POST /v1/account/api-keys.
type createAPIKeyRequest struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// CreateAPIKey handles POST /v1/account/api-keys.
// Returns the full key value ONCE — callers must store it securely.
func (h *APIKeyHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	var req createAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.Name == "" {
		response.Error(w, r, apperr.Validation("name is required", ""))
		return
	}

	ktype := keyTypeProduction
	if req.Type == keyTypeTest {
		ktype = keyTypeTest
	}

	rawKey, prefix, hash, err := generateAPIKey(ktype)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to generate key", err))
		return
	}

	keyID := idgen.New("key")
	expiresAt := pgtype.Timestamptz{} // no expiry for personal keys (D2: 90d enforced separately)

	k, err := h.q.CreateAPIKey(r.Context(), idcdmain.CreateAPIKeyParams{
		ID:         keyID,
		OwnerType:  apiKeyOwnerType,
		OwnerID:    userID,
		Name:       req.Name,
		Prefix:     prefix,
		SecretHash: hash,
		Scopes:     []string{"read", "write"},
		CreatedBy:  userID,
		KeyType:    ktype,
	})
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to create api key", err))
		return
	}

	pfx := apiKeyLivePrefix
	if ktype == keyTypeTest {
		pfx = apiKeyTestPrefix
	}

	resp := apiKeyCreateResponse{
		apiKeyResponse: toAPIKeyResponse(k),
		Key:            rawKey,
	}
	// Override the masked key_prefix to show full prefix in create response.
	resp.KeyPrefix = pfx + prefix + "..."

	_ = expiresAt // reserved for future expiry support

	response.JSON(w, r, http.StatusCreated, resp)
}

// ListAPIKeys handles GET /v1/account/api-keys.
func (h *APIKeyHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	keys, err := h.q.ListAPIKeysByOwner(r.Context(), idcdmain.ListAPIKeysByOwnerParams{
		OwnerType: apiKeyOwnerType,
		OwnerID:   userID,
	})
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list api keys", err))
		return
	}

	items := make([]apiKeyResponse, 0, len(keys))
	for _, k := range keys {
		items = append(items, toAPIKeyResponse(k))
	}

	response.JSON(w, r, http.StatusOK, map[string]any{"api_keys": items})
}

// RevokeAPIKey handles DELETE /v1/account/api-keys/:id.
func (h *APIKeyHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	keyID := chi.URLParam(r, "id")
	if keyID == "" {
		response.Error(w, r, apperr.Validation("key id is required", ""))
		return
	}

	// Verify ownership before revoking.
	k, err := h.q.GetAPIKeyByID(r.Context(), keyID)
	if err != nil {
		response.Error(w, r, apperr.NotFound("api key not found"))
		return
	}
	if k.OwnerID != userID {
		response.Error(w, r, apperr.Forbidden("not your api key"))
		return
	}

	if err := h.q.RevokeAPIKey(r.Context(), keyID); err != nil {
		response.Error(w, r, apperr.Internal("failed to revoke api key", err))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]string{"message": "api key revoked"})
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

// generateAPIKey creates a cryptographically random API key.
// Returns the full key (sk_live_<hex> or sk_test_<hex>), a prefix for lookup, and a SHA-256 hash.
func generateAPIKey(ktype string) (fullKey, prefix, hash string, err error) {
	b := make([]byte, apiKeyRawBytes)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", fmt.Errorf("rand.Read: %w", err)
	}
	rawHex := hex.EncodeToString(b)
	prefix = rawHex[:apiKeyPrefixLen]
	pfx := apiKeyLivePrefix
	if ktype == keyTypeTest {
		pfx = apiKeyTestPrefix
	}
	fullKey = pfx + rawHex
	sum := sha256.Sum256([]byte(fullKey))
	hash = hex.EncodeToString(sum[:])
	return fullKey, prefix, hash, nil
}
