package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestLimiter_Allow(t *testing.T) {
	// Start mock Redis server
	s := miniredis.RunT(t)
	defer s.Close()

	// Create Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	// Create limiter with 3 requests per 10 seconds
	config := Config{
		WindowSize:  10 * time.Second,
		MaxRequests: 3,
		KeyPrefix:   "test:",
	}
	limiter := NewLimiter(rdb, config)

	ctx := context.Background()
	key := "test-key"

	t.Run("allows requests within limit", func(t *testing.T) {
		// Clear any existing data
		s.FlushAll()

		// First 3 requests should be allowed
		for i := 0; i < 3; i++ {
			result, err := limiter.Allow(ctx, key)
			if err != nil {
				t.Fatalf("request %d: unexpected error: %v", i+1, err)
			}
			if !result.Allowed {
				t.Errorf("request %d: expected allowed=true, got false", i+1)
			}
			expectedRemaining := int64(3 - i - 1)
			if result.Remaining != expectedRemaining {
				t.Errorf("request %d: expected remaining=%d, got %d", i+1, expectedRemaining, result.Remaining)
			}
		}
	})

	t.Run("denies requests over limit", func(t *testing.T) {
		// 4th request should be denied
		result, err := limiter.Allow(ctx, key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Allowed {
			t.Error("expected allowed=false for request over limit, got true")
		}
		if result.Remaining != 0 {
			t.Errorf("expected remaining=0 for denied request, got %d", result.Remaining)
		}
	})

	t.Run("resets after window expires", func(t *testing.T) {
		// Inject a clock 11 seconds in the future so the Lua script sees
		// entries as expired (window is 10s). miniredis FastForward does not
		// affect the Go-side time.Now() used as the ARGV timestamp, so we
		// use the clock hook instead.
		limiter.clock = func() time.Time { return time.Now().Add(11 * time.Second) }
		defer func() { limiter.clock = nil }()

		result, err := limiter.Allow(ctx, key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Allowed {
			t.Error("expected allowed=true after window reset, got false")
		}
		if result.Remaining != 2 {
			t.Errorf("expected remaining=2 after window reset, got %d", result.Remaining)
		}
	})

	t.Run("different keys are independent", func(t *testing.T) {
		s.FlushAll()

		// Use up limit for key1
		key1 := "key1"
		for i := 0; i < 3; i++ {
			_, err := limiter.Allow(ctx, key1)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}

		// key1 should be denied
		result, err := limiter.Allow(ctx, key1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Allowed {
			t.Error("expected key1 to be denied")
		}

		// key2 should still be allowed
		key2 := "key2"
		result, err = limiter.Allow(ctx, key2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Allowed {
			t.Error("expected key2 to be allowed")
		}
	})

	t.Run("reset time is approximately window size in future", func(t *testing.T) {
		s.FlushAll()

		before := time.Now()
		result, err := limiter.Allow(ctx, "reset-test")
		after := time.Now()

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedMin := before.Add(config.WindowSize)
		expectedMax := after.Add(config.WindowSize)

		if result.ResetAt.Before(expectedMin) || result.ResetAt.After(expectedMax) {
			t.Errorf("reset time %v not within expected range [%v, %v]",
				result.ResetAt, expectedMin, expectedMax)
		}
	})
}

func TestKeyGenerators(t *testing.T) {
	tests := []struct {
		name     string
		function func(string) string
		input    string
		expected string
	}{
		{
			name:     "KeyIP generates correct format",
			function: KeyIP,
			input:    "192.168.1.1",
			expected: "ip:192.168.1.1",
		},
		{
			name:     "KeyUser generates correct format",
			function: KeyUser,
			input:    "user123",
			expected: "user:user123",
		},
		{
			name:     "KeyTarget generates correct format",
			function: KeyTarget,
			input:    "api.example.com",
			expected: "target:api.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.function(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestConfig_Validation(t *testing.T) {
	// Start mock Redis server
	s := miniredis.RunT(t)
	defer s.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	t.Run("works with different window sizes", func(t *testing.T) {
		configs := []Config{
			{WindowSize: time.Second, MaxRequests: 1, KeyPrefix: "1s:"},
			{WindowSize: time.Minute, MaxRequests: 60, KeyPrefix: "1m:"},
			{WindowSize: time.Hour, MaxRequests: 1000, KeyPrefix: "1h:"},
		}

		for i, config := range configs {
			limiter := NewLimiter(rdb, config)

			result, err := limiter.Allow(context.Background(), "test")
			if err != nil {
				t.Errorf("config %d: unexpected error: %v", i, err)
			}
			if !result.Allowed {
				t.Errorf("config %d: expected first request to be allowed", i)
			}
		}
	})
}

func TestLimiter_RedisError(t *testing.T) {
	// Create Redis client pointing to non-existent server
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:99999", // Non-existent port
	})

	limiter := NewLimiter(rdb, Config{
		WindowSize:  time.Second,
		MaxRequests: 1,
		KeyPrefix:   "test:",
	})

	// Should return error when Redis is unreachable
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := limiter.Allow(ctx, "test")
	if err == nil {
		t.Error("expected error when Redis is unreachable, got nil")
	}
}

// TestLimiter_NoMemberCollision_HighConcurrency is the regression test for
// P2#23. It pins the wall-clock to a single millisecond so that the only
// uniqueness left in the ZSET member is whatever the Lua script appends
// after the timestamp. Under the previous math.random() implementation
// this test would almost always under-count; with the INCR-based fix the
// counts must match exactly.
func TestLimiter_NoMemberCollision_HighConcurrency(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})

	const concurrency = 1000
	cfg := Config{
		WindowSize:  time.Hour,
		MaxRequests: int64(concurrency), // capacity == workers, all should be allowed
		KeyPrefix:   "collision:",
	}
	limiter := NewLimiter(rdb, cfg)

	// Pin the clock to a single instant — this is the worst case for member
	// collisions because the timestamp prefix is identical for every call.
	pinned := time.Unix(1_700_000_000, 0)
	limiter.clock = func() time.Time { return pinned }

	key := "stress-key"

	var (
		allowed   atomic.Int64
		denied    atomic.Int64
		errCount  atomic.Int64
		latencies = make([]time.Duration, concurrency)
		wg        sync.WaitGroup
		start     = make(chan struct{})
	)

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			<-start // release all goroutines at once for maximum contention
			t0 := time.Now()
			res, err := limiter.Allow(context.Background(), key)
			latencies[idx] = time.Since(t0)
			if err != nil {
				errCount.Add(1)
				return
			}
			if res.Allowed {
				allowed.Add(1)
			} else {
				denied.Add(1)
			}
		}(i)
	}

	close(start)
	wg.Wait()

	if errCount.Load() != 0 {
		t.Fatalf("unexpected errors during stress: %d", errCount.Load())
	}

	// Expectation: capacity == workers, so every call must be allowed exactly
	// once. Any "missing" count means a ZADD was deduped — exactly the bug
	// math.random() caused.
	if got := allowed.Load(); got != int64(concurrency) {
		t.Fatalf("expected allowed=%d, got %d (denied=%d) — member collision still under-counts",
			concurrency, got, denied.Load())
	}
	if got := denied.Load(); got != 0 {
		t.Fatalf("expected 0 denials when capacity == workers, got %d", got)
	}

	// Verify the ZSET on the Redis side has exactly `concurrency` distinct
	// members. With math.random() collisions this would be < concurrency.
	zcard, err := rdb.ZCard(context.Background(), "collision:"+key).Result()
	if err != nil {
		t.Fatalf("ZCARD failed: %v", err)
	}
	if zcard != int64(concurrency) {
		t.Fatalf("expected ZSET cardinality=%d, got %d — distinct members lost",
			concurrency, zcard)
	}

	// The concurrent-end-to-end latencies include head-of-line blocking
	// from miniredis serializing 1000 EVALs onto a single goroutine. That's
	// a test-fixture artifact, not a Lua-script-cost signal. So we report
	// the concurrent p99 for visibility but assert latency separately via
	// a serial measurement below — which actually reflects per-call cost.
	sortDurations(latencies)
	concurrentP99 := latencies[int(float64(len(latencies))*0.99)]
	t.Logf("stress ok: %d allowed, %d denied, ZCARD=%d, concurrent p99=%v (includes miniredis HOL blocking)",
		allowed.Load(), denied.Load(), zcard, concurrentP99)

	// Serial p99: now that the ZSET is at capacity, every call goes the
	// denial path. We pick the cheaper path so we measure the steady-state
	// hot loop. Sample 1000 calls back-to-back.
	const samples = 1000
	serialLats := make([]time.Duration, samples)
	for i := 0; i < samples; i++ {
		t0 := time.Now()
		if _, err := limiter.Allow(context.Background(), key); err != nil {
			t.Fatalf("serial sample %d: %v", i, err)
		}
		serialLats[i] = time.Since(t0)
	}
	sortDurations(serialLats)
	serialP99 := serialLats[int(float64(samples)*0.99)]
	if serialP99 > 10*time.Millisecond {
		t.Fatalf("serial per-call p99 latency %v exceeds 10ms budget", serialP99)
	}
	t.Logf("serial p99 per-call latency: %v (budget 10ms)", serialP99)
}

// sortDurations is a tiny in-place insertion sort. Avoids importing the
// sort package for one call inside a test.
func sortDurations(d []time.Duration) {
	for i := 1; i < len(d); i++ {
		for j := i; j > 0 && d[j-1] > d[j]; j-- {
			d[j-1], d[j] = d[j], d[j-1]
		}
	}
}

// TestLimiter_DenialsAreAccurate verifies the *denial* path under contention:
// when capacity < workers, the count of allowed calls must equal capacity
// exactly. Under the math.random() bug this would sometimes be > capacity
// because a ZADD that should have grown the set to `max` got deduped, so the
// next caller still saw `count < max` and was admitted.
func TestLimiter_DenialsAreAccurate(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})

	const (
		concurrency = 1000
		capacity    = 100
	)
	cfg := Config{
		WindowSize:  time.Hour,
		MaxRequests: capacity,
		KeyPrefix:   "denial:",
	}
	limiter := NewLimiter(rdb, cfg)
	pinned := time.Unix(1_700_000_000, 0)
	limiter.clock = func() time.Time { return pinned }

	var (
		allowed atomic.Int64
		denied  atomic.Int64
		wg      sync.WaitGroup
		start   = make(chan struct{})
	)

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			<-start
			res, err := limiter.Allow(context.Background(), "key")
			if err != nil {
				t.Errorf("unexpected err: %v", err)
				return
			}
			if res.Allowed {
				allowed.Add(1)
			} else {
				denied.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if allowed.Load() != capacity {
		t.Fatalf("expected exactly %d allowed (capacity), got %d", capacity, allowed.Load())
	}
	if allowed.Load()+denied.Load() != concurrency {
		t.Fatalf("accounting mismatch: %d+%d != %d",
			allowed.Load(), denied.Load(), concurrency)
	}
}

// Benchmark to ensure performance is reasonable
func BenchmarkLimiter_Allow(b *testing.B) {
	s := miniredis.RunT(b)
	defer s.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	limiter := NewLimiter(rdb, Config{
		WindowSize:  time.Hour,
		MaxRequests: 1000000, // High limit to avoid rate limiting during benchmark
		KeyPrefix:   "bench:",
	})

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := "bench-key-" + string(rune(i%10)) // Use 10 different keys
			_, err := limiter.Allow(ctx, key)
			if err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
			i++
		}
	})
}