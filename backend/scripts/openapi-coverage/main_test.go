// Tests for the small pure pieces of the openapi-coverage helper.
// Walking go/ast over a fake source tree would be overkill — the
// real validation is the script's run against the live repo (CI
// covers that path). Here we just lock in the path-normalisation
// and prefix-joining invariants so an accidental regex change
// doesn't silently break the comparison.
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormaliseSpecPath(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/auth/register", "/v1/auth/register"},
		{"/v1/cert/orders", "/v1/cert/orders"},
		{"/v1/cert/orders/{id}", "/v1/cert/orders/{id}"},
		{"/me", "/v1/me"},
		{"/v1", "/v1"},
	}
	for _, c := range cases {
		got := normaliseSpecPath(c.in)
		if got != c.want {
			t.Errorf("normaliseSpecPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestJoinPath(t *testing.T) {
	cases := []struct {
		prefix, path, want string
	}{
		{"", "/x", "/x"},
		{"/v1", "/auth", "/v1/auth"},
		{"/v1/auth", "/register", "/v1/auth/register"},
		{"/v1", "/", "/v1"},
		{"/v1/", "/users", "/v1/users"},
		{"/", "/", "/"},
		{"", "", "/"},
	}
	for _, c := range cases {
		got := joinPath(c.prefix, c.path)
		if got != c.want {
			t.Errorf("joinPath(%q, %q) = %q, want %q", c.prefix, c.path, got, c.want)
		}
	}
}

func TestShouldIgnore(t *testing.T) {
	yes := []string{
		"/health",
		"/healthz",
		"/metrics",
		"/internal/admin/users",
		"/v1/admin/beta-invitations",
		"/.well-known/acme-challenge/{token}",
		"/verify",
		"/webhooks/paymenthub",
	}
	no := []string{
		"/v1/auth/register",
		"/v1/cert/orders",
		"/v1/me",
		"/v1/billing/subscribe",
	}
	for _, p := range yes {
		if !shouldIgnore(p) {
			t.Errorf("shouldIgnore(%q) = false, want true", p)
		}
	}
	for _, p := range no {
		if shouldIgnore(p) {
			t.Errorf("shouldIgnore(%q) = true, want false", p)
		}
	}
}

func TestLoadSpec(t *testing.T) {
	dir := t.TempDir()
	spec := `openapi: 3.1.0
paths:
  /auth/register:
    post:
      operationId: authRegister
  /auth/login:
    post:
      summary: login
    get:
      summary: stub
  /v1/cert/orders:
    get:
      summary: list
    post:
      summary: create
  /me:
    parameters: []
    get:
      summary: me
components:
  schemas: {}
`
	path := filepath.Join(dir, "spec.yaml")
	if err := os.WriteFile(path, []byte(spec), 0o600); err != nil {
		t.Fatal(err)
	}
	eps, err := loadSpec(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"POST   /v1/auth/register": true,
		"POST   /v1/auth/login":    true,
		"GET    /v1/auth/login":    true,
		"GET    /v1/cert/orders":   true,
		"POST   /v1/cert/orders":   true,
		"GET    /v1/me":            true,
	}
	got := map[string]bool{}
	for _, e := range eps {
		got[e.String()] = true
	}
	for k := range want {
		if !got[k] {
			t.Errorf("missing endpoint %q in parsed spec", k)
		}
	}
	for k := range got {
		if !want[k] {
			t.Errorf("unexpected endpoint %q in parsed spec", k)
		}
	}
}

func TestExtractFromFile(t *testing.T) {
	dir := t.TempDir()
	src := `package fake

import "github.com/go-chi/chi/v5"

func setup() {
	r := chi.NewRouter()
	r.Get("/healthz", nil)
	r.Route("/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", nil)
			r.Post("/login", nil)
		})
		r.Get("/me", nil)
		r.With(nil).Get("/account/profile", nil)
		r.Handle("/cert/*", nil)
	})
}
`
	path := filepath.Join(dir, "fake.go")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	// Reset wildcard prefixes between tests — they're package-level
	// state by design (the real run aggregates across files).
	wildcardPrefixes = nil
	eps, err := extractFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, e := range eps {
		got[e.String()] = true
	}
	want := []string{
		"GET    /healthz",
		"POST   /v1/auth/register",
		"POST   /v1/auth/login",
		"GET    /v1/me",
		"GET    /v1/account/profile",
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing endpoint %q (got %v)", w, got)
		}
	}
	if len(wildcardPrefixes) == 0 {
		t.Errorf("expected wildcard prefix recorded for /v1/cert/*")
	}
}
