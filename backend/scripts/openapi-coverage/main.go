// Package main implements P0-5 OpenAPI contract coverage check.
//
// Reads docs/prd/16-api-spec.yaml + scans chi router registrations
// in backend/apps/api/internal/server/server.go (plus cert-svc and
// attest handler/cmd entrypoints), then reports drift in both
// directions:
//
//   - spec has  +  code missing  =>  handler not implemented
//     (let the PR fail; either add the handler or drop the spec entry)
//
//   - code has  +  spec missing  =>  spec out of date
//     (let the PR fail; document the new endpoint in the spec)
//
// Exit code: 0 = pass, 1 = drift found.
//
// Limitations (intentional, documented at the top of the report):
//
//   - Only chi.Router method calls are recognised (Get / Post / Put /
//     Patch / Delete / Handle / Route). chi.Mount / chi.Group with
//     no path prefix are partially handled (Group is transparent;
//     Mount is treated as a wildcard so we don't false-positive on it).
//
//   - YAML parsing is a tiny stdlib-only line scanner — it only knows
//     "top-level path keys" and "their HTTP method children". Anything
//     more elaborate would require a real YAML parser, but the spec
//     formatting has been stable since v2 and this catches the drift
//     class we care about.
//
//   - A wildcard mount like  r.Handle("/cert/*", proxy)  covers every
//     spec path under that prefix automatically; the script records
//     the wildcard and treats matching spec paths as "implemented".
//     (Real per-route check would require running the binary, which
//     this script intentionally doesn't do — that's the A-path job.)
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// endpoint is one (method, path) pair, normalised to /v1/... form.
// Both spec and code lookups feed into this representation.
type endpoint struct {
	Method string // upper-case GET / POST / ...
	Path   string // normalised, e.g. /v1/auth/register
}

func (e endpoint) String() string {
	return fmt.Sprintf("%-6s %s", e.Method, e.Path)
}

// ignorePatterns is the path-level whitelist. Paths matching ANY of
// these regexes are skipped from the comparison entirely (both spec
// and code sides). Keep this list short and well-commented: every
// entry is a known-and-accepted exception, not a "fix this later"
// dump.
var ignorePatterns = []*regexp.Regexp{
	// Operational endpoints — never part of the public contract.
	regexp.MustCompile(`^/health$`),
	regexp.MustCompile(`^/health/deep$`),
	regexp.MustCompile(`^/healthz$`),
	regexp.MustCompile(`^/readyz$`),
	regexp.MustCompile(`^/metrics$`),

	// Internal / admin surface — VPN-only, not in the public OpenAPI.
	regexp.MustCompile(`^/internal/`),
	regexp.MustCompile(`^/v1/admin/`),
	regexp.MustCompile(`^/v1/openapi\.json$`),
	regexp.MustCompile(`^/v1/csp-report$`),
	regexp.MustCompile(`^/v1/transparency$`),
	regexp.MustCompile(`^/v1/public/status/`),
	regexp.MustCompile(`^/v1/agent/enroll$`),
	regexp.MustCompile(`^/v1/leaderboard/cdn$`),
	regexp.MustCompile(`^/\.well-known/`),

	// Webhook surface — receivers, no client SDK.
	regexp.MustCompile(`^/v1/billing/webhook$`),
	regexp.MustCompile(`^/v1/billing/stub-confirm$`),
	regexp.MustCompile(`^/webhooks/`),

	// Verify endpoint on attest-server is mounted at /verify (no /v1
	// prefix in code). Spec writes it as /attest/verify under the
	// attest.idcd.com server URL — we accept this as a known mapping
	// rather than try to teach the script multi-host normalisation.
	regexp.MustCompile(`^/verify$`),
	regexp.MustCompile(`^/verify/`),
}

// wildcardPrefixes records r.Handle("/cert/*", proxy)-style mounts:
// every spec path under that prefix is treated as covered.
// Populated from scanned code; never hard-coded.
var wildcardPrefixes []string

// shouldIgnore reports whether a path is in the whitelist.
func shouldIgnore(path string) bool {
	for _, p := range ignorePatterns {
		if p.MatchString(path) {
			return true
		}
	}
	return false
}

// underWildcard reports whether the path falls under a code-side
// wildcard mount (e.g. /v1/cert/* covers /v1/cert/orders).
func underWildcard(path string) bool {
	for _, prefix := range wildcardPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// =====================================================================
// SPEC LOADER — stdlib line scanner, no yaml dep.
// =====================================================================

var (
	// Top-level path key: exactly 2-space indent, starts with '/',
	// ends with ':'. Example lines:
	//   "  /auth/register:"
	//   "  /v1/cert/orders/{id}:"
	specPathLine = regexp.MustCompile(`^  (/[^:]*):\s*$`)

	// HTTP method under a path: exactly 4-space indent + lowercase verb.
	specMethodLine = regexp.MustCompile(`^    (get|post|put|patch|delete|options|head|trace):\s*$`)
)

// loadSpec scans the OpenAPI YAML and returns (method, path) tuples.
// All paths are normalised so they always start with /v1 — matching
// the chi.Router prefix used by the api server.
func loadSpec(path string) ([]endpoint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read spec: %w", err)
	}

	var (
		out         []endpoint
		currentPath string
		inPaths     bool
	)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "paths:") {
			inPaths = true
			continue
		}
		if !inPaths {
			continue
		}
		// A top-level key without leading space (e.g. "components:")
		// ends the paths section.
		if len(line) > 0 && line[0] != ' ' && line[0] != '#' {
			break
		}
		if m := specPathLine.FindStringSubmatch(line); m != nil {
			currentPath = normaliseSpecPath(m[1])
			continue
		}
		if currentPath == "" {
			continue
		}
		if m := specMethodLine.FindStringSubmatch(line); m != nil {
			out = append(out, endpoint{
				Method: strings.ToUpper(m[1]),
				Path:   currentPath,
			})
		}
	}
	return out, nil
}

// normaliseSpecPath ensures the path starts with /v1.
// The OpenAPI spec uses server URL https://api.idcd.com/v1 with most
// paths relative to /v1, but the Cert section writes /v1/cert/...
// literally. We add /v1 only if it isn't already there.
func normaliseSpecPath(p string) string {
	if strings.HasPrefix(p, "/v1/") || p == "/v1" {
		return p
	}
	return "/v1" + p
}

// =====================================================================
// CODE LOADER — go/ast walk over the chi router setup files.
// =====================================================================

// chiMethodSet is the set of chi.Router methods we treat as
// "registers an HTTP handler on a path". Methods on this map MUST
// have signature  (path string, h http.HandlerFunc)  — chi keeps that
// shape stable across its v5 surface.
var chiMethodSet = map[string]string{
	"Get":     "GET",
	"Post":    "POST",
	"Put":     "PUT",
	"Patch":   "PATCH",
	"Delete":  "DELETE",
	"Head":    "HEAD",
	"Options": "OPTIONS",
	"Connect": "CONNECT",
	"Trace":   "TRACE",
}

// loadCode walks every file in roots and extracts chi route
// registrations. The extraction is prefix-aware: nested r.Route()
// calls accumulate path prefix correctly.
func loadCode(roots []string) ([]endpoint, error) {
	var out []endpoint
	seen := map[string]bool{}

	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			eps, err := extractFromFile(path)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			for _, ep := range eps {
				key := ep.Method + " " + ep.Path
				if seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, ep)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// extractFromFile parses one Go file and walks every function body
// looking for chi.Router method calls. Path prefixes from enclosing
// r.Route("/v1", ...) literals are tracked on a stack.
func extractFromFile(path string) ([]endpoint, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}

	var out []endpoint
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		visit(fn.Body, []string{""}, &out)
	}
	return out, nil
}

// visit walks node and records chi route registrations into out.
// prefixStack[len-1] is the active prefix; r.Route("/x", ...) pushes
// the joined prefix before walking the body.
func visit(node ast.Node, prefixStack []string, out *[]endpoint) {
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		name := sel.Sel.Name

		// r.Route(prefix, func(r chi.Router) { ... })
		if name == "Route" && len(call.Args) >= 2 {
			prefix, ok := stringLit(call.Args[0])
			if !ok {
				return true
			}
			body := funcLitBody(call.Args[1])
			if body == nil {
				return true
			}
			child := joinPath(prefixStack[len(prefixStack)-1], prefix)
			visit(body, append(prefixStack, child), out)
			// Skip descent — we already walked the body with the
			// extended prefix. Returning false tells ast.Inspect not
			// to dive in again on the parent walker.
			return false
		}

		// r.Handle(path, handler) — register raw handler; we also
		// detect wildcard patterns like "/cert/*" and stash them in
		// wildcardPrefixes so spec paths under that prefix count as
		// covered. Handle without a wildcard registers all methods at
		// that exact path; we record it as ANY-method.
		if name == "Handle" && len(call.Args) >= 1 {
			pathLit, ok := stringLit(call.Args[0])
			if !ok {
				return true
			}
			full := joinPath(prefixStack[len(prefixStack)-1], pathLit)
			if strings.HasSuffix(full, "/*") {
				wildcardPrefixes = append(wildcardPrefixes, strings.TrimSuffix(full, "/*")+"/")
				wildcardPrefixes = append(wildcardPrefixes, strings.TrimSuffix(full, "/*"))
			} else {
				*out = append(*out, endpoint{Method: "ANY", Path: full})
			}
			return true
		}

		// r.With(...).Method(path, handler) — call.Fun's receiver chain
		// is opaque, but we still see the final selector "Method".
		if method, isHTTP := chiMethodSet[name]; isHTTP {
			if len(call.Args) < 1 {
				return true
			}
			pathLit, ok := stringLit(call.Args[0])
			if !ok {
				return true
			}
			full := joinPath(prefixStack[len(prefixStack)-1], pathLit)
			*out = append(*out, endpoint{Method: method, Path: full})
		}
		return true
	})
}

// stringLit returns the string value of an *ast.BasicLit, or false
// if the expr isn't a string literal. We deliberately don't follow
// identifiers — paths in chi registrations are always inline strings
// in this codebase.
func stringLit(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s := lit.Value
	if len(s) >= 2 && (s[0] == '"' || s[0] == '`') {
		s = s[1 : len(s)-1]
	}
	return s, true
}

// funcLitBody returns the body of an *ast.FuncLit, or nil if the
// expr isn't a function literal (could be a named function reference,
// which we don't follow).
func funcLitBody(e ast.Expr) *ast.BlockStmt {
	fl, ok := e.(*ast.FuncLit)
	if !ok {
		return nil
	}
	return fl.Body
}

// joinPath concatenates a router prefix with a route path, collapsing
// duplicate slashes. "/v1" + "/auth" = "/v1/auth"; "" + "/x" = "/x".
func joinPath(prefix, path string) string {
	if path == "/" && prefix != "" {
		return prefix
	}
	combined := prefix + path
	// Collapse "//" -> "/".
	for strings.Contains(combined, "//") {
		combined = strings.ReplaceAll(combined, "//", "/")
	}
	if combined == "" {
		return "/"
	}
	return combined
}

// =====================================================================
// COMPARE + REPORT
// =====================================================================

func main() {
	var (
		specPath     string
		roots        stringSlice
		baselinePath string
		writeBase    bool
	)
	flag.StringVar(&specPath, "spec", "docs/prd/16-api-spec.yaml", "OpenAPI spec file")
	flag.Var(&roots, "root", "Code root to scan (repeatable); defaults to backend/apps/{api,attest,cert-svc}")
	flag.StringVar(&baselinePath, "baseline", "", "Path to baseline drift file (optional)")
	flag.BoolVar(&writeBase, "write-baseline", false, "Update the baseline file with current drift and exit 0")
	flag.Parse()

	if len(roots) == 0 {
		roots = []string{
			"backend/apps/api",
			"backend/apps/attest",
			"backend/apps/cert-svc",
		}
	}

	spec, err := loadSpec(specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}
	code, err := loadCode(roots)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}

	// Drop ignored paths up front so both summary counts and drift
	// lists reflect the same filter.
	specKept, specIgnored := filterIgnored(spec)
	codeKept, codeIgnored := filterIgnored(code)

	// Build lookup sets.
	specSet := map[string]bool{}
	for _, e := range specKept {
		specSet[e.String()] = true
	}
	codeSet := map[string]bool{}
	codeMethodlessPaths := map[string]bool{} // ANY-method handles cover all methods at exact path
	for _, e := range codeKept {
		codeSet[e.String()] = true
		if e.Method == "ANY" {
			codeMethodlessPaths[e.Path] = true
		}
	}

	// spec ∖ code = handler missing
	var specOnlyDrift []endpoint
	for _, e := range specKept {
		if codeSet[e.String()] {
			continue
		}
		if codeMethodlessPaths[e.Path] {
			continue
		}
		if underWildcard(e.Path) {
			continue
		}
		specOnlyDrift = append(specOnlyDrift, e)
	}

	// code ∖ spec = spec missing. We only flag method-bound entries
	// (Get/Post/...). ANY entries from r.Handle are infrastructure
	// (proxies, raw handlers); flagging them adds noise.
	var codeOnlyDrift []endpoint
	for _, e := range codeKept {
		if e.Method == "ANY" {
			continue
		}
		if specSet[e.String()] {
			continue
		}
		codeOnlyDrift = append(codeOnlyDrift, e)
	}

	sort.Slice(specOnlyDrift, func(i, j int) bool {
		return specOnlyDrift[i].String() < specOnlyDrift[j].String()
	})
	sort.Slice(codeOnlyDrift, func(i, j int) bool {
		return codeOnlyDrift[i].String() < codeOnlyDrift[j].String()
	})

	// =====================================================================
	// BASELINE — known drift that we accept (snapshot of current debt).
	// On every run we diff the new drift against the baseline:
	//   - new drift not in baseline => fail
	//   - baseline entry no longer present => warn (someone fixed it; bump baseline)
	// `-write-baseline` overwrites the file with the current drift and
	// exits 0 (use this after intentionally accepting more debt or after
	// reducing drift in a follow-up PR).
	// =====================================================================
	allDrift := make([]endpoint, 0, len(specOnlyDrift)+len(codeOnlyDrift))
	for _, e := range specOnlyDrift {
		allDrift = append(allDrift, endpoint{Method: "SPEC-ONLY:" + e.Method, Path: e.Path})
	}
	for _, e := range codeOnlyDrift {
		allDrift = append(allDrift, endpoint{Method: "CODE-ONLY:" + e.Method, Path: e.Path})
	}

	if writeBase {
		if baselinePath == "" {
			fmt.Fprintln(os.Stderr, "ERROR: -write-baseline requires -baseline")
			os.Exit(2)
		}
		if err := saveBaseline(baselinePath, allDrift); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(2)
		}
		fmt.Printf("baseline 已写入 %s (%d 条 drift)\n", baselinePath, len(allDrift))
		os.Exit(0)
	}

	baseline := map[string]bool{}
	if baselinePath != "" {
		entries, err := loadBaseline(baselinePath)
		if err != nil {
			// Missing baseline is OK; broken baseline is not.
			if !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
				os.Exit(2)
			}
		}
		for _, e := range entries {
			baseline[e.String()] = true
		}
	}

	var newDrift []endpoint
	seenNow := map[string]bool{}
	for _, e := range allDrift {
		seenNow[e.String()] = true
		if !baseline[e.String()] {
			newDrift = append(newDrift, e)
		}
	}
	var fixedDrift []string
	for key := range baseline {
		if !seenNow[key] {
			fixedDrift = append(fixedDrift, key)
		}
	}
	sort.Strings(fixedDrift)

	// =====================================================================
	// REPORT
	// =====================================================================
	fmt.Println("P0-5 OpenAPI 契约覆盖检查")
	fmt.Println("=========================")
	fmt.Println()
	fmt.Printf("spec 中的端点: %d (忽略 %d)\n", len(specKept), specIgnored)
	fmt.Printf("代码中的端点: %d (忽略 %d)\n", len(codeKept), codeIgnored)
	if len(wildcardPrefixes) > 0 {
		unique := map[string]bool{}
		var pretty []string
		for _, p := range wildcardPrefixes {
			if !strings.HasSuffix(p, "/") {
				continue // dedupe the helper variant
			}
			if unique[p] {
				continue
			}
			unique[p] = true
			pretty = append(pretty, p+"*")
		}
		sort.Strings(pretty)
		fmt.Printf("代码中的 wildcard mount: %s\n", strings.Join(pretty, ", "))
	}
	fmt.Println()

	if len(specOnlyDrift) == 0 && len(codeOnlyDrift) == 0 {
		fmt.Println("OK — spec 与代码完全匹配")
		os.Exit(0)
	}

	fmt.Printf("当前总 drift: %d (spec-only %d / code-only %d)\n",
		len(allDrift), len(specOnlyDrift), len(codeOnlyDrift))
	if baselinePath != "" {
		fmt.Printf("baseline 接受的 drift: %d\n", len(baseline))
		fmt.Printf("baseline 已修复 (可缩 baseline): %d\n", len(fixedDrift))
	}
	fmt.Println()

	if len(newDrift) == 0 {
		// All current drift is already in the baseline.
		if len(fixedDrift) > 0 {
			fmt.Println("INFO: 以下 baseline 条目已经不再 drift, 建议跑 -write-baseline 收紧:")
			for _, k := range fixedDrift {
				fmt.Printf("  - %s\n", k)
			}
			fmt.Println()
		}
		fmt.Println("OK — 全部 drift 已在 baseline 内 (没有新增 drift)")
		fmt.Println("提示: drift 总量降低后跑 `bash backend/scripts/check-openapi-coverage.sh --write-baseline` 收紧")
		os.Exit(0)
	}

	// New drift introduced by this change — fail loudly.
	var newSpecOnly, newCodeOnly []endpoint
	for _, e := range newDrift {
		switch {
		case strings.HasPrefix(e.Method, "SPEC-ONLY:"):
			newSpecOnly = append(newSpecOnly, endpoint{
				Method: strings.TrimPrefix(e.Method, "SPEC-ONLY:"),
				Path:   e.Path,
			})
		case strings.HasPrefix(e.Method, "CODE-ONLY:"):
			newCodeOnly = append(newCodeOnly, endpoint{
				Method: strings.TrimPrefix(e.Method, "CODE-ONLY:"),
				Path:   e.Path,
			})
		}
	}

	if len(newSpecOnly) > 0 {
		fmt.Printf("ERROR: 新增 spec 有 / 代码缺 (%d):\n", len(newSpecOnly))
		for _, e := range newSpecOnly {
			fmt.Printf("  - %s\n", e)
		}
		fmt.Println()
		fmt.Println("处理方式: 补 handler, 或从 spec 删除该端点 (单独 PR)")
		fmt.Println()
	}

	if len(newCodeOnly) > 0 {
		fmt.Printf("ERROR: 新增 代码有 / spec 缺 (%d):\n", len(newCodeOnly))
		for _, e := range newCodeOnly {
			fmt.Printf("  - %s\n", e)
		}
		fmt.Println()
		fmt.Println("处理方式: 把端点加入 spec, 或加白名单 (operational/internal)")
		fmt.Println()
	}

	if baselinePath != "" {
		fmt.Println("如果上面的新增 drift 是你预期的, 跑 -write-baseline 接受")
	}
	os.Exit(1)
}

// loadBaseline reads accepted drift entries from a file. Empty lines
// and lines starting with '#' are comments. Each non-comment line is
// "DIRECTION:METHOD  /path" (the same format Endpoint.String emits).
func loadBaseline(path string) ([]endpoint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []endpoint
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Split on whitespace, expect "DIR:METHOD /path"
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("malformed baseline line: %q", line)
		}
		out = append(out, endpoint{Method: fields[0], Path: fields[1]})
	}
	return out, nil
}

// saveBaseline writes the current drift to a baseline file with a
// header comment explaining how to read / regenerate it.
func saveBaseline(path string, drift []endpoint) error {
	var b strings.Builder
	b.WriteString("# OpenAPI coverage baseline — accepted drift between\n")
	b.WriteString("# docs/prd/16-api-spec.yaml and backend/apps/{api,attest}\n")
	b.WriteString("# chi router registrations. Format:\n")
	b.WriteString("#\n")
	b.WriteString("#   SPEC-ONLY:METHOD  /v1/path   (spec writes it, code missing)\n")
	b.WriteString("#   CODE-ONLY:METHOD  /v1/path   (code registers it, spec missing)\n")
	b.WriteString("#\n")
	b.WriteString("# Regenerate after intentional drift change:\n")
	b.WriteString("#   bash backend/scripts/check-openapi-coverage.sh --write-baseline\n")
	b.WriteString("#\n")
	b.WriteString("# TODO(P0-5): drive this list to zero. Each entry below is\n")
	b.WriteString("# documented technical debt against the OpenAPI contract.\n")
	b.WriteString("\n")
	for _, e := range drift {
		fmt.Fprintf(&b, "%s %s\n", e.Method, e.Path)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// filterIgnored splits eps into (kept, ignored-count).
func filterIgnored(eps []endpoint) ([]endpoint, int) {
	var kept []endpoint
	ignored := 0
	for _, e := range eps {
		if shouldIgnore(e.Path) {
			ignored++
			continue
		}
		kept = append(kept, e)
	}
	return kept, ignored
}

// stringSlice is a flag.Value that collects repeated -root flags.
type stringSlice []string

func (s *stringSlice) String() string     { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error { *s = append(*s, v); return nil }
