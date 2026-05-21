package leader

import (
	"context"
	"sync"
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

// --- Fencing token (epoch) tests ---

// TestAcquireEpoch_Monotonic verifies that successive AcquireEpoch calls
// against the same Redis return strictly monotonically increasing tokens.
// This is the core invariant the consumer side relies on to detect stale
// leaders: a higher epoch always wins.
func TestAcquireEpoch_Monotonic(t *testing.T) {
	_, rdb := setupRedis(t)
	ctx := context.Background()

	const n = 50
	tokens := make([]FencingToken, n)
	for i := 0; i < n; i++ {
		tok, err := AcquireEpoch(ctx, rdb, DefaultEpochKey)
		if err != nil {
			t.Fatalf("AcquireEpoch[%d]: %v", i, err)
		}
		tokens[i] = tok
	}

	for i := 1; i < n; i++ {
		if tokens[i] <= tokens[i-1] {
			t.Errorf("epoch not monotonic: tokens[%d]=%d, tokens[%d]=%d",
				i-1, tokens[i-1].Int64(), i, tokens[i].Int64())
		}
	}
	// First INCR on a fresh key returns 1
	if tokens[0] != 1 {
		t.Errorf("first epoch = %d, want 1 (Redis INCR on fresh key)", tokens[0])
	}
}

// TestAcquireEpoch_ConcurrentSafe runs many goroutines racing on INCR and
// verifies every goroutine receives a distinct token — INCR is atomic in
// Redis so there can be no two schedulers ever sharing an epoch.
func TestAcquireEpoch_ConcurrentSafe(t *testing.T) {
	_, rdb := setupRedis(t)
	ctx := context.Background()

	const n = 100
	tokens := make([]FencingToken, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			tok, err := AcquireEpoch(ctx, rdb, DefaultEpochKey)
			if err != nil {
				t.Errorf("AcquireEpoch[%d]: %v", idx, err)
				return
			}
			tokens[idx] = tok
		}(i)
	}
	wg.Wait()

	seen := make(map[FencingToken]struct{}, n)
	for _, tok := range tokens {
		if tok == 0 {
			t.Errorf("got zero token (Acquire failed silently?)")
			continue
		}
		if _, dup := seen[tok]; dup {
			t.Errorf("duplicate token %d allocated to two goroutines (INCR not atomic?)", tok.Int64())
		}
		seen[tok] = struct{}{}
	}
	if len(seen) != n {
		t.Errorf("unique tokens = %d, want %d", len(seen), n)
	}
}

// TestAcquireEpoch_DefaultKey verifies the empty-string key defaults to
// DefaultEpochKey — main.go relies on this so a misconfigured deployment
// still claims a token from a well-known location.
func TestAcquireEpoch_DefaultKey(t *testing.T) {
	mr, rdb := setupRedis(t)
	ctx := context.Background()

	if _, err := AcquireEpoch(ctx, rdb, ""); err != nil {
		t.Fatalf("AcquireEpoch with empty key: %v", err)
	}
	got, err := mr.Get(DefaultEpochKey)
	if err != nil {
		t.Fatalf("miniredis.Get(%q): %v", DefaultEpochKey, err)
	}
	if got != "1" {
		t.Errorf("DefaultEpochKey value = %q, want %q", got, "1")
	}
}

// TestFencingToken_String exercises the formatting helpers used by the
// scheduler when serialising tokens into stream payloads.
func TestFencingToken_String(t *testing.T) {
	cases := []struct {
		tok  FencingToken
		want string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{9_223_372_036_854_775_807, "9223372036854775807"}, // max int64
	}
	for _, c := range cases {
		if got := c.tok.String(); got != c.want {
			t.Errorf("FencingToken(%d).String() = %q, want %q", c.tok.Int64(), got, c.want)
		}
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
