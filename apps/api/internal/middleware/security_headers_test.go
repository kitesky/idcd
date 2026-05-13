package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		method   string
		expected map[string]string
	}{
		{
			name:   "sets all security headers for normal requests",
			path:   "/api/users",
			method: "GET",
			expected: map[string]string{
				"Content-Security-Policy":           "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; media-src 'self'; object-src 'none'; child-src 'none'; worker-src 'none'; frame-ancestors 'none'; form-action 'self'; base-uri 'self';",
				"Strict-Transport-Security":         "max-age=63072000; includeSubDomains; preload",
				"X-Frame-Options":                   "DENY",
				"X-Content-Type-Options":            "nosniff",
				"X-XSS-Protection":                  "1; mode=block",
				"Referrer-Policy":                   "strict-origin-when-cross-origin",
				"X-Download-Options":                "noopen",
				"X-Permitted-Cross-Domain-Policies": "none",
				"Cache-Control":                     "no-cache, no-store, must-revalidate",
				"Pragma":                            "no-cache",
				"Expires":                           "0",
			},
		},
		{
			name:   "sets security headers but no cache headers for health endpoint",
			path:   "/health",
			method: "GET",
			expected: map[string]string{
				"Content-Security-Policy":           "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; media-src 'self'; object-src 'none'; child-src 'none'; worker-src 'none'; frame-ancestors 'none'; form-action 'self'; base-uri 'self';",
				"Strict-Transport-Security":         "max-age=63072000; includeSubDomains; preload",
				"X-Frame-Options":                   "DENY",
				"X-Content-Type-Options":            "nosniff",
				"X-XSS-Protection":                  "1; mode=block",
				"Referrer-Policy":                   "strict-origin-when-cross-origin",
				"X-Download-Options":                "noopen",
				"X-Permitted-Cross-Domain-Policies": "none",
			},
		},
		{
			name:   "sets security headers but no cache headers for metrics endpoint",
			path:   "/metrics",
			method: "GET",
			expected: map[string]string{
				"Content-Security-Policy":           "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; media-src 'self'; object-src 'none'; child-src 'none'; worker-src 'none'; frame-ancestors 'none'; form-action 'self'; base-uri 'self';",
				"Strict-Transport-Security":         "max-age=63072000; includeSubDomains; preload",
				"X-Frame-Options":                   "DENY",
				"X-Content-Type-Options":            "nosniff",
				"X-XSS-Protection":                  "1; mode=block",
				"Referrer-Policy":                   "strict-origin-when-cross-origin",
				"X-Download-Options":                "noopen",
				"X-Permitted-Cross-Domain-Policies": "none",
			},
		},
		{
			name:   "sets cache headers for POST requests",
			path:   "/api/users",
			method: "POST",
			expected: map[string]string{
				"Cache-Control": "no-cache, no-store, must-revalidate",
				"Pragma":        "no-cache",
				"Expires":       "0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test handler
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Wrap with SecurityHeaders middleware
			handler := SecurityHeaders()(testHandler)

			// Create request
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()

			// Execute request
			handler.ServeHTTP(rr, req)

			// Verify expected headers are set
			for headerName, expectedValue := range tt.expected {
				actualValue := rr.Header().Get(headerName)
				if actualValue != expectedValue {
					t.Errorf("header %q: expected %q, got %q", headerName, expectedValue, actualValue)
				}
			}

			// For health and metrics endpoints, verify cache headers are NOT set
			if tt.path == "/health" || tt.path == "/metrics" {
				cacheHeaders := []string{"Cache-Control", "Pragma", "Expires"}
				for _, header := range cacheHeaders {
					if rr.Header().Get(header) != "" {
						t.Errorf("cache header %q should not be set for %s, but got: %q", header, tt.path, rr.Header().Get(header))
					}
				}
			}
		})
	}
}

// TestSecurityHeadersCallsNext verifies that the middleware calls the next handler
func TestSecurityHeadersCallsNext(t *testing.T) {
	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := SecurityHeaders()(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("SecurityHeaders middleware did not call the next handler")
	}

	if rr.Code != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, rr.Code)
	}
}