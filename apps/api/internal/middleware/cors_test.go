package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCORS_OriginEcho asserts the core invariant: an allowlisted Origin is
// echoed verbatim into Access-Control-Allow-Origin, a disallowed Origin
// yields no Allow-Origin header at all, and the wildcard `*` is never used.
func TestCORS_OriginEcho(t *testing.T) {
	tests := []struct {
		name                  string
		env                   string
		allowedOrigins        []string
		requestOrigin         string
		expectedOrigin        string // "" means header MUST be absent
		expectAllowCredential bool
	}{
		{
			name:                  "production: exact match allowed",
			env:                   "production",
			allowedOrigins:        []string{"https://idcd.com", "https://app.idcd.com"},
			requestOrigin:         "https://idcd.com",
			expectedOrigin:        "https://idcd.com",
			expectAllowCredential: true,
		},
		{
			name:                  "production: wildcard subdomain allowed",
			env:                   "production",
			allowedOrigins:        []string{"*.idcd.com"},
			requestOrigin:         "https://app.idcd.com",
			expectedOrigin:        "https://app.idcd.com",
			expectAllowCredential: true,
		},
		{
			name:                  "production: unauthorized origin blocked (no Allow-Origin, no Allow-Credentials)",
			env:                   "production",
			allowedOrigins:        []string{"https://idcd.com"},
			requestOrigin:         "https://evil.com",
			expectedOrigin:        "",
			expectAllowCredential: false,
		},
		{
			name:                  "production: empty Origin header => no Allow-Origin",
			env:                   "production",
			allowedOrigins:        []string{"https://idcd.com"},
			requestOrigin:         "",
			expectedOrigin:        "",
			expectAllowCredential: false,
		},
		{
			name:                  "development: empty allowlist echoes specific origin (still not wildcard)",
			env:                   "development",
			allowedOrigins:        nil,
			requestOrigin:         "http://localhost:3000",
			expectedOrigin:        "http://localhost:3000",
			expectAllowCredential: true,
		},
		{
			name:                  "development: with allowlist set, disallowed origin still blocked",
			env:                   "development",
			allowedOrigins:        []string{"https://idcd.com"},
			requestOrigin:         "https://evil.com",
			expectedOrigin:        "",
			expectAllowCredential: false,
		},
		{
			name:                  "development: with allowlist set, allowed origin echoed",
			env:                   "development",
			allowedOrigins:        []string{"https://idcd.com"},
			requestOrigin:         "https://idcd.com",
			expectedOrigin:        "https://idcd.com",
			expectAllowCredential: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := CORS(tt.env, tt.allowedOrigins)(testHandler)

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.requestOrigin != "" {
				req.Header.Set("Origin", tt.requestOrigin)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			verifyCommonCORSHeaders(t, rr)
			assertNeverWildcardOrigin(t, rr)
			assertVaryOrigin(t, rr)

			actualOrigin := rr.Header().Get("Access-Control-Allow-Origin")
			if actualOrigin != tt.expectedOrigin {
				t.Errorf("Access-Control-Allow-Origin: expected %q, got %q", tt.expectedOrigin, actualOrigin)
			}

			actualCred := rr.Header().Get("Access-Control-Allow-Credentials")
			if tt.expectAllowCredential {
				if actualCred != "true" {
					t.Errorf("Access-Control-Allow-Credentials: expected %q, got %q", "true", actualCred)
				}
			} else {
				if actualCred != "" {
					t.Errorf("Access-Control-Allow-Credentials: expected absent, got %q", actualCred)
				}
			}

			if rr.Code != http.StatusOK {
				t.Errorf("status code: expected 200, got %d", rr.Code)
			}
		})
	}
}

// TestCORS_NeverWildcard exhaustively guards the security invariant:
// across all env / allowlist combinations, Access-Control-Allow-Origin
// must never be the literal `*`.
func TestCORS_NeverWildcard(t *testing.T) {
	cases := []struct {
		env            string
		allowedOrigins []string
		origin         string
	}{
		{"production", []string{"https://idcd.com"}, "https://idcd.com"},
		{"production", []string{"https://idcd.com"}, "https://evil.com"},
		{"production", []string{"https://idcd.com"}, ""},
		{"production", []string{"*.idcd.com"}, "https://app.idcd.com"},
		{"development", nil, "http://localhost:3000"},
		{"development", nil, ""},
		{"development", []string{"https://idcd.com"}, "https://evil.com"},
		{"development", []string{"https://idcd.com"}, "https://idcd.com"},
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, c := range cases {
		t.Run(c.env+"|"+strings.Join(c.allowedOrigins, ",")+"|"+c.origin, func(t *testing.T) {
			handler := CORS(c.env, c.allowedOrigins)(testHandler)

			req := httptest.NewRequest("GET", "/test", nil)
			if c.origin != "" {
				req.Header.Set("Origin", c.origin)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			assertNeverWildcardOrigin(t, rr)
		})
	}
}

// TestCORS_VaryOrigin asserts that Vary: Origin is always set so CDN /
// reverse-proxy caches segment per-origin. Otherwise a response cached for
// idcd.com could leak to a different caller.
func TestCORS_VaryOrigin(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cases := []struct {
		name           string
		env            string
		allowedOrigins []string
		origin         string
	}{
		{"production allowed", "production", []string{"https://idcd.com"}, "https://idcd.com"},
		{"production blocked", "production", []string{"https://idcd.com"}, "https://evil.com"},
		{"production no origin", "production", []string{"https://idcd.com"}, ""},
		{"development empty allowlist", "development", nil, "http://localhost:3000"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			handler := CORS(c.env, c.allowedOrigins)(testHandler)

			req := httptest.NewRequest("GET", "/test", nil)
			if c.origin != "" {
				req.Header.Set("Origin", c.origin)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			assertVaryOrigin(t, rr)
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

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected status code %d, got %d", http.StatusNoContent, rr.Code)
	}

	verifyCommonCORSHeaders(t, rr)
	assertNeverWildcardOrigin(t, rr)
	assertVaryOrigin(t, rr)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://idcd.com" {
		t.Errorf("preflight Allow-Origin: expected %q, got %q", "https://idcd.com", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("preflight Allow-Credentials: expected %q, got %q", "true", got)
	}
}

// TestCORSPreflightRequest_Blocked verifies that a preflight from a
// disallowed origin still short-circuits with 204 but emits no Allow-Origin
// (the browser will block the actual request).
func TestCORSPreflightRequest_Blocked(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for OPTIONS request")
	})

	handler := CORS("production", []string{"https://idcd.com"})(testHandler)

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected status code %d, got %d", http.StatusNoContent, rr.Code)
	}

	assertNeverWildcardOrigin(t, rr)
	assertVaryOrigin(t, rr)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("blocked preflight Allow-Origin: expected empty, got %q", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("blocked preflight Allow-Credentials: expected empty, got %q", got)
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
		{
			name:           "empty allowlist never matches (prod-safe)",
			origin:         "https://idcd.com",
			allowedOrigins: nil,
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

// verifyCommonCORSHeaders checks the static headers that are always set
// (regardless of whether the origin was matched).
func verifyCommonCORSHeaders(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	expectedHeaders := map[string]string{
		"Access-Control-Allow-Methods":  "GET, POST, PUT, PATCH, DELETE, OPTIONS",
		"Access-Control-Allow-Headers":  "Accept, Authorization, Content-Type, X-API-Key, X-Request-ID, X-CSRF-Token",
		"Access-Control-Expose-Headers": "X-Request-ID",
		"Access-Control-Max-Age":        "86400",
	}

	for header, expected := range expectedHeaders {
		actual := rr.Header().Get(header)
		if actual != expected {
			t.Errorf("header %s: expected %q, got %q", header, expected, actual)
		}
	}
}

// assertNeverWildcardOrigin guards the security invariant that we never
// emit `Access-Control-Allow-Origin: *`. Wildcard + credentials is rejected
// by browsers and is unsafe for any cookie-bearing endpoint.
func assertNeverWildcardOrigin(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got == "*" {
		t.Fatalf("Access-Control-Allow-Origin must never be %q (credentials + wildcard is unsafe / browser-rejected)", "*")
	}
}

// assertVaryOrigin guards that Vary: Origin is always set so per-origin
// responses cannot be cached and leaked across callers.
func assertVaryOrigin(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	vary := rr.Header().Values("Vary")
	for _, v := range vary {
		for _, part := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(part), "Origin") {
				return
			}
		}
	}
	t.Errorf("Vary header must include Origin, got %v", vary)
}
