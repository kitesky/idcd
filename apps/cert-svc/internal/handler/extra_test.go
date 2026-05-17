package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"

	certmw "github.com/kite365/idcd/apps/cert-svc/internal/middleware"
	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
)

func TestCreateOrder_DNSCredentialNotFound(t *testing.T) {
	pool := newMockPool(t)
	rdb, _ := newMiniRedis(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Service: newTestService(t, pool, rdb)}

	pool.ExpectQuery(`SELECT .+ FROM cert\.dns_credentials\s+WHERE id`).
		WithArgs(int64(99)).
		WillReturnRows(pgxmock.NewRows(dnsCredFullColumns()))

	credID := int64(99)
	body := createOrderRequest{
		SANs:            []string{"example.com"},
		DNSCredentialID: &credID,
	}
	req := authedRequest(t, http.MethodPost, "/v1/cert/orders", body, "42")
	rec := httptest.NewRecorder()
	createOrder(deps).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
}

func TestCreateOrder_DNSCredentialForbidden(t *testing.T) {
	pool := newMockPool(t)
	rdb, _ := newMiniRedis(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Service: newTestService(t, pool, rdb)}

	pool.ExpectQuery(`SELECT .+ FROM cert\.dns_credentials\s+WHERE id`).
		WithArgs(int64(99)).
		WillReturnRows(pgxmock.NewRows(dnsCredFullColumns()).
			AddRow(int64(99), int64(999), "manual", "x",
				[]byte("{}"), []byte(nil), "kek",
				"valid", (*time.Time)(nil), time.Now().UTC(), (*time.Time)(nil)))

	credID := int64(99)
	body := createOrderRequest{SANs: []string{"example.com"}, DNSCredentialID: &credID}
	req := authedRequest(t, http.MethodPost, "/v1/cert/orders", body, "42")
	rec := httptest.NewRecorder()
	createOrder(deps).ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

func TestCreateOrder_DNSCredentialRevoked(t *testing.T) {
	pool := newMockPool(t)
	rdb, _ := newMiniRedis(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Service: newTestService(t, pool, rdb)}

	rev := time.Now().UTC()
	pool.ExpectQuery(`SELECT .+ FROM cert\.dns_credentials\s+WHERE id`).
		WithArgs(int64(99)).
		WillReturnRows(pgxmock.NewRows(dnsCredFullColumns()).
			AddRow(int64(99), int64(42), "manual", "x",
				[]byte("{}"), []byte(nil), "kek",
				"valid", (*time.Time)(nil), time.Now().UTC(), &rev))

	credID := int64(99)
	body := createOrderRequest{SANs: []string{"example.com"}, DNSCredentialID: &credID}
	req := authedRequest(t, http.MethodPost, "/v1/cert/orders", body, "42")
	rec := httptest.NewRecorder()
	createOrder(deps).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
}

func TestCreateOrder_QuotaExceeded(t *testing.T) {
	pool := newMockPool(t)
	rdb, _ := newMiniRedis(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Service: newTestService(t, pool, rdb)}

	// Fill 20 rows all created within the last 24h to trip the quota.
	rows := pgxmock.NewRows(orderRowColumns())
	for i := int64(0); i < dailyOrderQuota; i++ {
		rows.AddRow(sampleOrderRow(i+1, 42, "issued")...)
	}
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders`).
		WithArgs(int64(42), 100, 0).
		WillReturnRows(rows)

	body := createOrderRequest{SANs: []string{"example.com"}}
	req := authedRequest(t, http.MethodPost, "/v1/cert/orders", body, "42")
	rec := httptest.NewRecorder()
	createOrder(deps).ServeHTTP(rec, req)
	require.Equal(t, http.StatusTooManyRequests, rec.Code, rec.Body.String())
}

func TestCreateOrder_RejectsNonDNS01Challenge(t *testing.T) {
	pool := newMockPool(t)
	rdb, _ := newMiniRedis(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Service: newTestService(t, pool, rdb)}
	body := createOrderRequest{SANs: []string{"example.com"}, Challenge: "http-01"}
	req := authedRequest(t, http.MethodPost, "/v1/cert/orders", body, "42")
	rec := httptest.NewRecorder()
	createOrder(deps).ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestCreateOrder_MalformedJSON(t *testing.T) {
	pool := newMockPool(t)
	rdb, _ := newMiniRedis(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Service: newTestService(t, pool, rdb)}
	req := httptest.NewRequest(http.MethodPost, "/v1/cert/orders",
		strings.NewReader(`{not json}`))
	req = req.WithContext(authContextWith(req, "42"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	createOrder(deps).ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestRetryOrder_HappyPath(t *testing.T) {
	pool := newMockPool(t)
	rdb, _ := newMiniRedis(t)
	repos := repo.NewWithPool(pool)
	svc := newTestService(t, pool, rdb)
	deps := Deps{Repos: repos, Service: svc}

	// Handler reads the order once; service.RetryOrder reads it again.
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE id`).
		WithArgs(int64(101)).
		WillReturnRows(pgxmock.NewRows(orderRowColumns()).AddRow(sampleOrderRow(101, 42, "failed")...))
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE id`).
		WithArgs(int64(101)).
		WillReturnRows(pgxmock.NewRows(orderRowColumns()).AddRow(sampleOrderRow(101, 42, "failed")...))
	// Service.RetryOrder transitions status failed → validating.
	pool.ExpectExec(`UPDATE cert\.orders\s+SET status`).
		WithArgs("validating", pgxmock.AnyArg(), int64(101), "failed").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	req := authedRequest(t, http.MethodPost, "/v1/cert/orders/101/retry", nil, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/orders/{id}/retry", retryOrder(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
}

func TestRetryOrder_NotFound(t *testing.T) {
	pool := newMockPool(t)
	rdb, _ := newMiniRedis(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Service: newTestService(t, pool, rdb)}

	pool.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE id`).
		WithArgs(int64(101)).
		WillReturnRows(pgxmock.NewRows(orderRowColumns()))

	req := authedRequest(t, http.MethodPost, "/v1/cert/orders/101/retry", nil, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/orders/{id}/retry", retryOrder(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

func TestManualReady_MissingFields(t *testing.T) {
	pool := newMockPool(t)
	rdb, _ := newMiniRedis(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Service: newTestService(t, pool, rdb)}
	body := manualReadyRequest{FQDN: "", Value: "abc"}
	req := authedRequest(t, http.MethodPost, "/v1/cert/orders/101/manual-ready", body, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/orders/{id}/manual-ready", manualReadyOrder(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestListCerts_WithStatusFilter(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool)}
	v := newTestVault(t)
	row, _ := sampleCertRow(t, 9, 42, "issued", v)

	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE account_id = \$1 AND status`).
		WithArgs(int64(42), "issued", 20, 0).
		WillReturnRows(pgxmock.NewRows(certColumns()).AddRow(row...))

	req := authedRequest(t, http.MethodGet, "/v1/cert/certs?status=issued", nil, "42")
	rec := httptest.NewRecorder()
	listCerts(deps).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestGetCert_Forbidden(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool)}
	v := newTestVault(t)
	row, _ := sampleCertRow(t, 9, 999, "issued", v)

	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(9)).
		WillReturnRows(pgxmock.NewRows(certColumns()).AddRow(row...))

	req := authedRequest(t, http.MethodGet, "/v1/cert/certs/9", nil, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/certs/{id}", getCert(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

func TestDownloadCert_NginxFormat(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: v, Service: newDownloadService(t, pool)}
	row, _ := sampleCertRow(t, 9, 42, "issued", v)

	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(9)).
		WillReturnRows(pgxmock.NewRows(certColumns()).AddRow(row...))

	req := authedRequest(t, http.MethodPost, "/v1/cert/certs/9/download",
		downloadRequest{Format: "nginx"}, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/certs/{id}/download", downloadCert(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	// W5: format=nginx no longer streams the cert inline; the download URL
	// itself carries the format and the GET handler ships an nginx-named
	// zip. Asserting on Content-Disposition here is now meaningless —
	// instead verify the JSON contract.
	var body downloadResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, strings.HasPrefix(body.DownloadURL, "/v1/cert/dl/"))
}

func TestDownloadCert_RevokedRejected(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: v, Service: newDownloadService(t, pool)}
	row, _ := sampleCertRow(t, 9, 42, "revoked", v)

	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(9)).
		WillReturnRows(pgxmock.NewRows(certColumns()).AddRow(row...))

	req := authedRequest(t, http.MethodPost, "/v1/cert/certs/9/download",
		downloadRequest{Format: "pem"}, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/certs/{id}/download", downloadCert(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
}

func TestListOrders_WithFilters(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool)}

	pool.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE account_id = \$1 AND status`).
		WithArgs(int64(42), "issued", 50, 10).
		WillReturnRows(pgxmock.NewRows(orderRowColumns()).AddRow(sampleOrderRow(1, 42, "issued")...))

	req := authedRequest(t, http.MethodGet, "/v1/cert/orders?status=issued&limit=50&offset=10", nil, "42")
	rec := httptest.NewRecorder()
	listOrders(deps).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestPathInt64_InvalidReturns404(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/cert/abc", nil)
	r := chiRouterWith(t, "/cert/{id}", func(w http.ResponseWriter, r *http.Request) {
		_, ok := pathInt64(w, r, "id")
		if !ok {
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRequireUser_InvalidUserIDReturns401(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req = req.WithContext(authContextWith(req, "not-a-number"))
	_, ok := requireUser(rec, req)
	require.False(t, ok)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestQueryIntDefault_HandlesCases(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x?a=10&b=oops", nil)
	if got := queryIntDefault(req, "a", 99); got != 10 {
		t.Errorf("a = %d", got)
	}
	if got := queryIntDefault(req, "b", 99); got != 99 {
		t.Errorf("b = %d", got)
	}
	if got := queryIntDefault(req, "missing", 5); got != 5 {
		t.Errorf("missing = %d", got)
	}
}

func TestProjectOrder_WithFinalizedAt(t *testing.T) {
	ts := time.Now().UTC()
	o := &repo.Order{
		ID:          1,
		AccountID:   42,
		SANs:        []string{"a.com"},
		Status:      "issued",
		Tier:        "free-dv",
		CA:          "letsencrypt",
		FinalizedAt: &ts,
		CreatedAt:   ts,
	}
	out := projectOrder(o)
	require.NotNil(t, out.FinalizedAt)
}

func TestDecodeKeyHandle_AcceptsRawJSON(t *testing.T) {
	// raw JSON (no base64 wrapping) — older / test rows.
	raw := `{"KeyID":"x","Algorithm":"AES-256-GCM","Nonce":"","Ciphertext":"","Alg":"ecdsa-p256"}`
	ek, err := decodeKeyHandle(raw)
	require.NoError(t, err)
	require.Equal(t, "x", ek.KeyID)
}

func TestDecodeKeyHandle_RejectsEmpty(t *testing.T) {
	_, err := decodeKeyHandle("")
	require.Error(t, err)
}

func TestDecodeKeyHandle_RejectsGarbage(t *testing.T) {
	_, err := decodeKeyHandle("zzzzzz~!@#")
	require.Error(t, err)
}

func TestErrUnauthenticatedSurfacesAs401(t *testing.T) {
	// Sanity check that requireUser maps ErrUnauthenticated to 401.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	_, ok := requireUser(rec, req)
	require.False(t, ok)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	var er errResp
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &er))
	require.Equal(t, codeUnauthorized, er.Code)
}

func TestWriteNotImplementedShape(t *testing.T) {
	rec := httptest.NewRecorder()
	writeNotImplemented(rec, nil)
	require.Equal(t, http.StatusNotImplemented, rec.Code)
}

// authContextWith returns a context derived from r with an authenticated
// user id injected. Tests use it via req.WithContext(authContextWith(...))
// when they need a bare http.Request rather than the authedRequest helper.
func authContextWith(r *http.Request, userID string) context.Context {
	if r == nil {
		return certmw.WithUserID(context.Background(), userID)
	}
	return certmw.WithUserID(r.Context(), userID)
}
