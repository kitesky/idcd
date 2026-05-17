// Package protocol — auth_pg.go
//
// Production TokenValidator backed by Postgres.
//
// TODO(v2-S3): 替换为 mcp_token 表，schema 见 docs/prd/ER-DIAGRAM.md。
//   - 今天我们查 personal_access_tokens（migration 00014），因为专门的
//     mcp_token 表要等 v2-S3 才落库（详见 auth.go 顶端 + DECISIONS §M D2）。
//   - 接口稳定：v2-S3 切换只需替换本文件的 SQL + Scan 字段。
//
// personal_access_tokens schema (lib/db/migrations/idcd_main/00014):
//
//	id           TEXT PRIMARY KEY
//	user_id      TEXT NOT NULL
//	token_hash   TEXT NOT NULL UNIQUE   ← lookup key (SHA-256 hex)
//	token_prefix TEXT NOT NULL
//	scopes       TEXT[]
//	expires_at   TIMESTAMPTZ            ← nullable; v2-S3 will require NOT NULL
//	created_at   TIMESTAMPTZ
//	updated_at   TIMESTAMPTZ
//
// Revocation today is a DELETE in pat_handler.go — "row not found" already
// covers revoked tokens. We still emit ErrTokenNotFound (mapped to 401 at
// the HTTP layer) so the caller can't distinguish revoked vs unknown.

package protocol

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pgxQuerier is the minimal pgx surface PGTokenValidator needs. Matches the
// shape exposed by *pgxpool.Pool and by pgxmock.PgxPoolIface, so production
// + tests share the same code path. Kept package-private — callers should
// only see PGTokenValidator + NewPGTokenValidator / NewPGTokenValidatorFromPool.
type pgxQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// PGTokenValidator looks up MCP bearer tokens in Postgres.
//
// SECURITY:
//   - Never accepts the raw token in a query parameter — we hash it with
//     SHA-256 (HashToken) before the DB lookup. The DB never sees plaintext.
//   - SELECT pulls expires_at and enforces it in-process (uniform path with
//     StaticTokenValidator). DB-side "WHERE expires_at > NOW()" would
//     conflate "expired" with "not found", which is what we want for the
//     401 response — but we still want to LOG the distinction internally.
type PGTokenValidator struct {
	q       pgxQuerier
	nowFunc func() time.Time
}

// NewPGTokenValidator wires a *pgxpool.Pool. Use this from cmd/mcp/main.go.
func NewPGTokenValidator(pool *pgxpool.Pool) *PGTokenValidator {
	if pool == nil {
		// Defensive: nil pool means the caller forgot to initialise it.
		// Returning a validator with q==nil would explode on first
		// request — better to fail-loud here.
		panic("protocol: NewPGTokenValidator called with nil pool")
	}
	return newPGValidatorWithQuerier(pool)
}

// newPGValidatorWithQuerier is the test seam — accepts any querier
// (production pgxpool, or pgxmock in tests).
func newPGValidatorWithQuerier(q pgxQuerier) *PGTokenValidator {
	return &PGTokenValidator{q: q, nowFunc: time.Now}
}

// SetNow lets tests inject a deterministic clock.
func (v *PGTokenValidator) SetNow(f func() time.Time) {
	v.nowFunc = f
}

// Validate implements TokenValidator. The raw token is SHA-256 hashed and
// looked up in personal_access_tokens. Expired rows yield ErrTokenExpired;
// missing rows yield ErrTokenNotFound. All DB errors propagate as a wrapped
// generic error (mapped to 401 in writeAuthError — we never leak DB state).
func (v *PGTokenValidator) Validate(ctx context.Context, rawToken string) (*Principal, error) {
	hash := HashToken(rawToken)

	var (
		tokenID   string
		userID    string
		expiresAt *time.Time // nullable in v1 PAT schema
	)
	// NOTE(v2-S3): swap to mcp_token and include workspace_id / token_type / scopes.
	err := v.q.QueryRow(ctx,
		`SELECT id, user_id, expires_at FROM personal_access_tokens WHERE token_hash = $1`,
		hash,
	).Scan(&tokenID, &userID, &expiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenNotFound
		}
		return nil, fmt.Errorf("mcp token: db lookup: %w", err)
	}

	// D2 (no permanent MCP token): treat NULL expires_at as already
	// expired. The legacy PAT table still allows NULL — those rows must
	// be rotated to a bounded-lifetime token before they can be used over
	// the MCP HTTP transport.
	if expiresAt == nil {
		return nil, ErrTokenExpired
	}
	if !v.nowFunc().Before(*expiresAt) {
		return nil, ErrTokenExpired
	}

	// v1 PAT fallback: no workspace / token_type / scopes columns yet —
	// emit a "personal" principal so downstream tools can run, but record
	// the token id so per-token rate limits + audit trails work.
	return &Principal{
		TokenID:   tokenID,
		UserID:    userID,
		TokenType: "personal",
	}, nil
}
