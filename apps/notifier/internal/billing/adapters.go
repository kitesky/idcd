// Package billing wires production adapters for the notifier's D5 refund
// retry pipeline.
//
// The notifier's worker handler depends on three interfaces (defined in
// apps/notifier/internal/worker/handlers.go):
//
//   - PaymentRefunder     — call Paddle's refund API.
//   - PaymentStore        — persist the post-refund payment state.
//   - RefundRetryEnqueuer — schedule the next retry attempt with a delay.
//
// We keep the adapters in their own package (rather than inlining them in
// cmd/notifier/main.go) so:
//
//   - main.go stays focused on wiring + signal handling.
//   - Each adapter has a clear, isolated responsibility and is unit-testable
//     in principle (no public surface is exposed; this package is internal).
//
// The notifier deliberately does NOT import apps/api/internal/billing — that
// path is internal to the api binary. We instead talk to the payment SDK
// directly via packages/payment-go-sdk, mirroring NewPaymentHubProvider's
// behaviour for the single endpoint we care about (CreateRefund).
package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	payment "github.com/wangzheng/payment-go-sdk"

	"github.com/kite365/idcd/apps/notifier/internal/worker"
	"github.com/kite365/idcd/lib/shared/idgen"
)

// BillingQueue is the asynq queue that carries payment:refund_retry tasks.
// The notifier's asynq.Server must declare this queue in its Queues map.
const BillingQueue = "billing"

// ---------------------------------------------------------------------------
// PaymentRefunder adapter — wraps packages/payment-go-sdk.
// ---------------------------------------------------------------------------

// PaddleRefunder calls the payment aggregation platform's refund endpoint.
// It implements worker.PaymentRefunder.
//
// The same SDK + signing flow is used by apps/api/internal/billing.
// PaymentHubProvider.RefundPayment; we re-implement just the one method here
// instead of importing that internal package.
type PaddleRefunder struct {
	client payment.ClientInterface
}

// NewPaddleRefunder builds a PaddleRefunder against the platform identified
// by baseURL with the given credentials. Retry behaviour mirrors the
// api-side PaymentHubProvider (2 retries, 500ms initial delay).
func NewPaddleRefunder(baseURL, apiKey, apiSecret string) *PaddleRefunder {
	c := payment.New(baseURL,
		payment.WithAPIKey(apiKey),
		payment.WithAPISecret(apiSecret),
		payment.WithRetry(2, 500*time.Millisecond),
	)
	return &PaddleRefunder{client: c}
}

// NewPaddleRefunderWithClient is used in tests to inject a mock SDK client.
func NewPaddleRefunderWithClient(c payment.ClientInterface) *PaddleRefunder {
	return &PaddleRefunder{client: c}
}

// RefundPayment issues a refund against the payment platform.
// extTxnID is the platform's OrderNo recorded on the original payment row.
func (r *PaddleRefunder) RefundPayment(ctx context.Context, extTxnID string, amountCents int64, reason string) error {
	if extTxnID == "" {
		return errors.New("notifier/billing: RefundPayment: ext_txn_id is required")
	}
	if amountCents <= 0 {
		return fmt.Errorf("notifier/billing: RefundPayment: amount_cents must be positive, got %d", amountCents)
	}

	_, err := r.client.CreateRefund(ctx, &payment.RefundReq{
		OrderNo:     extTxnID,
		AppRefundID: idgen.Refund(),
		Amount:      amountCents,
		Reason:      reason,
	})
	if err != nil {
		return fmt.Errorf("notifier/billing: RefundPayment: %w", err)
	}
	return nil
}

// Compile-time interface check.
var _ worker.PaymentRefunder = (*PaddleRefunder)(nil)

// ---------------------------------------------------------------------------
// PaymentStore adapter — direct SQL via pgxpool.
// ---------------------------------------------------------------------------

// PgPaymentStore persists payment state transitions for the D5 refund retry
// pipeline. It implements worker.PaymentStore.
//
// We use raw SQL (rather than sqlc) because the only two operations we need
// are simple status flips on the payments table, and the notifier doesn't
// otherwise pull in lib/db/gen/idcdmain.
type PgPaymentStore struct {
	pool *pgxpool.Pool
}

// NewPgPaymentStore wires a PgPaymentStore against the given pool.
func NewPgPaymentStore(pool *pgxpool.Pool) *PgPaymentStore {
	return &PgPaymentStore{pool: pool}
}

// MarkRefunded transitions the payment row to status='refunded'.
// Called when the Paddle refund call finally succeeds during a retry.
func (s *PgPaymentStore) MarkRefunded(ctx context.Context, paymentID string) error {
	if paymentID == "" {
		return errors.New("notifier/billing: MarkRefunded: payment_id is required")
	}
	const q = `UPDATE payments SET status = 'refunded' WHERE id = $1`
	tag, err := s.pool.Exec(ctx, q, paymentID)
	if err != nil {
		return fmt.Errorf("notifier/billing: MarkRefunded: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Not a hard error — the row may have been resolved out-of-band by
		// the admin dashboard. The worker logs and moves on.
		return fmt.Errorf("notifier/billing: MarkRefunded: payment %s not found", paymentID)
	}
	return nil
}

// MarkRefundFailed bumps the refund_retry_count and stamps refund_failed_at.
// The status flip to 'refund_failed' is idempotent so re-running this on an
// already-failed row simply updates the counter and timestamp.
func (s *PgPaymentStore) MarkRefundFailed(ctx context.Context, paymentID string, retryCount int) error {
	if paymentID == "" {
		return errors.New("notifier/billing: MarkRefundFailed: payment_id is required")
	}
	const q = `
		UPDATE payments
		SET status = 'refund_failed',
		    refund_retry_count = $2,
		    refund_failed_at = NOW()
		WHERE id = $1
	`
	tag, err := s.pool.Exec(ctx, q, paymentID, retryCount)
	if err != nil {
		return fmt.Errorf("notifier/billing: MarkRefundFailed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("notifier/billing: MarkRefundFailed: payment %s not found", paymentID)
	}
	return nil
}

// Compile-time interface check.
var _ worker.PaymentStore = (*PgPaymentStore)(nil)

// ---------------------------------------------------------------------------
// RefundRetryEnqueuer adapter — wraps *asynq.Client.
// ---------------------------------------------------------------------------

// AsynqRefundEnqueuer schedules refund retry tasks via asynq with explicit
// asynq.ProcessIn delays (5min / 30min per D5). It implements
// worker.RefundRetryEnqueuer.
type AsynqRefundEnqueuer struct {
	client *asynq.Client
}

// NewAsynqRefundEnqueuer wires an enqueuer against the given asynq client.
// The caller owns the client's lifetime (Close on shutdown).
func NewAsynqRefundEnqueuer(client *asynq.Client) *AsynqRefundEnqueuer {
	return &AsynqRefundEnqueuer{client: client}
}

// EnqueueRefundRetry schedules a worker.TaskRefundRetry task on the billing
// queue with the given processing delay.
func (e *AsynqRefundEnqueuer) EnqueueRefundRetry(ctx context.Context, payload worker.RefundRetryPayload, delay time.Duration) error {
	if e.client == nil {
		return errors.New("notifier/billing: EnqueueRefundRetry: asynq client is nil")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notifier/billing: EnqueueRefundRetry: marshal payload: %w", err)
	}
	task := asynq.NewTask(worker.TaskRefundRetry, body)
	if _, err := e.client.EnqueueContext(ctx, task,
		asynq.Queue(BillingQueue),
		asynq.ProcessIn(delay),
	); err != nil {
		return fmt.Errorf("notifier/billing: EnqueueRefundRetry: %w", err)
	}
	return nil
}

// Compile-time interface check.
var _ worker.RefundRetryEnqueuer = (*AsynqRefundEnqueuer)(nil)
