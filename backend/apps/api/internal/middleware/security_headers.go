package middleware

import "net/http"

// SecurityHeaders middleware sets security-related HTTP headers.
// This implements E4 requirements from CLAUDE.md for CSP, HSTS, and other security headers.
func SecurityHeaders(env string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Enforced CSP — API only serves JSON; no scripts or styles needed.
			w.Header().Set("Content-Security-Policy",
				"default-src 'none'; "+
					"frame-ancestors 'none'; "+
					"report-uri /v1/csp-report")
			// Report-Only retains the broader policy for monitoring during transition.
			w.Header().Set("Content-Security-Policy-Report-Only",
				"default-src 'self'; "+
					"script-src 'self'; "+
					"style-src 'self'; "+
					"img-src 'self' data: https:; "+
					"connect-src 'self' https://api.idcd.com; "+
					"font-src 'self' data:; "+
					"report-uri /v1/csp-report")

			// Permissions-Policy - disable dangerous features
			w.Header().Set("Permissions-Policy",
				"camera=(), microphone=(), geolocation=(), payment=()")

			// HTTP Strict Transport Security — set for non-dev environments.
			if env != "development" {
				w.Header().Set("Strict-Transport-Security",
					"max-age=31536000; includeSubDomains")
			}

			// X-Frame-Options - prevent clickjacking
			w.Header().Set("X-Frame-Options", "DENY")

			// X-Content-Type-Options - prevent MIME sniffing
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// Referrer-Policy - control referrer information
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

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