package main

import (
	"context"
	"fmt"
	"time"

	"github.com/kite365/idcd/lib/shared/contracts"
	sharedstream "github.com/kite365/idcd/lib/shared/stream"
)

// refundInitiateStream is the Redis Stream the Refund Worker consumes
// when Self-Verify catches a bad PDF. Distinct from refund_retry_queue
// (PaymentHub webhook side, used when an outbound refund call fails); this
// one INITIATES the refund flow.
//
// The consumer is apps/attest/cmd/refund-worker. It calls the PaymentHub
// refund API, persists outcomes on verdict_order, and drives the D5
// 5min / 30min retry ladder via a shared delay-zone ZSET.
//
// 真值集中在 lib/shared/stream.RefundInitiateQueue — 此处只是本地常量别名,
// 避免 cmd 层调用方再 import shared/stream 两次。
const refundInitiateStream = sharedstream.RefundInitiateQueue

// streamEnqueuer is the narrow surface redisRefundEnqueuer needs from
// *sharedstream.Client. Keeps the test fixture independent of go-redis and
// avoids drag-in for what is conceptually just "one method writes one event".
type streamEnqueuer interface {
	AddRefundInitiateTyped(ctx context.Context, e contracts.RefundInitiateEvent) (string, error)
}

// redisRefundEnqueuer satisfies selfverify.RefundEnqueuer. Each call writes
// one typed RefundInitiateEvent via stream.Client.AddRefundInitiateTyped;
// the Refund Worker drains the stream and calls PaymentHub.
//
// P0-4 W3: switched from raw rdb.XAdd → typed contract; field-name typos
// now fail at compile time instead of silently dropping refund tickets.
type redisRefundEnqueuer struct {
	stream streamEnqueuer
}

func (e *redisRefundEnqueuer) EnqueueRefund(ctx context.Context, reportID, reason string) error {
	evt := contracts.RefundInitiateEvent{
		ReportID:   reportID,
		Reason:     reason,
		EnqueuedAt: time.Now().UTC(),
	}
	if _, err := e.stream.AddRefundInitiateTyped(ctx, evt); err != nil {
		return fmt.Errorf("refund enqueue %s: %w", sharedstream.RefundInitiateQueue, err)
	}
	return nil
}
