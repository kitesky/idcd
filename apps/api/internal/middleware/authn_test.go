package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kite365/idcd/lib/auth/jwt"
	"github.com/kite365/idcd/lib/auth/session"
)

// --- mock JWT verifier ---

type mockJWTVerifier struct {
	claims *jwt.Claims
	err    error
}

func (m *mockJWTVerifier) Verify(_ string) (*jwt.Claims, error) {
	return m.claims, m.err
}

// --- mock session checker ---

type mockSessionChecker struct {
	data *session.SessionData
	err  error
}

func (m *mockSessionChecker) Get(_ context.Context, _ string) (*session.SessionData, error) {
	return m.data, m.err
}

// --- helpers ---

func okHandler(w http.ResponseWriter, r *http.Request) {
	uid := UserIDFromContext(r.Context())
	w.Header().Set("X-User-ID", uid)
	w.WriteHeader(http.StatusOK)
}

func authnHandler(jwtSvc TokenVerifier, sessSvc SessionChecker) http.Handler {
	return Authn(jwtSvc, sessSvc)(http.HandlerFunc(okHandler))
}

// --- tests ---

func TestAuthn_missingHeader(t *testing.T) {
	h := authnHandler(&mockJWTVerifier{}, &mockSessionChecker{})

	req := httptest.NewRequest("GET", "/protected", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthn_invalidToken(t *testing.T) {
	jwtSvc := &mockJWTVerifier{err: errors.New("bad token")}
	h := authnHandler(jwtSvc, &mockSessionChecker{})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer bad.token.here")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthn_sessionExpired(t *testing.T) {
	claims := &jwt.Claims{UserID: "usr_001", SessionID: "sess_001"}
	jwtSvc := &mockJWTVerifier{claims: claims}
	sessSvc := &mockSessionChecker{err: errors.New("session not found")}

	h := authnHandler(jwtSvc, sessSvc)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer valid.token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthn_success(t *testing.T) {
	claims := &jwt.Claims{UserID: "usr_001", SessionID: "sess_001"}
	jwtSvc := &mockJWTVerifier{claims: claims}
	sessSvc := &mockSessionChecker{data: &session.SessionData{
		UserID:     "usr_001",
		CreatedAt:  time.Now(),
		LastSeenAt: time.Now(),
	}}

	h := authnHandler(jwtSvc, sessSvc)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer valid.token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if uid := rr.Header().Get("X-User-ID"); uid != "usr_001" {
		t.Errorf("expected user_id=usr_001 in context, got %q", uid)
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		header string
		want   string
	}{
		{"Bearer abc.def.ghi", "abc.def.ghi"},
		{"bearer abc", ""},
		{"Token abc", ""},
		{"", ""},
		{"Bearer ", ""},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", "/", nil)
		if tt.header != "" {
			req.Header.Set("Authorization", tt.header)
		}
		got := extractBearerToken(req)
		if got != tt.want {
			t.Errorf("header=%q: got %q, want %q", tt.header, got, tt.want)
		}
	}
}
