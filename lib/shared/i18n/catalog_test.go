package i18n

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeRegistry builds a registry suitable for catalog tests without
// touching config/locales.json. Keeps tests hermetic and avoids ordering
// dependencies on MustDefault().
func makeRegistry(t *testing.T) *Registry {
	t.Helper()
	raw := []byte(`{
        "default": "cn",
        "locales": [
          {"code": "cn", "bcp47": "zh-CN", "label": "中文", "nativeLabel": "中文", "baseLanguage": "zh", "acceptLanguageAliases": ["zh", "zh-CN"], "dir": "ltr", "fontStack": "cjk", "fallback": []},
          {"code": "en", "bcp47": "en-US", "label": "English", "nativeLabel": "English", "baseLanguage": "en", "acceptLanguageAliases": ["en", "en-US"], "dir": "ltr", "fontStack": "latin", "fallback": []}
        ]
    }`)
	r, err := loadFromBytes(raw)
	if err != nil {
		t.Fatalf("registry load: %v", err)
	}
	return r
}

func writeJSON(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func loadTestCatalog(t *testing.T) (*Catalog, *Registry) {
	t.Helper()
	dir := t.TempDir()

	writeJSON(t, filepath.Join(dir, "cn", "errors.json"), `{
      "AUTH_REQUIRED": "请先登录",
      "VALIDATION_FAILED": "输入有误"
    }`)
	writeJSON(t, filepath.Join(dir, "cn", "common.json"), `{
      "actions": {"save": "保存", "cancel": "取消"}
    }`)
	writeJSON(t, filepath.Join(dir, "cn", "validation.json"), `{
      "maxLength": "最多 {max} 个字符"
    }`)
	writeJSON(t, filepath.Join(dir, "en", "errors.json"), `{
      "AUTH_REQUIRED": "Please sign in to continue"
    }`)
	writeJSON(t, filepath.Join(dir, "en", "validation.json"), `{
      "maxLength": "{max, plural, one {At most # character} other {At most # characters}}"
    }`)

	reg := makeRegistry(t)
	cat, err := Load(dir, reg)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cat, reg
}

func TestCatalogBasicTranslation(t *testing.T) {
	cat, _ := loadTestCatalog(t)

	if got := cat.T("cn", "errors.AUTH_REQUIRED", nil); got != "请先登录" {
		t.Errorf("cn AUTH_REQUIRED = %q", got)
	}
	if got := cat.T("en", "errors.AUTH_REQUIRED", nil); got != "Please sign in to continue" {
		t.Errorf("en AUTH_REQUIRED = %q", got)
	}
	if got := cat.T("cn", "common.actions.save", nil); got != "保存" {
		t.Errorf("cn save = %q", got)
	}
}

func TestCatalogFallback(t *testing.T) {
	cat, _ := loadTestCatalog(t)

	// en is missing VALIDATION_FAILED — should fall back to cn (default).
	if got := cat.T("en", "errors.VALIDATION_FAILED", nil); got != "输入有误" {
		t.Errorf("en VALIDATION_FAILED fallback = %q want '输入有误'", got)
	}
}

func TestCatalogMissingKey(t *testing.T) {
	cat, _ := loadTestCatalog(t)
	missing := "errors.DOES_NOT_EXIST"
	if got := cat.T("cn", missing, nil); got != missing {
		t.Errorf("missing key should return raw key, got %q", got)
	}
	if cat.Has("cn", missing) {
		t.Error("Has() should be false for missing key")
	}
}

func TestCatalogUnknownLocaleFallsBack(t *testing.T) {
	cat, _ := loadTestCatalog(t)
	// "ja" is not in registry; FallbackChain still ends at default cn.
	if got := cat.T("ja", "errors.AUTH_REQUIRED", nil); got != "请先登录" {
		t.Errorf("unknown locale should fall back to default, got %q", got)
	}
}

func TestCatalogInterpolation(t *testing.T) {
	cat, _ := loadTestCatalog(t)
	got := cat.T("cn", "validation.maxLength", map[string]any{"max": 5})
	want := "最多 5 个字符"
	if got != want {
		t.Errorf("cn maxLength = %q want %q", got, want)
	}
}

func TestCatalogPluralOne(t *testing.T) {
	cat, _ := loadTestCatalog(t)
	got := cat.T("en", "validation.maxLength", map[string]any{"max": 1})
	want := "At most 1 character"
	if got != want {
		t.Errorf("en plural one = %q want %q", got, want)
	}
}

func TestCatalogPluralOther(t *testing.T) {
	cat, _ := loadTestCatalog(t)
	got := cat.T("en", "validation.maxLength", map[string]any{"max": 5})
	want := "At most 5 characters"
	if got != want {
		t.Errorf("en plural other = %q want %q", got, want)
	}
}

func TestCatalogPluralCNAlwaysOther(t *testing.T) {
	// Even with count=1, Chinese should not pick "one" form. Our cn template
	// doesn't use plural — but seed an English-style template under cn to
	// confirm pluralKey behavior.
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, "cn", "validation.json"), `{
      "items": "{count, plural, one {一个项目} other {{count} 个项目}}"
    }`)
	writeJSON(t, filepath.Join(dir, "en", "validation.json"), `{}`)
	reg := makeRegistry(t)
	cat, err := Load(dir, reg)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := cat.T("cn", "validation.items", map[string]any{"count": 1})
	if !strings.Contains(got, "1 个项目") {
		t.Errorf("cn count=1 should still pick 'other' form; got %q", got)
	}
}

func TestCatalogPluralExactMatch(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, "en", "validation.json"), `{
      "items": "{count, plural, =0 {no items} one {# item} other {# items}}"
    }`)
	writeJSON(t, filepath.Join(dir, "cn", "validation.json"), `{}`)
	reg := makeRegistry(t)
	cat, err := Load(dir, reg)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cat.T("en", "validation.items", map[string]any{"count": 0}); got != "no items" {
		t.Errorf("=0 exact match = %q", got)
	}
	if got := cat.T("en", "validation.items", map[string]any{"count": 1}); got != "1 item" {
		t.Errorf("one = %q", got)
	}
	if got := cat.T("en", "validation.items", map[string]any{"count": 7}); got != "7 items" {
		t.Errorf("other = %q", got)
	}
}

func TestCatalogIgnoresUnsupportedICU(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, "en", "ui.json"), `{
      "greeting": "{gender, select, male {Mr.} female {Ms.} other {Mx.}}"
    }`)
	writeJSON(t, filepath.Join(dir, "cn", "ui.json"), `{}`)
	reg := makeRegistry(t)
	cat, err := Load(dir, reg)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Select syntax is not supported — should return verbatim (no panic).
	got := cat.T("en", "ui.greeting", map[string]any{"gender": "male"})
	if !strings.Contains(got, "select") {
		t.Errorf("unsupported select syntax should remain verbatim, got %q", got)
	}
}

func TestInterpolateLeavesUnknownVarsAlone(t *testing.T) {
	got := Interpolate("hi {name}, you have {n} messages", map[string]any{"name": "Alice"})
	want := "hi Alice, you have {n} messages"
	if got != want {
		t.Errorf("Interpolate = %q want %q", got, want)
	}
}

func TestInterpolateHandlesMalformed(t *testing.T) {
	got := Interpolate("hello {name", map[string]any{"name": "Bob"})
	// Unclosed brace — current implementation writes the rest verbatim.
	if !strings.Contains(got, "{name") {
		t.Errorf("malformed brace should not panic; got %q", got)
	}
}

func TestCatalogLoadRejectsUnsupportedLocaleDirs(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, "cn", "errors.json"), `{"X": "x"}`)
	writeJSON(t, filepath.Join(dir, "ja", "errors.json"), `{"X": "X"}`)
	reg := makeRegistry(t)
	cat, err := Load(dir, reg)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := cat.messages["ja"]; ok {
		t.Error("unsupported locale dir should be skipped")
	}
	if _, ok := cat.messages["cn"]; !ok {
		t.Error("cn should be loaded")
	}
}

func TestCatalogNestedJSONFlattening(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, "cn", "common.json"), `{
      "actions": {"save": "保存", "nested": {"deep": "深层"}}
    }`)
	writeJSON(t, filepath.Join(dir, "en", "common.json"), `{}`)
	reg := makeRegistry(t)
	cat, err := Load(dir, reg)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cat.T("cn", "common.actions.save", nil); got != "保存" {
		t.Errorf("flatten level 2 = %q", got)
	}
	if got := cat.T("cn", "common.actions.nested.deep", nil); got != "深层" {
		t.Errorf("flatten level 3 = %q", got)
	}
}

func TestCatalogHas(t *testing.T) {
	cat, _ := loadTestCatalog(t)
	if !cat.Has("cn", "errors.AUTH_REQUIRED") {
		t.Error("Has should be true for cn key")
	}
	if !cat.Has("en", "errors.VALIDATION_FAILED") {
		t.Error("Has should be true via fallback")
	}
	if cat.Has("cn", "nope.nope") {
		t.Error("Has should be false for missing key")
	}
}
