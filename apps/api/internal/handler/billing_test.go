package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func prepReq(r *http.Request, userID string) *http.Request {
	r = withUser(r, userID)
	r = withRequestID(r, "test-req-id")
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
	req = withRequestID(req, "test-req-id")
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
	req = withRequestID(req, "test-req-id")
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
	req = withRequestID(req, "test-req-id")
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
	req = withRequestID(req, "test-req-id")
	rr := httptest.NewRecorder()

	h.Webhook(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestBillingHandler_Webhook_EmptyBody(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhook", bytes.NewReader([]byte{}))
	req = withRequestID(req, "test-req-id")
	rr := httptest.NewRecorder()

	h.Webhook(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestBillingHandler_Webhook_InvalidPayload(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhook", bytes.NewReader([]byte("not json")))
	req = withRequestID(req, "test-req-id")
	rr := httptest.NewRecorder()

	h.Webhook(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// stubVerdictPublisher records EnqueueVerdict calls for assertions.
type stubVerdictPublisher struct {
	calls []stubVerdictCall
	err   error
}

type stubVerdictCall struct {
	OrderID string
	OwnerID string
}

func (s *stubVerdictPublisher) EnqueueVerdict(_ context.Context, orderID, ownerID string) error {
	if s.err != nil {
		return s.err
	}
	s.calls = append(s.calls, stubVerdictCall{OrderID: orderID, OwnerID: ownerID})
	return nil
}

// TestBillingHandler_Webhook_PaymentSucceeded_VerdictOrderEnqueues asserts the
// S2 Evidence flow: when a payment.succeeded webhook carries
// metadata.verdict_order_id, the handler MUST
//   1. flip the verdict_order row pending→paid (with paid_at + ext_order_id + price_paid_cny),
//   2. push the order onto the Redis Stream `verdict_generation_queue` via
//      the VerdictStreamPublisher.
func TestBillingHandler_Webhook_PaymentSucceeded_VerdictOrderEnqueues(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()
	pub := &stubVerdictPublisher{}
	h = h.WithVerdictPublisher(pub)

	// 1. subscription activation UPDATE — fires when SubscriptionID != "".
	//    For verdict orders the subscription_id is empty, so no UPDATE on
	//    subscriptions. But we DO write a payment + invoice (AmountCents > 0).
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
	// 2. verdict_order UPDATE pending→paid.
	mockPool.ExpectExec(`UPDATE idcd_attest\.verdict_order`).
		WithArgs(
			"v_test_001",      // id
			pgxmock.AnyArg(),  // paid_at
			"stub_ext_txn_v1", // ext_order_id (= ExtTxnID)
			float64(199),      // price_paid_cny (yuan)
		).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	payload := billing.StubWebhookPayload{
		EventType:   billing.EventPaymentSucceeded,
		ExtTxnID:    "stub_ext_txn_v1",
		AmountCents: 19900,
		Currency:    "CNY",
		UserID:      "u_buyer",
		// SubscriptionID intentionally empty — verdict orders are one-shot.
		Metadata: map[string]string{
			"verdict_order_id": "v_test_001",
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhook", bytes.NewReader(body))
	req = withRequestID(req, "test-req-verdict")
	rr := httptest.NewRecorder()

	h.Webhook(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "body=%s", rr.Body.String())
	require.Len(t, pub.calls, 1, "verdict should be enqueued exactly once")
	assert.Equal(t, "v_test_001", pub.calls[0].OrderID)
	assert.Equal(t, "u_buyer", pub.calls[0].OwnerID)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

// TestBillingHandler_Webhook_PaymentSucceeded_VerdictDuplicateNoEnqueue checks
// that a replayed webhook (verdict_order already 'paid') does NOT re-push the
// generation job — guards against duplicate Stream entries.
func TestBillingHandler_Webhook_PaymentSucceeded_VerdictDuplicateNoEnqueue(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()
	pub := &stubVerdictPublisher{}
	h = h.WithVerdictPublisher(pub)

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
	// UPDATE matches WHERE status='pending' but no rows change (already paid).
	mockPool.ExpectExec(`UPDATE idcd_attest\.verdict_order`).
		WithArgs(
			"v_test_002", pgxmock.AnyArg(), "stub_ext_txn_v2", float64(199),
		).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	payload := billing.StubWebhookPayload{
		EventType:   billing.EventPaymentSucceeded,
		ExtTxnID:    "stub_ext_txn_v2",
		AmountCents: 19900,
		Currency:    "CNY",
		UserID:      "u_buyer",
		Metadata:    map[string]string{"verdict_order_id": "v_test_002"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhook", bytes.NewReader(body))
	req = withRequestID(req, "test-req-verdict-dup")
	rr := httptest.NewRecorder()

	h.Webhook(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Empty(t, pub.calls, "duplicate webhook must not re-enqueue")
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

// TestBillingHandler_Webhook_PaymentSucceeded_VerdictPublisherFailDoesNotFailWebhook
// verifies that an enqueue failure is logged-and-swallowed so the gateway
// still gets a 200 (otherwise it would retry forever).
func TestBillingHandler_Webhook_PaymentSucceeded_VerdictPublisherFailDoesNotFailWebhook(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()
	pub := &stubVerdictPublisher{err: assertEnqueueError{}}
	h = h.WithVerdictPublisher(pub)

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
	mockPool.ExpectExec(`UPDATE idcd_attest\.verdict_order`).
		WithArgs(
			"v_test_003", pgxmock.AnyArg(), "stub_ext_txn_v3", float64(199),
		).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	payload := billing.StubWebhookPayload{
		EventType:   billing.EventPaymentSucceeded,
		ExtTxnID:    "stub_ext_txn_v3",
		AmountCents: 19900,
		Currency:    "CNY",
		UserID:      "u_buyer",
		Metadata:    map[string]string{"verdict_order_id": "v_test_003"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhook", bytes.NewReader(body))
	req = withRequestID(req, "test-req-verdict-fail")
	rr := httptest.NewRecorder()

	h.Webhook(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code, "enqueue failure must not break webhook ack")
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

type assertEnqueueError struct{}

func (assertEnqueueError) Error() string { return "stub: redis stream unavailable" }

// TestBillingHandler_Webhook_RefundFailed_EnqueuesRetry asserts the D5 fix:
// on EventRefundFailed, the handler both records the failure in the DB AND
// schedules an asynq refund retry task 5 minutes out.  Without the enqueue
// path, refund_failed payments would sit forever waiting for manual admin
// intervention.
func TestBillingHandler_Webhook_RefundFailed_EnqueuesRetry(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()
	enq := &stubBillingEnqueuer{}
	h = h.WithEnqueuer(enq)

	extTxnID := "stub_txn_refund_fail"

	// 1. UPDATE payments to mark refund_failed (status + retry_count + timestamp).
	mockPool.ExpectExec("UPDATE payments").
		WithArgs(extTxnID, pgxmock.AnyArg(), "stub").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// 2. SELECT payment+user to build the retry payload.
	rows := pgxmock.NewRows([]string{
		"id", "user_id", "ext_txn_id", "amount_cents", "currency", "provider", "user_email",
	}).AddRow("pay_001", "u_user1", &extTxnID, int64(9900), "CNY", "stub", "user@example.com")
	mockPool.ExpectQuery("SELECT(.|\n)+FROM payments").
		WithArgs(extTxnID, "stub").
		WillReturnRows(rows)

	payload := billing.StubWebhookPayload{
		EventType: billing.EventRefundFailed,
		ExtTxnID:  extTxnID,
		UserID:    "u_user1",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhook", bytes.NewReader(body))
	req = withRequestID(req, "test-req-refund-fail")
	rr := httptest.NewRecorder()

	h.Webhook(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	calls := enq.Calls()
	require.Len(t, calls, 1, "exactly one refund retry should be enqueued")
	assert.Equal(t, 5*time.Minute, calls[0].Delay, "first retry must be scheduled 5 minutes out (D5)")
	assert.Equal(t, "pay_001", calls[0].Payload.PaymentID)
	assert.Equal(t, extTxnID, calls[0].Payload.ExtTxnID)
	assert.Equal(t, int64(9900), calls[0].Payload.AmountCents)
	assert.Equal(t, "user@example.com", calls[0].Payload.UserEmail)
	assert.Equal(t, "webhook_refund_failed", calls[0].Payload.Reason)
	assert.Equal(t, 0, calls[0].Payload.AttemptCount, "webhook-triggered retry starts at attempt 0")

	assert.NoError(t, mockPool.ExpectationsWereMet())
}

// TestBillingHandler_Webhook_RefundFailed_NoEnqueuerStillRecordsDB verifies
// backward compat: when no enqueuer is wired, the webhook still marks the
// payment refund_failed (so the admin dashboard / manual recovery still
// works), it just does not schedule an automated retry.
func TestBillingHandler_Webhook_RefundFailed_NoEnqueuerStillRecordsDB(t *testing.T) {
	h, mockPool, _ := newBillingTestHandler(t)
	defer mockPool.Close()
	// Intentionally do NOT wire an enqueuer.

	mockPool.ExpectExec("UPDATE payments").
		WithArgs("stub_txn_no_enq", pgxmock.AnyArg(), "stub").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	payload := billing.StubWebhookPayload{
		EventType: billing.EventRefundFailed,
		ExtTxnID:  "stub_txn_no_enq",
		UserID:    "u_user1",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhook", bytes.NewReader(body))
	req = withRequestID(req, "test-req-noenq")
	rr := httptest.NewRecorder()

	h.Webhook(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
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
	req = withRequestID(req, "test-req-id")
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
	req = withRequestID(req, "test-req-id")
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
	req = withRequestID(req, "test-req-id")
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


