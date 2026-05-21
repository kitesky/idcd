package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/lib/shared/contracts"
	sharedstream "github.com/kite365/idcd/lib/shared/stream"
)

// TestRedisRefundEnqueuer_XAddsAllFields verifies the end-to-end shape
// the refund-worker side observes: ParseRefundInitiateEvent must round-trip
// the exact business fields the producer set, including the schema_ver
// that AddRefundInitiateTyped auto-injects.
func TestRedisRefundEnqueuer_XAddsAllFields(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	enq := &redisRefundEnqueuer{stream: sharedstream.New(rdb)}
	if err := enq.EnqueueRefund(context.Background(), "vr_42", "bad sig"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if !mr.Exists(refundInitiateStream) {
		t.Fatalf("stream %s not created", refundInitiateStream)
	}
	entries, err := rdb.XRange(context.Background(), refundInitiateStream, "-", "+").Result()
	if err != nil {
		t.Fatalf("xrange: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries=%d want 1", len(entries))
	}
	got, err := contracts.ParseRefundInitiateEvent(entries[0].Values)
	if err != nil {
		t.Fatalf("ParseRefundInitiateEvent: %v", err)
	}
	if got.ReportID != "vr_42" {
		t.Errorf("report_id=%q want vr_42", got.ReportID)
	}
	if got.Reason != "bad sig" {
		t.Errorf("reason=%q want bad sig", got.Reason)
	}
	if got.EnqueuedAt.IsZero() {
		t.Errorf("enqueued_at zero — producer must stamp time.Now()")
	}
	if got.SchemaVer != contracts.RefundInitiateEventSchemaV1 {
		t.Errorf("schema_ver=%d want %d", got.SchemaVer, contracts.RefundInitiateEventSchemaV1)
	}
}

// erroringStreamEnqueuer satisfies streamEnqueuer but always returns an
// error, simulating Redis being down. Used to verify EnqueueRefund wraps
// the error with the stream name (so the operator sees what failed).
type erroringStreamEnqueuer struct{}

func (erroringStreamEnqueuer) AddRefundInitiateTyped(_ context.Context, _ contracts.RefundInitiateEvent) (string, error) {
	return "", errors.New("connection refused")
}

func TestRedisRefundEnqueuer_WrapsErrorWithStreamName(t *testing.T) {
	enq := &redisRefundEnqueuer{stream: erroringStreamEnqueuer{}}
	err := enq.EnqueueRefund(context.Background(), "vr_x", "reason")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), refundInitiateStream) {
		t.Fatalf("error %q missing stream name", err)
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("error %q missing underlying cause", err)
	}
}
