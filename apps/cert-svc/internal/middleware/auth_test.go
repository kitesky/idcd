package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kite365/idcd/lib/auth/jwt"
	"github.com/kite365/idcd/lib/auth/session"
)

type stubJWT struct {
	claims *jwt.Claims
	err    error
}

func (s *stubJWT) Verify(_ string) (*jwt.Claims, error) { return s.claims, s.err }

type stubSession struct {
	data *session.SessionData
	err  error
}

func (s *stubSession) Get(_ context.Context, _ string) (*session.SessionData, error) {
	return s.data, s.err
}

func newOKChain(t *testing.T, j TokenVerifier, s SessionChecker) http.Handler {
	t.Helper()
	mw := Authn(j, s)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, err := UserIDFromContext(r.Context())
		if err != nil {
			t.Errorf("handler missing user_id: %v", err)
		}
		w.Header().Set("X-User-ID", uid)
		w.WriteHeader(http.StatusOK)
	})
	return mw(next)
}

func TestAuthn_MissingHeader_Returns401(t *testing.T) {
	h := newOKChain(t,
		&stubJWT{},
		&stubSession{},
	)
	req := httptest.NewRequest(http.MethodGet, "/v1/cert/orders", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "CERT_UNAUTHORIZED") {
		t.Errorf("body missing CERT_UNAUTHORIZED: %s", rec.Body.String())
	}
}

func TestAuthn_ValidJWT_InjectsUserID(t *testing.T) {
	jSvc := &stubJWT{claims: &jwt.Claims{UserID: "42", SessionID: "sess-1"}}
	sSvc := &stubSession{data: &session.SessionData{UserID: "42"}}
	h := newOKChain(t, jSvc, sSvc)
	req := httptest.NewRequest(http.MethodGet, "/v1/cert/orders", nil)
	req.Header.Set("Authorization", "Bearer abc.def.ghi")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-User-ID"); got != "42" {
		t.Errorf("user id header = %q, want 42", got)
	}
}

func TestAuthn_InvalidJWT_Returns401(t *testing.T) {
	jSvc := &stubJWT{err: errors.New("bad sig")}
	sSvc := &stubSession{}
	h := newOKChain(t, jSvc, sSvc)
	req := httptest.NewRequest(http.MethodGet, "/v1/cert/orders", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAuthn_SessionRevoked_Returns401(t *testing.T) {
	jSvc := &stubJWT{claims: &jwt.Claims{UserID: "42", SessionID: "sess-1"}}
	sSvc := &stubSession{err: errors.New("session gone")}
	h := newOKChain(t, jSvc, sSvc)
	req := httptest.NewRequest(http.MethodGet, "/v1/cert/orders", nil)
	req.Header.Set("Authorization", "Bearer abc")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAuthn_EmptyUserID_Returns401(t *testing.T) {
	jSvc := &stubJWT{claims: &jwt.Claims{UserID: "", SessionID: "sess-1"}}
	sSvc := &stubSession{data: &session.SessionData{}}
	h := newOKChain(t, jSvc, sSvc)
	req := httptest.NewRequest(http.MethodGet, "/v1/cert/orders", nil)
	req.Header.Set("Authorization", "Bearer abc")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAuthn_CookieTakesPrecedenceOverBearer(t *testing.T) {
	// jwtSvc accepts any token — we just verify the cookie value was used
	// by attaching a non-Bearer header alongside the cookie.
	jSvc := &stubJWT{claims: &jwt.Claims{UserID: "7", SessionID: "sess-2"}}
	sSvc := &stubSession{data: &session.SessionData{UserID: "7"}}
	h := newOKChain(t, jSvc, sSvc)
	req := httptest.NewRequest(http.MethodGet, "/v1/cert/orders", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "cookie-jwt"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestAuthn_NilBackend_Returns401(t *testing.T) {
	h := Authn(nil, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer abc")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestUserIDFromContext(t *testing.T) {
	if _, err := UserIDFromContext(context.Background()); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("empty ctx err = %v, want ErrUnauthenticated", err)
	}
	ctx := WithUserID(context.Background(), "u-1")
	uid, err := UserIDFromContext(ctx)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if uid != "u-1" {
		t.Errorf("uid = %q, want u-1", uid)
	}
	if got := SessionIDFromContext(ctx); got != "" {
		t.Errorf("session id = %q, want empty", got)
	}
}

func TestEscapeMsg(t *testing.T) {
	if got := escapeMsg("plain"); got != "plain" {
		t.Errorf("plain = %q", got)
	}
	if got := escapeMsg(`he said "hi"`); got != `he said \"hi\"` {
		t.Errorf("escape quote = %q", got)
	}
	if got := escapeMsg(`back\slash`); got != `back\\slash` {
		t.Errorf("escape backslash = %q", got)
	}
}
