package middleware

import "net/http"

// SecurityHeaders middleware sets security-related HTTP headers.
// This implements E4 requirements from CLAUDE.md for CSP, HSTS, and other security headers.
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Content Security Policy - restrictive by default
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; media-src 'self'; object-src 'none'; child-src 'none'; worker-src 'none'; frame-ancestors 'none'; form-action 'self'; base-uri 'self';")

			// HTTP Strict Transport Security - enforce HTTPS
			w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")

			// X-Frame-Options - prevent clickjacking
			w.Header().Set("X-Frame-Options", "DENY")

			// X-Content-Type-Options - prevent MIME sniffing
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// X-XSS-Protection - legacy XSS protection (for older browsers)
			w.Header().Set("X-XSS-Protection", "1; mode=block")

			// Referrer-Policy - control referrer information
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// X-Download-Options - prevent file execution in IE
			w.Header().Set("X-Download-Options", "noopen")

			// X-Permitted-Cross-Domain-Policies - control Flash/PDF cross-domain access
			w.Header().Set("X-Permitted-Cross-Domain-Policies", "none")

			// Cache-Control for API responses - prevent caching of sensitive data
			if r.Method != "GET" || (r.URL.Path != "/health" && r.URL.Path != "/metrics") {
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
				w.Header().Set("Pragma", "no-cache")
				w.Header().Set("Expires", "0")
			}

			next.ServeHTTP(w, r)
		})
	}
}