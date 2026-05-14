// Package acme provides ACME/Let's Encrypt certificate management for custom domains.
//
// Architecture overview:
//   - Manager wraps golang.org/x/crypto/acme/autocert to obtain and renew TLS
//     certificates automatically via the ACME protocol (Let's Encrypt).
//   - DirCache persists certificates to a local directory (e.g. /var/cache/idcd-acme).
//   - HostPolicy guards which hostnames may be issued certificates — only status_pages
//     rows with custom_domain_verified_at IS NOT NULL are accepted.
//   - GetCertificate is wired into tls.Config.GetCertificate so the Go TLS stack
//     transparently serves the correct certificate per SNI host.
//
// Production setup (not required for tests):
//   - The Manager must be reachable on :443 for the TLS-ALPN-01 challenge, or
//     :80 for the HTTP-01 challenge (via Manager.HTTPHandler).
//   - DirCache path must be writable and persistent across restarts.
package acme

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/crypto/acme/autocert"
)

// DomainChecker is the interface the Manager uses to verify that a hostname is
// a verified custom domain in the database.
type DomainChecker interface {
	// IsVerifiedDomain reports whether the given hostname corresponds to a
	// status_pages row where custom_domain_verified_at IS NOT NULL.
	IsVerifiedDomain(ctx context.Context, host string) (bool, error)
}

// Config holds configuration for the ACME manager.
type Config struct {
	// CacheDir is the filesystem path where autocert stores certificates.
	// Defaults to "./.acme-cache" when empty.
	CacheDir string

	// Email is the contact address registered with Let's Encrypt.
	// Required for production ACME; may be empty in tests.
	Email string
}

// Manager wraps autocert.Manager with an application-layer HostPolicy that
// consults the database before allowing certificate issuance.
type Manager struct {
	autocert *autocert.Manager
	checker  DomainChecker
}

// New creates a Manager and configures the autocert.Manager.
//
// checker must not be nil — it is called for every TLS handshake on an
// unknown hostname.  Pass a lightweight implementation that hits a read
// replica or an in-process cache.
func New(cfg Config, checker DomainChecker) *Manager {
	if checker == nil {
		panic("acme.New: checker must not be nil")
	}

	cacheDir := cfg.CacheDir
	if cacheDir == "" {
		cacheDir = ".acme-cache"
	}

	m := &Manager{checker: checker}

	m.autocert = &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: m.HostPolicy,
		Cache:      autocert.DirCache(cacheDir),
		Email:      cfg.Email,
	}

	return m
}

// HostPolicy is wired into autocert.Manager.HostPolicy.
// It returns nil when host is a verified custom domain in the DB, or a
// descriptive error otherwise.
func (m *Manager) HostPolicy(ctx context.Context, host string) error {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return errors.New("acme: empty host")
	}

	ok, err := m.checker.IsVerifiedDomain(ctx, host)
	if err != nil {
		return fmt.Errorf("acme: host policy DB check failed for %q: %w", host, err)
	}
	if !ok {
		return fmt.Errorf("acme: host %q is not a verified custom domain", host)
	}
	return nil
}

// GetCertificate implements the tls.Config.GetCertificate callback.
// Wire this into your TLS listener:
//
//	tlsCfg := &tls.Config{GetCertificate: mgr.GetCertificate}
func (m *Manager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	return m.autocert.GetCertificate(hello)
}

// HTTPHandler returns an http.Handler that answers ACME HTTP-01 challenges.
// Mount this on port :80:
//
//	http.ListenAndServe(":80", mgr.HTTPHandler(nil))
func (m *Manager) HTTPHandler(fallback http.Handler) http.Handler {
	return m.autocert.HTTPHandler(fallback)
}
