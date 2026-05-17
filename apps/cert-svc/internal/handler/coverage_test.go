package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
)

// These tests cover the cheap error branches that the happy-path tests
// in orders_test / dns_credentials_test / certs_test skip.

func TestListOrders_RepoError(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool)}

	pool.ExpectQuery(`SELECT .+ FROM cert\.orders`).
		WithArgs(int64(42), 20, 0).
		WillReturnError(errors.New("boom"))

	req := authedRequest(t, http.MethodGet, "/v1/cert/orders", nil, "42")
	rec := httptest.NewRecorder()
	listOrders(deps).ServeHTTP(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestListOrders_NoRepos_500(t *testing.T) {
	deps := Deps{}
	req := authedRequest(t, http.MethodGet, "/v1/cert/orders", nil, "42")
	rec := httptest.NewRecorder()
	listOrders(deps).ServeHTTP(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestListCerts_RepoError(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool)}

	pool.ExpectQuery(`SELECT .+ FROM cert\.certs`).
		WithArgs(int64(42), 20, 0).
		WillReturnError(errors.New("boom"))

	req := authedRequest(t, http.MethodGet, "/v1/cert/certs", nil, "42")
	rec := httptest.NewRecorder()
	listCerts(deps).ServeHTTP(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestListCerts_NoUser(t *testing.T) {
	deps := Deps{}
	req := httptest.NewRequest(http.MethodGet, "/v1/cert/certs", nil)
	rec := httptest.NewRecorder()
	listCerts(deps).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGetCert_RepoError(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool)}

	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(9)).
		WillReturnError(errors.New("boom"))

	req := authedRequest(t, http.MethodGet, "/v1/cert/certs/9", nil, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/certs/{id}", getCert(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestDeleteDNSCredential_NotFound(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool)}

	pool.ExpectQuery(`SELECT .+ FROM cert\.dns_credentials\s+WHERE id`).
		WithArgs(int64(7)).
		WillReturnRows(pgxmock.NewRows(dnsCredFullColumns()))

	req := authedRequest(t, http.MethodDelete, "/v1/cert/dns-credentials/7", nil, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/dns-credentials/{id}", deleteDNSCredential(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHealthCheckDNSCredential_RevokedConflict(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	reg := newRegistryWithManual(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: v, DNSReg: reg}

	rev := time.Now().UTC()
	pool.ExpectQuery(`SELECT .+ FROM cert\.dns_credentials\s+WHERE id`).
		WithArgs(int64(7)).
		WillReturnRows(pgxmock.NewRows(dnsCredFullColumns()).
			AddRow(int64(7), int64(42), "manual", "creds",
				[]byte("{}"), []byte(nil), "kek-1",
				"valid", (*time.Time)(nil), time.Now().UTC(), &rev))

	req := authedRequest(t, http.MethodPost, "/v1/cert/dns-credentials/7/health-check", nil, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/dns-credentials/{id}/health-check", healthCheckDNSCredential(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusConflict, rec.Code)
}

func TestHealthCheckDNSCredential_Forbidden(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	reg := newRegistryWithManual(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: v, DNSReg: reg}

	pool.ExpectQuery(`SELECT .+ FROM cert\.dns_credentials\s+WHERE id`).
		WithArgs(int64(7)).
		WillReturnRows(pgxmock.NewRows(dnsCredFullColumns()).
			AddRow(int64(7), int64(999), "manual", "creds",
				[]byte("{}"), []byte(nil), "kek-1",
				"valid", (*time.Time)(nil), time.Now().UTC(), (*time.Time)(nil)))

	req := authedRequest(t, http.MethodPost, "/v1/cert/dns-credentials/7/health-check", nil, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/dns-credentials/{id}/health-check", healthCheckDNSCredential(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestManualReady_NoUser(t *testing.T) {
	deps := Deps{}
	req := httptest.NewRequest(http.MethodPost, "/v1/cert/orders/1/manual-ready", nil)
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/orders/{id}/manual-ready", manualReadyOrder(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestManualReady_Forbidden(t *testing.T) {
	pool := newMockPool(t)
	rdb, _ := newMiniRedis(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Service: newTestService(t, pool, rdb)}

	pool.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE id`).
		WithArgs(int64(101)).
		WillReturnRows(pgxmock.NewRows(orderRowColumns()).AddRow(sampleOrderRow(101, 999, "validating")...))

	body := manualReadyRequest{FQDN: "_acme-challenge.example.com.", Value: "abc"}
	req := authedRequest(t, http.MethodPost, "/v1/cert/orders/101/manual-ready", body, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/orders/{id}/manual-ready", manualReadyOrder(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRetryOrder_NoUser(t *testing.T) {
	deps := Deps{}
	req := httptest.NewRequest(http.MethodPost, "/v1/cert/orders/1/retry", nil)
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/orders/{id}/retry", retryOrder(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGetOrder_RepoError(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool)}

	pool.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE id`).
		WithArgs(int64(101)).
		WillReturnError(errors.New("boom"))

	req := authedRequest(t, http.MethodGet, "/v1/cert/orders/101", nil, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/orders/{id}", getOrder(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestDownloadCert_NoUser(t *testing.T) {
	deps := Deps{}
	req := httptest.NewRequest(http.MethodPost, "/v1/cert/certs/1/download", nil)
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/certs/{id}/download", downloadCert(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestListCerts_PathRespectsLimitClamp(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool)}

	// limit > MaxPageSize (100) → clamped down to MaxPageSize (was: re-default to 20).
	// 契约迁移到 lib/shared/pagination.Clamp：超过上限给上限，不静默退回默认值。
	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE account_id`).
		WithArgs(int64(42), 100, 0).
		WillReturnRows(pgxmock.NewRows(certColumns()))

	req := authedRequest(t, http.MethodGet, "/v1/cert/certs?limit=99999", nil, "42")
	rec := httptest.NewRecorder()
	listCerts(deps).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}
