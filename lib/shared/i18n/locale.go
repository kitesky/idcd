package i18n

import "context"

// localeCtxKey is the unexported key under which the negotiated locale is
// stored on the request context. Unexported type prevents collisions with
// any other package that also stores under string keys.
type localeCtxKey struct{}

// WithLocale returns a derived context that carries the given locale code.
// Pass an empty code to clear the slot (FromContext will then fall back to
// the registry default).
func WithLocale(ctx context.Context, code string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, localeCtxKey{}, code)
}

// FromContext returns the locale code stored on ctx. When the context
// has no locale (or the stored value is empty) it falls back to the
// default registry's default locale so callers never observe "".
//
// Callers that want to detect "not set" explicitly should use FromContextRaw.
func FromContext(ctx context.Context) string {
	if code, ok := FromContextRaw(ctx); ok && code != "" {
		return code
	}
	return MustDefault().DefaultCode()
}

// FromContextRaw returns the raw stored locale code and an ok flag indicating
// whether one was present at all. Exposed for middleware tests that need to
// distinguish "absent" from "explicitly empty".
func FromContextRaw(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	v, ok := ctx.Value(localeCtxKey{}).(string)
	return v, ok
}

// FromContextWith returns the locale code from ctx, falling back to the
// given registry's default rather than MustDefault(). Useful in tests
// where the global registry singleton is not initialized.
func FromContextWith(ctx context.Context, reg *Registry) string {
	if code, ok := FromContextRaw(ctx); ok && code != "" {
		return code
	}
	if reg != nil {
		return reg.DefaultCode()
	}
	return ""
}
