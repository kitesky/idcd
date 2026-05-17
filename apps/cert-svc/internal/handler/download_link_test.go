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
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
)

// dlRouter mounts the full chi tree so end-to-end tests can exercise
// POST .../download → GET /v1/cert/dl/{token} against a single deps
// graph. The POST side is wired through chiRouterWith (no auth
// middleware) — same pattern as the existing handler tests.
func dlRouter(t *testing.T, deps Deps) (chi.Router, chi.Router) {
	t.Helper()
	post := chiRouterWith(t, "/v1/cert/certs/{id}/download", downloadCert(deps))
	get := chiRouterWith(t, "/v1/cert/dl/{token}", downloadByToken(deps))
	return post, get
}

// issueDownload helper: drives the POST side and returns the URL the
// caller would follow on the GET side.
func issueDownload(t *testing.T, post chi.Router, certID int64, format, password, userID string) string {
	t.Helper()
	body := downloadRequest{Format: format, Password: password}
	req := authedRequest(t, http.MethodPost, "/v1/cert/certs/9/download", body, userID)
	rec := httptest.NewRecorder()
	post.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp downloadResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, strings.HasPrefix(resp.DownloadURL, "/v1/cert/dl/"))
	return resp.DownloadURL
}

// followDownload helper: issues the GET against the dl router.
func followDownload(t *testing.T, get chi.Router, url string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	get.ServeHTTP(rec, req)
	return rec
}

// expectCertGetByID queues a single GetByID expectation on the mock pool.
func expectCertGetByID(t *testing.T, pool pgxmock.PgxPoolIface, row []any) {
	t.Helper()
	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(9)).
		WillReturnRows(pgxmock.NewRows(certColumns()).AddRow(row...))
}

// TestDownloadByToken_PEM_EndToEnd: POST → GET round trip yields a real
// zip with fullchain.pem + privkey.pem and the right MIME headers.
func TestDownloadByToken_PEM_EndToEnd(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: v, Service: newDownloadService(t, pool)}
	row, _ := sampleCertRow(t, 9, 42, "issued", v)
	expectCertGetByID(t, pool, row)
	// GET side fetches the cert again — second expectation.
	expectCertGetByID(t, pool, row)

	post, get := dlRouter(t, deps)
	url := issueDownload(t, post, 9, "pem", "", "42")

	rec := followDownload(t, get, url)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "application/zip", rec.Header().Get("Content-Type"))
	require.Contains(t, rec.Header().Get("Content-Disposition"), ".zip")
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))

	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	require.NoError(t, err)
	got := map[string]bool{}
	for _, f := range zr.File {
		got[f.Name] = true
		rc, _ := f.Open()
		_, _ = io.ReadAll(rc)
		_ = rc.Close()
	}
	require.True(t, got["fullchain.pem"], "expected fullchain.pem in zip")
	require.True(t, got["privkey.pem"], "expected privkey.pem in zip")
}

// TestDownloadByToken_Nginx_EndToEnd: format=nginx renames the entries.
func TestDownloadByToken_Nginx_EndToEnd(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: v, Service: newDownloadService(t, pool)}
	row, _ := sampleCertRow(t, 9, 42, "issued", v)
	expectCertGetByID(t, pool, row)
	expectCertGetByID(t, pool, row)

	post, get := dlRouter(t, deps)
	url := issueDownload(t, post, 9, "nginx", "", "42")

	rec := followDownload(t, get, url)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Header().Get("Content-Disposition"), "nginx")

	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	require.NoError(t, err)
	got := map[string]bool{}
	for _, f := range zr.File {
		got[f.Name] = true
	}
	require.True(t, got["nginx.crt"])
	require.True(t, got["nginx.key"])
}

// TestDownloadByToken_PFX_Success: POST mints with a password; GET
// returns application/x-pkcs12 with non-empty bytes.
func TestDownloadByToken_PFX_Success(t *testing.T) {
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
		int64(9), int64(1), int64(42), []string{"pfx.example.com"},
		"lets-encrypt", "abc", "fp", leafPEM, chainPEM, handle,
		now.Add(-time.Hour), now.Add(time.Hour * 24 * 90), "issued",
		(*time.Time)(nil), (*string)(nil), now,
	}
	expectCertGetByID(t, pool, row)
	expectCertGetByID(t, pool, row)

	post, get := dlRouter(t, deps)
	url := issueDownload(t, post, 9, "pfx", "s3cret", "42")

	rec := followDownload(t, get, url)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "application/x-pkcs12", rec.Header().Get("Content-Type"))
	require.NotEmpty(t, rec.Body.Bytes())
	require.Contains(t, rec.Header().Get("Content-Disposition"), ".pfx")
}

// TestDownloadByToken_SingleUse: replaying a freshly redeemed URL must
// 410 — the redis nonce was DEL'd on first GET.
func TestDownloadByToken_SingleUse(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: v, Service: newDownloadService(t, pool)}
	row, _ := sampleCertRow(t, 9, 42, "issued", v)
	expectCertGetByID(t, pool, row) // POST
	expectCertGetByID(t, pool, row) // first GET succeeds

	post, get := dlRouter(t, deps)
	url := issueDownload(t, post, 9, "pem", "", "42")

	first := followDownload(t, get, url)
	require.Equal(t, http.StatusOK, first.Code)

	// Second hit: no extra GetByID expectation because Consume rejects
	// before we'd query the repo.
	second := followDownload(t, get, url)
	require.Equal(t, http.StatusGone, second.Code, second.Body.String())
	var body errResp
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &body))
	require.Equal(t, codeDownloadTokenInvalid, body.Code)
}

// TestDownloadByToken_BadToken: arbitrary garbage in the token path
// returns 410 — same wire shape as a single-used token, so callers can
// treat the URL as terminal regardless of failure mode.
func TestDownloadByToken_BadToken(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: v, Service: newDownloadService(t, pool)}

	_, get := dlRouter(t, deps)
	rec := followDownload(t, get, "/v1/cert/dl/garbage.notatoken")
	require.Equal(t, http.StatusGone, rec.Code, rec.Body.String())
}

// TestDownloadByToken_NoService: when the manager isn't wired the GET
// surfaces 503 rather than crashing.
func TestDownloadByToken_NoService(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: v}
	_, get := dlRouter(t, deps)
	rec := followDownload(t, get, "/v1/cert/dl/x.y")
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// TestDownloadByToken_PublicRoute: the dl endpoint must be reachable
// without authentication when mounted via the full New() router — even
// when no auth middleware is configured (rejectAllUnauthenticated).
func TestDownloadByToken_PublicRoute(t *testing.T) {
	r := New(Deps{})
	req := httptest.NewRequest(http.MethodGet, "/v1/cert/dl/anything.here", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	// Without a wired Downloads manager we expect 503, NOT 401 — proves
	// the route is outside the auth middleware.
	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
}

// TestDownloadByToken_CertOwnershipDrift: a token signed for one account
// pointed at a cert owned by another must 403. We trigger by mutating
// the repo response between Issue and Consume.
func TestDownloadByToken_CertOwnershipDrift(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: v, Service: newDownloadService(t, pool)}
	rowIssue, _ := sampleCertRow(t, 9, 42, "issued", v)
	rowDrift, _ := sampleCertRow(t, 9, 999, "issued", v) // owner swapped
	expectCertGetByID(t, pool, rowIssue)
	expectCertGetByID(t, pool, rowDrift)

	post, get := dlRouter(t, deps)
	url := issueDownload(t, post, 9, "pem", "", "42")
	rec := followDownload(t, get, url)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

// TestDownloadByToken_CertGoneOnConsume: cert deleted between Issue and
// Consume → 404.
func TestDownloadByToken_CertGoneOnConsume(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: v, Service: newDownloadService(t, pool)}
	row, _ := sampleCertRow(t, 9, 42, "issued", v)
	expectCertGetByID(t, pool, row)
	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(9)).
		WillReturnRows(pgxmock.NewRows(certColumns())) // empty → ErrNotFound

	post, get := dlRouter(t, deps)
	url := issueDownload(t, post, 9, "pem", "", "42")
	rec := followDownload(t, get, url)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

// TestDownloadByToken_RevokedAfterIssue: cert revoked between mint and
// redeem → 409. Defence-in-depth: a freshly revoked key must not ship
// via a token issued seconds earlier.
func TestDownloadByToken_RevokedAfterIssue(t *testing.T) {
	pool := newMockPool(t)
	v := newTestVault(t)
	deps := Deps{Repos: repo.NewWithPool(pool), Vault: v, Service: newDownloadService(t, pool)}
	rowIssue, _ := sampleCertRow(t, 9, 42, "issued", v)
	rowRevoked, _ := sampleCertRow(t, 9, 42, "revoked", v)
	expectCertGetByID(t, pool, rowIssue)
	expectCertGetByID(t, pool, rowRevoked)

	post, get := dlRouter(t, deps)
	url := issueDownload(t, post, 9, "pem", "", "42")
	rec := followDownload(t, get, url)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
}
