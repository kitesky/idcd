package handler

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/lib/cert/vault"
)

func certColumns() []string {
	return []string{
		"id", "order_id", "account_id", "sans", "issuer", "serial_hex",
		"fingerprint_sha256", "leaf_pem", "chain_pem", "key_kms_handle",
		"not_before", "not_after", "status", "revoked_at", "revoke_reason", "created_at",
	}
}

func sampleCertRow(t *testing.T, id, accountID int64, status string, v vault.Vault) ([]any, []byte) {
	t.Helper()
	plainPEM, ek, err := v.GenerateKey(context.Background(), vault.KeyAlgECDSAP256)
	require.NoError(t, err)
	ekJSON, _ := json.Marshal(ek)
	handle := base64.StdEncoding.EncodeToString(ekJSON)

	now := time.Now().UTC()
	return []any{
		id, int64(1), accountID, []string{"example.com"}, "Test CA", "1234",
		"sha-fp", "-----LEAF-----\n", "-----CHAIN-----\n", handle,
		now.Add(-time.Hour), now.Add(time.Hour * 24 * 90), status,
		(*time.Time)(nil), (*string)(nil), now,
	}, plainPEM
}

func TestListCerts_OK(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool)}
	v := newTestVault(t)
	row, _ := sampleCertRow(t, 9, 42, "issued", v)

	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE account_id`).
		WithArgs(int64(42), 20, 0).
		WillReturnRows(pgxmock.NewRows(certColumns()).AddRow(row...))

	req := authedRequest(t, http.MethodGet, "/v1/cert/certs", nil, "42")
	rec := httptest.NewRecorder()
	listCerts(deps).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestGetCert_OK(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool)}
	v := newTestVault(t)
	row, _ := sampleCertRow(t, 9, 42, "issued", v)

	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(9)).
		WillReturnRows(pgxmock.NewRows(certColumns()).AddRow(row...))

	req := authedRequest(t, http.MethodGet, "/v1/cert/certs/9", nil, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/certs/{id}", getCert(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestGetCert_NotFound(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool)}

	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(99)).
		WillReturnRows(pgxmock.NewRows(certColumns()))

	req := authedRequest(t, http.MethodGet, "/v1/cert/certs/99", nil, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/certs/{id}", getCert(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDownloadCert_PEMZip(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: v}
	row, _ := sampleCertRow(t, 9, 42, "issued", v)

	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(9)).
		WillReturnRows(pgxmock.NewRows(certColumns()).AddRow(row...))

	req := authedRequest(t, http.MethodPost, "/v1/cert/certs/9/download",
		downloadRequest{Format: "pem"}, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/certs/{id}/download", downloadCert(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "application/zip", rec.Header().Get("Content-Type"))

	// Verify we got a real zip with the expected entries.
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	require.NoError(t, err)
	got := map[string]bool{}
	for _, f := range zr.File {
		got[f.Name] = true
		rc, _ := f.Open()
		_, _ = io.ReadAll(rc)
		_ = rc.Close()
	}
	require.True(t, got["fullchain.pem"])
	require.True(t, got["privkey.pem"])
}

func TestDownloadCert_PFXNotImplemented(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: v}

	req := authedRequest(t, http.MethodPost, "/v1/cert/certs/9/download",
		downloadRequest{Format: "pfx"}, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/certs/{id}/download", downloadCert(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotImplemented, rec.Code, rec.Body.String())
}

func TestDownloadCert_BadFormat(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: newTestVault(t)}

	req := authedRequest(t, http.MethodPost, "/v1/cert/certs/9/download",
		downloadRequest{Format: "weird"}, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/certs/{id}/download", downloadCert(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestRevokeCert_NotImplemented(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool)}
	req := authedRequest(t, http.MethodPost, "/v1/cert/certs/9/revoke", nil, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/certs/{id}/revoke", revokeCert(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotImplemented, rec.Code)
}
