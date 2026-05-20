package leader

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupRedis(t *testing.T) (*miniredis.Miniredis, redis.Cmdable) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, rdb
}

func TestAcquire(t *testing.T) {
	_, rdb := setupRedis(t)
	ctx := context.Background()

	l1 := New(rdb, "test:leader", 10*time.Second, "node1")
	l2 := New(rdb, "test:leader", 10*time.Second, "node2")

	// l1 should acquire leadership
	ok, err := l1.Acquire(ctx)
	if err != nil {
		t.Fatalf("l1.Acquire: %v", err)
	}
	if !ok {
		t.Errorf("l1.Acquire() = false, want true")
	}
	if !l1.IsLeader() {
		t.Errorf("l1.IsLeader() = false, want true")
	}

	// l2 should fail to acquire leadership
	ok, err = l2.Acquire(ctx)
	if err != nil {
		t.Fatalf("l2.Acquire: %v", err)
	}
	if ok {
		t.Errorf("l2.Acquire() = true, want false")
	}
	if l2.IsLeader() {
		t.Errorf("l2.IsLeader() = true, want false")
	}
}

func TestRenew(t *testing.T) {
	mr, rdb := setupRedis(t)
	ctx := context.Background()

	l := New(rdb, "test:leader", 2*time.Second, "node1")

	// Cannot renew without acquiring first
	err := l.Renew(ctx)
	if err == nil {
		t.Errorf("l.Renew() before acquiring should fail")
	}

	// Acquire leadership
	ok, err := l.Acquire(ctx)
	if err != nil {
		t.Fatalf("l.Acquire: %v", err)
	}
	if !ok {
		t.Fatalf("l.Acquire() = false, want true")
	}

	// Renew should succeed
	err = l.Renew(ctx)
	if err != nil {
		t.Errorf("l.Renew: %v", err)
	}
	if !l.IsLeader() {
		t.Errorf("l.IsLeader() = false after renew, want true")
	}

	// Simulate key expiration
	mr.FastForward(3 * time.Second)

	// Another instance takes leadership
	l2 := New(rdb, "test:leader", 2*time.Second, "node2")
	ok, err = l2.Acquire(ctx)
	if err != nil {
		t.Fatalf("l2.Acquire: %v", err)
	}
	if !ok {
		t.Fatalf("l2.Acquire() = false after l1 expired, want true")
	}

	// l1 renew should fail (lost leadership)
	err = l.Renew(ctx)
	if err == nil {
		t.Errorf("l.Renew() after losing leadership should fail")
	}
	if l.IsLeader() {
		t.Errorf("l.IsLeader() = true after lost leadership, want false")
	}
}

func TestRelease(t *testing.T) {
	_, rdb := setupRedis(t)
	ctx := context.Background()

	l1 := New(rdb, "test:leader", 10*time.Second, "node1")
	l2 := New(rdb, "test:leader", 10*time.Second, "node2")

	// Acquire leadership
	ok, err := l1.Acquire(ctx)
	if err != nil {
		t.Fatalf("l1.Acquire: %v", err)
	}
	if !ok {
		t.Fatalf("l1.Acquire() = false, want true")
	}

	// Release leadership
	err = l1.Release(ctx)
	if err != nil {
		t.Errorf("l1.Release: %v", err)
	}
	if l1.IsLeader() {
		t.Errorf("l1.IsLeader() = true after release, want false")
	}

	// l2 should now be able to acquire
	ok, err = l2.Acquire(ctx)
	if err != nil {
		t.Fatalf("l2.Acquire: %v", err)
	}
	if !ok {
		t.Errorf("l2.Acquire() = false after l1 released, want true")
	}
}

func TestRelease_NotOwner(t *testing.T) {
	_, rdb := setupRedis(t)
	ctx := context.Background()

	l1 := New(rdb, "test:leader", 10*time.Second, "node1")
	l2 := New(rdb, "test:leader", 10*time.Second, "node2")

	// l1 acquires
	ok, err := l1.Acquire(ctx)
	if err != nil {
		t.Fatalf("l1.Acquire: %v", err)
	}
	if !ok {
		t.Fatalf("l1.Acquire() = false, want true")
	}

	// l2 tries to release (not the owner)
	err = l2.Release(ctx)
	// Should not return error when not leader (graceful no-op)
	if err != nil && l2.IsLeader() {
		t.Errorf("l2.Release() when not leader returned error: %v", err)
	}

	// l1 should still be leader
	if !l1.IsLeader() {
		t.Errorf("l1.IsLeader() = false after l2 tried to release, want true")
	}
}

func TestRelease_DoubleRelease(t *testing.T) {
	_, rdb := setupRedis(t)
	ctx := context.Background()

	l := New(rdb, "test:leader", 10*time.Second, "node1")

	// Acquire
	ok, err := l.Acquire(ctx)
	if err != nil {
		t.Fatalf("l.Acquire: %v", err)
	}
	if !ok {
		t.Fatalf("l.Acquire() = false, want true")
	}

	// First release
	err = l.Release(ctx)
	if err != nil {
		t.Errorf("l.Release: %v", err)
	}

	// Second release should be no-op
	err = l.Release(ctx)
	if err != nil {
		t.Errorf("l.Release (second time): %v", err)
	}
}
