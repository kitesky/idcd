package main

import (
	"context"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisRefundEnqueuer_XAddsAllFields(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	enq := &redisRefundEnqueuer{rdb: rdb, stream: refundInitiateStream}
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
	v := entries[0].Values
	if v["report_id"] != "vr_42" {
		t.Fatalf("report_id=%v want vr_42", v["report_id"])
	}
	if v["reason"] != "bad sig" {
		t.Fatalf("reason=%v want bad sig", v["reason"])
	}
	if s, ok := v["enqueued_at"].(string); !ok || s == "" {
		t.Fatalf("enqueued_at missing or wrong type: %v", v["enqueued_at"])
	}
}

type erroringXAdder struct{}

func (erroringXAdder) XAdd(_ context.Context, _ *redis.XAddArgs) *redis.StringCmd {
	cmd := redis.NewStringCmd(context.Background())
	cmd.SetErr(redisErr("connection refused"))
	return cmd
}

type redisErr string

func (e redisErr) Error() string { return string(e) }

func TestRedisRefundEnqueuer_WrapsErrorWithStreamName(t *testing.T) {
	enq := &redisRefundEnqueuer{rdb: erroringXAdder{}, stream: refundInitiateStream}
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
