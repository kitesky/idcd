package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/api/internal/middleware"
)

const testBetaAdminToken = "test-beta-admin-token"

func newBetaTestHandler(t *testing.T) (*BetaInvitationHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	return NewBetaInvitationHandler(mockPool), mockPool
}

func betaColumns() []string {
	return []string{
		"id", "code", "email", "status", "requested_by", "approved_by",
		"used_by", "used_at", "expires_at", "created_at", "updated_at",
	}
}

func betaRow(id, code, status string, requestedBy *string) []any {
	now := time.Now().UTC().Truncate(time.Second)
	return []any{id, code, (*string)(nil), status, requestedBy, (*string)(nil), (*string)(nil), (*time.Time)(nil), (*time.Time)(nil), now, now}
}

func injectBetaUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDContextKey(), userID)
	return r.WithContext(ctx)
}

func withReqID(r *http.Request, id string) *http.Request {
	ctx := context.WithValue(r.Context(), "request_id", id)
	return r.WithContext(ctx)
}

// --- POST /v1/beta/request ---

func TestRequestBeta_Success(t *testing.T) {
	h, mockPool := newBetaTestHandler(t)
	defer mockPool.Close()

	uid := "u_testuser"

	mockPool.ExpectQuery(`SELECT id FROM beta_invitations WHERE requested_by`).
		WithArgs(uid).
		WillReturnError(errors.New("no rows"))

	mockPool.ExpectQuery(`INSERT INTO beta_invitations`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), uid, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows(betaColumns()).AddRow(betaRow("bid_abc", "", "pending", &uid)...))

	req := httptest.NewRequest(http.MethodPost, "/v1/beta/request", nil)
	req = injectBetaUserID(req, uid)
	req = withReqID(req, "test-request-beta")
	rr := httptest.NewRecorder()

	h.RequestBeta(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var resp struct {
		Data betaInvitationResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "bid_abc", resp.Data.ID)
	assert.Equal(t, "pending", resp.Data.Status)

	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestRequestBeta_Unauthenticated(t *testing.T) {
	h, mockPool := newBetaTestHandler(t)
	defer mockPool.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/beta/request", nil)
	req = withReqID(req, "test-request-beta-unauth")
	rr := httptest.NewRecorder()

	h.RequestBeta(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestRequestBeta_AlreadyExists(t *testing.T) {
	h, mockPool := newBetaTestHandler(t)
	defer mockPool.Close()

	uid := "u_existing"
	mockPool.ExpectQuery(`SELECT id FROM beta_invitations WHERE requested_by`).
		WithArgs(uid).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("bid_existing"))

	req := httptest.NewRequest(http.MethodPost, "/v1/beta/request", nil)
	req = injectBetaUserID(req, uid)
	req = withReqID(req, "test-request-beta-exists")
	rr := httptest.NewRecorder()

	h.RequestBeta(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

// --- POST /v1/beta/redeem ---

func TestRedeemBeta_Success(t *testing.T) {
	h, mockPool := newBetaTestHandler(t)
	defer mockPool.Close()

	uid := "u_redeemer"
	code := "ABCD1234"
	invID := "bid_inv1"

	mockPool.ExpectQuery(`SELECT id, code, email, status`).
		WithArgs(code).
		WillReturnRows(pgxmock.NewRows(betaColumns()).AddRow(betaRow(invID, code, "approved", nil)...))

	// CAS-style claim now matches WHERE code = $3 (not id) and bakes the
	// status/expiry checks into the UPDATE so the third arg is the code.
	mockPool.ExpectQuery(`UPDATE beta_invitations`).
		WithArgs(uid, pgxmock.AnyArg(), code).
		WillReturnRows(pgxmock.NewRows(betaColumns()).AddRow(betaRow(invID, code, "used", nil)...))

	body, _ := json.Marshal(map[string]string{"code": code})
	req := httptest.NewRequest(http.MethodPost, "/v1/beta/redeem", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = injectBetaUserID(req, uid)
	req = withReqID(req, "test-redeem-beta")
	rr := httptest.NewRecorder()

	h.RedeemBeta(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Data betaInvitationResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "used", resp.Data.Status)

	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestRedeemBeta_InvalidCode(t *testing.T) {
	h, mockPool := newBetaTestHandler(t)
	defer mockPool.Close()

	uid := "u_redeemer"
	code := "NOTFOUND"

	mockPool.ExpectQuery(`SELECT id, code, email, status`).
		WithArgs(code).
		WillReturnError(errors.New("no rows"))

	body, _ := json.Marshal(map[string]string{"code": code})
	req := httptest.NewRequest(http.MethodPost, "/v1/beta/redeem", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = injectBetaUserID(req, uid)
	req = withReqID(req, "test-redeem-notfound")
	rr := httptest.NewRecorder()

	h.RedeemBeta(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestRedeemBeta_Unauthenticated(t *testing.T) {
	h, mockPool := newBetaTestHandler(t)
	defer mockPool.Close()

	body, _ := json.Marshal(map[string]string{"code": "ABCD1234"})
	req := httptest.NewRequest(http.MethodPost, "/v1/beta/redeem", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withReqID(req, "test-redeem-unauth")
	rr := httptest.NewRecorder()

	h.RedeemBeta(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

// --- GET /v1/admin/beta-invitations ---

func TestAdminListBetaInvitations_RequiresAdminToken(t *testing.T) {
	_, mockPool := newBetaTestHandler(t)
	defer mockPool.Close()

	adminH := NewAdminHandler(mockPool, testBetaAdminToken)

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	handler := adminH.AdminAuthMiddleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/beta-invitations", nil)
	req = withReqID(req, "test-admin-list-notoken")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.False(t, called)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestAdminListBetaInvitations_Success(t *testing.T) {
	h, mockPool := newBetaTestHandler(t)
	defer mockPool.Close()

	uid1 := "u_user1"
	mockPool.ExpectQuery(`SELECT id, code, email, status`).
		WillReturnRows(pgxmock.NewRows(betaColumns()).
			AddRow(betaRow("bid_1", "AAAAAAAA", "pending", &uid1)...).
			AddRow(betaRow("bid_2", "BBBBBBBB", "approved", nil)...))

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/beta-invitations", nil)
	req = withReqID(req, "test-admin-list")
	rr := httptest.NewRecorder()

	h.AdminListBetaInvitations(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Data struct {
			Invitations []betaInvitationResponse `json:"invitations"`
			Total       int                      `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Data.Total)
	assert.Len(t, resp.Data.Invitations, 2)

	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestAdminListBetaInvitations_StatusFilter(t *testing.T) {
	h, mockPool := newBetaTestHandler(t)
	defer mockPool.Close()

	mockPool.ExpectQuery(`SELECT id, code, email, status`).
		WithArgs("pending").
		WillReturnRows(pgxmock.NewRows(betaColumns()))

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/beta-invitations?status=pending", nil)
	req = withReqID(req, "test-admin-list-filter")
	rr := httptest.NewRecorder()

	h.AdminListBetaInvitations(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

// --- POST /v1/admin/beta-invitations ---

func TestAdminCreateBetaInvitation_Success(t *testing.T) {
	h, mockPool := newBetaTestHandler(t)
	defer mockPool.Close()

	mockPool.ExpectQuery(`INSERT INTO beta_invitations`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows(betaColumns()).AddRow(betaRow("bid_new", "ZZZZZZZZ", "approved", nil)...))

	body, _ := json.Marshal(map[string]any{"email": "", "expires_days": 30})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/beta-invitations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withReqID(req, "test-admin-create")
	rr := httptest.NewRecorder()

	h.AdminCreateBetaInvitation(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var resp struct {
		Data betaInvitationResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "approved", resp.Data.Status)

	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestAdminCreateBetaInvitation_WithEmail(t *testing.T) {
	h, mockPool := newBetaTestHandler(t)
	defer mockPool.Close()

	email := "invite@example.com"
	mockPool.ExpectQuery(`INSERT INTO beta_invitations`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows(betaColumns()).AddRow(betaRow("bid_email", "EMAILCODE", "approved", nil)...))

	body, _ := json.Marshal(map[string]any{"email": email, "expires_days": 7})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/beta-invitations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withReqID(req, "test-admin-create-email")
	rr := httptest.NewRecorder()

	h.AdminCreateBetaInvitation(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

// --- PATCH /v1/admin/beta-invitations/{id} ---

func TestAdminUpdateBetaInvitation_Approve(t *testing.T) {
	h, mockPool := newBetaTestHandler(t)
	defer mockPool.Close()

	invID := "bid_pending"
	mockPool.ExpectQuery(`UPDATE beta_invitations`).
		WithArgs("approved", pgxmock.AnyArg(), pgxmock.AnyArg(), invID).
		WillReturnRows(pgxmock.NewRows(betaColumns()).AddRow(betaRow(invID, "NEWCODE1", "approved", nil)...))

	body, _ := json.Marshal(map[string]string{"action": "approve"})
	req := httptest.NewRequest(http.MethodPatch, "/v1/admin/beta-invitations/"+invID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withReqID(req, "test-admin-approve")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", invID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	h.AdminUpdateBetaInvitation(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Data betaInvitationResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "approved", resp.Data.Status)

	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestAdminUpdateBetaInvitation_Revoke(t *testing.T) {
	h, mockPool := newBetaTestHandler(t)
	defer mockPool.Close()

	invID := "bid_active"
	mockPool.ExpectQuery(`UPDATE beta_invitations`).
		WithArgs("revoked", pgxmock.AnyArg(), invID).
		WillReturnRows(pgxmock.NewRows(betaColumns()).AddRow(betaRow(invID, "REVOKED1", "revoked", nil)...))

	body, _ := json.Marshal(map[string]string{"action": "revoke"})
	req := httptest.NewRequest(http.MethodPatch, "/v1/admin/beta-invitations/"+invID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withReqID(req, "test-admin-revoke")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", invID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	h.AdminUpdateBetaInvitation(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestAdminUpdateBetaInvitation_InvalidAction(t *testing.T) {
	h, mockPool := newBetaTestHandler(t)
	defer mockPool.Close()

	invID := "bid_x"
	body, _ := json.Marshal(map[string]string{"action": "delete"})
	req := httptest.NewRequest(http.MethodPatch, "/v1/admin/beta-invitations/"+invID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withReqID(req, "test-admin-invalid-action")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", invID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	h.AdminUpdateBetaInvitation(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}
