package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
)

const (
	csrfCookieName = "csrf_token"
	csrfHeaderName = "X-CSRF-Token"
	csrfTokenLen   = 32 // 32 bytes = 64 hex chars
)

// CSRF implements double-submit cookie pattern for CSRF protection.
// On GET requests: generates and sets csrf_token cookie (HttpOnly=false for JS access).
// On POST/PUT/PATCH/DELETE: validates X-CSRF-Token header matches cookie.
// Exempt paths and Bearer token requests are not validated.
func CSRF(exemptPaths ...string) func(http.Handler) http.Handler {
	// Build exempt path map for fast lookup
	exempt := make(map[string]bool)
	for _, p := range exemptPaths {
		exempt[p] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			// Exempt paths - no CSRF check
			if exempt[path] {
				next.ServeHTTP(w, r)
				return
			}

			// Exempt paths by prefix
			if strings.HasPrefix(path, "/v1/auth/") ||
				strings.HasPrefix(path, "/v1/probe/") ||
				strings.HasPrefix(path, "/v1/info/") {
				next.ServeHTTP(w, r)
				return
			}

			// Exempt Bearer token requests - already authenticated
			if authHeader := r.Header.Get("Authorization"); authHeader != "" {
				if strings.HasPrefix(authHeader, "Bearer ") {
					next.ServeHTTP(w, r)
					return
				}
			}

			// OPTIONS: pass through without CSRF (preflight requests)
			if r.Method == "OPTIONS" {
				next.ServeHTTP(w, r)
				return
			}

			// Safe methods (GET, HEAD): generate/refresh token if missing
			if r.Method == "GET" || r.Method == "HEAD" {
				token := getCSRFToken(r)
				if token == "" {
					token = generateCSRFToken()
					http.SetCookie(w, &http.Cookie{
						Name:     csrfCookieName,
						Value:    token,
						Path:     "/",
						HttpOnly: false, // Allow JS to read for header submission
						Secure:   r.TLS != nil,
						SameSite: http.SameSiteStrictMode,
					})
				}
				next.ServeHTTP(w, r)
				return
			}

			// Mutating methods: validate CSRF token
			if !validateCSRFToken(r) {
				http.Error(w, "CSRF token invalid or missing", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// generateCSRFToken creates a random 32-byte token as hex string.
func generateCSRFToken() string {
	b := make([]byte, csrfTokenLen)
	if _, err := rand.Read(b); err != nil {
		// Fallback to insecure but better-than-nothing
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// getCSRFToken extracts CSRF token from cookie.
func getCSRFToken(r *http.Request) string {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// validateCSRFToken checks that X-CSRF-Token header matches csrf_token cookie.
func validateCSRFToken(r *http.Request) bool {
	headerToken := r.Header.Get(csrfHeaderName)
	cookieToken := getCSRFToken(r)

	if headerToken == "" || cookieToken == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(headerToken), []byte(cookieToken)) == 1
}
