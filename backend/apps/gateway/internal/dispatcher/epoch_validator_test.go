package dispatcher

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newValidatorTestRedis(t *testing.T) (*miniredis.Miniredis, redis.Cmdable) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, rdb
}

func newSilentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// TestConsumer_AcceptsFirstEpoch covers the cold-start path: a fresh gateway
// with no persisted high-water sees its first epoched message and must
// accept it (highWater is 0; any positive epoch >= 0).
func TestConsumer_AcceptsFirstEpoch(t *testing.T) {
	_, rdb := newValidatorTestRedis(t)
	v := newEpochValidator(rdb, newSilentLogger())
	ctx := context.Background()

	dec, obs := v.validate(ctx, map[string]any{"epoch": "5"})
	if dec != epochAccept {
		t.Errorf("decision = %v, want epochAccept", dec)
	}
	if obs != 5 {
		t.Errorf("observed = %d, want 5", obs)
	}
}

// TestConsumer_DropsStaleEpoch exercises the headline split-brain defence:
// after seeing epoch=5 the validator must drop epoch=4.
func TestConsumer_DropsStaleEpoch(t *testing.T) {
	_, rdb := newValidatorTestRedis(t)
	v := newEpochValidator(rdb, newSilentLogger())
	ctx := context.Background()

	if dec, _ := v.validate(ctx, map[string]any{"epoch": "5"}); dec != epochAccept {
		t.Fatalf("priming epoch=5 should accept, got %v", dec)
	}

	dec, obs := v.validate(ctx, map[string]any{"epoch": "4"})
	if dec != epochDropStale {
		t.Errorf("decision = %v, want epochDropStale", dec)
	}
	if obs != 4 {
		t.Errorf("observed = %d, want 4", obs)
	}
}

// TestConsumer_AcceptsEqualOrGreater verifies the validator accepts messages
// at the current high-water mark (duplicate from same leader) and at higher
// epochs (new leader).
func TestConsumer_AcceptsEqualOrGreater(t *testing.T) {
	_, rdb := newValidatorTestRedis(t)
	v := newEpochValidator(rdb, newSilentLogger())
	ctx := context.Background()

	if dec, _ := v.validate(ctx, map[string]any{"epoch": "3"}); dec != epochAccept {
		t.Fatalf("priming epoch=3 should accept, got %v", dec)
	}

	// Same epoch — accept (within-leader retry).
	if dec, _ := v.validate(ctx, map[string]any{"epoch": "3"}); dec != epochAccept {
		t.Errorf("equal epoch should accept, got %v", dec)
	}

	// Higher epoch — accept and advance.
	if dec, _ := v.validate(ctx, map[string]any{"epoch": "4"}); dec != epochAccept {
		t.Errorf("higher epoch should accept, got %v", dec)
	}

	// Going back to 3 must now be stale.
	if dec, _ := v.validate(ctx, map[string]any{"epoch": "3"}); dec != epochDropStale {
		t.Errorf("post-advance, epoch=3 should be stale, got %v", dec)
	}
}

// TestConsumer_HighWaterPersisted covers the cross-restart durability case:
// validator A bumps the mark to 9, then a brand-new validator B (simulating
// a restarted gateway) sees epoch=7 and must reject it because the persisted
// mark is 9 from before.
func TestConsumer_HighWaterPersisted(t *testing.T) {
	_, rdb := newValidatorTestRedis(t)
	ctx := context.Background()
	log := newSilentLogger()

	vA := newEpochValidator(rdb, log)
	if dec, _ := vA.validate(ctx, map[string]any{"epoch": "9"}); dec != epochAccept {
		t.Fatalf("priming epoch=9 should accept, got %v", dec)
	}

	// Sanity check that the Redis key holds 9.
	raw, err := rdb.Get(ctx, epochHighWaterKey).Result()
	if err != nil {
		t.Fatalf("GET high-water: %v", err)
	}
	if raw != "9" {
		t.Fatalf("persisted high-water = %q, want %q", raw, "9")
	}

	// Brand-new validator instance — simulates a restarted gateway with
	// in-memory mark=0. Must load 9 from Redis and reject 7.
	vB := newEpochValidator(rdb, log)
	dec, obs := vB.validate(ctx, map[string]any{"epoch": "7"})
	if dec != epochDropStale {
		t.Errorf("post-restart, epoch=7 vs persisted=9 should drop, got %v", dec)
	}
	if obs != 7 {
		t.Errorf("observed = %d, want 7", obs)
	}
}

// TestConsumer_MissingEpoch_BackwardCompat covers the migration window:
// messages from a not-yet-upgraded scheduler have no epoch field and must
// be accepted with a "missing" decision so the dispatcher can still
// forward the work (counter fires for observability).
func TestConsumer_MissingEpoch_BackwardCompat(t *testing.T) {
	_, rdb := newValidatorTestRedis(t)
	v := newEpochValidator(rdb, newSilentLogger())
	ctx := context.Background()

	dec, _ := v.validate(ctx, map[string]any{"task_id": "pt_legacy"})
	if dec != epochAcceptMissing {
		t.Errorf("missing epoch field should be epochAcceptMissing, got %v", dec)
	}
}

// TestConsumer_ZeroEpochBackwardCompat covers the case where the scheduler
// has been redeployed but its Config.Epoch defaulted to zero (e.g.
// AcquireEpoch wiring not in place yet). Treated identically to missing.
func TestConsumer_ZeroEpochBackwardCompat(t *testing.T) {
	_, rdb := newValidatorTestRedis(t)
	v := newEpochValidator(rdb, newSilentLogger())
	ctx := context.Background()

	dec, _ := v.validate(ctx, map[string]any{"epoch": "0"})
	if dec != epochAcceptMissing {
		t.Errorf("zero epoch should be epochAcceptMissing, got %v", dec)
	}
}

// TestConsumer_UnparseableEpoch verifies a malformed epoch field doesn't
// crash the validator — it's logged + treated as missing under the compat
// window.
func TestConsumer_UnparseableEpoch(t *testing.T) {
	_, rdb := newValidatorTestRedis(t)
	v := newEpochValidator(rdb, newSilentLogger())
	ctx := context.Background()

	dec, _ := v.validate(ctx, map[string]any{"epoch": "not-a-number"})
	if dec != epochAcceptMissing {
		t.Errorf("unparseable epoch should be epochAcceptMissing, got %v", dec)
	}
}

// TestConsumer_CASMaxSemantics directly exercises the Lua script: concurrent
// writes of differing epochs must converge on max, not whichever wrote last.
func TestConsumer_CASMaxSemantics(t *testing.T) {
	_, rdb := newValidatorTestRedis(t)
	v := newEpochValidator(rdb, newSilentLogger())
	ctx := context.Background()

	// Walk through 10 → 20 → 15. Final persisted value must be 20.
	if dec, _ := v.validate(ctx, map[string]any{"epoch": "10"}); dec != epochAccept {
		t.Fatalf("epoch=10 should accept, got %v", dec)
	}
	if dec, _ := v.validate(ctx, map[string]any{"epoch": "20"}); dec != epochAccept {
		t.Fatalf("epoch=20 should accept, got %v", dec)
	}
	// Direct CAS via a fresh validator: epoch=15 must NOT regress the
	// persisted mark. (We use a fresh validator so its in-memory cache
	// doesn't short-circuit the Redis hit.)
	v2 := newEpochValidator(rdb, newSilentLogger())
	dec, _ := v2.validate(ctx, map[string]any{"epoch": "15"})
	if dec != epochDropStale {
		t.Errorf("epoch=15 vs persisted=20 should drop, got %v", dec)
	}

	raw, err := rdb.Get(ctx, epochHighWaterKey).Result()
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	n, _ := strconv.ParseInt(raw, 10, 64)
	if n != 20 {
		t.Errorf("persisted high-water = %d, want 20 (never regress)", n)
	}
}
