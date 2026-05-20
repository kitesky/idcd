// Package i18n wires the shared lib/shared/i18n locale registry into the
// API HTTP stack. The Middleware resolves the request locale from a fixed
// chain of sources (header → JWT claim → Accept-Language → default) and
// stashes the result on the request context so handlers / response helpers
// can render localized messages.
package i18n

import (
	"context"
	"net/http"

	jwtlib "github.com/kite365/idcd/lib/auth/jwt"
	shared "github.com/kite365/idcd/lib/shared/i18n"
)

// claimsCtxKey identifies an optional *jwt.Claims value already present
// on the context. We deliberately don't depend on apps/api/internal/middleware
// (which would create an import cycle), so callers that have parsed claims
// upstream can opt in by stashing the claims under this key.
//
// In practice the Authn middleware in apps/api/internal/middleware doesn't
// store the claims object directly today (only UserID / SessionID). When
// Phase 2 wires claim-based locale resolution it can either:
//
//   - call i18n.WithClaims(ctx, claims) before continuing the chain, or
//   - move the claim-locale read into Authn itself (preferred long term).
//
// Either way the middleware here keeps working — it just won't find a
// claims object on the context until that wiring happens.
type claimsCtxKey struct{}

// WithClaims stores parsed JWT claims on ctx so the i18n Middleware can
// read locale information from them. Optional — Middleware falls through
// to Accept-Language / default if no claims are present.
func WithClaims(ctx context.Context, claims *jwtlib.Claims) context.Context {
	if claims == nil {
		return ctx
	}
	return context.WithValue(ctx, claimsCtxKey{}, claims)
}

// ClaimsFromContext returns previously stored claims, if any. Exposed so
// tests and downstream packages can compose without re-parsing.
func ClaimsFromContext(ctx context.Context) (*jwtlib.Claims, bool) {
	if ctx == nil {
		return nil, false
	}
	v, ok := ctx.Value(claimsCtxKey{}).(*jwtlib.Claims)
	return v, ok
}

// resolver is one step in the locale resolution chain. Returning an empty
// string means "this source had nothing usable, try the next one". The
// list form keeps everything iterable and lint-friendly — no if/else
// pyramids and no `locale == "en" ? ... : ...` style branches.
type resolver func(reg *shared.Registry, r *http.Request) string

// resolvers is the ordered chain. Composed as a package-level var so
// tests can replace it (the export is intentional indirection).
var resolvers = []resolver{
	resolveFromHeader,
	resolveFromJWT,
	resolveFromAcceptLanguage,
}

// Middleware returns an HTTP middleware that resolves the request locale
// and writes it onto the context using shared.WithLocale.
//
// Resolution order (first non-empty supported value wins):
//
//  1. X-Locale request header
//  2. JWT claim "locale" (if claims have been stashed on ctx upstream)
//  3. Accept-Language negotiation via registry
//  4. registry default
//
// The chosen locale is also echoed back as the X-Locale response header so
// clients can detect what the server actually used (useful when debugging
// Accept-Language quirks).
func Middleware(reg *shared.Registry) func(http.Handler) http.Handler {
	if reg == nil {
		panic("i18n.Middleware: registry must not be nil")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			code := negotiate(reg, r)
			w.Header().Set("X-Locale", code)
			ctx := shared.WithLocale(r.Context(), code)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// negotiate walks the resolver chain and returns the first supported code,
// defaulting to reg.DefaultCode() when nothing matches.
//
// Exposed for unit tests in this package; not part of the public API.
func negotiate(reg *shared.Registry, r *http.Request) string {
	for _, res := range resolvers {
		if code := res(reg, r); code != "" && reg.IsSupported(code) {
			return code
		}
	}
	return reg.DefaultCode()
}

func resolveFromHeader(reg *shared.Registry, r *http.Request) string {
	return r.Header.Get("X-Locale")
}

func resolveFromJWT(_ *shared.Registry, r *http.Request) string {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		return ""
	}
	return claims.Locale
}

func resolveFromAcceptLanguage(reg *shared.Registry, r *http.Request) string {
	header := r.Header.Get("Accept-Language")
	if header == "" {
		return ""
	}
	// Negotiate always returns a supported code (defaulting), but we want
	// to treat "we had to fall back to default" as "this source did not
	// match" so the chain can be tested independently. If the negotiated
	// result equals the default AND the header didn't actually contain
	// any token resolving to default, that's still fine — IsSupported in
	// the caller will accept it. Returning the result directly is the
	// simplest correct behaviour.
	return reg.Negotiate(header)
}
