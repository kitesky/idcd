// Package middleware provides HTTP middleware for the API server.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/auth/jwt"
	"github.com/kite365/idcd/lib/auth/session"
	"github.com/kite365/idcd/lib/shared/apperr"
)

type contextKey string

const (
	userIDKey    contextKey = "user_id"
	sessionIDKey contextKey = "session_id"
)

// TokenVerifier verifies a JWT and returns its claims.
type TokenVerifier interface {
	Verify(token string) (*jwt.Claims, error)
}

// SessionChecker checks if a session is still active.
type SessionChecker interface {
	Get(ctx context.Context, sessionID string) (*session.SessionData, error)
}

// Authn enforces JWT + Redis session authentication.
func Authn(jwtSvc TokenVerifier, sessSvc SessionChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				response.Error(w, r, apperr.Unauthorized("missing or malformed Authorization header"))
				return
			}

			claims, err := jwtSvc.Verify(token)
			if err != nil {
				response.Error(w, r, apperr.Unauthorized("invalid token"))
				return
			}

			// Verify the session is still active in Redis.
			if _, err := sessSvc.Get(r.Context(), claims.SessionID); err != nil {
				response.Error(w, r, apperr.Unauthorized("session expired or revoked"))
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
			ctx = context.WithValue(ctx, sessionIDKey, claims.SessionID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext retrieves the authenticated user ID from the request context.
// Returns "" if not authenticated (only call after Authn middleware).
func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}

// SessionIDFromContext retrieves the authenticated session ID from the request context.
// Returns "" if not authenticated (only call after Authn middleware).
func SessionIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(sessionIDKey).(string)
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

// extractBearerToken extracts a JWT from the Authorization header or the
// access_token HttpOnly cookie. Cookie takes precedence for browser clients;
// Bearer is kept for non-browser API clients (CLI tools, server-to-server).
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
