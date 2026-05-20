// Package protocol — auth.go
//
// MCP HTTP-transport authentication.
//
// Background (see docs/REVIEW-FINDINGS-2026-05-16.md P0#7 + DECISIONS §M D2):
//   - MCP HTTP transport (SSE + /messages) previously had ZERO auth — anyone
//     could POST to /messages, drive tools/call, and reuse the shared
//     IDCD_API_KEY (tenant isolation lost).
//   - All MCP tokens MUST have an expiry (no permanent tokens):
//     personal 24h / workspace 90d / service 90d (auto_renewal).
//
// Token format expected: "idcd_mcp_<random_hex>". We accept the existing PAT
// format ("idcd_pat_<hex>") as a fallback today because the dedicated
// `mcp_token` table (planned in v2 S3 / 15-data-model §4.X.8) does not yet
// exist in migrations — only `personal_access_tokens` does (00014). Once
// `mcp_token` lands, swap the validator implementation; the interface here
// stays stable.
//
// TODO(v2-S3): replace PAT fallback with dedicated mcp_token validator.
// TODO(v2-S3): add per-token rate limit (token-bucket on token_hash).

package protocol

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Principal identifies the authenticated caller derived from an MCP token.
type Principal struct {
	TokenID     string
	UserID      string
	WorkspaceID string // empty for personal tokens
	TokenType   string // "personal" / "workspace" / "service"
	Scopes      []string
}

// TokenValidator looks up a raw bearer token and returns the authenticated
// Principal. Implementations MUST:
//   - hash the raw token with SHA-256 before DB lookup (never store / compare
//     plaintext);
//   - reject revoked tokens (return ErrTokenRevoked);
//   - reject expired tokens (return ErrTokenExpired);
//   - return ErrTokenNotFound for unknown hashes.
//
// Implementations MUST use constant-time comparison when comparing hashes
// against DB rows (the DB index lookup is acceptable since the hash itself
// is already random and full-entropy).
type TokenValidator interface {
	Validate(ctx context.Context, rawToken string) (*Principal, error)
}

// Sentinel errors returned by TokenValidator implementations. The HTTP layer
// maps all of them to 401 — the response body never reveals which one fired
// (don't help attackers distinguish "expired" vs "revoked" vs "not found").
var (
	ErrTokenNotFound = errors.New("mcp token: not found")
	ErrTokenExpired  = errors.New("mcp token: expired")
	ErrTokenRevoked  = errors.New("mcp token: revoked")
	ErrTokenMissing  = errors.New("mcp token: missing Authorization header")
	ErrTokenMalformed = errors.New("mcp token: malformed bearer token")
)

// HashToken computes the canonical SHA-256 hex hash used to look up tokens
// in the database. Exported so the API server (which mints tokens) can share
// the exact same hashing routine.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// validTokenPrefixes is the set of accepted token prefixes. Today we accept
// "idcd_mcp_" (the target format) and "idcd_pat_" (the fallback against
// personal_access_tokens until the mcp_token table lands).
var validTokenPrefixes = []string{"idcd_mcp_", "idcd_pat_"}

// extractBearerToken pulls the bearer token out of the Authorization header
// and validates its surface format (prefix + min length). Returns
// ErrTokenMissing if absent, ErrTokenMalformed if the shape is wrong.
func extractBearerToken(r *http.Request) (string, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", ErrTokenMissing
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return "", ErrTokenMalformed
	}
	tok := strings.TrimPrefix(auth, prefix)
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return "", ErrTokenMalformed
	}
	ok := false
	for _, p := range validTokenPrefixes {
		if strings.HasPrefix(tok, p) {
			ok = true
			break
		}
	}
	if !ok {
		return "", ErrTokenMalformed
	}
	// Sanity: full-entropy hex tokens are >= 16 chars beyond the prefix.
	if len(tok) < 24 {
		return "", ErrTokenMalformed
	}
	return tok, nil
}

// ─────────────────────────────────────────────
// Context plumbing
// ─────────────────────────────────────────────

type principalCtxKey struct{}

// PrincipalFromContext returns the authenticated principal, or nil if the
// request wasn't authenticated (should never happen on routes mounted under
// AuthMiddleware).
func PrincipalFromContext(ctx context.Context) *Principal {
	v, _ := ctx.Value(principalCtxKey{}).(*Principal)
	return v
}

// withPrincipal attaches the principal to the request context.
func withPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalCtxKey{}, p)
}

// ─────────────────────────────────────────────
// In-memory validator (used for tests + dev wiring before mcp_token lands)
// ─────────────────────────────────────────────

// StaticTokenRecord describes a single token entry for StaticTokenValidator.
type StaticTokenRecord struct {
	RawToken  string // full token value, e.g. "idcd_mcp_abc..."
	TokenID   string
	UserID    string
	Workspace string
	Type      string // personal / workspace / service
	Scopes    []string
	ExpiresAt time.Time // zero ⇒ never expires (only valid for tests)
	Revoked   bool
}

// StaticTokenValidator is an in-memory TokenValidator suitable for tests and
// for dev environments before the mcp_token table is created. It is NOT a
// production validator — production must hit Postgres.
type StaticTokenValidator struct {
	mu      sync.RWMutex
	byHash  map[string]StaticTokenRecord
	nowFunc func() time.Time
}

// NewStaticTokenValidator constructs a validator seeded with the given
// records. Each record's RawToken is SHA-256 hashed for storage.
func NewStaticTokenValidator(records ...StaticTokenRecord) *StaticTokenValidator {
	v := &StaticTokenValidator{
		byHash:  make(map[string]StaticTokenRecord, len(records)),
		nowFunc: time.Now,
	}
	for _, rec := range records {
		v.byHash[HashToken(rec.RawToken)] = rec
	}
	return v
}

// SetNow overrides the clock for deterministic tests.
func (v *StaticTokenValidator) SetNow(f func() time.Time) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.nowFunc = f
}

// Validate implements TokenValidator.
func (v *StaticTokenValidator) Validate(_ context.Context, raw string) (*Principal, error) {
	h := HashToken(raw)
	v.mu.RLock()
	defer v.mu.RUnlock()

	// Constant-time scan: iterate all records and compare hashes with
	// subtle.ConstantTimeCompare. For an in-memory store this avoids a
	// timing side channel that could let an attacker distinguish "first
	// 8 bytes of hash match" from "no match". For Postgres-backed
	// validators the index lookup is fine because the lookup key (a
	// SHA-256 hash) has full entropy.
	var found *StaticTokenRecord
	for storedHash, rec := range v.byHash {
		if subtle.ConstantTimeCompare([]byte(storedHash), []byte(h)) == 1 {
			r := rec
			found = &r
		}
	}
	if found == nil {
		return nil, ErrTokenNotFound
	}
	if found.Revoked {
		return nil, ErrTokenRevoked
	}
	if !found.ExpiresAt.IsZero() && !v.nowFunc().Before(found.ExpiresAt) {
		return nil, ErrTokenExpired
	}
	return &Principal{
		TokenID:     found.TokenID,
		UserID:      found.UserID,
		WorkspaceID: found.Workspace,
		TokenType:   found.Type,
		Scopes:      append([]string(nil), found.Scopes...),
	}, nil
}
