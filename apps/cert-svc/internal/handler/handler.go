// Package handler wires the cert-svc HTTP routes and implements the
// business logic for /v1/cert/* endpoints (orders, DNS credentials,
// certs). Auth lives in the sibling middleware package.
//
// Handlers depend on three collaborators packaged in Deps:
//
//   - *service.Service for write-side state-machine entry points
//   - *repo.Repos for read-side queries (and a small set of writes the
//     service layer would otherwise have to surface explicitly)
//   - vault.Vault for symmetric encryption of credentials / private keys
//   - *dns.Registry for provider-specific validate/health hooks
//
// When the auth middleware is nil (test harness, S1 W1 deploy probe) the
// router still mounts /healthz + /readyz unauthenticated so a smoke test
// can confirm cert-svc is alive without holding a JWT.
package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	certmw "github.com/kite365/idcd/apps/cert-svc/internal/middleware"
	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/apps/cert-svc/internal/service"
	"github.com/kite365/idcd/lib/cert/dns"
	"github.com/kite365/idcd/lib/cert/vault"
)

// ErrorResponse is retained for backwards compatibility with the existing
// /healthz / 501 response shape that lib clients already key on. New
// business errors use the internal errResp type from util.go.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Deps bundles the runtime collaborators every handler needs.
type Deps struct {
	DB      Pinger
	Redis   Pinger
	Service *service.Service
	Repos   *repo.Repos
	Vault   vault.Vault
	DNSReg  *dns.Registry

	// AuthnMiddleware wraps every /v1/cert route. When nil, /v1/cert
	// routes still mount but every request gets a 401 — this surfaces a
	// misconfigured deploy quickly instead of silently letting traffic
	// through unauthenticated.
	AuthnMiddleware func(http.Handler) http.Handler
}

// Pinger is the minimum surface readyz needs. Both pgxpool.Pool and
// *redis.Client satisfy it (after a thin adapter).
type Pinger interface {
	Ping(ctx context.Context) error
}

// New returns a chi router with every cert-svc route mounted.
func New(deps Deps) chi.Router {
	r := chi.NewRouter()

	r.Get("/healthz", healthz)
	r.Get("/readyz", readyz(deps))

	// W5: the one-shot download endpoint is mounted OUTSIDE the auth
	// middleware. The token itself is the only credential — every
	// authenticated /v1/cert/certs/{id}/download response embeds a
	// short-lived signed token in download_url. Adding auth on top would
	// break the "share with CI / colleague" use case the URL exists for.
	mountDownloadLink(r, deps)

	r.Route("/v1/cert", func(r chi.Router) {
		if deps.AuthnMiddleware != nil {
			r.Use(deps.AuthnMiddleware)
		} else {
			r.Use(rejectAllUnauthenticated)
		}
		mountOrders(r, deps)
		mountDNSCredentials(r, deps)
		mountCerts(r, deps)
	})

	return r
}

// rejectAllUnauthenticated is the fallback when no auth middleware is
// wired. Every request is rejected with 401 + CERT_UNAUTHORIZED so
// callers see a predictable failure mode rather than data leakage.
func rejectAllUnauthenticated(_ http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeErr(w, http.StatusUnauthorized, codeUnauthorized,
			"cert-svc auth middleware not configured", nil)
	})
}

// writeNotImplemented renders the standard 501 payload. Used by endpoints
// we explicitly defer to S2/W4 (PFX download, revoke).
func writeNotImplemented(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusNotImplemented, ErrorResponse{
		Error:   "not_implemented",
		Code:    codeNotImplemented,
		Message: "endpoint not implemented yet",
	})
}

// writeJSON is the single place we set Content-Type + encode bodies, so
// tests can assert on the header without grepping every handler.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// Ensure the certmw import is retained even if all handler files end up
// using only the helpers in util.go. Linters that prune unused imports
// would otherwise drop it.
var _ = certmw.WithUserID
