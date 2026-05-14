// Package handler implements HTTP handlers for the API server.
package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// BillingPool is the subset of pgxpool.Pool used by AdminBillingHandler.
// It is satisfied by both *pgxpool.Pool and pgxmock.PgxPoolIface.
type BillingPool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// AdminBillingHandler handles admin billing endpoints.
//
// TODO: Enforce role=admin authentication by checking users.is_admin field
// once the admin role column is added to the users table. For now, all
// requests reaching these handlers are treated as authorized — they should
// be protected at the network / VPN layer until proper auth is in place.
type AdminBillingHandler struct {
	pool BillingPool
}

// NewAdminBillingHandler creates a new AdminBillingHandler.
func NewAdminBillingHandler(pool BillingPool) *AdminBillingHandler {
	return &AdminBillingHandler{pool: pool}
}

// RefundFailedPayment represents a payment with refund_failed status.
type RefundFailedPayment struct {
	ID               string     `json:"id"`
	UserID           string     `json:"user_id"`
	InvoiceID        *string    `json:"invoice_id,omitempty"`
	AmountCents      int        `json:"amount_cents"`
	Currency         string     `json:"currency"`
	RefundRetryCount int        `json:"refund_retry_count"`
	RefundFailedAt   *time.Time `json:"refund_failed_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// RefundFailedListResponse is the response for GET /v1/admin/refund-failed.
type RefundFailedListResponse struct {
	Payments []RefundFailedPayment `json:"payments"`
	Total    int                   `json:"total"`
}

// ListRefundFailed handles GET /v1/admin/refund-failed.
// Returns up to 100 payments with status='refund_failed', ordered by refund_failed_at DESC.
func (h *AdminBillingHandler) ListRefundFailed(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	rows, err := h.pool.Query(ctx, `
		SELECT
			id,
			user_id,
			invoice_id,
			amount_cents,
			currency,
			refund_retry_count,
			refund_failed_at,
			created_at
		FROM payments
		WHERE status = 'refund_failed'
		ORDER BY refund_failed_at DESC NULLS LAST
		LIMIT 100
	`)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query refund_failed payments", err))
		return
	}
	defer rows.Close()

	var payments []RefundFailedPayment
	for rows.Next() {
		var p RefundFailedPayment
		if err := rows.Scan(
			&p.ID,
			&p.UserID,
			&p.InvoiceID,
			&p.AmountCents,
			&p.Currency,
			&p.RefundRetryCount,
			&p.RefundFailedAt,
			&p.CreatedAt,
		); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan payment row", err))
			return
		}
		payments = append(payments, p)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("row iteration error", err))
		return
	}

	if payments == nil {
		payments = []RefundFailedPayment{}
	}

	response.JSON(w, r, http.StatusOK, RefundFailedListResponse{
		Payments: payments,
		Total:    len(payments),
	})
}

// RetryRefundResponse is the response for POST /v1/admin/refund-failed/:id/retry.
type RetryRefundResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// RetryRefund handles POST /v1/admin/refund-failed/:id/retry.
// Simulates a manual refund retry by transitioning the payment status to 'refunded'.
//
// In production this would call the Paddle API to re-attempt the refund.
// For now it updates the status in-DB to simulate success (as specified in D5 TODO).
func (h *AdminBillingHandler) RetryRefund(w http.ResponseWriter, r *http.Request) {
	paymentID := chi.URLParam(r, "id")
	if paymentID == "" {
		response.Error(w, r, apperr.Validation("missing payment id", ""))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	tag, err := h.pool.Exec(ctx, `
		UPDATE payments
		SET
			status = 'refunded',
			refund_retry_count = refund_retry_count + 1,
			refund_failed_at = NULL
		WHERE id = $1 AND status = 'refund_failed'
	`, paymentID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to update payment", err))
		return
	}

	if tag.RowsAffected() == 0 {
		response.Error(w, r, apperr.NotFound("payment not found or not in refund_failed state"))
		return
	}

	response.JSON(w, r, http.StatusOK, RetryRefundResponse{
		ID:     paymentID,
		Status: "refunded",
	})
}
