package i18n

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Catalog is an in-memory message catalog. Messages are addressed by a
// "namespace.key" string (e.g. "errors.AUTH_REQUIRED") so each JSON file in a
// locale directory contributes one namespace (the file name without the
// .json suffix).
//
// Catalog is safe for concurrent use after Load returns.
type Catalog struct {
	registry *Registry
	messages map[string]map[string]string // locale -> namespace.key -> message

	logOnce sync.Map // dedupe one-shot warnings keyed by message
}

// Load reads every locale directory under dir and assembles a Catalog.
//
// Layout expected:
//
//	dir/
//	  cn/
//	    errors.json   -> contributes "errors.<key>"
//	    common.json   -> contributes "common.<key>"
//	  en/
//	    ...
//
// Locales not present in reg are skipped (with a log warning) so registry
// stays the single source of truth for "what is supported".
func Load(dir string, reg *Registry) (*Catalog, error) {
	if reg == nil {
		return nil, fmt.Errorf("i18n: Load requires non-nil registry")
	}
	if dir == "" {
		return nil, fmt.Errorf("i18n: Load requires non-empty dir")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("i18n: read dir %s: %w", dir, err)
	}

	c := &Catalog{
		registry: reg,
		messages: map[string]map[string]string{},
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		locale := e.Name()
		if !reg.IsSupported(locale) {
			slog.Warn("i18n: skipping message dir for unsupported locale", "locale", locale, "dir", dir)
			continue
		}
		ns, err := loadLocaleDir(filepath.Join(dir, locale))
		if err != nil {
			return nil, err
		}
		c.messages[locale] = ns
	}

	// Ensure every supported locale has at least an empty map so T() can
	// safely look it up without nil-map traps.
	for _, code := range reg.Codes() {
		if _, ok := c.messages[code]; !ok {
			c.messages[code] = map[string]string{}
		}
	}

	return c, nil
}

// loadLocaleDir reads every *.json file in a single locale directory and
// returns the flat namespace.key map.
func loadLocaleDir(dir string) (map[string]string, error) {
	out := map[string]string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("i18n: read locale dir %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		namespace := strings.TrimSuffix(name, ".json")
		full := filepath.Join(dir, name)
		raw, err := os.ReadFile(full)
		if err != nil {
			return nil, fmt.Errorf("i18n: read %s: %w", full, err)
		}
		var data map[string]any
		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, fmt.Errorf("i18n: unmarshal %s: %w", full, err)
		}
		flatten(namespace, data, out)
	}
	return out, nil
}

// flatten walks a possibly-nested JSON map and writes "a.b.c" -> "string"
// entries into out. Non-string leaves are ignored with a log warning so a
// stray number doesn't crash the loader.
func flatten(prefix string, in map[string]any, out map[string]string) {
	for k, v := range in {
		key := prefix + "." + k
		switch val := v.(type) {
		case string:
			out[key] = val
		case map[string]any:
			flatten(key, val, out)
		default:
			slog.Warn("i18n: ignoring non-string message", "key", key, "type", fmt.Sprintf("%T", v))
		}
	}
}

// T translates key into the requested locale, walking the registry fallback
// chain. params are first used to drive ICU plural selection (if the message
// uses {var, plural, ...} syntax), then injected as {name} placeholders.
//
// When no locale in the fallback chain has the key, T returns the key itself
// so the missing string is visible in the UI / logs (and easy to grep).
func (c *Catalog) T(locale, key string, params map[string]any) string {
	if c == nil {
		return key
	}
	for _, loc := range c.registry.FallbackChain(locale) {
		if msg, ok := c.messages[loc][key]; ok {
			return c.render(msg, loc, params)
		}
	}
	return key
}

// Has reports whether the key is defined for the locale's fallback chain.
// Useful for unit tests and callers that need to decide between rendering a
// translated string vs falling back to a different code path.
func (c *Catalog) Has(locale, key string) bool {
	for _, loc := range c.registry.FallbackChain(locale) {
		if _, ok := c.messages[loc][key]; ok {
			return true
		}
	}
	return false
}

// Registry returns the registry the catalog was constructed with.
func (c *Catalog) Registry() *Registry { return c.registry }

// Interpolate replaces {name} placeholders in msg with params[name]'s string
// representation. Missing keys leave the placeholder intact so unrendered
// templates are visible. Exported for tests and callers that want to render
// a literal pre-fetched message.
func Interpolate(msg string, params map[string]any) string {
	if len(params) == 0 || !strings.ContainsRune(msg, '{') {
		return msg
	}
	var b strings.Builder
	b.Grow(len(msg))
	i := 0
	for i < len(msg) {
		ch := msg[i]
		if ch != '{' {
			b.WriteByte(ch)
			i++
			continue
		}
		// Look for a closing '}'. If none found, write the rest verbatim.
		end := strings.IndexByte(msg[i:], '}')
		if end < 0 {
			b.WriteString(msg[i:])
			break
		}
		token := msg[i+1 : i+end]
		// Reject tokens that look like ICU sub-syntax (contain comma) — those
		// belong to render(), not Interpolate(). Leave them as-is.
		if strings.ContainsRune(token, ',') {
			b.WriteString(msg[i : i+end+1])
			i += end + 1
			continue
		}
		token = strings.TrimSpace(token)
		if v, ok := params[token]; ok {
			b.WriteString(stringify(v))
		} else {
			b.WriteString(msg[i : i+end+1])
		}
		i += end + 1
	}
	return b.String()
}

// render handles ICU plural syntax (the only ICU sub-DSL we support) and
// then Interpolate-substitutes the remaining {var} placeholders. Unsupported
// ICU forms (select / selectordinal / ordinal) trigger a one-shot warning
// and are returned verbatim.
func (c *Catalog) render(msg, locale string, params map[string]any) string {
	out := c.expandPlurals(msg, locale, params)
	return Interpolate(out, params)
}

// expandPlurals scans msg for {var, plural, one {…} other {…}} blocks and
// expands each to the chosen form. Multiple plural blocks in one message
// are supported. Unsupported ICU sub-syntax is left intact.
func (c *Catalog) expandPlurals(msg, locale string, params map[string]any) string {
	if !strings.Contains(msg, ", plural") && !strings.Contains(msg, ",plural") {
		// Cheap path: also detect select/selectordinal so we can warn once.
		c.warnUnsupportedICU(msg)
		return msg
	}
	var b strings.Builder
	b.Grow(len(msg))
	i := 0
	for i < len(msg) {
		if msg[i] != '{' {
			b.WriteByte(msg[i])
			i++
			continue
		}
		block, consumed, ok := extractBalanced(msg[i:])
		if !ok {
			// Unbalanced — bail out, write rest as-is.
			b.WriteString(msg[i:])
			break
		}
		// block includes the outer braces; inspect inner content.
		inner := block[1 : len(block)-1]
		varName, kind, body, parsed := parseICUHeader(inner)
		switch {
		case !parsed:
			// Not an ICU construct — likely a plain {var} placeholder.
			// Interpolate() will handle it later; emit verbatim here.
			b.WriteString(block)
		case kind == "plural":
			b.WriteString(c.renderPlural(varName, body, locale, params))
		case kind == "select" || kind == "selectordinal" || kind == "ordinal":
			c.warnOnce("i18n: unsupported ICU form %q, returning raw", kind)
			b.WriteString(block)
		default:
			b.WriteString(block)
		}
		i += consumed
	}
	return b.String()
}

func (c *Catalog) warnUnsupportedICU(msg string) {
	for _, form := range []string{", select", ",select", ", selectordinal", ",selectordinal"} {
		if strings.Contains(msg, form) {
			c.warnOnce("i18n: unsupported ICU form detected in message %q", msg)
			return
		}
	}
}

func (c *Catalog) warnOnce(format string, args ...any) {
	if c == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if _, loaded := c.logOnce.LoadOrStore(msg, struct{}{}); loaded {
		return
	}
	slog.Warn(msg)
}

// extractBalanced reads a balanced {...} run starting at s[0]=='{' and
// returns (block, consumed, ok). Used to peel one ICU construct (or one
// {var}) at a time.
func extractBalanced(s string) (string, int, bool) {
	if len(s) == 0 || s[0] != '{' {
		return "", 0, false
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[:i+1], i + 1, true
			}
		}
	}
	return "", 0, false
}

// parseICUHeader inspects the inside of an outer {...} block. For an ICU
// construct we expect "varName, plural, <body>". For a bare {varName} the
// returned parsed=false signals "this is just a placeholder".
func parseICUHeader(inner string) (varName, kind, body string, parsed bool) {
	// Find first comma.
	idx := strings.IndexByte(inner, ',')
	if idx < 0 {
		return "", "", "", false
	}
	varName = strings.TrimSpace(inner[:idx])
	rest := strings.TrimSpace(inner[idx+1:])
	idx2 := strings.IndexByte(rest, ',')
	if idx2 < 0 {
		// e.g. "{count, plural}" with no body is malformed; treat as not-parsed.
		return "", "", "", false
	}
	kind = strings.TrimSpace(rest[:idx2])
	body = strings.TrimSpace(rest[idx2+1:])
	return varName, kind, body, true
}

// renderPlural picks the right form from a plural body like:
//
//	"=0 {no items} one {# item} other {# items}"
//
// Selection rules (deliberately minimal):
//   - Exact match: "=N {...}" wins over keyword forms.
//   - English-style "one": count == 1 (used when locale base language is en).
//   - All other locales (including cn) use "other".
//
// Returns the chosen form with "#" replaced by the count and inner {var}
// placeholders left for Interpolate to handle.
func (c *Catalog) renderPlural(varName, body, locale string, params map[string]any) string {
	count, hasCount := readInt(params[varName])
	forms := parsePluralForms(body)
	if hasCount {
		if form, ok := forms["="+strconv.FormatInt(count, 10)]; ok {
			return strings.ReplaceAll(form, "#", strconv.FormatInt(count, 10))
		}
	}
	pluralForm := c.pluralKey(locale, count, hasCount)
	if form, ok := forms[pluralForm]; ok {
		return strings.ReplaceAll(form, "#", stringifyCount(count, hasCount))
	}
	if form, ok := forms["other"]; ok {
		return strings.ReplaceAll(form, "#", stringifyCount(count, hasCount))
	}
	// No usable form — return literal placeholder so missing data is visible.
	return "{" + varName + "}"
}

// pluralKey returns the CLDR-ish plural category for the given locale +
// count. We only need a one/other split:
//   - English base language: count == 1 → "one", else "other"
//   - Everything else (cn, ja, etc.): always "other"
//
// Future locales with richer plural systems should extend this switch
// (or call out to golang.org/x/text/feature/plural).
func (c *Catalog) pluralKey(locale string, count int64, hasCount bool) string {
	if !hasCount {
		return "other"
	}
	if c == nil || c.registry == nil {
		return "other"
	}
	entry, err := c.registry.Entry(locale)
	if err != nil {
		return "other"
	}
	switch entry.BaseLanguage {
	case "en":
		if count == 1 {
			return "one"
		}
		return "other"
	default:
		return "other"
	}
}

// parsePluralForms breaks a plural body into form-name -> body map.
// Input example: "=0 {no items} one {# item} other {# items}".
func parsePluralForms(body string) map[string]string {
	out := map[string]string{}
	i := 0
	for i < len(body) {
		// Skip whitespace
		for i < len(body) && (body[i] == ' ' || body[i] == '\t' || body[i] == '\n') {
			i++
		}
		if i >= len(body) {
			break
		}
		// Read form name until whitespace or '{'.
		start := i
		for i < len(body) && body[i] != '{' && body[i] != ' ' && body[i] != '\t' && body[i] != '\n' {
			i++
		}
		name := body[start:i]
		// Skip whitespace before brace
		for i < len(body) && (body[i] == ' ' || body[i] == '\t' || body[i] == '\n') {
			i++
		}
		if i >= len(body) || body[i] != '{' {
			break
		}
		block, consumed, ok := extractBalanced(body[i:])
		if !ok {
			break
		}
		out[name] = block[1 : len(block)-1]
		i += consumed
	}
	return out
}

// readInt coerces JSON-flavored numbers to int64. Returns ok=false when the
// value is missing or non-numeric.
func readInt(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case uint:
		return int64(n), true
	case uint32:
		return int64(n), true
	case uint64:
		return int64(n), true
	case float32:
		return int64(n), true
	case float64:
		return int64(n), true
	case string:
		if parsed, err := strconv.ParseInt(n, 10, 64); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func stringifyCount(count int64, has bool) string {
	if !has {
		return "#"
	}
	return strconv.FormatInt(count, 10)
}

func stringify(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}
