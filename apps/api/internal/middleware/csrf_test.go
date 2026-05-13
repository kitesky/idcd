package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCSRF(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	tests := []struct {
		name           string
		method         string
		path           string
		authHeader     string
		csrfCookie     string
		csrfHeader     string
		expectedStatus int
		expectCookie   bool
	}{
		{
			name:           "GET request - generates CSRF token cookie",
			method:         "GET",
			path:           "/v1/account/profile",
			expectedStatus: http.StatusOK,
			expectCookie:   true,
		},
		{
			name:           "GET request - existing token preserved",
			method:         "GET",
			path:           "/v1/account/profile",
			csrfCookie:     "existing-token-123",
			expectedStatus: http.StatusOK,
			expectCookie:   false, // Should not overwrite
		},
		{
			name:           "POST request - no token returns 403",
			method:         "POST",
			path:           "/v1/account/profile",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "POST request - valid token passes",
			method:         "POST",
			path:           "/v1/account/profile",
			csrfCookie:     "valid-token-abc",
			csrfHeader:     "valid-token-abc",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST request - mismatched token fails",
			method:         "POST",
			path:           "/v1/account/profile",
			csrfCookie:     "cookie-token",
			csrfHeader:     "header-token",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "DELETE request - valid token passes",
			method:         "DELETE",
			path:           "/v1/account",
			csrfCookie:     "valid-token-xyz",
			csrfHeader:     "valid-token-xyz",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "PATCH request - valid token passes",
			method:         "PATCH",
			path:           "/v1/account/profile",
			csrfCookie:     "valid-token-patch",
			csrfHeader:     "valid-token-patch",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "PUT request - valid token passes",
			method:         "PUT",
			path:           "/v1/account/settings",
			csrfCookie:     "valid-token-put",
			csrfHeader:     "valid-token-put",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "exempt path /v1/auth/login - no CSRF check",
			method:         "POST",
			path:           "/v1/auth/login",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "exempt path /v1/auth/register - no CSRF check",
			method:         "POST",
			path:           "/v1/auth/register",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "exempt path /v1/probe/http - no CSRF check",
			method:         "POST",
			path:           "/v1/probe/http",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "exempt path /v1/info/ip - no CSRF check",
			method:         "GET",
			path:           "/v1/info/ip",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Bearer token - no CSRF check",
			method:         "POST",
			path:           "/v1/account/profile",
			authHeader:     "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "OPTIONS request - no CSRF check",
			method:         "OPTIONS",
			path:           "/v1/account/profile",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "HEAD request - no CSRF check",
			method:         "HEAD",
			path:           "/v1/account/profile",
			expectedStatus: http.StatusOK,
			expectCookie:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := CSRF()(testHandler)

			req := httptest.NewRequest(tt.method, tt.path, nil)

			// Set CSRF cookie if provided
			if tt.csrfCookie != "" {
				req.AddCookie(&http.Cookie{
					Name:  csrfCookieName,
					Value: tt.csrfCookie,
				})
			}

			// Set CSRF header if provided
			if tt.csrfHeader != "" {
				req.Header.Set(csrfHeaderName, tt.csrfHeader)
			}

			// Set Authorization header if provided
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			// Check status code
			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Check CSRF cookie presence
			cookies := rr.Result().Cookies()
			hasCookie := false
			for _, c := range cookies {
				if c.Name == csrfCookieName {
					hasCookie = true
					// Verify cookie properties
					if c.HttpOnly {
						t.Error("CSRF cookie should not be HttpOnly (JS needs to read it)")
					}
					if c.SameSite != http.SameSiteStrictMode {
						t.Errorf("expected SameSite=Strict, got %v", c.SameSite)
					}
					if len(c.Value) != csrfTokenLen*2 { // hex encoding doubles length
						t.Errorf("expected token length %d, got %d", csrfTokenLen*2, len(c.Value))
					}
				}
			}
			if tt.expectCookie && !hasCookie {
				t.Error("expected CSRF cookie to be set, but it was not")
			}
			if !tt.expectCookie && hasCookie && tt.csrfCookie == "" {
				t.Error("unexpected CSRF cookie was set")
			}
		})
	}
}

func TestCSRFExemptPaths(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	exemptPaths := []string{"/custom/exempt", "/another/exempt"}
	handler := CSRF(exemptPaths...)(testHandler)

	tests := []struct {
		path           string
		expectedStatus int
	}{
		{"/custom/exempt", http.StatusOK},
		{"/another/exempt", http.StatusOK},
		{"/not/exempt", http.StatusForbidden}, // No CSRF token
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("POST", tt.path, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("path %s: expected status %d, got %d", tt.path, tt.expectedStatus, rr.Code)
			}
		})
	}
}

func TestGenerateCSRFToken(t *testing.T) {
	token1 := generateCSRFToken()
	token2 := generateCSRFToken()

	// Tokens should be hex strings
	if len(token1) != csrfTokenLen*2 {
		t.Errorf("expected token length %d, got %d", csrfTokenLen*2, len(token1))
	}

	// Tokens should be different (randomness check)
	if token1 == token2 {
		t.Error("generateCSRFToken returned identical tokens, randomness broken")
	}

	// Tokens should be valid hex
	for _, c := range token1 {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("token contains non-hex character: %c", c)
		}
	}
}

func TestGetCSRFToken(t *testing.T) {
	tests := []struct {
		name           string
		cookieValue    string
		expectedResult string
	}{
		{
			name:           "returns token from cookie",
			cookieValue:    "test-token-123",
			expectedResult: "test-token-123",
		},
		{
			name:           "returns empty string when cookie missing",
			cookieValue:    "",
			expectedResult: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.cookieValue != "" {
				req.AddCookie(&http.Cookie{
					Name:  csrfCookieName,
					Value: tt.cookieValue,
				})
			}

			result := getCSRFToken(req)
			if result != tt.expectedResult {
				t.Errorf("expected %q, got %q", tt.expectedResult, result)
			}
		})
	}
}

func TestValidateCSRFToken(t *testing.T) {
	tests := []struct {
		name         string
		cookieValue  string
		headerValue  string
		expectedPass bool
	}{
		{
			name:         "matching tokens pass",
			cookieValue:  "same-token",
			headerValue:  "same-token",
			expectedPass: true,
		},
		{
			name:         "mismatched tokens fail",
			cookieValue:  "cookie-token",
			headerValue:  "header-token",
			expectedPass: false,
		},
		{
			name:         "missing cookie fails",
			cookieValue:  "",
			headerValue:  "header-token",
			expectedPass: false,
		},
		{
			name:         "missing header fails",
			cookieValue:  "cookie-token",
			headerValue:  "",
			expectedPass: false,
		},
		{
			name:         "both missing fails",
			cookieValue:  "",
			headerValue:  "",
			expectedPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", nil)
			if tt.cookieValue != "" {
				req.AddCookie(&http.Cookie{
					Name:  csrfCookieName,
					Value: tt.cookieValue,
				})
			}
			if tt.headerValue != "" {
				req.Header.Set(csrfHeaderName, tt.headerValue)
			}

			result := validateCSRFToken(req)
			if result != tt.expectedPass {
				t.Errorf("expected %v, got %v", tt.expectedPass, result)
			}
		})
	}
}
