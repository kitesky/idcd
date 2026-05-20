package template

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/kite365/idcd/lib/shared/i18n"
)

// TemplateExister abstracts the "does this file exist?" check used during
// fallback-chain resolution. The production code paths back it with the
// on-disk filesystem (TemplatePath) or the package-level embed.FS
// (TemplatePathFS). Tests can inject an fstest.MapFS or any other fs.StatFS.
type TemplateExister interface {
	Exists(name string) bool
}

// fsExister adapts any fs.StatFS into a TemplateExister.
type fsExister struct{ fsys fs.StatFS }

func (f fsExister) Exists(name string) bool {
	if f.fsys == nil {
		return false
	}
	if _, err := f.fsys.Stat(name); err != nil {
		return false
	}
	return true
}

// osExister checks an on-disk directory.
type osExister struct{ root string }

func (o osExister) Exists(name string) bool {
	_, err := os.Stat(filepath.Join(o.root, name))
	return err == nil
}

// embedExister checks the package-level embed.FS. Implemented as a struct
// (not a function literal) so callers can pass embedExister{} without
// allocating a closure.
type embedExister struct{}

func (embedExister) Exists(name string) bool {
	f, err := embedFS.Open(name)
	if err != nil {
		// Both fs.ErrNotExist and other open errors mean "treat as missing".
		_ = errors.Is(err, fs.ErrNotExist)
		return false
	}
	_ = f.Close()
	return true
}

// TemplatePath returns the localized template path for the given base name
// and locale, following the registry fallback chain. Returns an error only
// when no candidate exists for any locale in the chain.
//
// The returned path uses the OS path separator so callers can pass it
// directly to os.ReadFile / template.ParseFiles. Selection walks
// registry.FallbackChain(locale) — there is deliberately no `if locale ==
// "en"` style branching.
func TemplatePath(dir, base, locale string) (string, error) {
	for _, loc := range i18n.MustDefault().FallbackChain(locale) {
		candidate := fmt.Sprintf("%s.%s.html", base, loc)
		if (osExister{root: dir}).Exists(candidate) {
			return filepath.Join(dir, candidate), nil
		}
	}
	return "", fmt.Errorf("template: no candidate for base=%q in registry fallback chain", base)
}

// TemplatePathFS is the embed.FS / fs.StatFS variant. It returns the slash-
// separated path the supplied filesystem understands. Pass embedExister{}
// for the package-level embed; pass an fsExister wrapping fstest.MapFS in
// tests.
func TemplatePathFS(exister TemplateExister, base, locale string) (string, error) {
	for _, loc := range i18n.MustDefault().FallbackChain(locale) {
		candidate := fmt.Sprintf("%s.%s.html", base, loc)
		if exister.Exists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("template: no candidate for base=%q in registry fallback chain", base)
}

// FSExister wraps an fs.StatFS in the TemplateExister contract. Exposed so
// tests can wrap fstest.MapFS without touching internal types.
func FSExister(fsys fs.StatFS) TemplateExister {
	return fsExister{fsys: fsys}
}

// EmbedExister returns a TemplateExister backed by the package-level embed.FS.
// Production callers use this when they want to drive TemplatePathFS against
// the same files that templates.go pre-parses.
func EmbedExister() TemplateExister { return embedExister{} }
