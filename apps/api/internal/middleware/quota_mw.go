package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// APIQuotaRateLimiter is the interface required by APIQuotaMiddleware.
// *quota.APIRateLimiter satisfies this interface.
type APIQuotaRateLimiter interface {
	Allow(ctx context.Context, userID string, plan string) (allowed bool, used int, limit int, err error)
}

// APIPlanLookup fetches the subscription plan for a user.
// The function should return "free" when no active subscription exists.
type APIPlanLookup func(ctx context.Context, userID string) string

// APIQuotaMiddleware enforces per-user daily API call quota.
// It is applied only to authenticated routes (requests where UserIDFromContext
// returns a non-empty value). Unauthenticated requests are passed through.
// Test keys (Bearer token starting with sk_test_) bypass quota deduction.
//
// On quota exceeded the middleware responds with HTTP 429 and a JSON body
// containing the error code and reset_at timestamp.
func APIQuotaMiddleware(rateLimiter APIQuotaRateLimiter, planLookup APIPlanLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := UserIDFromContext(r.Context())
			if userID == "" {
				// Unauthenticated request — do not deduct quota.
				next.ServeHTTP(w, r)
				return
			}

			if isTestAPIKey(r) {
				next.ServeHTTP(w, r)
				return
			}

			plan := "free"
			if planLookup != nil {
				plan = planLookup(r.Context(), userID)
			}

			allowed, _, _, err := rateLimiter.Allow(r.Context(), userID, plan)
			if err != nil {
				// On Redis errors, allow the request to proceed (fail open).
				// This prevents a Redis outage from blocking all API traffic.
				next.ServeHTTP(w, r)
				return
			}

			if !allowed {
				type body struct {
					Error   string `json:"error"`
					ResetAt string `json:"reset_at"`
				}
				// reset_at: midnight UTC of the next day
				now := time.Now().UTC()
				nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(body{
					Error:   "api_quota_exceeded",
					ResetAt: nextMidnight.Format(time.RFC3339),
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isTestAPIKey returns true when the request carries a sk_test_ API key Bearer token.
func isTestAPIKey(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer sk_test_") {
		return true
	}
	if cookie, err := r.Cookie("access_token"); err == nil {
		return strings.HasPrefix(cookie.Value, "sk_test_")
	}
	return false
}
