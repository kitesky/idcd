package handler

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/apps/cert-svc/internal/service"
	"github.com/kite365/idcd/lib/cert/ca"
	"github.com/kite365/idcd/lib/cert/vault"
)

// newRevokeHandlerService wires a Service that satisfies the runtime
// preconditions of RevokeCert (account key + router) but never reaches
// a real CA in the tests below — every test arranges a repo-layer
// failure that short-circuits the call.
func newRevokeHandlerService(t *testing.T, pool pgxmock.PgxPoolIface) *service.Service {
	t.Helper()
	accountKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return service.New(service.Config{
		Repos:      repo.NewWithPool(pool),
		Router:     service.NewRouter(&inertCA{}),
		AccountKey: accountKey,
	})
}

// inertCA is a ca.AcmeCA stand-in that does nothing. It exists purely to
// satisfy NewRouter's non-nil requirement; tests that touch Revoke fail
// at the repo layer before this CA is consulted.
type inertCA struct{}

func (i *inertCA) Name() string                            { return "inert" }
func (i *inertCA) Tier() ca.Tier                           { return ca.TierFreeDV }
func (i *inertCA) SupportsWildcard() bool                  { return true }
func (i *inertCA) ValidityDays() int                       { return 90 }
func (i *inertCA) SupportedChallenges() []ca.ChallengeType { return []ca.ChallengeType{ca.ChallengeDNS01} }
func (i *inertCA) RequestCertificate(_ context.Context, _ ca.CertificateRequest) (ca.CertificateResult, error) {
	return ca.CertificateResult{}, nil
}
func (i *inertCA) Revoke(_ context.Context, _ []byte, _ ca.RevokeReason, _ crypto.Signer) error {
	return nil
}

func certColumns() []string {
	return []string{
		"id", "order_id", "account_id", "sans", "issuer", "serial_hex",
		"fingerprint_sha256", "leaf_pem", "chain_pem", "key_kms_handle",
		"not_before", "not_after", "status", "revoked_at", "revoke_reason", "created_at",
	}
}

func sampleCertRow(t *testing.T, id int64, accountID string, status string, v vault.Vault) ([]any, []byte) {
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
	row, _ := sampleCertRow(t, 9, "42", "issued", v)

	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE account_id`).
		WithArgs("42", 20, 0).
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
	row, _ := sampleCertRow(t, 9, "42", "issued", v)

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

// TestDownloadCert_IssuesToken is the W5 contract test: POST returns a
// signed URL pointing at /v1/cert/dl/{token} plus an RFC3339 expires_at,
// and never the cert bytes themselves. Content-level assertions moved to
// download_link_test.go where the GET flow can be exercised end-to-end.
func TestDownloadCert_IssuesToken(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: v, Service: newDownloadService(t, pool)}
	row, _ := sampleCertRow(t, 9, "42", "issued", v)

	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(9)).
		WillReturnRows(pgxmock.NewRows(certColumns()).AddRow(row...))

	req := authedRequest(t, http.MethodPost, "/v1/cert/certs/9/download",
		downloadRequest{Format: "pem"}, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/certs/{id}/download", downloadCert(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var body downloadResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, strings.HasPrefix(body.DownloadURL, "/v1/cert/dl/"),
		"download_url should point at the public dl endpoint, got %q", body.DownloadURL)
	require.NotEmpty(t, body.ExpiresAt)
	// Sanity: expires_at parses as RFC3339 in the future.
	exp, err := time.Parse(time.RFC3339, body.ExpiresAt)
	require.NoError(t, err)
	require.True(t, exp.After(time.Now().Add(-time.Second)))
}

func TestDownloadCert_PFX_EmptyPasswordRejected(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: v, Service: newDownloadService(t, pool)}

	req := authedRequest(t, http.MethodPost, "/v1/cert/certs/9/download",
		downloadRequest{Format: "pfx"}, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/certs/{id}/download", downloadCert(deps))
	r.ServeHTTP(rec, req)
	// Empty password is rejected up-front with 422 — PFX without a
	// password is non-portable and most consumers refuse to load it.
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
}

func TestDownloadCert_BadFormat(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: newTestVault(t), Service: newDownloadService(t, pool)}

	req := authedRequest(t, http.MethodPost, "/v1/cert/certs/9/download",
		downloadRequest{Format: "weird"}, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/certs/{id}/download", downloadCert(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

// TestDownloadCert_NoService surfaces the 503 boot-misconfig path: if the
// service was constructed without a DownloadSecret the handler refuses
// to mint forgeable tokens.
func TestDownloadCert_NoService(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: v}
	req := authedRequest(t, http.MethodPost, "/v1/cert/certs/9/download",
		downloadRequest{Format: "pem"}, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/certs/{id}/download", downloadCert(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
}

func TestRevokeCert_ServiceUnconfigured(t *testing.T) {
	// Deps without a wired Service surfaces 503 — operators can fix the
	// deploy without losing the request.
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool)}
	req := authedRequest(t, http.MethodPost, "/v1/cert/certs/9/revoke", nil, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/certs/{id}/revoke", revokeCert(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
}

// pfxCertFixture generates a fresh ECDSA leaf + its PEM-encoded private
// key, returning the strings the handler expects on a real cert.certs
// row. Used by both buildPFX unit tests and the end-to-end download
// handler test.
func pfxCertFixture(t *testing.T) (leafPEM, chainPEM string, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(123456),
		Subject:      pkix.Name{CommonName: "pfx.example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(90 * 24 * time.Hour),
		DNSNames:     []string{"pfx.example.com"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	leafPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	// Chain is empty (self-signed); buildPFX must handle that.
	return leafPEM, "", keyPEM
}

// TestDownloadCert_PFX_IssuesToken: POST with format=pfx + password mints
// a download URL (no PFX bytes inline). The actual PFX encoding path is
// covered by download_link_test.go::TestDownloadByToken_PFX_Success.
func TestDownloadCert_PFX_IssuesToken(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: v, Service: newDownloadService(t, pool)}

	leafPEM, chainPEM, keyPEM := pfxCertFixture(t)
	ek, err := v.EncryptKey(context.Background(), keyPEM)
	require.NoError(t, err)
	ekBytes, _ := json.Marshal(ek)
	handle := base64.StdEncoding.EncodeToString(ekBytes)
	now := time.Now().UTC()
	row := []any{
		int64(9), int64(1), "42", []string{"pfx.example.com"},
		"lets-encrypt", "abc", "fp", leafPEM, chainPEM, handle,
		now.Add(-time.Hour), now.Add(time.Hour * 24 * 90), "issued",
		(*time.Time)(nil), (*string)(nil), now,
	}

	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(9)).
		WillReturnRows(pgxmock.NewRows(certColumns()).AddRow(row...))

	req := authedRequest(t, http.MethodPost, "/v1/cert/certs/9/download",
		downloadRequest{Format: "pfx", Password: "s3cret"}, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/certs/{id}/download", downloadCert(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var body downloadResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, strings.HasPrefix(body.DownloadURL, "/v1/cert/dl/"))
}

// TestBuildPFX_HappyPath: drive the PFX builder with a real leaf + key;
// verify we get a non-empty byte slice.
func TestBuildPFX_HappyPath(t *testing.T) {
	leaf, chain, key := pfxCertFixture(t)
	body, err := buildPFX(leaf, chain, key, "s3cret")
	require.NoError(t, err)
	require.NotEmpty(t, body)
}

// TestBuildPFX_BadLeaf: a non-PEM leaf is rejected cleanly.
func TestBuildPFX_BadLeaf(t *testing.T) {
	_, _, key := pfxCertFixture(t)
	_, err := buildPFX("not pem", "", key, "s3cret")
	require.Error(t, err)
}

// TestBuildPFX_BadKey: a malformed key fails before encoding.
func TestBuildPFX_BadKey(t *testing.T) {
	leaf, chain, _ := pfxCertFixture(t)
	_, err := buildPFX(leaf, chain, []byte("not pem"), "s3cret")
	require.Error(t, err)
}

// TestParseRevokeReason covers every alias the wire accepts plus the
// fallback for unknown values. We avoid importing ca.RevokeReason String()
// (it has none) and instead round-trip through the handler+service
// reasoning by checking ordinal equality.
func TestParseRevokeReason(t *testing.T) {
	require.Equal(t, parseRevokeReason("keyCompromise"), parseRevokeReason("key_compromise"))
	require.Equal(t, parseRevokeReason("cessationOfOperation"), parseRevokeReason("cessation_of_operation"))
	require.Equal(t, parseRevokeReason("certificateHold"), parseRevokeReason("certificate_hold"))
	// Unknown / empty falls through to "unspecified" — the default value
	// of ca.RevokeReason is 0 == RevokeUnspecified.
	require.Equal(t, parseRevokeReason(""), parseRevokeReason("garbage"))
}

// TestRevokeCert_ForbiddenViaService verifies the handler maps the
// service-layer ErrForbidden sentinel onto a 403 CERT_FORBIDDEN envelope.
// We arrange the failure entirely at the repo layer (GetByID returns a
// cert owned by a different account); the fakeAcmeCA is never reached.
func TestRevokeCert_ForbiddenViaService(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	svc := newRevokeHandlerService(t, pool)
	deps := Deps{Repos: repo.NewWithPool(pool), Service: svc, Vault: v}

	row, _ := sampleCertRow(t, 9, "999", "issued", v)
	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(9)).
		WillReturnRows(pgxmock.NewRows(certColumns()).AddRow(row...))

	req := authedRequest(t, http.MethodPost, "/v1/cert/certs/9/revoke",
		nil, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/certs/{id}/revoke", revokeCert(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

// TestRevokeCert_NotFoundViaService: repo returns ErrNotFound → handler
// renders 404 CERT_NOT_FOUND.
func TestRevokeCert_NotFoundViaService(t *testing.T) {
	pool := newMockPool(t)
	svc := newRevokeHandlerService(t, pool)
	deps := Deps{Repos: repo.NewWithPool(pool), Service: svc}

	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(9)).
		WillReturnRows(pgxmock.NewRows(certColumns()))

	req := authedRequest(t, http.MethodPost, "/v1/cert/certs/9/revoke",
		nil, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/certs/{id}/revoke", revokeCert(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

// TestRevokeCert_AlreadyRevokedViaService: a cert in 'revoked' status
// produces ErrInvalidStatus → handler renders 409.
func TestRevokeCert_AlreadyRevokedViaService(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	svc := newRevokeHandlerService(t, pool)
	deps := Deps{Repos: repo.NewWithPool(pool), Service: svc, Vault: v}

	row, _ := sampleCertRow(t, 9, "42", "revoked", v)
	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(9)).
		WillReturnRows(pgxmock.NewRows(certColumns()).AddRow(row...))

	req := authedRequest(t, http.MethodPost, "/v1/cert/certs/9/revoke",
		nil, "42")
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/certs/{id}/revoke", revokeCert(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
}

// TestRevokeCert_NoAuth_401: unauthenticated requests are rejected
// before the service is even consulted.
func TestRevokeCert_NoAuth_401(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Service: newRevokeHandlerService(t, pool)}
	req := httptest.NewRequest(http.MethodPost, "/v1/cert/certs/9/revoke", nil)
	rec := httptest.NewRecorder()
	r := chiRouterWith(t, "/v1/cert/certs/{id}/revoke", revokeCert(deps))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
}

