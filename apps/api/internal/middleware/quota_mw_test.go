package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ─────────────────────────────────────────────
// Mock rate limiter
// ─────────────────────────────────────────────

type mockAPIRateLimiter struct {
	allowed bool
	used    int
	limit   int
	err     error
}

func (m *mockAPIRateLimiter) Allow(_ context.Context, _ string, _ string) (bool, int, int, error) {
	return m.allowed, m.used, m.limit, m.err
}

// testHandler is a simple handler that writes 200 OK.
func testHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// injectUserID injects a user ID into the request context via the exported key helper.
func injectUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), UserIDContextKey(), userID)
	return r.WithContext(ctx)
}

// ─────────────────────────────────────────────
// Tests — APIQuotaMiddleware
// ─────────────────────────────────────────────

func TestAPIQuotaMiddleware_UnauthenticatedPassThrough(t *testing.T) {
	// Unauthenticated requests must bypass quota checks.
	rl := &mockAPIRateLimiter{allowed: false} // would deny if checked
	mw := APIQuotaMiddleware(rl, nil)
	h := mw(http.HandlerFunc(testHandler))

	req := httptest.NewRequest(http.MethodGet, "/v1/test", nil)
	// No user ID injected → unauthenticated.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("unauthenticated request should pass through; got %d", rr.Code)
	}
}

func TestAPIQuotaMiddleware_AllowedRequest(t *testing.T) {
	rl := &mockAPIRateLimiter{allowed: true, used: 1, limit: 100}
	mw := APIQuotaMiddleware(rl, nil)
	h := mw(http.HandlerFunc(testHandler))

	req := httptest.NewRequest(http.MethodGet, "/v1/test", nil)
	req = injectUserID(req, "u_test")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("allowed request should return 200; got %d", rr.Code)
	}
}

func TestAPIQuotaMiddleware_DeniedRequest_Returns429(t *testing.T) {
	rl := &mockAPIRateLimiter{allowed: false, used: 101, limit: 100}
	mw := APIQuotaMiddleware(rl, nil)
	h := mw(http.HandlerFunc(testHandler))

	req := httptest.NewRequest(http.MethodGet, "/v1/test", nil)
	req = injectUserID(req, "u_over_quota")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("over-quota request should return 429; got %d", rr.Code)
	}

	var body struct {
		Error   string `json:"error"`
		ResetAt string `json:"reset_at"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error != "api_quota_exceeded" {
		t.Errorf("error field should be 'api_quota_exceeded', got %q", body.Error)
	}
	if body.ResetAt == "" {
		t.Error("reset_at should not be empty")
	}
}

func TestAPIQuotaMiddleware_RedisError_FailOpen(t *testing.T) {
	// On Redis error, the middleware must fail open (allow the request).
	rl := &mockAPIRateLimiter{err: errors.New("redis: connection refused")}
	mw := APIQuotaMiddleware(rl, nil)
	h := mw(http.HandlerFunc(testHandler))

	req := httptest.NewRequest(http.MethodGet, "/v1/test", nil)
	req = injectUserID(req, "u_test")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("redis error should fail open (200); got %d", rr.Code)
	}
}

func TestAPIQuotaMiddleware_WithPlanLookup(t *testing.T) {
	rl := &mockAPIRateLimiter{allowed: true, used: 1, limit: 5000}
	planLookup := APIPlanLookup(func(_ context.Context, _ string) string {
		return "pro"
	})
	mw := APIQuotaMiddleware(rl, planLookup)
	h := mw(http.HandlerFunc(testHandler))

	req := httptest.NewRequest(http.MethodGet, "/v1/test", nil)
	req = injectUserID(req, "u_pro")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("pro plan request should pass through; got %d", rr.Code)
	}
}

func TestAPIQuotaMiddleware_ContentTypeOnDenial(t *testing.T) {
	rl := &mockAPIRateLimiter{allowed: false}
	mw := APIQuotaMiddleware(rl, nil)
	h := mw(http.HandlerFunc(testHandler))

	req := httptest.NewRequest(http.MethodGet, "/v1/test", nil)
	req = injectUserID(req, "u_over")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type should be application/json on denial; got %q", ct)
	}
}
