package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/apps/cert-svc/internal/service"
)

// adminRequest builds a request body+JSON without any auth header — the
// admin handler functions are invoked directly in the unit tests so we
// don't depend on Deps.AdminAuthnMiddleware semantics here.
func adminRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// chiAdminRouter mounts the admin handlers on a fresh chi router so chi
// URL params resolve correctly inside the handler.
func chiAdminRouter(deps Deps) chi.Router {
	r := chi.NewRouter()
	r.Route("/v1/admin/cert", func(r chi.Router) {
		mountAdmin(r, deps)
	})
	return r
}

// --- Router gating ---------------------------------------------------------

func TestAdmin_NilAdminAuth_Returns401(t *testing.T) {
	// With no AdminAuthnMiddleware wired, every /v1/admin/cert route must reject.
	r := New(Deps{})
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/v1/admin/cert/orders"},
		{http.MethodPost, "/v1/admin/cert/orders/42/force-fail"},
		{http.MethodGet, "/v1/admin/cert/ca-quota"},
		{http.MethodGet, "/v1/admin/cert/dns-health"},
		{http.MethodPost, "/v1/admin/cert/accounts/9/ban"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(""))
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			require.Equalf(t, http.StatusUnauthorized, rec.Code, "body=%s", rec.Body.String())
			var body errResp
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			require.Equal(t, codeUnauthorized, body.Code)
		})
	}
}

func TestAdmin_WithMiddleware_PassesThrough(t *testing.T) {
	// A trivial pass-through middleware should let requests reach the
	// handler — which then 500s because repos isn't wired. The point is
	// to confirm the middleware indeed wraps the admin routes.
	deps := Deps{AdminAuthnMiddleware: func(next http.Handler) http.Handler {
		return next
	}}
	r := New(deps)
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/cert/orders", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
}

// --- List orders -----------------------------------------------------------

func TestAdminListOrders_NoRepos_500(t *testing.T) {
	rec := httptest.NewRecorder()
	adminListOrders(Deps{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/admin/cert/orders", nil))
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAdminListOrders_HappyPath(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool)}

	pool.ExpectQuery(`SELECT .+ FROM cert\.orders ORDER BY created_at DESC LIMIT \$1 OFFSET \$2`).
		WithArgs(50, 0).
		WillReturnRows(pgxmock.NewRows(orderRowColumns()).
			AddRow(sampleOrderRow(7, 42, "issued")...))

	req := adminRequest(t, http.MethodGet, "/v1/admin/cert/orders", nil)
	rec := httptest.NewRecorder()
	adminListOrders(deps).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body adminListOrdersResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Orders, 1)
	require.Equal(t, int64(7), body.Orders[0].ID)
	require.Equal(t, 50, body.Limit)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestAdminListOrders_WithFilters(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool)}
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders WHERE status = \$1 AND account_id = \$2 AND ca = \$3 ORDER BY created_at DESC LIMIT \$4 OFFSET \$5`).
		WithArgs("issued", int64(42), "lets-encrypt", 25, 5).
		WillReturnRows(pgxmock.NewRows(orderRowColumns()))

	req := adminRequest(t, http.MethodGet,
		"/v1/admin/cert/orders?status=issued&account_id=42&ca=lets-encrypt&limit=25&offset=5", nil)
	rec := httptest.NewRecorder()
	adminListOrders(deps).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body adminListOrdersResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 25, body.Limit)
	require.Equal(t, 5, body.Offset)
}

func TestAdminListOrders_InvalidAccountID_400(t *testing.T) {
	deps := Deps{Repos: repo.NewWithPool(newMockPool(t))}
	req := adminRequest(t, http.MethodGet, "/v1/admin/cert/orders?account_id=zero", nil)
	rec := httptest.NewRecorder()
	adminListOrders(deps).ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestAdminListOrders_NegativeAccountID_400(t *testing.T) {
	deps := Deps{Repos: repo.NewWithPool(newMockPool(t))}
	req := adminRequest(t, http.MethodGet, "/v1/admin/cert/orders?account_id=-1", nil)
	rec := httptest.NewRecorder()
	adminListOrders(deps).ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminListOrders_DBError_500(t *testing.T) {
	pool := newMockPool(t)
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders`).
		WithArgs(50, 0).
		WillReturnError(errors.New("db down"))
	rec := httptest.NewRecorder()
	adminListOrders(Deps{Repos: repo.NewWithPool(pool)}).
		ServeHTTP(rec, adminRequest(t, http.MethodGet, "/v1/admin/cert/orders", nil))
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAdminListOrders_IncludesFinalizedAt(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool)}
	finalized := time.Now().UTC()
	row := sampleOrderRow(7, 42, "issued")
	// Replace last column (finalized_at) with a non-nil time pointer.
	row[len(row)-1] = &finalized
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders`).
		WithArgs(50, 0).
		WillReturnRows(pgxmock.NewRows(orderRowColumns()).AddRow(row...))
	req := adminRequest(t, http.MethodGet, "/v1/admin/cert/orders", nil)
	rec := httptest.NewRecorder()
	adminListOrders(deps).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body adminListOrdersResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Orders, 1)
	require.NotNil(t, body.Orders[0].FinalizedAt)
}

func TestAdminListOrders_NegativeLimitClampedAndEchoed(t *testing.T) {
	pool := newMockPool(t)
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders`).
		WithArgs(50, 0).
		WillReturnRows(pgxmock.NewRows(orderRowColumns()))
	rec := httptest.NewRecorder()
	adminListOrders(Deps{Repos: repo.NewWithPool(pool)}).
		ServeHTTP(rec, adminRequest(t, http.MethodGet, "/v1/admin/cert/orders?limit=-7&offset=-3", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body adminListOrdersResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 50, body.Limit)
	require.Equal(t, 0, body.Offset)
}

// --- Force fail ------------------------------------------------------------

func TestAdminForceFail_NoRepos_500(t *testing.T) {
	rec := httptest.NewRecorder()
	adminForceFailOrder(Deps{}).
		ServeHTTP(rec, adminRequest(t, http.MethodPost, "/", nil))
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAdminForceFail_HappyPath(t *testing.T) {
	pool := newMockPool(t)
	deps := Deps{Repos: repo.NewWithPool(pool)}

	// GetByID
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE id`).
		WithArgs(int64(101)).
		WillReturnRows(pgxmock.NewRows(orderRowColumns()).
			AddRow(sampleOrderRow(101, 42, "validating")...))
	// UpdateStatus
	pool.ExpectExec(`UPDATE cert\.orders\s+SET status`).
		WithArgs("failed", pgxmock.AnyArg(), int64(101), "validating").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	// NextActionSeq
	pool.ExpectQuery(`SELECT COALESCE\(MAX\(action_seq`).
		WithArgs(int64(101)).
		WillReturnRows(pgxmock.NewRows([]string{"next"}).AddRow(5))
	// Append event
	pool.ExpectQuery(`INSERT INTO cert\.order_events`).
		WithArgs(int64(101), 5, "admin.force_fail", pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "occurred_at"}).
			AddRow(int64(999), time.Now().UTC()))

	r := chiAdminRouter(deps)
	body := adminForceFailRequest{Reason: "operator override"}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, adminRequest(t, http.MethodPost, "/v1/admin/cert/orders/101/force-fail", body))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp adminForceFailResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, int64(101), resp.OrderID)
	require.Equal(t, "failed", resp.Status)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestAdminForceFail_BadID_404(t *testing.T) {
	deps := Deps{Repos: repo.NewWithPool(newMockPool(t))}
	r := chiAdminRouter(deps)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, adminRequest(t, http.MethodPost, "/v1/admin/cert/orders/abc/force-fail", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminForceFail_NotFound_404(t *testing.T) {
	pool := newMockPool(t)
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE id`).
		WithArgs(int64(404)).
		WillReturnError(repoNoRowsError())
	r := chiAdminRouter(Deps{Repos: repo.NewWithPool(pool)})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, adminRequest(t, http.MethodPost, "/v1/admin/cert/orders/404/force-fail", nil))
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

func TestAdminForceFail_GetDBError_500(t *testing.T) {
	pool := newMockPool(t)
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE id`).
		WithArgs(int64(101)).
		WillReturnError(errors.New("io"))
	r := chiAdminRouter(Deps{Repos: repo.NewWithPool(pool)})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, adminRequest(t, http.MethodPost, "/v1/admin/cert/orders/101/force-fail", nil))
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAdminForceFail_AlreadyTerminal_409(t *testing.T) {
	pool := newMockPool(t)
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE id`).
		WithArgs(int64(101)).
		WillReturnRows(pgxmock.NewRows(orderRowColumns()).
			AddRow(sampleOrderRow(101, 42, "issued")...))
	r := chiAdminRouter(Deps{Repos: repo.NewWithPool(pool)})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, adminRequest(t, http.MethodPost, "/v1/admin/cert/orders/101/force-fail", nil))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
}

func TestAdminForceFail_UpdateConflict_409(t *testing.T) {
	pool := newMockPool(t)
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE id`).
		WithArgs(int64(101)).
		WillReturnRows(pgxmock.NewRows(orderRowColumns()).
			AddRow(sampleOrderRow(101, 42, "validating")...))
	pool.ExpectExec(`UPDATE cert\.orders\s+SET status`).
		WithArgs("failed", pgxmock.AnyArg(), int64(101), "validating").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0)) // optimistic lock miss
	r := chiAdminRouter(Deps{Repos: repo.NewWithPool(pool)})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, adminRequest(t, http.MethodPost, "/v1/admin/cert/orders/101/force-fail", nil))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
}

func TestAdminForceFail_UpdateDBError_500(t *testing.T) {
	pool := newMockPool(t)
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE id`).
		WithArgs(int64(101)).
		WillReturnRows(pgxmock.NewRows(orderRowColumns()).
			AddRow(sampleOrderRow(101, 42, "validating")...))
	pool.ExpectExec(`UPDATE cert\.orders\s+SET status`).
		WillReturnError(errors.New("io"))
	r := chiAdminRouter(Deps{Repos: repo.NewWithPool(pool)})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, adminRequest(t, http.MethodPost, "/v1/admin/cert/orders/101/force-fail", nil))
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAdminForceFail_MalformedJSON_400(t *testing.T) {
	deps := Deps{Repos: repo.NewWithPool(newMockPool(t))}
	r := chiAdminRouter(deps)
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/cert/orders/101/force-fail",
		strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- CA quota --------------------------------------------------------------

type fakeQuotaSource struct {
	usage map[string]service.QuotaUsage
	err   error
}

func (f *fakeQuotaSource) Usage(_ context.Context, ca string) (service.QuotaUsage, error) {
	if f.err != nil {
		return service.QuotaUsage{}, f.err
	}
	return f.usage[ca], nil
}

func TestAdminCAQuota_NilSource_503(t *testing.T) {
	rec := httptest.NewRecorder()
	adminCAQuota(Deps{}).
		ServeHTTP(rec, adminRequest(t, http.MethodGet, "/v1/admin/cert/ca-quota", nil))
	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
}

func TestAdminCAQuota_HappyPath(t *testing.T) {
	src := &fakeQuotaSource{usage: map[string]service.QuotaUsage{
		"lets-encrypt": {PerAccount3h: 0.5, PerRegisteredDomain: 0.8},
		"zerossl":      {PerAccount3h: 0.1, PerRegisteredDomain: 0.1},
	}}
	rec := httptest.NewRecorder()
	adminCAQuota(Deps{AdminQuota: src}).
		ServeHTTP(rec, adminRequest(t, http.MethodGet, "/v1/admin/cert/ca-quota", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body adminCAQuotaResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Rows, 3) // lets-encrypt + zerossl + buypass(zero)
	require.Equal(t, service.SwitchThreshold, body.Threshold)
	// LE row should be Switched because 0.8 ≥ 0.70.
	for _, r := range body.Rows {
		if r.CA == "lets-encrypt" {
			require.True(t, r.Switched, "lets-encrypt at 0.8 should trip threshold")
		}
		if r.CA == "zerossl" {
			require.False(t, r.Switched)
		}
		if r.CA == "buypass" {
			// Default zero-value row — never tripped.
			require.False(t, r.Switched)
			require.Equal(t, float64(0), r.PerAccount3h)
		}
	}
}

func TestAdminCAQuota_PerRowError_Surfaced(t *testing.T) {
	src := &fakeQuotaSource{err: errors.New("kaboom")}
	rec := httptest.NewRecorder()
	adminCAQuota(Deps{AdminQuota: src}).
		ServeHTTP(rec, adminRequest(t, http.MethodGet, "/v1/admin/cert/ca-quota", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body adminCAQuotaResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	for _, r := range body.Rows {
		require.NotEmpty(t, r.Err)
	}
}

// --- DNS health ------------------------------------------------------------

type fakeDNSHealth struct {
	totals    map[string]int
	successes map[string]int
	err       error
}

func (f *fakeDNSHealth) CountByActionAndDNSProviderSince(_ context.Context,
	_ []string, _ time.Time,
) (map[string]int, map[string]int, error) {
	if f.err != nil {
		return nil, nil, f.err
	}
	return f.totals, f.successes, nil
}

func TestAdminDNSHealth_NoSource_ReturnsUnknownRows(t *testing.T) {
	rec := httptest.NewRecorder()
	adminDNSHealth(Deps{}).
		ServeHTTP(rec, adminRequest(t, http.MethodGet, "/v1/admin/cert/dns-health", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body adminDNSHealthResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Rows, len(dnsProvidersForHealth))
	for _, r := range body.Rows {
		require.Equal(t, float64(-1), r.SuccessRate, "row %s should be unknown", r.Provider)
		require.Equal(t, 24, r.WindowHours)
	}
}

func TestAdminDNSHealth_HappyPath(t *testing.T) {
	src := &fakeDNSHealth{
		totals:    map[string]int{"cloudflare": 100, "aliyun": 50, "manual": 0},
		successes: map[string]int{"cloudflare": 95, "aliyun": 40, "manual": 0},
	}
	req := adminRequest(t, http.MethodGet, "/v1/admin/cert/dns-health", nil)
	req = req.WithContext(context.WithValue(req.Context(), healthSourceKey{}, dnsHealthSource(src)))
	rec := httptest.NewRecorder()
	adminDNSHealth(Deps{}).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body adminDNSHealthResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	got := map[string]adminDNSHealthRow{}
	for _, r := range body.Rows {
		got[r.Provider] = r
	}
	require.InDelta(t, 0.95, got["cloudflare"].SuccessRate, 0.0001)
	require.Equal(t, 100, got["cloudflare"].Samples)
	require.InDelta(t, 0.8, got["aliyun"].SuccessRate, 0.0001)
	require.Equal(t, float64(-1), got["manual"].SuccessRate) // zero samples → unknown
}

func TestAdminDNSHealth_DBError_500(t *testing.T) {
	src := &fakeDNSHealth{err: errors.New("io")}
	req := adminRequest(t, http.MethodGet, "/v1/admin/cert/dns-health", nil)
	req = req.WithContext(context.WithValue(req.Context(), healthSourceKey{}, dnsHealthSource(src)))
	rec := httptest.NewRecorder()
	adminDNSHealth(Deps{}).ServeHTTP(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- Ban -------------------------------------------------------------------

type fakeAbuseGate struct {
	banned    map[int64]string
	err       error
	unbanErr  error
	unbanned  map[int64]string
}

func (f *fakeAbuseGate) Ban(_ context.Context, id int64, reason string) error {
	if f.err != nil {
		return f.err
	}
	if f.banned == nil {
		f.banned = map[int64]string{}
	}
	f.banned[id] = reason
	return nil
}

func (f *fakeAbuseGate) Unban(_ context.Context, id int64, reason string) error {
	if f.unbanErr != nil {
		return f.unbanErr
	}
	if f.unbanned == nil {
		f.unbanned = map[int64]string{}
	}
	f.unbanned[id] = reason
	delete(f.banned, id)
	return nil
}

func TestAdminBan_NilGate_503(t *testing.T) {
	r := chiAdminRouter(Deps{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, adminRequest(t, http.MethodPost, "/v1/admin/cert/accounts/42/ban", nil))
	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
}

func TestAdminBan_HappyPath_NoAuditRepo(t *testing.T) {
	gate := &fakeAbuseGate{}
	r := chiAdminRouter(Deps{AdminAbuse: gate})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, adminRequest(t, http.MethodPost, "/v1/admin/cert/accounts/42/ban",
		adminBanRequest{Reason: "fraud"}))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "fraud", gate.banned[42])
	var resp adminBanResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "banned", resp.Status)
	require.Equal(t, int64(42), resp.AccountID)
}

func TestAdminBan_DefaultsReason_WhenEmpty(t *testing.T) {
	gate := &fakeAbuseGate{}
	r := chiAdminRouter(Deps{AdminAbuse: gate})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, adminRequest(t, http.MethodPost, "/v1/admin/cert/accounts/42/ban", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "admin ban", gate.banned[42])
}

func TestAdminBan_GateError_500(t *testing.T) {
	gate := &fakeAbuseGate{err: errors.New("kms down")}
	r := chiAdminRouter(Deps{AdminAbuse: gate})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, adminRequest(t, http.MethodPost, "/v1/admin/cert/accounts/42/ban",
		adminBanRequest{Reason: "x"}))
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
}

func TestAdminBan_BadID_404(t *testing.T) {
	gate := &fakeAbuseGate{}
	r := chiAdminRouter(Deps{AdminAbuse: gate})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, adminRequest(t, http.MethodPost, "/v1/admin/cert/accounts/notanint/ban", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminBan_WithAuditRepo_WritesRow(t *testing.T) {
	pool := newMockPool(t)
	pool.ExpectQuery(`INSERT INTO cert\.audit_logs`).
		WithArgs(pgxmock.AnyArg(), "admin", "admin.account_ban", pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "occurred_at"}).
			AddRow(int64(1), time.Now().UTC()))

	gate := &fakeAbuseGate{}
	deps := Deps{AdminAbuse: gate, Repos: repo.NewWithPool(pool)}
	r := chiAdminRouter(deps)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, adminRequest(t, http.MethodPost, "/v1/admin/cert/accounts/42/ban",
		adminBanRequest{Reason: "trademark"}))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestAdminBan_MalformedJSON_400(t *testing.T) {
	gate := &fakeAbuseGate{}
	r := chiAdminRouter(Deps{AdminAbuse: gate})
	req := httptest.NewRequest(http.MethodPost,
		"/v1/admin/cert/accounts/42/ban", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Helpers ---------------------------------------------------------------

// repoNoRowsError returns the canonical "no rows" sentinel that pgx
// surfaces and the repo layer translates into repo.ErrNotFound. Imported
// from pgx directly so the translation in GetByID fires correctly.
func repoNoRowsError() error {
	return pgx.ErrNoRows
}

// --- Unban -----------------------------------------------------------------

func TestAdminUnban_NilGate_503(t *testing.T) {
	r := chiAdminRouter(Deps{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, adminRequest(t, http.MethodPost,
		"/v1/admin/cert/accounts/42/unban", nil))
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestAdminUnban_BadID_404(t *testing.T) {
	// pathInt64 returns 404 (not 400) for invalid IDs — see util.go.
	gate := &fakeAbuseGate{}
	r := chiAdminRouter(Deps{AdminAbuse: gate})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, adminRequest(t, http.MethodPost,
		"/v1/admin/cert/accounts/notanint/unban", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminUnban_NotBanned_404(t *testing.T) {
	gate := &fakeAbuseGate{unbanErr: errors.New("repo: account is not currently banned")}
	r := chiAdminRouter(Deps{AdminAbuse: gate})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, adminRequest(t, http.MethodPost,
		"/v1/admin/cert/accounts/42/unban", nil))
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

func TestAdminUnban_Success(t *testing.T) {
	gate := &fakeAbuseGate{}
	r := chiAdminRouter(Deps{AdminAbuse: gate})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, adminRequest(t, http.MethodPost,
		"/v1/admin/cert/accounts/42/unban",
		adminBanRequest{Reason: "false positive"}))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "false positive", gate.unbanned[42])
}

func TestAdminUnban_DefaultReason(t *testing.T) {
	gate := &fakeAbuseGate{}
	r := chiAdminRouter(Deps{AdminAbuse: gate})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, adminRequest(t, http.MethodPost,
		"/v1/admin/cert/accounts/42/unban", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "admin unban", gate.unbanned[42])
}

func TestAdminUnban_InternalError_500(t *testing.T) {
	gate := &fakeAbuseGate{unbanErr: errors.New("db down")}
	r := chiAdminRouter(Deps{AdminAbuse: gate})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, adminRequest(t, http.MethodPost,
		"/v1/admin/cert/accounts/42/unban", nil))
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAdminUnban_AuditOnlyWhenRepoPresent(t *testing.T) {
	pool := newMockPool(t)
	pool.ExpectQuery(`INSERT INTO cert\.audit_logs`).
		WithArgs(pgxmock.AnyArg(), "admin", "admin.account_unban",
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "occurred_at"}).
			AddRow(int64(11), time.Now().UTC()))

	gate := &fakeAbuseGate{}
	deps := Deps{AdminAbuse: gate, Repos: repo.NewWithPool(pool)}
	r := chiAdminRouter(deps)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, adminRequest(t, http.MethodPost,
		"/v1/admin/cert/accounts/42/unban",
		adminBanRequest{Reason: "review complete"}))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, pool.ExpectationsWereMet())
}
