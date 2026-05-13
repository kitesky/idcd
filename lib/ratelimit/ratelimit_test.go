package ratelimit

import (
	"context"
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