package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	tests := []struct {
		name     string
		env      string
		path     string
		method   string
		expected map[string]string
		absent   []string
	}{
		{
			name:   "production - sets all security headers including HSTS",
			env:    "production",
			path:   "/api/users",
			method: "GET",
			expected: map[string]string{
				"Content-Security-Policy":              "default-src 'none'; frame-ancestors 'none'; report-uri /v1/csp-report",
				"Content-Security-Policy-Report-Only":  "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: https:; connect-src 'self' https://api.idcd.com; font-src 'self' data:; report-uri /v1/csp-report",
				"Permissions-Policy":                   "camera=(), microphone=(), geolocation=(), payment=()",
				"Strict-Transport-Security":             "max-age=31536000; includeSubDomains",
				"X-Frame-Options":                       "DENY",
				"X-Content-Type-Options":                "nosniff",
				"Referrer-Policy":                       "strict-origin-when-cross-origin",
				"Cache-Control":                         "no-cache, no-store, must-revalidate",
				"Pragma":                                "no-cache",
				"Expires":                               "0",
			},
		},
		{
			name:   "development - no HSTS",
			env:    "development",
			path:   "/api/users",
			method: "GET",
			expected: map[string]string{
				"Content-Security-Policy":              "default-src 'none'; frame-ancestors 'none'; report-uri /v1/csp-report",
				"Content-Security-Policy-Report-Only":  "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: https:; connect-src 'self' https://api.idcd.com; font-src 'self' data:; report-uri /v1/csp-report",
				"Permissions-Policy":                   "camera=(), microphone=(), geolocation=(), payment=()",
				"X-Frame-Options":                      "DENY",
				"X-Content-Type-Options":               "nosniff",
				"Referrer-Policy":                      "strict-origin-when-cross-origin",
				"Cache-Control":                        "no-cache, no-store, must-revalidate",
			},
			absent: []string{"Strict-Transport-Security"},
		},
		{
			name:   "staging - has HSTS",
			env:    "staging",
			path:   "/api/users",
			method: "GET",
			expected: map[string]string{
				"X-Frame-Options":           "DENY",
				"X-Content-Type-Options":    "nosniff",
				"Referrer-Policy":           "strict-origin-when-cross-origin",
				"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
			},
		},
		{
			name:   "health endpoint - no cache headers",
			env:    "production",
			path:   "/health",
			method: "GET",
			expected: map[string]string{
				"Content-Security-Policy":             "default-src 'none'; frame-ancestors 'none'; report-uri /v1/csp-report",
				"Content-Security-Policy-Report-Only": "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: https:; connect-src 'self' https://api.idcd.com; font-src 'self' data:; report-uri /v1/csp-report",
				"X-Frame-Options":                    "DENY",
				"X-Content-Type-Options":              "nosniff",
			},
			absent: []string{"Cache-Control", "Pragma", "Expires"},
		},
		{
			name:   "metrics endpoint - no cache headers (metrics now on internal port)",
			env:    "production",
			path:   "/metrics",
			method: "GET",
			expected: map[string]string{
				"X-Frame-Options":        "DENY",
				"X-Content-Type-Options": "nosniff",
			},
			absent: []string{"Cache-Control", "Pragma", "Expires"},
		},
		{
			name:   "POST request - has cache headers",
			env:    "development",
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
			handler := SecurityHeaders(tt.env)(testHandler)

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

			// Verify absent headers are not set
			for _, headerName := range tt.absent {
				if actualValue := rr.Header().Get(headerName); actualValue != "" {
					t.Errorf("header %q should not be set, but got: %q", headerName, actualValue)
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

	handler := SecurityHeaders("production")(testHandler)

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
