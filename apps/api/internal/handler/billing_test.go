package handler

import (
	"bytes"
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

	"github.com/kite365/idcd/apps/api/internal/billing"
	"github.com/kite365/idcd/apps/api/internal/middleware"
)

// ---- helpers ----

func newBillingTestHandler(t *testing.T) (*BillingHandler, pgxmock.PgxPoolIface, *billing.StubProvider) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	stub := billing.NewStubProvider()
	h := NewBillingHandler(mockPool, stub)
	return h, mockPool, stub
}

func withUser(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDContextKey(), userID)
	return r.WithContext(ctx)
}

func withRequestID(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), "request_id", "test-req-id")
	return r.WithContext(ctx)
}

func prepReq(r *http.Request, userID string) *http.Request {
	r = withUser(r, userID)
	r = withRequestID(r)
	return r
}

// ---- Subscribe ----

func TestBillingHandler_Subscribe_Success(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()

	mockPool.ExpectExec("INSERT INTO subscriptions").
		WithArgs(
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body, _ := json.Marshal(map[string]string{"plan": "pro"})
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/subscribe", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = prepReq(req, "u_test_user")
	rr := httptest.NewRecorder()

	h.Subscribe(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Data subscribeResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Data.SubscriptionID)
	assert.Contains(t, resp.Data.PayURL, "/billing/stub-confirm")
	assert.False(t, resp.Data.ExpiresAt.IsZero())
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestBillingHandler_Subscribe_NoAuth(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()

	body, _ := json.Marshal(map[string]string{"plan": "pro"})
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/subscribe", bytes.NewReader(body))
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.Subscribe(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestBillingHandler_Subscribe_InvalidPlan(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()

	body, _ := json.Marshal(map[string]string{"plan": "diamond"})
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/subscribe", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = prepReq(req, "u_test_user")
	rr := httptest.NewRecorder()

	h.Subscribe(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestBillingHandler_Subscribe_InvalidBody(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/billing/subscribe", bytes.NewReader([]byte("not json")))
	req = prepReq(req, "u_test_user")
	rr := httptest.NewRecorder()

	h.Subscribe(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ---- Cancel ----

func TestBillingHandler_Cancel_Success(t *testing.T) {
	h, mockPool, stub := newBillingTestHandler(t)
	defer mockPool.Close()

	// Pre-create a subscription in the stub so provider.Cancel succeeds.
	res, err := stub.Subscribe(context.Background(), billing.SubscribeRequest{
		UserID: "u_test_user", Plan: billing.PlanPro,
	})
	require.NoError(t, err)
	subID := res.SubscriptionID

	rows := pgxmock.NewRows([]string{"id", "provider"}).
		AddRow(subID, "stub")
	mockPool.ExpectQuery("SELECT id, provider FROM subscriptions").
		WithArgs("u_test_user").
		WillReturnRows(rows)
	mockPool.ExpectExec("UPDATE subscriptions").
		WithArgs(subID, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	req := httptest.NewRequest(http.MethodPost, "/v1/billing/cancel", nil)
	req = prepReq(req, "u_test_user")
	rr := httptest.NewRecorder()

	h.Cancel(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestBillingHandler_Cancel_NoAuth(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/billing/cancel", nil)
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.Cancel(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestBillingHandler_Cancel_NoActiveSubscription(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()

	rows := pgxmock.NewRows([]string{"id", "provider"}) // empty
	mockPool.ExpectQuery("SELECT id, provider FROM subscriptions").
		WithArgs("u_test_user").
		WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodPost, "/v1/billing/cancel", nil)
	req = prepReq(req, "u_test_user")
	rr := httptest.NewRecorder()

	h.Cancel(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

// ---- GetSubscription ----

func TestBillingHandler_GetSubscription_Found(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()

	now := time.Now().UTC().Truncate(time.Second)
	rows := pgxmock.NewRows([]string{
		"id", "plan", "status", "provider", "ext_sub_id",
		"current_period_start", "current_period_end", "cancel_at", "created_at",
	}).AddRow("sub_abc123", "pro", "active", "stub", nil, &now, &now, nil, now)

	mockPool.ExpectQuery("SELECT id, plan, status").
		WithArgs("u_test_user").
		WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodGet, "/v1/billing/subscription", nil)
	req = prepReq(req, "u_test_user")
	rr := httptest.NewRecorder()

	h.GetSubscription(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Data subscriptionResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "sub_abc123", resp.Data.ID)
	assert.Equal(t, "pro", resp.Data.Plan)
	assert.Equal(t, "active", resp.Data.Status)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestBillingHandler_GetSubscription_NotFound(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()

	rows := pgxmock.NewRows([]string{
		"id", "plan", "status", "provider", "ext_sub_id",
		"current_period_start", "current_period_end", "cancel_at", "created_at",
	}) // empty

	mockPool.ExpectQuery("SELECT id, plan, status").
		WithArgs("u_test_user").
		WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodGet, "/v1/billing/subscription", nil)
	req = prepReq(req, "u_test_user")
	rr := httptest.NewRecorder()

	h.GetSubscription(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

// ---- ListInvoices ----

func TestBillingHandler_ListInvoices_Empty(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()

	rows := pgxmock.NewRows([]string{
		"id", "subscription_id", "amount_cents", "currency", "status",
		"provider", "ext_invoice_id", "paid_at", "created_at",
	})
	mockPool.ExpectQuery("SELECT id, subscription_id").
		WithArgs("u_test_user", 20, 0).
		WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodGet, "/v1/billing/invoices", nil)
	req = prepReq(req, "u_test_user")
	rr := httptest.NewRecorder()

	h.ListInvoices(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Data invoicesResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Data.Total)
	assert.Empty(t, resp.Data.Invoices)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestBillingHandler_ListInvoices_WithData(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()

	now := time.Now().UTC().Truncate(time.Second)
	subID := "sub_xyz"
	rows := pgxmock.NewRows([]string{
		"id", "subscription_id", "amount_cents", "currency", "status",
		"provider", "ext_invoice_id", "paid_at", "created_at",
	}).AddRow("inv_001", &subID, int64(9900), "CNY", "paid", "stub", nil, &now, now)

	mockPool.ExpectQuery("SELECT id, subscription_id").
		WithArgs("u_test_user", 20, 0).
		WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodGet, "/v1/billing/invoices", nil)
	req = prepReq(req, "u_test_user")
	rr := httptest.NewRecorder()

	h.ListInvoices(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Data invoicesResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Data.Total)
	assert.Equal(t, "inv_001", resp.Data.Invoices[0].ID)
	assert.Equal(t, int64(9900), resp.Data.Invoices[0].AmountCents)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestBillingHandler_ListInvoices_NoAuth(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()

	req := httptest.NewRequest(http.MethodGet, "/v1/billing/invoices", nil)
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.ListInvoices(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// ---- Webhook ----

func TestBillingHandler_Webhook_PaymentSucceeded(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()

	mockPool.ExpectExec("UPDATE subscriptions").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mockPool.ExpectExec("INSERT INTO payments").
		WithArgs(
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mockPool.ExpectExec("INSERT INTO invoices").
		WithArgs(
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	payload := billing.StubWebhookPayload{
		EventType:      billing.EventPaymentSucceeded,
		ExtTxnID:       "stub_txn_abc",
		ExtSubID:       "stub_sub_abc",
		AmountCents:    9900,
		Currency:       "CNY",
		UserID:         "u_test_user",
		SubscriptionID: "sub_abc123",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhook", bytes.NewReader(body))
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.Webhook(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestBillingHandler_Webhook_EmptyBody(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhook", bytes.NewReader([]byte{}))
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.Webhook(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestBillingHandler_Webhook_InvalidPayload(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhook", bytes.NewReader([]byte("not json")))
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.Webhook(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ---- StubConfirm ----

func TestBillingHandler_StubConfirm_Success(t *testing.T) {
	h, mockPool, stub := newBillingTestHandler(t)
	defer mockPool.Close()

	// Pre-create a subscription in the stub.
	res, err := stub.Subscribe(context.Background(), billing.SubscribeRequest{
		UserID: "u_test_user", Plan: billing.PlanPro,
	})
	require.NoError(t, err)

	extSubID := res.ExtSubID // pass as *string to match scan target
	rows := pgxmock.NewRows([]string{"user_id", "plan", "provider", "ext_sub_id"}).
		AddRow("u_test_user", "pro", "stub", &extSubID)
	mockPool.ExpectQuery("SELECT user_id, plan, provider, ext_sub_id").
		WithArgs(res.SubscriptionID).
		WillReturnRows(rows)
	mockPool.ExpectExec("UPDATE subscriptions").
		WithArgs(res.SubscriptionID, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mockPool.ExpectExec("INSERT INTO invoices").
		WithArgs(
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(),
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mockPool.ExpectExec("INSERT INTO payments").
		WithArgs(
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(),
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	req := httptest.NewRequest(http.MethodGet, "/v1/billing/stub-confirm?sub_id="+res.SubscriptionID+"&plan=pro", nil)
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.StubConfirm(rr, req)

	assert.Equal(t, http.StatusFound, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "/app/billing?success=1")
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestBillingHandler_StubConfirm_MissingSubID(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()

	req := httptest.NewRequest(http.MethodGet, "/v1/billing/stub-confirm", nil)
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.StubConfirm(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestBillingHandler_StubConfirm_SubNotInDB(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()

	rows := pgxmock.NewRows([]string{"user_id", "plan", "provider", "ext_sub_id"}) // empty
	mockPool.ExpectQuery("SELECT user_id, plan, provider, ext_sub_id").
		WithArgs("sub_nonexistent").
		WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodGet, "/v1/billing/stub-confirm?sub_id=sub_nonexistent", nil)
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.StubConfirm(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

// ---- NewBillingHandler ----

func TestNewBillingHandler_NotNil(t *testing.T) {
	h := NewBillingHandler(nil, billing.NewStubProvider())
	assert.NotNil(t, h)
}

// ---- parseIntParam ----

func TestParseIntParam(t *testing.T) {
	assert.Equal(t, 1, parseIntParam("", 1))
	assert.Equal(t, 5, parseIntParam("5", 1))
	assert.Equal(t, 1, parseIntParam("abc", 1))
	assert.Equal(t, 1, parseIntParam("-1", 1))
}

// ---- chi URL param helper for router test ----

func newChiCtxWithParam(r *http.Request, key, value string) *http.Request {
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add(key, value)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, chiCtx)
	return r.WithContext(ctx)
}
