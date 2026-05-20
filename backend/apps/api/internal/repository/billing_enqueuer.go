package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kite365/idcd/apps/api/internal/handler"
)

// asynqClient is the minimal subset of *asynq.Client needed to enqueue a
// task. Declared as an interface so unit tests can supply an in-memory stub
// without spinning up Redis — production wiring passes a real *asynq.Client.
type asynqClient interface {
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// BillingEnqueuer implements handler.BillingEnqueuer over an asynq client.
//
// The transport contract (queue name + task type) lives in handler/admin_billing.go:
//
//   - queue       = handler.QueueBilling ("billing")
//   - task type   = handler.TaskRefundRetry ("payment:refund_retry")
//   - delay       = passed through to asynq.ProcessIn
//
// D5 (DECISIONS.md §M) mandates explicit 5min / 30min scheduling — we relay
// the caller-provided delay verbatim and do NOT let asynq's generic retry
// backoff move it around.
type BillingEnqueuer struct {
	client asynqClient
	queue  string
}

// NewBillingEnqueuer constructs a BillingEnqueuer over the given asynq
// client. Uses handler.QueueBilling ("billing") as the queue name.
func NewBillingEnqueuer(client *asynq.Client) *BillingEnqueuer {
	return &BillingEnqueuer{
		client: client,
		queue:  handler.QueueBilling,
	}
}

// newBillingEnqueuerWithClient is the test seam — accepts the interface
// directly so callers can pass an in-memory stub. Kept package-private so the
// public API (NewBillingEnqueuer) stays tied to the concrete *asynq.Client
// type that production wiring uses.
func newBillingEnqueuerWithClient(client asynqClient) *BillingEnqueuer {
	return &BillingEnqueuer{
		client: client,
		queue:  handler.QueueBilling,
	}
}

// EnqueueRefundRetry serializes the payload, builds an asynq.Task, and
// schedules it on the billing queue with the explicit delay. A delay of 0
// runs the task as soon as a worker picks it up.
func (e *BillingEnqueuer) EnqueueRefundRetry(ctx context.Context, payload handler.RefundRetryPayload, delay time.Duration) error {
	if e == nil || e.client == nil {
		return errors.New("billing enqueuer not configured")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal refund retry payload: %w", err)
	}

	opts := []asynq.Option{asynq.Queue(e.queue)}
	if delay > 0 {
		opts = append(opts, asynq.ProcessIn(delay))
	}

	if _, err := e.client.EnqueueContext(ctx,
		asynq.NewTask(handler.TaskRefundRetry, body),
		opts...,
	); err != nil {
		return fmt.Errorf("enqueue refund retry: %w", err)
	}
	return nil
}

// Compile-time interface check.
var _ handler.BillingEnqueuer = (*BillingEnqueuer)(nil)
