package main

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// refundInitiateStream is the Redis Stream the Refund Worker consumes
// when Self-Verify catches a bad PDF. Distinct from refund_retry_queue
// (PaymentHub webhook side, used when an outbound refund call fails); this
// one INITIATES the refund flow.
//
// The consumer is apps/attest/cmd/refund-worker. It calls the PaymentHub
// refund API, persists outcomes on verdict_order, and drives the D5
// 5min / 30min retry ladder via a shared delay-zone ZSET.
const refundInitiateStream = "refund_initiate_queue"

// redisRefundEnqueuer satisfies selfverify.RefundEnqueuer. Each call
// XAdds one entry; the Refund Worker drains the stream and calls PaymentHub.
type redisRefundEnqueuer struct {
	rdb    redisXAdder
	stream string
}

// redisXAdder is the narrow surface we exercise; both *redis.Client and
// test fakes satisfy it without dragging the full client into tests.
type redisXAdder interface {
	XAdd(ctx context.Context, a *redis.XAddArgs) *redis.StringCmd
}

func (e *redisRefundEnqueuer) EnqueueRefund(ctx context.Context, reportID, reason string) error {
	cmd := e.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: e.stream,
		Values: map[string]any{
			"report_id":   reportID,
			"reason":      reason,
			"enqueued_at": time.Now().UTC().Format(time.RFC3339Nano),
		},
	})
	if err := cmd.Err(); err != nil {
		return fmt.Errorf("refund enqueue %s: %w", e.stream, err)
	}
	return nil
}
