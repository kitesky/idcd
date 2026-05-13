package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS(t *testing.T) {
	tests := []struct {
		name            string
		env             string
		allowedOrigins  []string
		requestOrigin   string
		expectedOrigin  string
		shouldAllowCORS bool
	}{
		{
			name:            "development environment allows all origins",
			env:             "development",
			allowedOrigins:  []string{"https://idcd.com"},
			requestOrigin:   "https://evil.com",
			expectedOrigin:  "*",
			shouldAllowCORS: true,
		},
		{
			name:            "production environment allows exact match",
			env:             "production",
			allowedOrigins:  []string{"https://idcd.com", "https://app.idcd.com"},
			requestOrigin:   "https://idcd.com",
			expectedOrigin:  "https://idcd.com",
			shouldAllowCORS: true,
		},
		{
			name:            "production environment allows wildcard subdomain",
			env:             "production",
			allowedOrigins:  []string{"*.idcd.com"},
			requestOrigin:   "https://app.idcd.com",
			expectedOrigin:  "https://app.idcd.com",
			shouldAllowCORS: true,
		},
		{
			name:            "production environment blocks unauthorized origin",
			env:             "production",
			allowedOrigins:  []string{"https://idcd.com"},
			requestOrigin:   "https://evil.com",
			expectedOrigin:  "",
			shouldAllowCORS: false,
		},
		{
			name:            "production environment blocks empty origin",
			env:             "production",
			allowedOrigins:  []string{"https://idcd.com"},
			requestOrigin:   "",
			expectedOrigin:  "",
			shouldAllowCORS: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test handler
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Wrap with CORS middleware
			handler := CORS(tt.env, tt.allowedOrigins)(testHandler)

			// Create request with origin header
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.requestOrigin != "" {
				req.Header.Set("Origin", tt.requestOrigin)
			}
			rr := httptest.NewRecorder()

			// Execute request
			handler.ServeHTTP(rr, req)

			// Verify CORS headers
			verifyCommonCORSHeaders(t, rr)

			// Verify Access-Control-Allow-Origin header
			actualOrigin := rr.Header().Get("Access-Control-Allow-Origin")
			if actualOrigin != tt.expectedOrigin {
				t.Errorf("Access-Control-Allow-Origin: expected %q, got %q", tt.expectedOrigin, actualOrigin)
			}
		})
	}
}

func TestCORSPreflightRequest(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for OPTIONS request")
	})

	handler := CORS("production", []string{"https://idcd.com"})(testHandler)

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://idcd.com")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Verify preflight response
	if rr.Code != http.StatusNoContent {
		t.Errorf("expected status code %d, got %d", http.StatusNoContent, rr.Code)
	}

	verifyCommonCORSHeaders(t, rr)

	// Verify that the handler was not called (status should be 204, not from handler)
	if rr.Code != http.StatusNoContent {
		t.Errorf("preflight request should return 204 No Content, got %d", rr.Code)
	}
}

func TestIsAllowedOrigin(t *testing.T) {
	tests := []struct {
		name           string
		origin         string
		allowedOrigins []string
		expected       bool
	}{
		{
			name:           "exact match",
			origin:         "https://idcd.com",
			allowedOrigins: []string{"https://idcd.com", "https://app.idcd.com"},
			expected:       true,
		},
		{
			name:           "wildcard subdomain match",
			origin:         "https://app.idcd.com",
			allowedOrigins: []string{"*.idcd.com"},
			expected:       true,
		},
		{
			name:           "wildcard root domain match",
			origin:         "https://idcd.com",
			allowedOrigins: []string{"*.idcd.com"},
			expected:       true,
		},
		{
			name:           "no match",
			origin:         "https://evil.com",
			allowedOrigins: []string{"https://idcd.com"},
			expected:       false,
		},
		{
			name:           "empty origin",
			origin:         "",
			allowedOrigins: []string{"https://idcd.com"},
			expected:       false,
		},
		{
			name:           "subdomain not matching wildcard",
			origin:         "https://app.notidcd.com",
			allowedOrigins: []string{"*.idcd.com"},
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := isAllowedOrigin(tt.origin, tt.allowedOrigins)
			if actual != tt.expected {
				t.Errorf("isAllowedOrigin(%q, %v) = %v, expected %v",
					tt.origin, tt.allowedOrigins, actual, tt.expected)
			}
		})
	}
}

// verifyCommonCORSHeaders checks that all expected CORS headers are set correctly
func verifyCommonCORSHeaders(t *testing.T, rr *httptest.ResponseRecorder) {
	expectedHeaders := map[string]string{
		"Access-Control-Allow-Methods":     "GET, POST, PUT, PATCH, DELETE, OPTIONS",
		"Access-Control-Allow-Headers":     "Accept, Authorization, Content-Type, X-API-Key, X-Request-ID",
		"Access-Control-Expose-Headers":    "X-Request-ID",
		"Access-Control-Allow-Credentials": "true",
		"Access-Control-Max-Age":           "86400",
	}

	for header, expected := range expectedHeaders {
		actual := rr.Header().Get(header)
		if actual != expected {
			t.Errorf("header %s: expected %q, got %q", header, expected, actual)
		}
	}
}