package middleware

import (
	"net/http"
	"strings"
)

// CORS middleware handles Cross-Origin Resource Sharing headers.
//
// Security guarantees:
//   - NEVER emits `Access-Control-Allow-Origin: *`. We require credentialed
//     cross-origin requests (cookies / Authorization), and the browser will
//     reject `*` combined with `Access-Control-Allow-Credentials: true`.
//     A wildcard origin is also dangerous for any cookie-bearing endpoint.
//   - Request `Origin` MUST strictly match the allowlist (exact host or
//     wildcard subdomain like `*.idcd.com`) before we echo it back.
//   - Mismatched / missing origin -> no Allow-Origin and no Allow-Credentials
//     header is sent; the browser blocks the response automatically. We do
//     NOT reject the request server-side (preserves same-origin / curl usage).
//   - `Vary: Origin` is always set so CDNs / proxies cache per-origin.
//
// Dev mode (env == "development"):
//   - If the allowlist is empty, any non-empty Origin is echoed back (still
//     specific, never `*`). This keeps localhost / preview workflows easy.
//   - If the allowlist is non-empty, the same strict rules as production apply.
func CORS(env string, allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Always advertise that the response varies by Origin so any
			// upstream cache (CDN, reverse proxy) keys per-origin and does
			// not leak a "wrong" Allow-Origin to another caller.
			w.Header().Add("Vary", "Origin")

			origin := r.Header.Get("Origin")
			originAllowed := false

			switch {
			case origin == "":
				// No Origin header: same-origin / non-browser caller.
				// Nothing to echo. Do not set Allow-Origin or Allow-Credentials.
			case env == "development" && len(allowedOrigins) == 0:
				// Dev convenience: empty allowlist -> echo any specific origin.
				// Still never wildcard. Production never takes this branch.
				w.Header().Set("Access-Control-Allow-Origin", origin)
				originAllowed = true
			case isAllowedOrigin(origin, allowedOrigins):
				w.Header().Set("Access-Control-Allow-Origin", origin)
				originAllowed = true
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-API-Key, X-Request-ID, X-CSRF-Token, X-Locale")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
			w.Header().Set("Access-Control-Max-Age", "86400")

			// Allow-Credentials is ONLY paired with a specific Allow-Origin.
			// Browsers reject `*` + credentials; we never emit `*` anyway,
			// but this guard keeps the invariant explicit.
			if originAllowed {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Preflight: short-circuit with 204. Headers set above are sufficient.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isAllowedOrigin checks if the given origin is in the allowed origins list.
//
// Matching rules:
//   - Exact match against any entry (e.g., "https://idcd.com").
//   - Wildcard subdomain via "*.idcd.com" matches "https://app.idcd.com"
//     and the apex "https://idcd.com". Protocol is stripped from the
//     request origin for the suffix comparison.
//
// Empty origin is never allowed.
func isAllowedOrigin(origin string, allowedOrigins []string) bool {
	if origin == "" {
		return false
	}

	for _, allowed := range allowedOrigins {
		// Exact match
		if origin == allowed {
			return true
		}
		// Wildcard subdomain match (e.g., *.idcd.com matches app.idcd.com)
		if strings.HasPrefix(allowed, "*.") {
			domain := allowed[2:] // Remove "*."
			// Remove protocol from origin for comparison
			cleanOrigin := strings.TrimPrefix(origin, "https://")
			cleanOrigin = strings.TrimPrefix(cleanOrigin, "http://")

			if strings.HasSuffix(cleanOrigin, "."+domain) || cleanOrigin == domain {
				return true
			}
		}
	}
	return false
}
