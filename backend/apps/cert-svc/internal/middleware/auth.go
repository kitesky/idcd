// Package middleware provides HTTP middleware for cert-svc.
//
// cert-svc lives behind apps/api in production but exposes its own HTTP
// surface for the certificate management endpoints. We re-use lib/auth/jwt
// + lib/auth/session so a single signed-in user session works across both
// services, but keep a slim middleware here so cert-svc never depends on
// apps/api's broader auth surface (PATs, API keys, etc.).
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/kite365/idcd/lib/auth/jwt"
	"github.com/kite365/idcd/lib/auth/session"
)

type contextKey string

const (
	userIDKey    contextKey = "user_id"
	sessionIDKey contextKey = "session_id"
)

// ErrUnauthenticated is returned by UserIDFromContext when no user id is
// present on the context. Handlers should treat this as a programming
// error — the auth middleware must run before any handler that needs the
// user id.
var ErrUnauthenticated = errors.New("middleware: unauthenticated request")

// TokenVerifier verifies a JWT and returns its claims. *jwt.Service
// implements this; tests provide a stub.
type TokenVerifier interface {
	Verify(token string) (*jwt.Claims, error)
}

// SessionChecker validates that a session id is still active. The
// *session.Service Get method satisfies this contract.
type SessionChecker interface {
	Get(ctx context.Context, sessionID string) (*session.SessionData, error)
}

// Authn enforces JWT + Redis session authentication for every wrapped
// route. Anonymous requests, requests with malformed or expired tokens
// and requests whose session has been revoked all collapse to a generic
// 401 — we do not leak which check failed.
//
// On success the authenticated user id and session id are attached to
// the request context; handlers read them via UserIDFromContext.
func Authn(jwtSvc TokenVerifier, sessSvc SessionChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				writeUnauthorized(w, "missing or malformed Authorization header")
				return
			}
			if jwtSvc == nil || sessSvc == nil {
				writeUnauthorized(w, "auth backend not configured")
				return
			}
			claims, err := jwtSvc.Verify(token)
			if err != nil || claims == nil {
				writeUnauthorized(w, "invalid token")
				return
			}
			if _, err := sessSvc.Get(r.Context(), claims.SessionID); err != nil {
				writeUnauthorized(w, "session expired or revoked")
				return
			}
			if claims.UserID == "" {
				writeUnauthorized(w, "token has no user id")
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
			ctx = context.WithValue(ctx, sessionIDKey, claims.SessionID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext returns the authenticated user id, or
// ErrUnauthenticated if the Authn middleware did not run.
func UserIDFromContext(ctx context.Context) (string, error) {
	v, ok := ctx.Value(userIDKey).(string)
	if !ok || v == "" {
		return "", ErrUnauthenticated
	}
	return v, nil
}

// SessionIDFromContext returns the authenticated session id, or "" if
// the Authn middleware did not run.
func SessionIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(sessionIDKey).(string)
	return v
}

// UserIDContextKey is exported so tests can inject a user id directly
// onto a context without running the full middleware.
func UserIDContextKey() any { return userIDKey }

// WithUserID returns a derived context carrying the given user id. Use
// from tests that need an authenticated context without running the
// middleware.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// extractBearerToken reads the access token from the access_token cookie
// (browser session) or the Authorization: Bearer header (CLI / SDK).
// Cookie takes precedence so the cookie-set CSRF protection in apps/api
// remains effective when both are present.
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

// writeUnauthorized renders a uniform 401 body. Kept in this package so
// every middleware-level rejection emits an identical shape — the
// handler package has its own writeJSON for business errors.
func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized","code":"CERT_UNAUTHORIZED","message":"` + escapeMsg(msg) + `"}`))
}

// escapeMsg trims any quotes / backslashes that could break the
// hand-rolled JSON in writeUnauthorized. We intentionally avoid pulling
// encoding/json in here because the message strings are static and
// allocation-sensitive in the 401 hot path.
func escapeMsg(s string) string {
	if !strings.ContainsAny(s, `"\`) {
		return s
	}
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' || c == '\\' {
			b = append(b, '\\')
		}
		b = append(b, c)
	}
	return string(b)
}
