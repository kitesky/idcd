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

// Task type constants for billing-related asynq tasks.
// Must match the notifier worker's task registry.
const (
	// TaskRefundRetry is the asynq task type for retrying a failed refund.
	// Payload: RefundRetryPayload (JSON).
	TaskRefundRetry = "payment:refund_retry"

	// QueueBilling is the asynq queue name for billing tasks.
	QueueBilling = "billing"

	// RefundRetryFirstDelay is the delay for the first automatic retry after
	// a Paddle refund.failed webhook (D5: 5 minutes).
	RefundRetryFirstDelay = 5 * time.Minute

	// RefundRetrySecondDelay is the delay for the second automatic retry after
	// the first retry attempt fails (D5: 30 minutes from t0, so 25 minutes after
	// the first retry).  We use 30 minutes as a conservative upper bound — the
	// total elapsed time (first delay + second delay) stays at ~30 minutes from
	// the original refund.failed webhook.
	RefundRetrySecondDelay = 25 * time.Minute

	// RefundRetryMaxAttempts is the number of automated retries before we
	// give up, send an apology email, and surface the payment in the admin
	// dashboard (D5).  attempt_count starts at 0 (first webhook-triggered try)
	// and increments to 1 (after the 5min retry fails), 2 (after the 30min
	// retry fails).  At attempt_count >= 2, we stop retrying and trigger the
	// apology email.
	RefundRetryMaxAttempts = 2
)

// RefundRetryPayload is the asynq payload for a refund retry task.
//
// The notifier worker consumes this payload, calls the Paddle Refund API,
// updates the payment status, and either re-schedules another retry or
// triggers the apology email + admin dashboard escalation.
type RefundRetryPayload struct {
	PaymentID    string `json:"payment_id"`
	ExtTxnID     string `json:"ext_txn_id"`
	UserID       string `json:"user_id"`
	UserEmail    string `json:"user_email,omitempty"`
	AmountCents  int64  `json:"amount_cents"`
	Currency     string `json:"currency"`
	Provider     string `json:"provider"`
	Reason       string `json:"reason,omitempty"`
	AttemptCount int    `json:"attempt_count"`
	// Locale is the recipient's preferred short locale code (cn/en) for the
	// downstream apology email. The notifier worker reads this — when empty,
	// resolveLocale falls back to the registry default.
	Locale string `json:"locale,omitempty"`
}

// BillingEnqueuer enqueues billing-related asynq tasks (refund retry, etc.).
// Implementations must be safe for concurrent use.
//
// The Server wires this against an *asynq.Client adapter that maps delay to
// asynq.ProcessIn — the worker-side default retry/backoff is intentionally NOT
// used for refund retries, because D5 mandates explicit 5min/30min scheduling.
type BillingEnqueuer interface {
	// EnqueueRefundRetry schedules a refund retry task with the given delay.
	// A delay of 0 schedules the task for immediate processing.
	EnqueueRefundRetry(ctx context.Context, payload RefundRetryPayload, delay time.Duration) error
}

// AdminBillingHandler handles admin billing endpoints.
//
// TODO: Enforce role=admin authentication by checking users.is_admin field
// once the admin role column is added to the users table. For now, all
// requests reaching these handlers are treated as authorized — they should
// be protected at the network / VPN layer until proper auth is in place.
type AdminBillingHandler struct {
	pool     BillingPool
	enqueuer BillingEnqueuer // optional: nil falls back to direct DB status change (legacy / tests)
}

// NewAdminBillingHandler creates a new AdminBillingHandler.
func NewAdminBillingHandler(pool BillingPool) *AdminBillingHandler {
	return &AdminBillingHandler{pool: pool}
}

// WithEnqueuer wires a BillingEnqueuer used by RetryRefund.  When wired,
// RetryRefund schedules an immediate asynq task instead of mutating the
// payment row directly — this ensures the Paddle Refund API is actually
// called (D5 fix: previously the admin button set status='refunded' without
// hitting Paddle, causing ledger divergence).
func (h *AdminBillingHandler) WithEnqueuer(enq BillingEnqueuer) *AdminBillingHandler {
	h.enqueuer = enq
	return h
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
//
// D5 fix: instead of directly mutating payment.status='refunded' (which
// previously created a ledger drift because Paddle was never re-called),
// this handler enqueues an immediate asynq task that the notifier worker
// processes — the worker actually calls Paddle Refund API and updates DB
// only after a successful provider response.
//
// When no BillingEnqueuer is wired (legacy/tests without a Redis-backed
// asynq client), the handler falls back to the previous in-DB status
// transition so tests and dev environments without Redis still function.
func (h *AdminBillingHandler) RetryRefund(w http.ResponseWriter, r *http.Request) {
	paymentID := chi.URLParam(r, "id")
	if paymentID == "" {
		response.Error(w, r, apperr.Validation("missing payment id", ""))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Read the payment fields needed for the retry payload + the owner's
	// locale (joined from users) so the apology email lands in their language.
	// D1 forbids cross-schema FKs but `payments` and `users` live in the same
	// schema (idcd_main), so a regular LEFT JOIN is fine here.
	rows, err := h.pool.Query(ctx, `
		SELECT
			p.id, p.user_id, p.ext_txn_id, p.amount_cents, p.currency, p.provider,
			p.refund_retry_count, COALESCE(u.locale, '')
		FROM payments p
		LEFT JOIN users u ON u.id = p.user_id
		WHERE p.id = $1 AND p.status = 'refund_failed'
	`, paymentID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query payment", err))
		return
	}
	defer rows.Close()

	if !rows.Next() {
		response.Error(w, r, apperr.NotFound("payment not found or not in refund_failed state"))
		return
	}

	var (
		id          string
		userID      string
		extTxnID    *string
		amountCents int64
		currency    string
		provider    string
		retryCount  int
		userLocale  string
	)
	if err := rows.Scan(&id, &userID, &extTxnID, &amountCents, &currency, &provider, &retryCount, &userLocale); err != nil {
		response.Error(w, r, apperr.Internal("failed to scan payment row", err))
		return
	}
	rows.Close()

	// Build payload — admin-initiated retries always start at attempt_count=0
	// (a fresh attempt from the admin dashboard, independent of automated
	// retry counters).  The worker logic handles delay scheduling.
	payload := RefundRetryPayload{
		PaymentID:    id,
		UserID:       userID,
		AmountCents:  amountCents,
		Currency:     currency,
		Provider:     provider,
		Reason:       "admin_manual_retry",
		AttemptCount: 0,
		Locale:       userLocale,
	}
	if extTxnID != nil {
		payload.ExtTxnID = *extTxnID
	}

	// Preferred path: enqueue an immediate asynq task.  The notifier worker
	// will call Paddle Refund and update payment status only after a
	// successful provider response.
	if h.enqueuer != nil {
		if err := h.enqueuer.EnqueueRefundRetry(ctx, payload, 0); err != nil {
			response.Error(w, r, apperr.Internal("failed to enqueue refund retry", err))
			return
		}
		response.JSON(w, r, http.StatusOK, RetryRefundResponse{
			ID:     paymentID,
			Status: "retry_enqueued",
		})
		return
	}

	// Fallback (no enqueuer wired): legacy in-DB transition.  Kept so tests
	// and offline dev environments without Redis still work — production
	// MUST wire an enqueuer (see server.go).
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

