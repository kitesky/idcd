package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kite365/idcd/lib/ratelimit"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// mockRateLimiter implements RateLimitFunc for testing.
type mockRateLimiter struct {
	allowResult *ratelimit.Result
	allowError  error
}

func (m *mockRateLimiter) Allow(ctx context.Context, key string) (*ratelimit.Result, error) {
	return m.allowResult, m.allowError
}

func TestRateLimit(t *testing.T) {
	// Test handler that just responds with 200 OK
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	t.Run("allows request when under limit", func(t *testing.T) {
		mockLimiter := &mockRateLimiter{
			allowResult: &ratelimit.Result{
				Allowed:   true,
				Remaining: 42,
				ResetAt:   time.Now().Add(time.Hour),
			},
		}

		handler := RateLimit(mockLimiter)(testHandler)

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		// Check rate limit headers are set
		remaining := rr.Header().Get("X-RateLimit-Remaining")
		if remaining != "42" {
			t.Errorf("expected X-RateLimit-Remaining=42, got %q", remaining)
		}

		resetHeader := rr.Header().Get("X-RateLimit-Reset")
		if resetHeader == "" {
			t.Error("expected X-RateLimit-Reset header to be set")
		}

		// Should not have Retry-After when allowed
		retryAfter := rr.Header().Get("Retry-After")
		if retryAfter != "" {
			t.Errorf("expected no Retry-After header when allowed, got %q", retryAfter)
		}
	})

	t.Run("denies request when over limit", func(t *testing.T) {
		mockLimiter := &mockRateLimiter{
			allowResult: &ratelimit.Result{
				Allowed:   false,
				Remaining: 0,
				ResetAt:   time.Now().Add(30 * time.Second),
			},
		}

		handler := RateLimit(mockLimiter)(testHandler)

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		// Should return 429
		if rr.Code != http.StatusTooManyRequests {
			t.Errorf("expected status 429, got %d", rr.Code)
		}

		// Check response is JSON error
		contentType := rr.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected content-type application/json, got %q", contentType)
		}

		// Check rate limit headers
		remaining := rr.Header().Get("X-RateLimit-Remaining")
		if remaining != "0" {
			t.Errorf("expected X-RateLimit-Remaining=0, got %q", remaining)
		}

		retryAfter := rr.Header().Get("Retry-After")
		if retryAfter == "" {
			t.Error("expected Retry-After header when rate limited")
		}

		// Retry-After should be a reasonable number (1-30 seconds)
		retrySeconds, err := strconv.Atoi(retryAfter)
		if err != nil {
			t.Errorf("Retry-After should be numeric, got %q", retryAfter)
		}
		if retrySeconds < 1 || retrySeconds > 35 {
			t.Errorf("Retry-After should be 1-35 seconds, got %d", retrySeconds)
		}

		// Test handler should not have been called
		body := rr.Body.String()
		if strings.Contains(body, "OK") {
			t.Error("test handler should not have been called when rate limited")
		}
	})

	t.Run("allows request when rate limiter errors", func(t *testing.T) {
		mockLimiter := &mockRateLimiter{
			allowError: apperr.Internal("Redis connection failed", nil),
		}

		handler := RateLimit(mockLimiter)(testHandler)

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		// Should allow request when rate limiter fails (fail open)
		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200 when rate limiter errors, got %d", rr.Code)
		}
	})

	t.Run("allows request when no client IP", func(t *testing.T) {
		mockLimiter := &mockRateLimiter{
			allowResult: &ratelimit.Result{
				Allowed:   false,
				Remaining: 0,
				ResetAt:   time.Now(),
			},
		}

		handler := RateLimit(mockLimiter)(testHandler)

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "" // clear default so getClientIP returns empty string
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		// Should allow request when IP cannot be determined
		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200 when no client IP, got %d", rr.Code)
		}
	})
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name        string
		remoteAddr  string
		headers     map[string]string
		expectedIP  string
	}{
		{
			name:       "extracts IP from RemoteAddr",
			remoteAddr: "192.168.1.1:12345",
			expectedIP: "192.168.1.1",
		},
		{
			name:       "prefers X-Forwarded-For header",
			remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.42",
			},
			expectedIP: "203.0.113.42",
		},
		{
			name:       "uses first IP from X-Forwarded-For chain",
			remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.42, 10.0.0.2, 10.0.0.3",
			},
			expectedIP: "203.0.113.42",
		},
		{
			name:       "falls back to X-Real-IP",
			remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{
				"X-Real-IP": "203.0.113.42",
			},
			expectedIP: "203.0.113.42",
		},
		{
			name:       "ignores non-standard X-Forwarded header (only X-Forwarded-For and X-Real-IP are trusted)",
			remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{
				"X-Forwarded": "for=203.0.113.42;proto=http;by=10.0.0.2",
			},
			expectedIP: "10.0.0.1",
		},
		{
			name:       "ignores invalid IPs in headers",
			remoteAddr: "192.168.1.1:12345",
			headers: map[string]string{
				"X-Forwarded-For": "invalid-ip",
				"X-Real-IP":       "also-invalid",
			},
			expectedIP: "192.168.1.1",
		},
		{
			name:       "handles IPv6",
			remoteAddr: "[2001:db8::1]:12345",
			expectedIP: "2001:db8::1",
		},
		{
			name:       "returns RemoteAddr as-is when parsing fails",
			remoteAddr: "invalid-remote-addr",
			expectedIP: "invalid-remote-addr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr

			for header, value := range tt.headers {
				req.Header.Set(header, value)
			}

			actualIP := getClientIP(req)
			if actualIP != tt.expectedIP {
				t.Errorf("expected IP %q, got %q", tt.expectedIP, actualIP)
			}
		})
	}
}