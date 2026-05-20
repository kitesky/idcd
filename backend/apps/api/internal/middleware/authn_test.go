package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kite365/idcd/lib/auth/jwt"
	"github.com/kite365/idcd/lib/auth/session"
)

// ─────────────────────────────────────────────
// Mocks
// ─────────────────────────────────────────────

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

// --- mock PAT verifier ---

type mockPATVerifier struct {
	info *PATInfo
	err  error

	mu        sync.Mutex
	touched   []string
	touchDone chan struct{}
}

func newMockPATVerifier() *mockPATVerifier {
	return &mockPATVerifier{touchDone: make(chan struct{}, 1)}
}

func (m *mockPATVerifier) VerifyPAT(_ context.Context, _ string) (*PATInfo, error) {
	return m.info, m.err
}

func (m *mockPATVerifier) TouchLastUsed(_ context.Context, patID string) error {
	m.mu.Lock()
	m.touched = append(m.touched, patID)
	m.mu.Unlock()
	select {
	case m.touchDone <- struct{}{}:
	default:
	}
	return nil
}

func (m *mockPATVerifier) touchedIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.touched))
	copy(out, m.touched)
	return out
}

// --- mock APIKey verifier ---

type mockAPIKeyVerifier struct {
	info *APIKeyInfo
	err  error

	mu        sync.Mutex
	touched   []string
	touchDone chan struct{}
}

func newMockAPIKeyVerifier() *mockAPIKeyVerifier {
	return &mockAPIKeyVerifier{touchDone: make(chan struct{}, 1)}
}

func (m *mockAPIKeyVerifier) VerifyAPIKey(_ context.Context, _ string) (*APIKeyInfo, error) {
	return m.info, m.err
}

func (m *mockAPIKeyVerifier) TouchLastUsed(_ context.Context, apiKeyID string) error {
	m.mu.Lock()
	m.touched = append(m.touched, apiKeyID)
	m.mu.Unlock()
	select {
	case m.touchDone <- struct{}{}:
	default:
	}
	return nil
}

func (m *mockAPIKeyVerifier) touchedIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.touched))
	copy(out, m.touched)
	return out
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

// downstream is a marker handler that records what auth-related context
// values were observed by the wrapped handler.
type captured struct {
	called     int32
	userID     string
	sessionID  string
	authMethod string
	patID      string
	apiKeyID   string
}

func captureHandler() (*captured, http.Handler) {
	c := &captured{}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&c.called, 1)
		c.userID = UserIDFromContext(r.Context())
		c.sessionID = SessionIDFromContext(r.Context())
		c.authMethod = AuthMethodFromContext(r.Context())
		c.patID = PATIDFromContext(r.Context())
		c.apiKeyID = APIKeyIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	return c, h
}

func okHandler(w http.ResponseWriter, r *http.Request) {
	uid := UserIDFromContext(r.Context())
	w.Header().Set("X-User-ID", uid)
	w.WriteHeader(http.StatusOK)
}

func authnHandler(jwtSvc TokenVerifier, sessSvc SessionChecker) http.Handler {
	return Authn(jwtSvc, sessSvc)(http.HandlerFunc(okHandler))
}

func authnAllHandler(
	jwtSvc TokenVerifier,
	sessSvc SessionChecker,
	patSvc PATVerifier,
	apiKeySvc APIKeyVerifier,
	inner http.Handler,
) http.Handler {
	if inner == nil {
		inner = http.HandlerFunc(okHandler)
	}
	return AuthnWithTokens(jwtSvc, sessSvc, patSvc, apiKeySvc)(inner)
}

func ptrTime(t time.Time) *time.Time { return &t }

// ─────────────────────────────────────────────
// Legacy JWT-only tests (kept to guarantee Authn() compatibility)
// ─────────────────────────────────────────────

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

	c, inner := captureHandler()
	h := authnAllHandler(jwtSvc, sessSvc, nil, nil, inner)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer valid.token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if c.userID != "usr_001" {
		t.Errorf("expected user_id=usr_001, got %q", c.userID)
	}
	if c.sessionID != "sess_001" {
		t.Errorf("expected session_id=sess_001, got %q", c.sessionID)
	}
	if c.authMethod != AuthMethodJWT {
		t.Errorf("expected auth_method=jwt, got %q", c.authMethod)
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

func TestExtractBearerToken_cookieTakesPrecedence(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "cookie.jwt.value"})
	req.Header.Set("Authorization", "Bearer header.jwt.value")
	if got := extractBearerToken(req); got != "cookie.jwt.value" {
		t.Errorf("cookie should win, got %q", got)
	}
}

// ─────────────────────────────────────────────
// PAT tests
// ─────────────────────────────────────────────

func TestAuthn_PAT_success(t *testing.T) {
	patSvc := newMockPATVerifier()
	patSvc.info = &PATInfo{ID: "pat_abc", UserID: "usr_pat"}

	c, inner := captureHandler()
	h := authnAllHandler(&mockJWTVerifier{}, &mockSessionChecker{}, patSvc, nil, inner)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer idcd_pat_deadbeef")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if c.userID != "usr_pat" {
		t.Errorf("user_id: got %q want usr_pat", c.userID)
	}
	if c.authMethod != AuthMethodPAT {
		t.Errorf("auth_method: got %q want pat", c.authMethod)
	}
	if c.patID != "pat_abc" {
		t.Errorf("pat_id: got %q want pat_abc", c.patID)
	}
	if c.sessionID != "" {
		t.Errorf("session_id should be empty for PAT, got %q", c.sessionID)
	}
	if got := rr.Header().Get("X-Auth-Method"); got != AuthMethodPAT {
		t.Errorf("X-Auth-Method header: got %q want pat", got)
	}

	// Wait briefly for the async TouchLastUsed; bail if it doesn't fire.
	select {
	case <-patSvc.touchDone:
	case <-time.After(time.Second):
		t.Fatal("TouchLastUsed was not called within 1s")
	}
	if ids := patSvc.touchedIDs(); len(ids) != 1 || ids[0] != "pat_abc" {
		t.Errorf("TouchLastUsed got %v, want [pat_abc]", ids)
	}
}

func TestAuthn_PAT_revoked(t *testing.T) {
	// Revocation in pat_handler.go is implemented as DELETE — the verifier
	// returns an error / nil info to signal "not found".
	patSvc := newMockPATVerifier()
	patSvc.err = errors.New("no rows")

	h := authnAllHandler(&mockJWTVerifier{}, &mockSessionChecker{}, patSvc, nil, nil)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer idcd_pat_revokedtoken")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for revoked PAT, got %d", rr.Code)
	}
}

func TestAuthn_PAT_revokedNilInfoNoError(t *testing.T) {
	// Defensive: verifier returns (nil, nil). Treat as 401, never panic.
	patSvc := newMockPATVerifier()

	h := authnAllHandler(&mockJWTVerifier{}, &mockSessionChecker{}, patSvc, nil, nil)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer idcd_pat_missing")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthn_PAT_expired(t *testing.T) {
	patSvc := newMockPATVerifier()
	patSvc.info = &PATInfo{
		ID:        "pat_old",
		UserID:    "usr_pat",
		ExpiresAt: ptrTime(time.Now().UTC().Add(-time.Hour)),
	}

	h := authnAllHandler(&mockJWTVerifier{}, &mockSessionChecker{}, patSvc, nil, nil)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer idcd_pat_expired")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired PAT, got %d", rr.Code)
	}
}

func TestAuthn_PAT_futureExpiryAccepted(t *testing.T) {
	patSvc := newMockPATVerifier()
	patSvc.info = &PATInfo{
		ID:        "pat_live",
		UserID:    "usr_pat",
		ExpiresAt: ptrTime(time.Now().UTC().Add(time.Hour)),
	}

	c, inner := captureHandler()
	h := authnAllHandler(&mockJWTVerifier{}, &mockSessionChecker{}, patSvc, nil, inner)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer idcd_pat_live")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for non-expired PAT, got %d", rr.Code)
	}
	if c.userID != "usr_pat" {
		t.Errorf("user_id: got %q want usr_pat", c.userID)
	}
}

func TestAuthn_PAT_notSupportedWhenVerifierNil(t *testing.T) {
	// If the deployment doesn't wire a PAT verifier, an idcd_pat_* token
	// must NOT fall through to the JWT verifier (which would misclassify
	// it). It should be flatly rejected.
	jwtSvc := &mockJWTVerifier{claims: &jwt.Claims{UserID: "should_not_see", SessionID: "x"}}
	sessSvc := &mockSessionChecker{data: &session.SessionData{}}

	h := authnAllHandler(jwtSvc, sessSvc, nil, nil, nil)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer idcd_pat_xxxxxxxx")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when PAT verifier nil, got %d", rr.Code)
	}
}

// ─────────────────────────────────────────────
// API key tests
// ─────────────────────────────────────────────

func TestAuthn_APIKey_success_live(t *testing.T) {
	akSvc := newMockAPIKeyVerifier()
	akSvc.info = &APIKeyInfo{
		ID:        "key_abc",
		OwnerType: "user",
		OwnerID:   "usr_ak",
		Status:    "active",
	}

	c, inner := captureHandler()
	h := authnAllHandler(&mockJWTVerifier{}, &mockSessionChecker{}, nil, akSvc, inner)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer sk_live_abcd1234")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if c.userID != "usr_ak" {
		t.Errorf("user_id: got %q want usr_ak", c.userID)
	}
	if c.authMethod != AuthMethodAPIKey {
		t.Errorf("auth_method: got %q want apikey", c.authMethod)
	}
	if c.apiKeyID != "key_abc" {
		t.Errorf("api_key_id: got %q want key_abc", c.apiKeyID)
	}

	select {
	case <-akSvc.touchDone:
	case <-time.After(time.Second):
		t.Fatal("TouchLastUsed not called within 1s")
	}
	if ids := akSvc.touchedIDs(); len(ids) != 1 || ids[0] != "key_abc" {
		t.Errorf("TouchLastUsed got %v want [key_abc]", ids)
	}
}

func TestAuthn_APIKey_success_test(t *testing.T) {
	akSvc := newMockAPIKeyVerifier()
	akSvc.info = &APIKeyInfo{
		ID:      "key_test",
		OwnerID: "usr_ak",
		Status:  "active",
	}

	h := authnAllHandler(&mockJWTVerifier{}, &mockSessionChecker{}, nil, akSvc, nil)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer sk_test_abcd1234")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("sk_test_* token should auth via APIKeyVerifier; got %d", rr.Code)
	}
}

func TestAuthn_APIKey_revoked(t *testing.T) {
	akSvc := newMockAPIKeyVerifier()
	akSvc.info = &APIKeyInfo{ID: "key_x", OwnerID: "usr_ak", Status: "revoked"}

	h := authnAllHandler(&mockJWTVerifier{}, &mockSessionChecker{}, nil, akSvc, nil)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer sk_live_revoked")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for revoked API key, got %d", rr.Code)
	}
}

func TestAuthn_APIKey_expired(t *testing.T) {
	akSvc := newMockAPIKeyVerifier()
	akSvc.info = &APIKeyInfo{
		ID:        "key_old",
		OwnerID:   "usr_ak",
		Status:    "active",
		ExpiresAt: ptrTime(time.Now().UTC().Add(-time.Hour)),
	}

	h := authnAllHandler(&mockJWTVerifier{}, &mockSessionChecker{}, nil, akSvc, nil)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer sk_live_expired")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired API key, got %d", rr.Code)
	}
}

func TestAuthn_APIKey_lookupError(t *testing.T) {
	akSvc := newMockAPIKeyVerifier()
	akSvc.err = errors.New("no rows")

	h := authnAllHandler(&mockJWTVerifier{}, &mockSessionChecker{}, nil, akSvc, nil)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer sk_live_missing")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when API key not found, got %d", rr.Code)
	}
}

func TestAuthn_APIKey_notSupportedWhenVerifierNil(t *testing.T) {
	jwtSvc := &mockJWTVerifier{claims: &jwt.Claims{UserID: "should_not_see"}}
	sessSvc := &mockSessionChecker{}

	h := authnAllHandler(jwtSvc, sessSvc, nil, nil, nil)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer sk_live_xxxxxxxx")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when API key verifier nil, got %d", rr.Code)
	}
}

// ─────────────────────────────────────────────
// Routing / cross-cutting tests
// ─────────────────────────────────────────────

func TestAuthn_unknownPrefixFallsBackToJWT(t *testing.T) {
	// A token that's neither idcd_pat_* nor sk_live_*/sk_test_* must be
	// handed to the JWT verifier. PAT/APIKey verifiers should be
	// completely untouched.
	claims := &jwt.Claims{UserID: "usr_jwt", SessionID: "sess_jwt"}
	jwtSvc := &mockJWTVerifier{claims: claims}
	sessSvc := &mockSessionChecker{data: &session.SessionData{}}

	patSvc := newMockPATVerifier()
	patSvc.info = &PATInfo{ID: "should_not_be_used", UserID: "wrong_user"}
	akSvc := newMockAPIKeyVerifier()
	akSvc.info = &APIKeyInfo{ID: "should_not_be_used", OwnerID: "wrong_user"}

	c, inner := captureHandler()
	h := authnAllHandler(jwtSvc, sessSvc, patSvc, akSvc, inner)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.payload.sig")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if c.userID != "usr_jwt" {
		t.Errorf("user_id: got %q want usr_jwt", c.userID)
	}
	if c.authMethod != AuthMethodJWT {
		t.Errorf("auth_method: got %q want jwt", c.authMethod)
	}
	if got := patSvc.touchedIDs(); len(got) != 0 {
		t.Errorf("PAT TouchLastUsed should NOT have been called, got %v", got)
	}
	if got := akSvc.touchedIDs(); len(got) != 0 {
		t.Errorf("APIKey TouchLastUsed should NOT have been called, got %v", got)
	}
}

func TestAuthn_cookieRoutesToJWT(t *testing.T) {
	// Cookie auth always means a browser → JWT path, even when PAT/APIKey
	// verifiers are wired.
	claims := &jwt.Claims{UserID: "usr_cookie", SessionID: "sess_cookie"}
	jwtSvc := &mockJWTVerifier{claims: claims}
	sessSvc := &mockSessionChecker{data: &session.SessionData{}}

	patSvc := newMockPATVerifier()
	akSvc := newMockAPIKeyVerifier()

	c, inner := captureHandler()
	h := authnAllHandler(jwtSvc, sessSvc, patSvc, akSvc, inner)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "browser.jwt"})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if c.authMethod != AuthMethodJWT {
		t.Errorf("auth_method: got %q want jwt", c.authMethod)
	}
}

func TestAuthn_authnWithTokensCompatibleWithLegacyAuthn(t *testing.T) {
	// Authn() must behave identically to AuthnWithTokens(jwt, sess, nil, nil)
	// for non-token Bearer headers so the existing server.go wiring keeps
	// working.
	claims := &jwt.Claims{UserID: "usr", SessionID: "sess"}
	jwtSvc := &mockJWTVerifier{claims: claims}
	sessSvc := &mockSessionChecker{data: &session.SessionData{}}

	legacy := Authn(jwtSvc, sessSvc)(http.HandlerFunc(okHandler))
	dual := AuthnWithTokens(jwtSvc, sessSvc, nil, nil)(http.HandlerFunc(okHandler))

	for _, tt := range []struct {
		name string
		h    http.Handler
	}{
		{"legacy", legacy},
		{"dual", dual},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/protected", nil)
			req.Header.Set("Authorization", "Bearer normal.jwt.token")
			rr := httptest.NewRecorder()
			tt.h.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Errorf("%s: expected 200, got %d", tt.name, rr.Code)
			}
			if rr.Header().Get("X-User-ID") != "usr" {
				t.Errorf("%s: user_id not injected", tt.name)
			}
		})
	}
}

// ─────────────────────────────────────────────
// HashToken + small surface tests
// ─────────────────────────────────────────────

func TestHashToken_matchesSHA256(t *testing.T) {
	raw := "idcd_pat_deadbeefcafe"
	want := sha256.Sum256([]byte(raw))
	got := HashToken(raw)
	if got != hex.EncodeToString(want[:]) {
		t.Errorf("HashToken mismatch: got %q", got)
	}
}

func TestHashToken_deterministic(t *testing.T) {
	a := HashToken("sk_live_xxx")
	b := HashToken("sk_live_xxx")
	if a != b {
		t.Errorf("HashToken not deterministic: %q vs %q", a, b)
	}
	if a == HashToken("sk_live_yyy") {
		t.Errorf("HashToken collided for distinct inputs")
	}
}

func TestContextKeys_distinctAndStable(t *testing.T) {
	// Make sure the three exported context-key accessors return distinct
	// values — if two collided we'd silently leak state between auth modes.
	keys := []any{UserIDContextKey(), SessionIDContextKey(), AuthMethodContextKey()}
	for i := range keys {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] == keys[j] {
				t.Errorf("context keys %d and %d collide: %v", i, j, keys[i])
			}
		}
	}

	// And the values they return must round-trip through the public getters.
	ctx := context.WithValue(context.Background(), UserIDContextKey(), "u1")
	ctx = context.WithValue(ctx, SessionIDContextKey(), "s1")
	ctx = context.WithValue(ctx, AuthMethodContextKey(), AuthMethodJWT)
	if got := UserIDFromContext(ctx); got != "u1" {
		t.Errorf("UserIDFromContext: got %q want u1", got)
	}
	if got := SessionIDFromContext(ctx); got != "s1" {
		t.Errorf("SessionIDFromContext: got %q want s1", got)
	}
	if got := AuthMethodFromContext(ctx); got != AuthMethodJWT {
		t.Errorf("AuthMethodFromContext: got %q want jwt", got)
	}
}

func TestIsPATTokenAndIsAPIKeyToken(t *testing.T) {
	tests := []struct {
		token   string
		wantPAT bool
		wantAK  bool
	}{
		{"idcd_pat_abc", true, false},
		{"sk_live_abc", false, true},
		{"sk_test_abc", false, true},
		{"sk_other_abc", false, false},
		{"eyJ.foo.bar", false, false},
		{"", false, false},
	}
	for _, tt := range tests {
		if got := isPATToken(tt.token); got != tt.wantPAT {
			t.Errorf("isPATToken(%q): got %v want %v", tt.token, got, tt.wantPAT)
		}
		if got := isAPIKeyToken(tt.token); got != tt.wantAK {
			t.Errorf("isAPIKeyToken(%q): got %v want %v", tt.token, got, tt.wantAK)
		}
	}
}

func TestAuthn_PAT_downstreamHandlerCalledExactlyOnce(t *testing.T) {
	// Guard against accidental double-dispatch in the prefix-routing logic.
	patSvc := newMockPATVerifier()
	patSvc.info = &PATInfo{ID: "pat_solo", UserID: "u"}

	c, inner := captureHandler()
	h := authnAllHandler(&mockJWTVerifier{}, &mockSessionChecker{}, patSvc, nil, inner)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer idcd_pat_solo")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if atomic.LoadInt32(&c.called) != 1 {
		t.Errorf("downstream called %d times, want 1", atomic.LoadInt32(&c.called))
	}
	// Drain to keep -race happy.
	<-patSvc.touchDone
}
