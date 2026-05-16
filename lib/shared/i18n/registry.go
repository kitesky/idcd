// Package i18n provides the shared locale registry and message catalog used by
// every Go service in the idcd monorepo. The registry is loaded once at process
// start from config/locales.json, the single source of truth shared with the
// web frontend.
package i18n

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// LocaleEntry mirrors a single record in config/locales.json.
type LocaleEntry struct {
	Code                  string   `json:"code"`
	BCP47                 string   `json:"bcp47"`
	Label                 string   `json:"label"`
	NativeLabel           string   `json:"nativeLabel"`
	BaseLanguage          string   `json:"baseLanguage"`
	AcceptLanguageAliases []string `json:"acceptLanguageAliases"`
	Dir                   string   `json:"dir"`
	FontStack             string   `json:"fontStack"`
	Fallback              []string `json:"fallback"`
}

// Registry is the in-memory representation of config/locales.json.
type Registry struct {
	Default string        `json:"default"`
	Locales []LocaleEntry `json:"locales"`

	byCode map[string]LocaleEntry
}

var (
	defaultRegistry     *Registry
	defaultRegistryOnce sync.Once
	defaultRegistryErr  error
)

// LoadFromFile reads and validates a registry JSON file from the given path.
func LoadFromFile(path string) (*Registry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("i18n: read %s: %w", path, err)
	}
	return loadFromBytes(raw)
}

// LoadFromBytesForTesting exposes the otherwise-internal byte loader so
// downstream tests (e.g. apps/api/internal/i18n) can build hermetic
// registries without writing JSON to disk. Not meant for production use —
// the name keeps callers honest.
func LoadFromBytesForTesting(raw []byte) (*Registry, error) {
	return loadFromBytes(raw)
}

func loadFromBytes(raw []byte) (*Registry, error) {
	var r Registry
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("i18n: unmarshal locales.json: %w", err)
	}
	if err := r.validate(); err != nil {
		return nil, err
	}
	r.byCode = make(map[string]LocaleEntry, len(r.Locales))
	for _, l := range r.Locales {
		r.byCode[l.Code] = l
	}
	return &r, nil
}

// MustDefault loads the registry from the conventional config/locales.json
// location, searching upward from the current working directory and from
// $IDCD_REGISTRY_PATH. Subsequent calls return the cached value.
func MustDefault() *Registry {
	defaultRegistryOnce.Do(func() {
		path, err := locateDefaultRegistry()
		if err != nil {
			defaultRegistryErr = err
			return
		}
		defaultRegistry, defaultRegistryErr = LoadFromFile(path)
	})
	if defaultRegistryErr != nil {
		panic(defaultRegistryErr)
	}
	return defaultRegistry
}

func locateDefaultRegistry() (string, error) {
	if envPath := os.Getenv("IDCD_REGISTRY_PATH"); envPath != "" {
		return envPath, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for range 8 {
		candidate := filepath.Join(dir, "config", "locales.json")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("i18n: config/locales.json not found in %s or 8 ancestors; set IDCD_REGISTRY_PATH", cwd)
}

func (r *Registry) validate() error {
	if r.Default == "" {
		return fmt.Errorf("i18n: default locale missing")
	}
	if len(r.Locales) == 0 {
		return fmt.Errorf("i18n: locales array empty")
	}
	defaultSeen := false
	for _, l := range r.Locales {
		if l.Code == "" || l.BCP47 == "" || l.BaseLanguage == "" || l.Dir == "" {
			return fmt.Errorf("i18n: locale entry %+v missing required field", l)
		}
		if l.Code == r.Default {
			defaultSeen = true
		}
	}
	if !defaultSeen {
		return fmt.Errorf("i18n: default %q not present in locales[]", r.Default)
	}
	return nil
}

// DefaultCode returns the configured default locale code.
func (r *Registry) DefaultCode() string { return r.Default }

// Codes returns every supported locale code.
func (r *Registry) Codes() []string {
	out := make([]string, len(r.Locales))
	for i, l := range r.Locales {
		out[i] = l.Code
	}
	return out
}

// All returns every locale entry.
func (r *Registry) All() []LocaleEntry { return r.Locales }

// IsSupported reports whether the given code is in the registry.
func (r *Registry) IsSupported(code string) bool {
	_, ok := r.byCode[code]
	return ok
}

// Entry returns the locale entry for a code, or an error if unknown.
func (r *Registry) Entry(code string) (LocaleEntry, error) {
	if entry, ok := r.byCode[code]; ok {
		return entry, nil
	}
	return LocaleEntry{}, fmt.Errorf("i18n: unknown locale %q", code)
}

// BCP47Of returns the BCP 47 standard tag for the given locale code, falling
// back to the default locale's tag for unknown codes (never panics).
func (r *Registry) BCP47Of(code string) string {
	if e, ok := r.byCode[code]; ok {
		return e.BCP47
	}
	return r.byCode[r.Default].BCP47
}

// FallbackChain returns the lookup order for a code: itself, explicit
// fallback entries, any locale sharing the same baseLanguage, then default.
// Used by the message catalog when a key is missing in the requested locale.
func (r *Registry) FallbackChain(code string) []string {
	out := []string{}
	seen := map[string]bool{}
	add := func(c string) {
		if c == "" || seen[c] {
			return
		}
		if _, ok := r.byCode[c]; !ok {
			return
		}
		seen[c] = true
		out = append(out, c)
	}
	if entry, ok := r.byCode[code]; ok {
		add(entry.Code)
		for _, f := range entry.Fallback {
			add(f)
		}
		for _, l := range r.Locales {
			if l.Code != entry.Code && l.BaseLanguage == entry.BaseLanguage {
				add(l.Code)
			}
		}
	}
	add(r.Default)
	return out
}

// Negotiate parses an Accept-Language header and returns the best-matching
// supported locale code, defaulting to the registry default when no acceptable
// match is found. Implements a subset of RFC 4647 best-match lookup.
func (r *Registry) Negotiate(header string) string {
	if header == "" {
		return r.Default
	}
	type rankedTag struct {
		tag string
		q   float64
	}
	var tags []rankedTag
	for piece := range strings.SplitSeq(header, ",") {
		piece = strings.TrimSpace(piece)
		if piece == "" {
			continue
		}
		parts := strings.Split(piece, ";")
		tag := strings.TrimSpace(parts[0])
		if tag == "" || tag == "*" {
			continue
		}
		q := 1.0
		for _, p := range parts[1:] {
			kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
			if len(kv) == 2 && kv[0] == "q" {
				var parsed float64
				if _, err := fmt.Sscanf(kv[1], "%f", &parsed); err == nil {
					q = parsed
				}
			}
		}
		tags = append(tags, rankedTag{tag: tag, q: q})
	}
	sort.SliceStable(tags, func(i, j int) bool { return tags[i].q > tags[j].q })
	for _, t := range tags {
		req := strings.ToLower(t.tag)
		for _, locale := range r.Locales {
			for _, alias := range locale.AcceptLanguageAliases {
				if tagMatches(req, strings.ToLower(alias)) {
					return locale.Code
				}
			}
		}
	}
	return r.Default
}

func tagMatches(req, alias string) bool {
	if req == alias {
		return true
	}
	if strings.HasPrefix(req, alias+"-") || strings.HasPrefix(alias, req+"-") {
		return true
	}
	return false
}
