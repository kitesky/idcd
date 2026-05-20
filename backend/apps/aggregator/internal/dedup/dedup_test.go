package dedup

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestDeduper(t *testing.T) (*Deduper, *miniredis.Miniredis) {
	t.Helper()
	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	return New(rdb), s
}

func TestIsDuplicate_notSeen(t *testing.T) {
	d, _ := newTestDeduper(t)

	dup, err := d.IsDuplicate(context.Background(), "task_001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dup {
		t.Error("expected not duplicate for unseen key")
	}
}

func TestMarkProcessed_and_IsDuplicate(t *testing.T) {
	d, _ := newTestDeduper(t)
	ctx := context.Background()

	if err := d.MarkProcessed(ctx, "task_001"); err != nil {
		t.Fatalf("MarkProcessed: %v", err)
	}

	dup, err := d.IsDuplicate(ctx, "task_001")
	if err != nil {
		t.Fatalf("IsDuplicate: %v", err)
	}
	if !dup {
		t.Error("expected duplicate after MarkProcessed")
	}
}

func TestMarkProcessed_twice_fails(t *testing.T) {
	d, _ := newTestDeduper(t)
	ctx := context.Background()

	if err := d.MarkProcessed(ctx, "task_001"); err != nil {
		t.Fatalf("first MarkProcessed: %v", err)
	}
	if err := d.MarkProcessed(ctx, "task_001"); err == nil {
		t.Error("expected error on second MarkProcessed, got nil")
	}
}

func TestIsProcessedAndMark_firstCall(t *testing.T) {
	d, _ := newTestDeduper(t)

	dup, err := d.IsProcessedAndMark(context.Background(), "task_new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dup {
		t.Error("expected not duplicate on first call")
	}
}

func TestIsProcessedAndMark_secondCall(t *testing.T) {
	d, _ := newTestDeduper(t)
	ctx := context.Background()

	_, _ = d.IsProcessedAndMark(ctx, "task_001")

	dup, err := d.IsProcessedAndMark(ctx, "task_001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dup {
		t.Error("expected duplicate on second call")
	}
}

func TestClear(t *testing.T) {
	d, _ := newTestDeduper(t)
	ctx := context.Background()

	if err := d.MarkProcessed(ctx, "task_001"); err != nil {
		t.Fatal(err)
	}
	if err := d.Clear(ctx, "task_001"); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	dup, err := d.IsDuplicate(ctx, "task_001")
	if err != nil {
		t.Fatal(err)
	}
	if dup {
		t.Error("expected not duplicate after Clear")
	}
}

func TestDedupTTL_expires(t *testing.T) {
	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	d := NewWithTTL(rdb, 100*time.Millisecond)
	ctx := context.Background()

	if err := d.MarkProcessed(ctx, "task_ttl"); err != nil {
		t.Fatal(err)
	}

	s.FastForward(200 * time.Millisecond)

	dup, err := d.IsDuplicate(ctx, "task_ttl")
	if err != nil {
		t.Fatal(err)
	}
	if dup {
		t.Error("expected not duplicate after TTL expiry")
	}
}

func TestStats(t *testing.T) {
	d, _ := newTestDeduper(t)
	ctx := context.Background()

	if err := d.MarkProcessed(ctx, "task_001"); err != nil {
		t.Fatal(err)
	}
	if err := d.MarkProcessed(ctx, "task_002"); err != nil {
		t.Fatal(err)
	}

	stats, err := d.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.TrackedTaskIDs != 2 {
		t.Errorf("expected 2 tracked, got %d", stats.TrackedTaskIDs)
	}
}
