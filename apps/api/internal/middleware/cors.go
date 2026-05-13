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

			// Set allowed origin
			if env == "development" {
				// Development: allow all origins
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				// Production: check against allowed origins list
				if isAllowedOrigin(origin, allowedOrigins) {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
			}

			// Set other CORS headers
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-API-Key, X-Request-ID")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours

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