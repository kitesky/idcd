package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubBillingEnqueuer records EnqueueRefundRetry calls for assertions.
type stubBillingEnqueuer struct {
	mu      sync.Mutex
	calls   []stubEnqueueCall
	err     error
}

type stubEnqueueCall struct {
	Payload RefundRetryPayload
	Delay   time.Duration
}

func (s *stubBillingEnqueuer) EnqueueRefundRetry(_ context.Context, payload RefundRetryPayload, delay time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.calls = append(s.calls, stubEnqueueCall{Payload: payload, Delay: delay})
	return nil
}

func (s *stubBillingEnqueuer) Calls() []stubEnqueueCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]stubEnqueueCall, len(s.calls))
	copy(out, s.calls)
	return out
}

// newAdminBillingTestHandler creates an AdminBillingHandler backed by a pgxmock pool.
func newAdminBillingTestHandler(t *testing.T) (*AdminBillingHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	// pgxmock.PgxPoolIface satisfies BillingPool interface
	h := NewAdminBillingHandler(mockPool)
	return h, mockPool
}

func TestAdminBillingHandler_ListRefundFailed_Empty(t *testing.T) {
	h, mockPool := newAdminBillingTestHandler(t)
	defer mockPool.Close()

	rows := pgxmock.NewRows([]string{
		"id", "user_id", "invoice_id", "amount_cents", "currency",
		"refund_retry_count", "refund_failed_at", "created_at",
	})
	mockPool.ExpectQuery("SELECT").WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/refund-failed", nil)
	ctx := context.WithValue(req.Context(), "request_id", "test-req-list-empty")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.ListRefundFailed(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Data struct {
			Payments []RefundFailedPayment `json:"payments"`
			Total    int                   `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Data.Total)
	assert.Empty(t, resp.Data.Payments)

	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestAdminBillingHandler_ListRefundFailed_WithRows(t *testing.T) {
	h, mockPool := newAdminBillingTestHandler(t)
	defer mockPool.Close()

	now := time.Now().UTC().Truncate(time.Second)
	invoiceID := "inv_abc123"

	rows := pgxmock.NewRows([]string{
		"id", "user_id", "invoice_id", "amount_cents", "currency",
		"refund_retry_count", "refund_failed_at", "created_at",
	}).
		AddRow("pay_001", "u_user1", &invoiceID, 9900, "CNY", 2, &now, now).
		AddRow("pay_002", "u_user2", nil, 4900, "CNY", 1, &now, now)

	mockPool.ExpectQuery("SELECT").WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/refund-failed", nil)
	ctx := context.WithValue(req.Context(), "request_id", "test-req-list")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.ListRefundFailed(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Data struct {
			Payments []RefundFailedPayment `json:"payments"`
			Total    int                   `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Data.Total)
	assert.Equal(t, "pay_001", resp.Data.Payments[0].ID)
	assert.Equal(t, 9900, resp.Data.Payments[0].AmountCents)
	assert.Equal(t, "pay_002", resp.Data.Payments[1].ID)
	assert.Nil(t, resp.Data.Payments[1].InvoiceID)

	assert.NoError(t, mockPool.ExpectationsWereMet())
}

// retryRequest builds a request with the chi URL param wired so the handler
// can read paymentID via chi.URLParam.
func retryRequest(paymentID, reqID string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/refund-failed/"+paymentID+"/retry", nil)
	ctx := context.WithValue(req.Context(), "request_id", reqID)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("id", paymentID)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, chiCtx)
	return req.WithContext(ctx)
}

// TestAdminBillingHandler_RetryRefund_EnqueuesTask verifies the D5 fix:
// RetryRefund schedules an asynq task (ProcessIn=0) instead of mutating the
// payment row directly.  Status='refunded' MUST come from the worker after a
// real Paddle refund response, not from the admin button.
func TestAdminBillingHandler_RetryRefund_EnqueuesTask(t *testing.T) {
	h, mockPool := newAdminBillingTestHandler(t)
	defer mockPool.Close()
	enq := &stubBillingEnqueuer{}
	h = h.WithEnqueuer(enq)

	extTxnID := "ph_txn_001"
	rows := pgxmock.NewRows([]string{
		"id", "user_id", "ext_txn_id", "amount_cents", "currency", "provider",
		"refund_retry_count",
	}).AddRow("pay_test_001", "u_user1", &extTxnID, int64(9900), "CNY", "payment_hub", 1)

	mockPool.ExpectQuery("SELECT(.|\n)+FROM payments").
		WithArgs("pay_test_001").
		WillReturnRows(rows)

	req := retryRequest("pay_test_001", "test-req-retry")
	rr := httptest.NewRecorder()

	h.RetryRefund(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Data struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "pay_test_001", resp.Data.ID)
	assert.Equal(t, "retry_enqueued", resp.Data.Status)

	calls := enq.Calls()
	require.Len(t, calls, 1, "expected exactly one EnqueueRefundRetry call")
	assert.Equal(t, time.Duration(0), calls[0].Delay, "admin retry should enqueue with delay=0")
	assert.Equal(t, "pay_test_001", calls[0].Payload.PaymentID)
	assert.Equal(t, "ph_txn_001", calls[0].Payload.ExtTxnID)
	assert.Equal(t, int64(9900), calls[0].Payload.AmountCents)
	assert.Equal(t, "CNY", calls[0].Payload.Currency)
	assert.Equal(t, "payment_hub", calls[0].Payload.Provider)
	assert.Equal(t, "admin_manual_retry", calls[0].Payload.Reason)
	assert.Equal(t, 0, calls[0].Payload.AttemptCount, "admin retries start at attempt 0")

	assert.NoError(t, mockPool.ExpectationsWereMet())
}

// TestAdminBillingHandler_RetryRefund_NotFound asserts the handler returns
// 404 when the payment is missing or not in refund_failed state.
func TestAdminBillingHandler_RetryRefund_NotFound(t *testing.T) {
	h, mockPool := newAdminBillingTestHandler(t)
	defer mockPool.Close()
	enq := &stubBillingEnqueuer{}
	h = h.WithEnqueuer(enq)

	// Empty result set → not found.
	rows := pgxmock.NewRows([]string{
		"id", "user_id", "ext_txn_id", "amount_cents", "currency", "provider",
		"refund_retry_count",
	})
	mockPool.ExpectQuery("SELECT(.|\n)+FROM payments").
		WithArgs("pay_nonexistent").
		WillReturnRows(rows)

	req := retryRequest("pay_nonexistent", "test-req-notfound")
	rr := httptest.NewRecorder()

	h.RetryRefund(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Empty(t, enq.Calls(), "no enqueue should occur for missing payment")
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

// TestAdminBillingHandler_RetryRefund_EnqueueError returns 500 if the
// enqueuer fails — the handler must NOT silently fall through to direct
// status mutation when a real enqueuer is wired.
func TestAdminBillingHandler_RetryRefund_EnqueueError(t *testing.T) {
	h, mockPool := newAdminBillingTestHandler(t)
	defer mockPool.Close()
	enq := &stubBillingEnqueuer{err: errors.New("redis down")}
	h = h.WithEnqueuer(enq)

	extTxnID := "ph_txn_001"
	rows := pgxmock.NewRows([]string{
		"id", "user_id", "ext_txn_id", "amount_cents", "currency", "provider",
		"refund_retry_count",
	}).AddRow("pay_test_001", "u_user1", &extTxnID, int64(9900), "CNY", "payment_hub", 1)
	mockPool.ExpectQuery("SELECT(.|\n)+FROM payments").
		WithArgs("pay_test_001").
		WillReturnRows(rows)

	req := retryRequest("pay_test_001", "test-req-enq-err")
	rr := httptest.NewRecorder()

	h.RetryRefund(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

// TestAdminBillingHandler_RetryRefund_FallbackNoEnqueuer keeps the legacy
// in-DB transition path working when no enqueuer is wired (tests / offline
// dev).  Production wiring MUST always provide an enqueuer; see server.go.
func TestAdminBillingHandler_RetryRefund_FallbackNoEnqueuer(t *testing.T) {
	h, mockPool := newAdminBillingTestHandler(t)
	defer mockPool.Close()

	extTxnID := "ph_txn_001"
	rows := pgxmock.NewRows([]string{
		"id", "user_id", "ext_txn_id", "amount_cents", "currency", "provider",
		"refund_retry_count",
	}).AddRow("pay_test_001", "u_user1", &extTxnID, int64(9900), "CNY", "payment_hub", 0)
	mockPool.ExpectQuery("SELECT(.|\n)+FROM payments").
		WithArgs("pay_test_001").
		WillReturnRows(rows)
	mockPool.ExpectExec("UPDATE payments").
		WithArgs("pay_test_001").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	req := retryRequest("pay_test_001", "test-req-fallback")
	rr := httptest.NewRecorder()

	h.RetryRefund(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp struct {
		Data struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "refunded", resp.Data.Status, "fallback path returns legacy status")
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestAdminBillingHandler_RetryRefund_MissingID(t *testing.T) {
	h, mockPool := newAdminBillingTestHandler(t)
	defer mockPool.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/refund-failed//retry", nil)
	ctx := context.WithValue(req.Context(), "request_id", "test-req-missing-id")
	// No chi URL param set → id == ""
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.RetryRefund(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestNewAdminBillingHandler(t *testing.T) {
	h := NewAdminBillingHandler(nil)
	assert.NotNil(t, h)
}
