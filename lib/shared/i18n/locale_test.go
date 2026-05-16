package i18n

import (
	"context"
	"testing"
)

func TestWithLocaleAndFromContext(t *testing.T) {
	ctx := context.Background()
	reg := makeRegistry(t)

	// Empty ctx -> default via reg.
	if got := FromContextWith(ctx, reg); got != "cn" {
		t.Errorf("empty ctx fallback = %q want cn", got)
	}

	// Explicit locale roundtrip.
	ctx = WithLocale(ctx, "en")
	if got := FromContextWith(ctx, reg); got != "en" {
		t.Errorf("WithLocale roundtrip = %q want en", got)
	}

	// Empty code stored -> falls back to default.
	ctx2 := WithLocale(context.Background(), "")
	if got := FromContextWith(ctx2, reg); got != "cn" {
		t.Errorf("empty code stored falls back to default; got %q", got)
	}

	// Raw inspection.
	if v, ok := FromContextRaw(ctx); !ok || v != "en" {
		t.Errorf("FromContextRaw = (%q,%v) want (en,true)", v, ok)
	}
	if _, ok := FromContextRaw(context.Background()); ok {
		t.Error("FromContextRaw should report not-present for plain ctx")
	}
}

func TestWithLocaleNilContext(t *testing.T) {
	// Defensive: nil ctx should not panic.
	ctx := WithLocale(nil, "en")
	if v, _ := FromContextRaw(ctx); v != "en" {
		t.Errorf("WithLocale(nil, ...) lost value; got %q", v)
	}
}
