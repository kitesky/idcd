package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAdminToken = "test-admin-secret-token"

// newAdminTestHandler creates an AdminHandler backed by a pgxmock pool.
func newAdminTestHandler(t *testing.T) (*AdminHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	h := NewAdminHandler(mockPool, testAdminToken)
	return h, mockPool
}

// reqWithAdminToken adds the admin Bearer token to a request.
func reqWithAdminToken(req *http.Request) *http.Request {
	req.Header.Set("Authorization", "Bearer "+testAdminToken)
	return req
}

// withRequestID sets a request_id in the context (needed by response package).
func withRequestID(req *http.Request, id string) *http.Request {
	ctx := context.WithValue(req.Context(), "request_id", id)
	return req.WithContext(ctx)
}

// --- AdminAuthMiddleware tests ---

func TestAdminAuthMiddleware_NoToken(t *testing.T) {
	h, mockPool := newAdminTestHandler(t)
	defer mockPool.Close()

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	handler := h.AdminAuthMiddleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/internal/admin/metrics", nil)
	req = withRequestID(req, "test-no-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.False(t, called, "inner handler should not be called")
}

func TestAdminAuthMiddleware_WrongToken(t *testing.T) {
	h, mockPool := newAdminTestHandler(t)
	defer mockPool.Close()

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	handler := h.AdminAuthMiddleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/internal/admin/metrics", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	req = withRequestID(req, "test-wrong-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.False(t, called)
}

func TestAdminAuthMiddleware_CorrectToken(t *testing.T) {
	h, mockPool := newAdminTestHandler(t)
	defer mockPool.Close()

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := h.AdminAuthMiddleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/internal/admin/metrics", nil)
	req = reqWithAdminToken(req)
	req = withRequestID(req, "test-correct-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, called)
}

// --- AdminMetrics tests ---

func TestAdminMetrics_ReturnsCorrectFormat(t *testing.T) {
	h, mockPool := newAdminTestHandler(t)
	defer mockPool.Close()

	// Expect queries in order
	mockPool.ExpectQuery(`SELECT COUNT\(\*\) FROM "user"`).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1234))
	mockPool.ExpectQuery(`SELECT COUNT\(\*\) FROM "user" WHERE created_at`).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(456))
	mockPool.ExpectQuery(`SELECT COUNT\(\*\) FROM monitors`).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(789))
	mockPool.ExpectQuery(`SELECT COUNT\(\*\) FROM monitors WHERE status`).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(650))
	mockPool.ExpectQuery(`SELECT COUNT\(\*\) FROM enrolled_nodes`).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(100))
	mockPool.ExpectQuery(`SELECT COUNT\(\*\) FROM enrolled_nodes WHERE status`).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(95))
	mockPool.ExpectQuery(`SELECT plan, COUNT\(\*\) FROM subscriptions`).
		WillReturnRows(pgxmock.NewRows([]string{"plan", "count"}).
			AddRow("free", 900).
			AddRow("pro", 300).
			AddRow("team", 30).
			AddRow("enterprise", 4))

	req := httptest.NewRequest(http.MethodGet, "/internal/admin/metrics", nil)
	req = withRequestID(req, "test-metrics")
	rr := httptest.NewRecorder()

	h.AdminMetrics(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Data AdminMetricsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))

	d := resp.Data
	assert.Equal(t, 1234, d.TotalUsers)
	assert.Equal(t, 456, d.ActiveUsers7d)
	assert.Equal(t, 789, d.TotalMonitors)
	assert.Equal(t, 650, d.ActiveMonitors)
	assert.Equal(t, 100, d.TotalNodes)
	assert.Equal(t, 95, d.OnlineNodes)
	assert.Equal(t, 900, d.Subscriptions.Free)
	assert.Equal(t, 300, d.Subscriptions.Pro)
	assert.Equal(t, 30, d.Subscriptions.Team)
	assert.Equal(t, 4, d.Subscriptions.Enterprise)
	// MRR: pro*99 + team*299 + enterprise*999 = 29700 + 8970 + 3996 = 42666
	assert.Equal(t, 300*99+30*299+4*999, d.MRREstimateCNY)

	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestAdminMetrics_EmptyDB(t *testing.T) {
	h, mockPool := newAdminTestHandler(t)
	defer mockPool.Close()

	mockPool.ExpectQuery(`SELECT COUNT\(\*\) FROM "user"`).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))
	mockPool.ExpectQuery(`SELECT COUNT\(\*\) FROM "user" WHERE created_at`).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))
	mockPool.ExpectQuery(`SELECT COUNT\(\*\) FROM monitors`).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))
	mockPool.ExpectQuery(`SELECT COUNT\(\*\) FROM monitors WHERE status`).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))
	mockPool.ExpectQuery(`SELECT COUNT\(\*\) FROM enrolled_nodes`).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))
	mockPool.ExpectQuery(`SELECT COUNT\(\*\) FROM enrolled_nodes WHERE status`).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))
	mockPool.ExpectQuery(`SELECT plan, COUNT\(\*\) FROM subscriptions`).
		WillReturnRows(pgxmock.NewRows([]string{"plan", "count"}))

	req := httptest.NewRequest(http.MethodGet, "/internal/admin/metrics", nil)
	req = withRequestID(req, "test-metrics-empty")
	rr := httptest.NewRecorder()

	h.AdminMetrics(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp struct {
		Data AdminMetricsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Data.TotalUsers)
	assert.Equal(t, 0, resp.Data.MRREstimateCNY)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

// --- AdminUsers tests ---

func TestAdminUsers_DefaultPagination(t *testing.T) {
	h, mockPool := newAdminTestHandler(t)
	defer mockPool.Close()

	now := time.Now().UTC().Truncate(time.Second)

	// Count query (no search)
	mockPool.ExpectQuery(`SELECT COUNT\(\*\) FROM "user"`).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))

	// Users query — page=1, per_page=20 → LIMIT 20 OFFSET 0
	mockPool.ExpectQuery(`SELECT`).
		WithArgs(20, 0).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "email", "status", "plan", "monitor_count", "created_at",
		}).
			AddRow("usr_001", "alice@example.com", "active", "pro", 5, now).
			AddRow("usr_002", "bob@example.com", "active", "free", 0, now))

	req := httptest.NewRequest(http.MethodGet, "/internal/admin/users", nil)
	req = withRequestID(req, "test-users")
	rr := httptest.NewRecorder()

	h.AdminUsers(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Data AdminUsersResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Data.Total)
	assert.Equal(t, 1, resp.Data.Page)
	assert.Equal(t, 20, resp.Data.PerPage)
	require.Len(t, resp.Data.Users, 2)
	assert.Equal(t, "usr_001", resp.Data.Users[0].ID)
	assert.Equal(t, "pro", resp.Data.Users[0].Plan)
	assert.Equal(t, 5, resp.Data.Users[0].MonitorCount)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestAdminUsers_SearchFilter(t *testing.T) {
	h, mockPool := newAdminTestHandler(t)
	defer mockPool.Close()

	now := time.Now().UTC().Truncate(time.Second)

	// Count query with ILIKE
	mockPool.ExpectQuery(`SELECT COUNT\(\*\) FROM "user" WHERE email ILIKE`).
		WithArgs("%alice%").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	// Users query with ILIKE — args: likeQ, per_page=20, offset=0
	mockPool.ExpectQuery(`SELECT`).
		WithArgs("%alice%", 20, 0).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "email", "status", "plan", "monitor_count", "created_at",
		}).AddRow("usr_001", "alice@example.com", "active", "pro", 3, now))

	req := httptest.NewRequest(http.MethodGet, "/internal/admin/users?q=alice", nil)
	req = withRequestID(req, "test-users-search")
	rr := httptest.NewRecorder()

	h.AdminUsers(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp struct {
		Data AdminUsersResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Data.Total)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestAdminUsers_PageTwo(t *testing.T) {
	h, mockPool := newAdminTestHandler(t)
	defer mockPool.Close()

	now := time.Now().UTC().Truncate(time.Second)

	mockPool.ExpectQuery(`SELECT COUNT\(\*\) FROM "user"`).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(25))
	// page=2, per_page=20 → LIMIT 20 OFFSET 20
	mockPool.ExpectQuery(`SELECT`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "email", "status", "plan", "monitor_count", "created_at",
		}).AddRow("usr_021", "charlie@example.com", "active", "free", 0, now))

	req := httptest.NewRequest(http.MethodGet, "/internal/admin/users?page=2&per_page=20", nil)
	req = withRequestID(req, "test-users-page2")
	rr := httptest.NewRecorder()

	h.AdminUsers(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp struct {
		Data AdminUsersResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Data.Page)
	assert.Equal(t, 25, resp.Data.Total)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

// --- AdminUserDetail tests ---

func TestAdminUserDetail_NotFound(t *testing.T) {
	h, mockPool := newAdminTestHandler(t)
	defer mockPool.Close()

	mockPool.ExpectQuery(`SELECT`).
		WithArgs("usr_nonexistent").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "email", "status", "plan", "monitor_count", "created_at",
		}))

	req := httptest.NewRequest(http.MethodGet, "/internal/admin/users/usr_nonexistent", nil)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("id", "usr_nonexistent")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx)
	req = withRequestID(req.WithContext(ctx), "test-detail-notfound")
	rr := httptest.NewRecorder()

	h.AdminUserDetail(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestAdminUserDetail_MissingID(t *testing.T) {
	h, mockPool := newAdminTestHandler(t)
	defer mockPool.Close()

	req := httptest.NewRequest(http.MethodGet, "/internal/admin/users/", nil)
	req = withRequestID(req, "test-detail-missing-id")
	rr := httptest.NewRecorder()

	h.AdminUserDetail(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestNewAdminHandler(t *testing.T) {
	h := NewAdminHandler(nil, "token")
	assert.NotNil(t, h)
}
