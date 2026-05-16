package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// TestTemplatePath_HitsExactLocale verifies that when both cn and en files
// exist on disk, requesting en returns the en file directly (no fallback).
func TestTemplatePath_HitsExactLocale(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "verify_email.cn.html"), "cn body")
	write(t, filepath.Join(dir, "verify_email.en.html"), "en body")

	got, err := TemplatePath(dir, "verify_email", "en")
	if err != nil {
		t.Fatalf("TemplatePath: %v", err)
	}
	if !strings.HasSuffix(got, "verify_email.en.html") {
		t.Errorf("expected en suffix, got %q", got)
	}
}

// TestTemplatePath_FallsBackToDefault asserts that a missing locale falls
// back along the registry chain to the default locale (cn).
func TestTemplatePath_FallsBackToDefault(t *testing.T) {
	dir := t.TempDir()
	// Only cn exists — request en should fall back.
	write(t, filepath.Join(dir, "verify_email.cn.html"), "cn body")

	got, err := TemplatePath(dir, "verify_email", "en")
	if err != nil {
		t.Fatalf("TemplatePath: %v", err)
	}
	if !strings.HasSuffix(got, "verify_email.cn.html") {
		t.Errorf("expected cn fallback, got %q", got)
	}
}

// TestTemplatePath_NoCandidate covers the "fully missing" error path.
func TestTemplatePath_NoCandidate(t *testing.T) {
	dir := t.TempDir()
	if _, err := TemplatePath(dir, "verify_email", "en"); err == nil {
		t.Fatal("expected error when no template files exist")
	}
}

// TestTemplatePathFS_UsingMapFS exercises the same fallback chain logic
// against an in-memory filesystem to keep the test hermetic.
func TestTemplatePathFS_UsingMapFS(t *testing.T) {
	cases := []struct {
		name         string
		fs           fstest.MapFS
		locale       string
		base         string
		expectErr    bool
		expectSuffix string
	}{
		{
			name:         "exact cn",
			fs:           fstest.MapFS{"welcome.cn.html": {Data: []byte("cn")}},
			locale:       "cn",
			base:         "welcome",
			expectSuffix: "welcome.cn.html",
		},
		{
			name:         "exact en",
			fs:           fstest.MapFS{"welcome.en.html": {Data: []byte("en")}, "welcome.cn.html": {Data: []byte("cn")}},
			locale:       "en",
			base:         "welcome",
			expectSuffix: "welcome.en.html",
		},
		{
			name:         "en falls back to cn (default)",
			fs:           fstest.MapFS{"welcome.cn.html": {Data: []byte("cn")}},
			locale:       "en",
			base:         "welcome",
			expectSuffix: "welcome.cn.html",
		},
		{
			name:      "neither present errors",
			fs:        fstest.MapFS{},
			locale:    "en",
			base:      "welcome",
			expectErr: true,
		},
		{
			name:         "unknown locale falls back to default",
			fs:           fstest.MapFS{"welcome.cn.html": {Data: []byte("cn")}},
			locale:       "ja",
			base:         "welcome",
			expectSuffix: "welcome.cn.html",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := TemplatePathFS(FSExister(tc.fs), tc.base, tc.locale)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error, got path %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expectSuffix {
				t.Errorf("got %q, want %q", got, tc.expectSuffix)
			}
		})
	}
}

// TestTemplatePathFS_AllRegisteredBasesResolveForEveryLocale asserts that
// the production embed.FS has every (base, locale) combination, so New()
// can never fail at runtime after a successful build.
func TestTemplatePathFS_AllRegisteredBasesResolveForEveryLocale(t *testing.T) {
	exister := EmbedExister()
	// Hard-code the locales we ship today; the registry already enforces
	// cn + en in config/locales.json, so this guards against accidental
	// deletion of a template file.
	for _, base := range Bases() {
		for _, loc := range []string{"cn", "en"} {
			path, err := TemplatePathFS(exister, base, loc)
			if err != nil {
				t.Errorf("missing template base=%s locale=%s: %v", base, loc, err)
				continue
			}
			expected := base + "." + loc + ".html"
			if path != expected {
				t.Errorf("base=%s locale=%s: expected exact-locale hit %q, got %q",
					base, loc, expected, path)
			}
		}
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
