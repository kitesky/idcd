// Package handler wires the cert-svc HTTP routes.
//
// S1 W1 (this commit) only mounts the route table and returns 501 for
// every business endpoint — the actual ACME orchestration lands in W2.
// Health endpoints are real so deploy probes can already track the
// service.
package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/kite365/idcd/apps/cert-svc/internal/service"
)

// ErrorResponse is the JSON shape returned for every non-2xx response.
// Stable across CERT_* error codes (see PRD §10.3).
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Deps bundles the runtime collaborators the handlers will need once the
// business logic lands in W2. Today only the readiness probes are wired,
// so the fields are optional — handlers must nil-check before use.
type Deps struct {
	DB      Pinger
	Redis   Pinger
	Service *service.Service
}

// Pinger is the minimum surface readyz needs. Both pgxpool.Pool and
// *redis.Client satisfy variants of this (after a thin adapter).
type Pinger interface {
	Ping(ctx context.Context) error
}

// New returns a chi router with every cert-svc route mounted.
func New(deps Deps) chi.Router {
	r := chi.NewRouter()

	r.Get("/healthz", healthz)
	r.Get("/readyz", readyz(deps))

	r.Route("/v1/cert", func(r chi.Router) {
		mountOrders(r)
		mountDNSCredentials(r)
		mountCerts(r)
	})

	return r
}

// writeNotImplemented renders the standard 501 payload. Centralised so
// every stub endpoint emits an identical shape that the frontend / SDK
// can branch on safely.
func writeNotImplemented(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusNotImplemented, ErrorResponse{
		Error:   "not_implemented",
		Code:    "CERT_NOT_IMPL",
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
