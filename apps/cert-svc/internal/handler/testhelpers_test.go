package handler

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
)

// chiRouterWith mounts a single handler on a chi router under the given
// route pattern, so {id} path params resolve. Used by handler tests that
// need to exercise a route in isolation (without the full /v1/cert/*
// auth + dependency wiring from New()).
func chiRouterWith(t *testing.T, pattern string, h http.HandlerFunc) chi.Router {
	t.Helper()
	r := chi.NewRouter()
	// chi's Mux uses the request method to dispatch — accept all the
	// methods our handlers test against.
	r.MethodFunc(http.MethodGet, pattern, h)
	r.MethodFunc(http.MethodPost, pattern, h)
	r.MethodFunc(http.MethodDelete, pattern, h)
	return r
}
