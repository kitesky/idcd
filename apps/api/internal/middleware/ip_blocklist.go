package middleware

import (
	"context"
	"net/http"

	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// IPBlocklistStore is the Redis interface for checking blocked IPs.
type IPBlocklistStore interface {
	// SIsMember checks if ip is in the blocklist set.
	SIsMember(ctx context.Context, key, member string) (bool, error)
}

const blocklistKey = "idcd:ip:blocklist"

// IPBlocklist returns a middleware that rejects requests from blocked IPs with 403.
// If the store is nil or returns an error, the request passes through (fail-open).
func IPBlocklist(store IPBlocklistStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if store == nil {
				next.ServeHTTP(w, r)
				return
			}
			ip := getClientIP(r)
			if ip != "" {
				blocked, err := store.SIsMember(r.Context(), blocklistKey, ip)
				if err == nil && blocked {
					response.Error(w, r, apperr.Forbidden("access denied"))
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
