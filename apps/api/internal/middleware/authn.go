// Package middleware provides HTTP middleware for the API server.
package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	apiI18n "github.com/kite365/idcd/apps/api/internal/i18n"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/auth/jwt"
	"github.com/kite365/idcd/lib/auth/session"
	"github.com/kite365/idcd/lib/shared/apperr"
)

type contextKey string

const (
	userIDKey     contextKey = "user_id"
	sessionIDKey  contextKey = "session_id"
	authMethodKey contextKey = "auth_method"
	patIDKey      contextKey = "pat_id"
	apiKeyIDKey   contextKey = "api_key_id"
)

// Authentication method identifiers exposed via context.
const (
	AuthMethodJWT    = "jwt"
	AuthMethodPAT    = "pat"
	AuthMethodAPIKey = "apikey"
)

// Token prefix constants. Kept in sync with handler/pat_handler.go and
// handler/apikey_handler.go — the middleware routes Bearer tokens to the
// right verifier without round-tripping through every backend.
const (
	patBearerPrefix       = "idcd_pat_"
	apiKeyLiveBearerPfx   = "sk_live_"
	apiKeyTestBearerPfx   = "sk_test_"
	apiKeyStatusActive    = "active"
	lastUsedUpdateTimeout = 2 * time.Second
)

// TokenVerifier verifies a JWT and returns its claims.
type TokenVerifier interface {
	Verify(token string) (*jwt.Claims, error)
}

// SessionChecker checks if a session is still active.
type SessionChecker interface {
	Get(ctx context.Context, sessionID string) (*session.SessionData, error)
}

// PATInfo is the minimal projection the middleware needs from a personal
// access token row. Decoupled from the DB schema so tests can mock it
// cleanly and the verifier can live in any package without import cycles.
type PATInfo struct {
	ID        string
	UserID    string
	ExpiresAt *time.Time
}

// PATVerifier resolves a raw PAT token to its owning user.
//
// Implementations should:
//   - Hash the raw token (SHA-256, see HashToken) and look it up in
//     personal_access_tokens by token_hash.
//   - Return an error if the row is missing — revocation is a DELETE in
//     pat_handler.go, so "not found" already covers revoked tokens.
//   - Return the row regardless of expiry. The middleware enforces expiry
//     itself so the rejection path stays uniform across verifiers.
type PATVerifier interface {
	VerifyPAT(ctx context.Context, rawToken string) (*PATInfo, error)
	// TouchLastUsed updates last_used_at for the given PAT id. Called
	// fire-and-forget after a successful auth; errors are swallowed.
	TouchLastUsed(ctx context.Context, patID string) error
}

// APIKeyInfo is the minimal projection the middleware needs from an api_key row.
type APIKeyInfo struct {
	ID        string
	OwnerType string // expected "user" for end-user API keys
	OwnerID   string
	Status    string // expected "active" for usable keys
	ExpiresAt *time.Time
}

// APIKeyVerifier resolves a raw API key (sk_live_*/sk_test_*) to its owning
// user. Same contract as PATVerifier — return the row, middleware checks
// status/expiry uniformly.
type APIKeyVerifier interface {
	VerifyAPIKey(ctx context.Context, rawKey string) (*APIKeyInfo, error)
	TouchLastUsed(ctx context.Context, apiKeyID string) error
}

// Authn enforces JWT + Redis session authentication. Bearer tokens that
// look like a PAT or API key are rejected with 401 — use AuthnWithTokens to
// also accept those.
func Authn(jwtSvc TokenVerifier, sessSvc SessionChecker) func(http.Handler) http.Handler {
	return AuthnWithTokens(jwtSvc, sessSvc, nil, nil)
}

// AuthnWithTokens enforces multi-modal authentication:
//
//   - Bearer tokens prefixed "idcd_pat_*" → PAT lookup (SHA-256 hash).
//   - Bearer tokens prefixed "sk_live_*" / "sk_test_*" → API key lookup.
//   - Anything else → JWT verification + Redis session check.
//
// Pass nil for patSvc/apiKeySvc to disable that path (e.g. for endpoints
// that only allow browser sessions).
func AuthnWithTokens(
	jwtSvc TokenVerifier,
	sessSvc SessionChecker,
	patSvc PATVerifier,
	apiKeySvc APIKeyVerifier,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				response.Error(w, r, apperr.Unauthorized("missing or malformed Authorization header"))
				return
			}

			switch {
			case isPATToken(token):
				if patSvc == nil {
					response.Error(w, r, apperr.Unauthorized("personal access tokens not supported on this endpoint"))
					return
				}
				ctx, ok := verifyPAT(w, r, patSvc, token)
				if !ok {
					return
				}
				w.Header().Set("X-Auth-Method", AuthMethodPAT)
				next.ServeHTTP(w, r.WithContext(ctx))

			case isAPIKeyToken(token):
				if apiKeySvc == nil {
					response.Error(w, r, apperr.Unauthorized("api keys not supported on this endpoint"))
					return
				}
				ctx, ok := verifyAPIKey(w, r, apiKeySvc, token)
				if !ok {
					return
				}
				w.Header().Set("X-Auth-Method", AuthMethodAPIKey)
				next.ServeHTTP(w, r.WithContext(ctx))

			default:
				ctx, ok := verifyJWT(w, r, jwtSvc, sessSvc, token)
				if !ok {
					return
				}
				next.ServeHTTP(w, r.WithContext(ctx))
			}
		})
	}
}

// verifyJWT runs the legacy JWT + Redis session path.
func verifyJWT(w http.ResponseWriter, r *http.Request, jwtSvc TokenVerifier, sessSvc SessionChecker, token string) (context.Context, bool) {
	claims, err := jwtSvc.Verify(token)
	if err != nil {
		response.Error(w, r, apperr.Unauthorized("invalid token"))
		return nil, false
	}
	if _, err := sessSvc.Get(r.Context(), claims.SessionID); err != nil {
		response.Error(w, r, apperr.Unauthorized("session expired or revoked"))
		return nil, false
	}
	ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
	ctx = context.WithValue(ctx, sessionIDKey, claims.SessionID)
	ctx = context.WithValue(ctx, authMethodKey, AuthMethodJWT)
	// Stash claims so the i18n middleware (which ran upstream with only
	// header/Accept-Language) can re-resolve locale from JWT on downstream
	// handlers if they propagate the request context. The middleware itself
	// only runs once at the chain head, but exporters that re-evaluate the
	// chain (or response helpers that pull locale from the JWT) read claims
	// via i18n.ClaimsFromContext.
	ctx = apiI18n.WithClaims(ctx, claims)
	return ctx, true
}

// verifyPAT looks up a PAT, validates expiry, injects context, and kicks
// off an async last_used_at update. Writes the error response itself; the
// bool return signals "OK to call next".
func verifyPAT(w http.ResponseWriter, r *http.Request, patSvc PATVerifier, rawToken string) (context.Context, bool) {
	info, err := patSvc.VerifyPAT(r.Context(), rawToken)
	if err != nil || info == nil {
		// Don't leak whether the token exists or is the wrong shape — all
		// PAT failures collapse to a generic 401. Hash mismatch, missing
		// row (revoked via DELETE), and DB errors all land here.
		response.Error(w, r, apperr.Unauthorized("invalid or expired personal access token"))
		return nil, false
	}
	if info.ExpiresAt != nil && !info.ExpiresAt.IsZero() && time.Now().UTC().After(*info.ExpiresAt) {
		response.Error(w, r, apperr.Unauthorized("invalid or expired personal access token"))
		return nil, false
	}

	ctx := context.WithValue(r.Context(), userIDKey, info.UserID)
	ctx = context.WithValue(ctx, authMethodKey, AuthMethodPAT)
	ctx = context.WithValue(ctx, patIDKey, info.ID)

	// Fire-and-forget last_used update. Detached from request context so a
	// caller cancellation doesn't drop the write. Bounded timeout prevents
	// leaks if the backend is slow.
	go touchLastUsedPAT(patSvc, info.ID)

	return ctx, true
}

// verifyAPIKey is the API-key counterpart of verifyPAT.
func verifyAPIKey(w http.ResponseWriter, r *http.Request, apiKeySvc APIKeyVerifier, rawKey string) (context.Context, bool) {
	info, err := apiKeySvc.VerifyAPIKey(r.Context(), rawKey)
	if err != nil || info == nil {
		response.Error(w, r, apperr.Unauthorized("invalid or expired api key"))
		return nil, false
	}
	// Status defaults to "" only in mock/legacy scenarios; production rows
	// always carry a status. Reject anything that isn't explicitly active.
	if info.Status != "" && info.Status != apiKeyStatusActive {
		response.Error(w, r, apperr.Unauthorized("invalid or expired api key"))
		return nil, false
	}
	if info.ExpiresAt != nil && !info.ExpiresAt.IsZero() && time.Now().UTC().After(*info.ExpiresAt) {
		response.Error(w, r, apperr.Unauthorized("invalid or expired api key"))
		return nil, false
	}

	ctx := context.WithValue(r.Context(), userIDKey, info.OwnerID)
	ctx = context.WithValue(ctx, authMethodKey, AuthMethodAPIKey)
	ctx = context.WithValue(ctx, apiKeyIDKey, info.ID)

	go touchLastUsedAPIKey(apiKeySvc, info.ID)

	return ctx, true
}

// touchLastUsedPAT updates last_used_at out-of-band. Errors are swallowed
// because failing to bump the timestamp must not block authenticated requests.
func touchLastUsedPAT(svc PATVerifier, patID string) {
	ctx, cancel := context.WithTimeout(context.Background(), lastUsedUpdateTimeout)
	defer cancel()
	_ = svc.TouchLastUsed(ctx, patID)
}

func touchLastUsedAPIKey(svc APIKeyVerifier, apiKeyID string) {
	ctx, cancel := context.WithTimeout(context.Background(), lastUsedUpdateTimeout)
	defer cancel()
	_ = svc.TouchLastUsed(ctx, apiKeyID)
}

// isPATToken reports whether the raw token is a personal access token.
func isPATToken(token string) bool {
	return strings.HasPrefix(token, patBearerPrefix)
}

// isAPIKeyToken reports whether the raw token is an API key.
func isAPIKeyToken(token string) bool {
	return strings.HasPrefix(token, apiKeyLiveBearerPfx) || strings.HasPrefix(token, apiKeyTestBearerPfx)
}

// HashToken returns the SHA-256 hex digest of the given raw token. Exposed
// so PAT/APIKey verifier implementations in other packages can share the
// exact algorithm used by the handlers when storing token_hash / secret_hash.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// UserIDFromContext retrieves the authenticated user ID from the request context.
// Returns "" if not authenticated (only call after Authn middleware).
func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}

// SessionIDFromContext retrieves the authenticated session ID from the request context.
// Returns "" if not authenticated (only call after Authn middleware).
// PAT/APIKey-authenticated requests have no session ID.
func SessionIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(sessionIDKey).(string)
	return v
}

// AuthMethodFromContext returns "jwt", "pat", "apikey", or "" depending on
// how the request was authenticated. Useful for downstream middleware
// (audit logs, CSRF bypass) that needs to behave differently for
// token-based callers.
func AuthMethodFromContext(ctx context.Context) string {
	v, _ := ctx.Value(authMethodKey).(string)
	return v
}

// PATIDFromContext returns the PAT id if the request was authenticated via
// PAT, else "".
func PATIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(patIDKey).(string)
	return v
}

// APIKeyIDFromContext returns the API key id if the request was authenticated
// via API key, else "".
func APIKeyIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(apiKeyIDKey).(string)
	return v
}

// UserIDContextKey returns the context key used to store user IDs.
// Exposed for tests that need to inject a user ID without running the full middleware.
func UserIDContextKey() any {
	return userIDKey
}

// SessionIDContextKey returns the context key used to store session IDs.
// Exposed for tests that need to inject a session ID without running the full middleware.
func SessionIDContextKey() any {
	return sessionIDKey
}

// AuthMethodContextKey returns the context key used to store the auth method.
// Exposed for tests.
func AuthMethodContextKey() any {
	return authMethodKey
}

// extractBearerToken extracts a token from the Authorization header or the
// access_token HttpOnly cookie. Cookie takes precedence for browser clients;
// Bearer is kept for non-browser API clients (CLI tools, server-to-server,
// PATs, API keys).
func extractBearerToken(r *http.Request) string {
	if cookie, err := r.Cookie("access_token"); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(auth, "Bearer ")
}
