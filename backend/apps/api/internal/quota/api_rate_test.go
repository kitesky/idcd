package quota

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newTestRateLimiter creates an APIRateLimiter backed by an in-process miniredis.
func newTestRateLimiter(t *testing.T) (*APIRateLimiter, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	return NewAPIRateLimiter(rdb), mr
}

// withFixedTime returns a rate limiter whose clock always returns t.
func withFixedTime(rl *APIRateLimiter, fixedTime time.Time) *APIRateLimiter {
	rl.clock = func() time.Time { return fixedTime }
	return rl
}

// ─────────────────────────────────────────────────────────────────────────────
// Basic allow / deny
// ─────────────────────────────────────────────────────────────────────────────

func TestAPIRateLimiter_FreeAllows100(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	ctx := context.Background()
	const userID = "u_free"

	for i := 1; i <= 100; i++ {
		allowed, used, limit, err := rl.Allow(ctx, userID, "free")
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
		if !allowed {
			t.Fatalf("call %d should be allowed, got denied (used=%d limit=%d)", i, used, limit)
		}
		if used != i {
			t.Errorf("call %d: used=%d want %d", i, used, i)
		}
		if limit != 100 {
			t.Errorf("call %d: limit=%d want 100", i, limit)
		}
	}
}

func TestAPIRateLimiter_FreeDeniesAt101(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	ctx := context.Background()
	const userID = "u_free101"

	// Exhaust the quota.
	for i := 0; i < 100; i++ {
		_, _, _, err := rl.Allow(ctx, userID, "free")
		if err != nil {
			t.Fatalf("setup call %d failed: %v", i, err)
		}
	}

	// 101st call should be denied.
	allowed, used, limit, err := rl.Allow(ctx, userID, "free")
	if err != nil {
		t.Fatalf("unexpected error on 101st call: %v", err)
	}
	if allowed {
		t.Errorf("101st call should be denied (used=%d limit=%d)", used, limit)
	}
}

func TestAPIRateLimiter_ProAllows5000(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	ctx := context.Background()
	const userID = "u_pro"

	for i := 1; i <= 5000; i++ {
		allowed, _, _, err := rl.Allow(ctx, userID, "pro")
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
		if !allowed {
			t.Fatalf("call %d should be allowed for pro plan", i)
		}
	}
}

func TestAPIRateLimiter_ProDeniesAt5001(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	ctx := context.Background()
	const userID = "u_pro5001"

	for i := 0; i < 5000; i++ {
		_, _, _, _ = rl.Allow(ctx, userID, "pro")
	}

	allowed, _, _, err := rl.Allow(ctx, userID, "pro")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("5001st call should be denied for pro plan")
	}
}

func TestAPIRateLimiter_BusinessUnlimited(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	ctx := context.Background()
	const userID = "u_business"

	// Even after many calls, business plan should never be denied.
	for i := 0; i < 200; i++ {
		allowed, _, limit, err := rl.Allow(ctx, userID, "business")
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
		if !allowed {
			t.Errorf("call %d should be allowed for business (unlimited), limit=%d", i, limit)
		}
		if limit != 0 {
			t.Errorf("call %d: business plan limit should be 0 (unlimited), got %d", i, limit)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Cross-day reset
// ─────────────────────────────────────────────────────────────────────────────

func TestAPIRateLimiter_ResetsNextDay(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	ctx := context.Background()
	const userID = "u_crossday"

	// Exhaust quota on "today".
	day1 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	rlDay1 := withFixedTime(rl, day1)
	for i := 0; i < 100; i++ {
		_, _, _, _ = rlDay1.Allow(ctx, userID, "free")
	}

	// Verify denied.
	allowed, _, _, _ := rlDay1.Allow(ctx, userID, "free")
	if allowed {
		t.Error("should be denied on day1 after exhaustion")
	}

	// Advance to the next day — different Redis key.
	day2 := day1.Add(24 * time.Hour)
	rlDay2 := withFixedTime(rl, day2)

	allowed, used, _, err := rlDay2.Allow(ctx, userID, "free")
	if err != nil {
		t.Fatalf("day2 call: unexpected error: %v", err)
	}
	if !allowed {
		t.Error("first call on day2 should be allowed (counter reset)")
	}
	if used != 1 {
		t.Errorf("day2 first call: used=%d want 1", used)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// CurrentUsage
// ─────────────────────────────────────────────────────────────────────────────

func TestAPIRateLimiter_CurrentUsage_Zero(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	ctx := context.Background()

	used, err := rl.CurrentUsage(ctx, "u_notrequests")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if used != 0 {
		t.Errorf("want 0, got %d", used)
	}
}

func TestAPIRateLimiter_CurrentUsage_AfterCalls(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	ctx := context.Background()
	const userID = "u_usage"

	for i := 0; i < 5; i++ {
		_, _, _, _ = rl.Allow(ctx, userID, "pro")
	}

	used, err := rl.CurrentUsage(ctx, userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if used != 5 {
		t.Errorf("want 5, got %d", used)
	}
}

func TestAPIRateLimiter_CurrentUsage_DoesNotIncrement(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	ctx := context.Background()
	const userID = "u_currentonly"

	_, _, _, _ = rl.Allow(ctx, userID, "free") // used=1

	// Calling CurrentUsage multiple times should not change the counter.
	for i := 0; i < 3; i++ {
		used, err := rl.CurrentUsage(ctx, userID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if used != 1 {
			t.Errorf("iteration %d: CurrentUsage returned %d, want 1", i, used)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Used + limit return values
// ─────────────────────────────────────────────────────────────────────────────

func TestAPIRateLimiter_ReturnedLimitMatchesPlan(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	ctx := context.Background()

	cases := []struct {
		plan  string
		limit int
	}{
		{"free", 100},
		{"pro", 5000},
		{"team", 50000},
		{"business", 0},
	}
	for _, tc := range cases {
		_, _, limit, err := rl.Allow(ctx, "u_"+tc.plan, tc.plan)
		if err != nil {
			t.Fatalf("plan=%s: unexpected error: %v", tc.plan, err)
		}
		if limit != tc.limit {
			t.Errorf("plan=%s: limit=%d want %d", tc.plan, limit, tc.limit)
		}
	}
}
