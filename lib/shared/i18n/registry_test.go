package i18n

import (
	"path/filepath"
	"runtime"
	"testing"
)

func loadRegistry(t *testing.T) *Registry {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	path := filepath.Join(root, "config", "locales.json")
	r, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	return r
}

func TestRegistryBasics(t *testing.T) {
	r := loadRegistry(t)

	if r.DefaultCode() == "" {
		t.Fatal("default code empty")
	}
	if !r.IsSupported(r.DefaultCode()) {
		t.Fatalf("default %q not in registry", r.DefaultCode())
	}
	if !r.IsSupported("cn") {
		t.Error("cn should be supported")
	}
	if !r.IsSupported("en") {
		t.Error("en should be supported")
	}
	if r.IsSupported("xx") {
		t.Error("xx should not be supported")
	}

	cn, err := r.Entry("cn")
	if err != nil {
		t.Fatalf("entry cn: %v", err)
	}
	if cn.BCP47 != "zh-CN" {
		t.Errorf("cn.BCP47 = %q want zh-CN", cn.BCP47)
	}
}

func TestBCP47Of(t *testing.T) {
	r := loadRegistry(t)
	cases := []struct{ in, want string }{
		{"cn", "zh-CN"},
		{"en", "en-US"},
		{"unknown", r.BCP47Of(r.DefaultCode())},
	}
	for _, c := range cases {
		if got := r.BCP47Of(c.in); got != c.want {
			t.Errorf("BCP47Of(%q) = %q want %q", c.in, got, c.want)
		}
	}
}

func TestFallbackChain(t *testing.T) {
	r := loadRegistry(t)
	chain := r.FallbackChain("en")
	if len(chain) == 0 || chain[0] != "en" {
		t.Errorf("en chain starts with %v", chain)
	}
	last := chain[len(chain)-1]
	if last != r.DefaultCode() {
		t.Errorf("chain should end at default; got %v", chain)
	}

	seen := map[string]bool{}
	for _, c := range chain {
		if seen[c] {
			t.Errorf("duplicate %q in chain %v", c, chain)
		}
		seen[c] = true
	}

	unknown := r.FallbackChain("ja")
	if len(unknown) == 0 || unknown[len(unknown)-1] != r.DefaultCode() {
		t.Errorf("unknown locale chain should still terminate at default; got %v", unknown)
	}
}

func TestNegotiate(t *testing.T) {
	r := loadRegistry(t)
	cases := []struct {
		header string
		want   string
	}{
		{"", r.DefaultCode()},
		{"en-US,en;q=0.9", "en"},
		{"zh-CN,zh;q=0.9,en;q=0.8", "cn"},
		{"zh-Hans-HK;q=0.9", "cn"},
		{"de-DE,en;q=0.5", "en"},
		{"de-DE,fr;q=0.5", r.DefaultCode()},
		{"*", r.DefaultCode()},
	}
	for _, c := range cases {
		if got := r.Negotiate(c.header); got != c.want {
			t.Errorf("Negotiate(%q) = %q want %q", c.header, got, c.want)
		}
	}
}
