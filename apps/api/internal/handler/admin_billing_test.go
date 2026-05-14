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

func TestAdminBillingHandler_RetryRefund_Success(t *testing.T) {
	h, mockPool := newAdminBillingTestHandler(t)
	defer mockPool.Close()

	mockPool.ExpectExec("UPDATE payments").
		WithArgs("pay_test_001").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/refund-failed/pay_test_001/retry", nil)
	ctx := context.WithValue(req.Context(), "request_id", "test-req-retry")
	// Inject chi URL param
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("id", "pay_test_001")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, chiCtx)
	req = req.WithContext(ctx)
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
	assert.Equal(t, "refunded", resp.Data.Status)

	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestAdminBillingHandler_RetryRefund_NotFound(t *testing.T) {
	h, mockPool := newAdminBillingTestHandler(t)
	defer mockPool.Close()

	// 0 rows affected → payment not found or wrong status
	mockPool.ExpectExec("UPDATE payments").
		WithArgs("pay_nonexistent").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/refund-failed/pay_nonexistent/retry", nil)
	ctx := context.WithValue(req.Context(), "request_id", "test-req-notfound")
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("id", "pay_nonexistent")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, chiCtx)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.RetryRefund(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
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
