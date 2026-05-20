package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/apps/api/internal/middleware"
)

// apiKeyPool mirrors patPool — kept type-distinct so the two verifiers stay
// independently mockable. (Same underlying *pgxpool.Pool / pgxmock works for
// both.)
type apiKeyPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// APIKeyVerifier looks up api_key rows by SHA-256 secret hash.
//
// The api_key table is created by sqlc — schema lives in lib/db (singular
// "api_key", columns id, owner_type, owner_id, secret_hash, status,
// expires_at). Status/expiry checks happen in the middleware layer; this
// verifier just resolves the row.
//
// Per CLAUDE.md D1 (no cross-schema FKs), we read api_key directly here
// without any join — there's nothing to join against in another schema.
type APIKeyVerifier struct {
	pool apiKeyPool
}

// NewAPIKeyVerifier returns a verifier backed by the given pgx pool.
func NewAPIKeyVerifier(pool apiKeyPool) *APIKeyVerifier {
	return &APIKeyVerifier{pool: pool}
}

// VerifyAPIKey hashes the raw key and looks it up in api_key. Returns the
// minimal projection middleware needs (id / owner / status / expiry); the
// middleware enforces status='active' and expires_at uniformly.
func (v *APIKeyVerifier) VerifyAPIKey(ctx context.Context, rawKey string) (*middleware.APIKeyInfo, error) {
	if rawKey == "" {
		return nil, errors.New("empty api key")
	}
	hash := middleware.HashToken(rawKey)

	var info middleware.APIKeyInfo
	err := v.pool.QueryRow(ctx, `
		SELECT id, owner_type, owner_id, status, expires_at
		FROM api_key
		WHERE secret_hash = $1
	`, hash).Scan(&info.ID, &info.OwnerType, &info.OwnerID, &info.Status, &info.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("api key not found")
		}
		return nil, fmt.Errorf("api key lookup: %w", err)
	}
	return &info, nil
}

// TouchLastUsed updates last_used_at = NOW() and increments usage_total for
// the given api_key id. We deliberately don't write last_used_ip here —
// passing the request IP through would couple middleware to this verifier and
// the existing UpdateAPIKeyLastUsed sqlc query already covers that path for
// callers that need it. Fire-and-forget from the middleware.
func (v *APIKeyVerifier) TouchLastUsed(ctx context.Context, apiKeyID string) error {
	if apiKeyID == "" {
		return errors.New("empty api key id")
	}
	_, err := v.pool.Exec(ctx, `
		UPDATE api_key
		SET last_used_at = $1, usage_total = usage_total + 1
		WHERE id = $2
	`, time.Now().UTC(), apiKeyID)
	if err != nil {
		return fmt.Errorf("api key touch: %w", err)
	}
	return nil
}
