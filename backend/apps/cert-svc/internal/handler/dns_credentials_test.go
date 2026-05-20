package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/lib/cert/dns"
	"github.com/kite365/idcd/lib/cert/dns/manual"
	"github.com/kite365/idcd/lib/cert/vault"
	"github.com/kite365/idcd/lib/cert/vault/envmaster"
)

func newTestVault(t *testing.T) vault.Vault {
	t.Helper()
	// 32 bytes of zeroes — fine for unit-test envelope encryption.
	v, err := envmaster.NewWithKey(make([]byte, 32))
	require.NoError(t, err)
	return v
}

func newRegistryWithManual(t *testing.T) *dns.Registry {
	t.Helper()
	r := dns.NewRegistry()
	require.NoError(t, r.Register(manual.New(manual.Config{})))
	return r
}

func dnsCredColumns() []string {
	return []string{
		"id", "account_id", "provider", "display_name", "kek_key_id",
		"health_status", "health_checked_at", "created_at", "revoked_at",
	}
}

func dnsCredFullColumns() []string {
	return []string{
		"id", "account_id", "provider", "display_name",
		"encrypted_blob", "dek_wrapped", "kek_key_id",
		"health_status", "health_checked_at", "created_at", "revoked_at",
	}
}

func TestCreateDNSCredential_Manual_HappyPath(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	reg := newRegistryWithManual(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: v, DNSReg: reg}

	pool.ExpectQuery(`INSERT INTO cert\.dns_credentials`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow(int64(5), time.Now().UTC()))

	body := createDNSCredentialRequest{
		Provider:    "manual",
		DisplayName: "test creds",
		Secrets:     map[string]string{},
	}
	req := authedRequest(t, http.MethodPost, "/v1/cert/dns-credentials", body, "42")
	rec := httptest.NewRecorder()
	createDNSCredential(deps).ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestCreateDNSCredential_UnknownProvider(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	reg := newRegistryWithManual(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: v, DNSReg: reg}

	body := createDNSCredentialRequest{Provider: "no-such", DisplayName: "x", Secrets: map[string]string{}}
	req := authedRequest(t, http.MethodPost, "/v1/cert/dns-credentials", body, "42")
	rec := httptest.NewRecorder()
	createDNSCredential(deps).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
}

func TestCreateDNSCredential_MissingFields(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	reg := newRegistryWithManual(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: v, DNSReg: reg}
	for _, body := range []createDNSCredentialRequest{
		{Provider: "", DisplayName: "x", Secrets: map[string]string{}},
		{Provider: "manual", DisplayName: "", Secrets: map[string]string{}},
	} {
		req := authedRequest(t, http.MethodPost, "/v1/cert/dns-credentials", body, "42")
		rec := httptest.NewRecorder()
		createDNSCredential(deps).ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestListDNSCredentials_OK(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool)}

	pool.ExpectQuery(`SELECT .+ FROM cert\.dns_credentials\s+WHERE account_id`).
		WithArgs("42").
		WillReturnRows(pgxmock.NewRows(dnsCredColumns()).
			AddRow(int64(1), "42", "manual", "creds", "kek-1",
				"valid", (*time.Time)(nil), time.Now().UTC(), (*time.Time)(nil)))

	req := authedRequest(t, http.MethodGet, "/v1/cert/dns-credentials", nil, "42")
	rec := httptest.NewRecorder()
	listDNSCredentials(deps).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestDeleteDNSCredential_OK(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool)}

	pool.ExpectQuery(`SELECT .+ FROM cert\.dns_credentials\s+WHERE id`).
		WithArgs(int64(7)).
		WillReturnRows(pgxmock.NewRows(dnsCredFullColumns()).
			AddRow(int64(7), "42", "manual", "creds",
				[]byte("{}"), []byte(nil), "kek-1",
				"valid", (*time.Time)(nil), time.Now().UTC(), (*time.Time)(nil)))
	pool.ExpectExec(`UPDATE cert\.dns_credentials\s+SET revoked_at`).
		WithArgs(pgxmock.AnyArg(), int64(7)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	req := authedRequest(t, http.MethodDelete, "/v1/cert/dns-credentials/7", nil, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/dns-credentials/{id}", deleteDNSCredential(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
}

func TestDeleteDNSCredential_Forbidden(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool)}

	pool.ExpectQuery(`SELECT .+ FROM cert\.dns_credentials\s+WHERE id`).
		WithArgs(int64(7)).
		WillReturnRows(pgxmock.NewRows(dnsCredFullColumns()).
			AddRow(int64(7), "999", "manual", "creds",
				[]byte("{}"), []byte(nil), "kek-1",
				"valid", (*time.Time)(nil), time.Now().UTC(), (*time.Time)(nil)))

	req := authedRequest(t, http.MethodDelete, "/v1/cert/dns-credentials/7", nil, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/dns-credentials/{id}", deleteDNSCredential(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

func TestHealthCheckDNSCredential_Manual(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	reg := newRegistryWithManual(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: v, DNSReg: reg}

	// Pre-encrypt a manual (empty) credential to feed back.
	enc, err := v.EncryptBlob(context.Background(), []byte("{}"))
	require.NoError(t, err)
	encJSON, _ := json.Marshal(enc)

	pool.ExpectQuery(`SELECT .+ FROM cert\.dns_credentials\s+WHERE id`).
		WithArgs(int64(7)).
		WillReturnRows(pgxmock.NewRows(dnsCredFullColumns()).
			AddRow(int64(7), "42", "manual", "creds",
				encJSON, []byte(nil), v.KeyID(),
				"unknown", (*time.Time)(nil), time.Now().UTC(), (*time.Time)(nil)))
	pool.ExpectExec(`UPDATE cert\.dns_credentials\s+SET health_status`).
		WithArgs("valid", pgxmock.AnyArg(), int64(7)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	req := authedRequest(t, http.MethodPost, "/v1/cert/dns-credentials/7/health-check", nil, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/dns-credentials/{id}/health-check", healthCheckDNSCredential(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Equal(t, "valid", out["health_status"])
}
