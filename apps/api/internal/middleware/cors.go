package middleware

import (
	"net/http"
	"strings"
)

// CORS middleware handles Cross-Origin Resource Sharing headers.
// In development mode, it allows all origins (*).
// In production, it restricts to idcd.com domain list.
func CORS(env string, allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Allow-Credentials must NOT be combined with a wildcard origin.
			// Only set it when the request origin is explicitly matched.
			originAllowed := false
			if env == "development" {
				// Development: allow all origins but still echo the specific origin
				// so credentials can be sent (wildcard + credentials is rejected by browsers).
				if origin != "" {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					originAllowed = true
				} else {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				}
			} else {
				if isAllowedOrigin(origin, allowedOrigins) {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					originAllowed = true
				}
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-API-Key, X-Request-ID, X-CSRF-Token")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
			w.Header().Set("Access-Control-Max-Age", "86400")
			if originAllowed {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isAllowedOrigin checks if the given origin is in the allowed origins list.
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