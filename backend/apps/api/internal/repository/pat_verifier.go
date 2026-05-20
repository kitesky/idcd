// Package repository provides server-side database adapters for API handlers
// and middleware. The verifiers in this package satisfy interfaces declared in
// apps/api/internal/middleware (PATVerifier, APIKeyVerifier) and decouple the
// middleware from the underlying pgx schema.
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

// patPool is the minimal pgx surface PATVerifier needs. Implemented by
// *pgxpool.Pool and pgxmock.PgxPoolIface — keeps the type test-friendly.
type patPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// PATVerifier looks up personal_access_tokens rows by SHA-256 token hash.
//
// The hash algorithm and column layout (token_hash, id, user_id, expires_at)
// mirror handler/pat_handler.go — keep them in sync if the schema changes.
// Expiry and revocation checks live in the middleware layer; this verifier
// only resolves a raw token to its row (or "not found"). Revocation is a
// DELETE in pat_handler.go, so a missing row already covers "revoked".
type PATVerifier struct {
	pool patPool
}

// NewPATVerifier returns a verifier backed by the given pgx pool.
func NewPATVerifier(pool patPool) *PATVerifier {
	return &PATVerifier{pool: pool}
}

// VerifyPAT hashes the raw token (SHA-256, hex) and looks it up in
// personal_access_tokens. Returns middleware.PATInfo on success, or an error
// when no row matches. Caller (the middleware) collapses all error variants
// into a generic 401 — we keep error detail here for logs and tests but
// never expose it across the request boundary.
func (v *PATVerifier) VerifyPAT(ctx context.Context, rawToken string) (*middleware.PATInfo, error) {
	if rawToken == "" {
		return nil, errors.New("empty token")
	}
	tokenHash := middleware.HashToken(rawToken)

	var info middleware.PATInfo
	err := v.pool.QueryRow(ctx, `
		SELECT id, user_id, expires_at
		FROM personal_access_tokens
		WHERE token_hash = $1
	`, tokenHash).Scan(&info.ID, &info.UserID, &info.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("pat not found")
		}
		return nil, fmt.Errorf("pat lookup: %w", err)
	}
	return &info, nil
}

// TouchLastUsed bumps last_used_at to NOW() for the given PAT id. Called
// fire-and-forget by the middleware after a successful auth; the middleware
// detaches the request context and applies its own 2s timeout, so this method
// just needs to issue the UPDATE and surface any error for logging.
func (v *PATVerifier) TouchLastUsed(ctx context.Context, patID string) error {
	if patID == "" {
		return errors.New("empty pat id")
	}
	_, err := v.pool.Exec(ctx, `
		UPDATE personal_access_tokens
		SET last_used_at = $1
		WHERE id = $2
	`, time.Now().UTC(), patID)
	if err != nil {
		return fmt.Errorf("pat touch: %w", err)
	}
	return nil
}
