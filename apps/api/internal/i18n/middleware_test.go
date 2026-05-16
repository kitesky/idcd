package i18n

import (
	"net/http"
	"net/http/httptest"
	"testing"

	jwtlib "github.com/kite365/idcd/lib/auth/jwt"
	shared "github.com/kite365/idcd/lib/shared/i18n"
)

// testRegistry builds a fresh registry from inline JSON so middleware tests
// don't depend on the global config/locales.json being reachable from the
// test cwd (worktrees etc).
func testRegistry(t *testing.T) *shared.Registry {
	t.Helper()
	raw := []byte(`{
        "default": "cn",
        "locales": [
          {"code": "cn", "bcp47": "zh-CN", "label": "中文", "nativeLabel": "中文", "baseLanguage": "zh", "acceptLanguageAliases": ["zh", "zh-CN", "zh-Hans"], "dir": "ltr", "fontStack": "cjk", "fallback": []},
          {"code": "en", "bcp47": "en-US", "label": "English", "nativeLabel": "English", "baseLanguage": "en", "acceptLanguageAliases": ["en", "en-US"], "dir": "ltr", "fontStack": "latin", "fallback": []}
        ]
    }`)
	r, err := shared.LoadFromBytesForTesting(raw)
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	return r
}

// runMiddleware exercises Middleware with the given request and returns the
// locale that the wrapped handler observed on its request context, plus the
// X-Locale response header value.
func runMiddleware(t *testing.T, reg *shared.Registry, req *http.Request) (ctxLocale, hdrLocale string) {
	t.Helper()
	var observed string
	handler := Middleware(reg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observed, _ = shared.FromContextRaw(r.Context())
	}))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return observed, rr.Header().Get("X-Locale")
}

func TestMiddlewareHeaderWins(t *testing.T) {
	reg := testRegistry(t)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Locale", "en")
	req.Header.Set("Accept-Language", "zh-CN")

	ctxLocale, hdr := runMiddleware(t, reg, req)
	if ctxLocale != "en" {
		t.Errorf("ctx locale = %q want en", ctxLocale)
	}
	if hdr != "en" {
		t.Errorf("response X-Locale = %q want en", hdr)
	}
}

func TestMiddlewareHeaderUnsupportedFallsThrough(t *testing.T) {
	reg := testRegistry(t)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Locale", "ja")
	req.Header.Set("Accept-Language", "en-US")

	ctxLocale, _ := runMiddleware(t, reg, req)
	if ctxLocale != "en" {
		t.Errorf("unsupported header should fall through to Accept-Language; got %q", ctxLocale)
	}
}

func TestMiddlewareJWTClaim(t *testing.T) {
	reg := testRegistry(t)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Language", "zh-CN") // should be overridden by claim
	claims := &jwtlib.Claims{Locale: "en"}
	req = req.WithContext(WithClaims(req.Context(), claims))

	ctxLocale, _ := runMiddleware(t, reg, req)
	if ctxLocale != "en" {
		t.Errorf("ctx locale = %q want en (from JWT claim)", ctxLocale)
	}
}

func TestMiddlewareJWTClaimUnsupportedFallsThrough(t *testing.T) {
	reg := testRegistry(t)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Language", "en-US")
	claims := &jwtlib.Claims{Locale: "ja"}
	req = req.WithContext(WithClaims(req.Context(), claims))

	ctxLocale, _ := runMiddleware(t, reg, req)
	if ctxLocale != "en" {
		t.Errorf("unsupported claim should fall through; got %q", ctxLocale)
	}
}

func TestMiddlewareAcceptLanguage(t *testing.T) {
	reg := testRegistry(t)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	ctxLocale, _ := runMiddleware(t, reg, req)
	if ctxLocale != "en" {
		t.Errorf("Accept-Language en should resolve to en; got %q", ctxLocale)
	}
}

func TestMiddlewareAcceptLanguageChinese(t *testing.T) {
	reg := testRegistry(t)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.5")

	ctxLocale, _ := runMiddleware(t, reg, req)
	if ctxLocale != "cn" {
		t.Errorf("Accept-Language zh-CN should resolve to cn; got %q", ctxLocale)
	}
}

func TestMiddlewareDefault(t *testing.T) {
	reg := testRegistry(t)
	req := httptest.NewRequest("GET", "/", nil)
	ctxLocale, hdr := runMiddleware(t, reg, req)
	if ctxLocale != "cn" {
		t.Errorf("empty request should resolve to default cn; got %q", ctxLocale)
	}
	if hdr != "cn" {
		t.Errorf("response X-Locale = %q want cn", hdr)
	}
}

func TestMiddlewareNoClaimsOnCtxStillWorks(t *testing.T) {
	reg := testRegistry(t)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Language", "en-US")
	// No WithClaims call — JWT resolver returns empty, chain proceeds.

	ctxLocale, _ := runMiddleware(t, reg, req)
	if ctxLocale != "en" {
		t.Errorf("missing claims should not break chain; got %q", ctxLocale)
	}
}

func TestMiddlewareNilRegistryPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Middleware(nil) should panic")
		}
	}()
	Middleware(nil)
}

func TestClaimsFromContextEmpty(t *testing.T) {
	if _, ok := ClaimsFromContext(nil); ok {
		t.Error("nil context should return ok=false")
	}
}

func TestWithClaimsNilNoop(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	ctx := WithClaims(req.Context(), nil)
	if _, ok := ClaimsFromContext(ctx); ok {
		t.Error("WithClaims(nil) should be a no-op")
	}
}
