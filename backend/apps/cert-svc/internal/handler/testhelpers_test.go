package handler

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/apps/cert-svc/internal/service"
)

// newDownloadService spins up a *service.Service whose only wired
// surface is the W5 download token manager. Tests that exercise the
// POST /certs/{id}/download path use this to satisfy
// deps.Service.Downloads without the full orchestrator wiring.
func newDownloadService(t *testing.T, pool pgxmock.PgxPoolIface) *service.Service {
	t.Helper()
	rdb, _ := newMiniRedis(t)
	svc := service.New(service.Config{
		Repos:          repo.NewWithPool(pool),
		Redis:          rdb,
		DownloadSecret: []byte("download-test-secret-32-bytes-aaa"),
	})
	require.NotNil(t, svc.Downloads, "DownloadTokenManager must be wired")
	return svc
}

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
