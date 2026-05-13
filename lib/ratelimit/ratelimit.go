// Package ratelimit implements Redis-based sliding window rate limiting.
// Uses atomic Lua scripts for consistent rate limit enforcement.
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter implements Redis-based sliding window rate limiting.
type Limiter struct {
	rdb    RedisClient
	config Config
	clock  func() time.Time // injectable for testing; defaults to time.Now
}

// RedisClient interface allows mocking Redis for tests.
type RedisClient interface {
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
}

// Config holds rate limiting configuration.
type Config struct {
	// WindowSize is the sliding window duration (e.g., 1h, 24h)
	WindowSize time.Duration
	// MaxRequests is the maximum number of requests allowed within the window
	MaxRequests int64
	// KeyPrefix is the Redis key prefix to avoid collisions
	KeyPrefix string
}

// Result contains the result of a rate limit check.
type Result struct {
	Allowed   bool      // Whether the request is allowed
	Remaining int64     // Number of requests remaining in the window
	ResetAt   time.Time // When the window resets (next oldest request expires)
}

// NewLimiter creates a new rate limiter instance.
func NewLimiter(rdb RedisClient, cfg Config) *Limiter {
	return &Limiter{
		rdb:    rdb,
		config: cfg,
	}
}

// Allow checks if a request should be allowed based on the sliding window algorithm.
// Returns a Result indicating whether the request is allowed, remaining quota, and reset time.
func (l *Limiter) Allow(ctx context.Context, key string) (*Result, error) {
	fullKey := l.config.KeyPrefix + key
	now := l.nowTime()
	nowMs := now.UnixMilli()
	windowMs := l.config.WindowSize.Milliseconds()
	ttlSec := int64(l.config.WindowSize.Seconds()) + 60

	luaScript := `
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])
		local max = tonumber(ARGV[3])
		local ttl = tonumber(ARGV[4])

		redis.call('ZREMRANGEBYSCORE', key, 0, now - window)

		local count = redis.call('ZCARD', key)

		if count < max then
			local member = now .. ':' .. math.random()
			redis.call('ZADD', key, now, member)
			redis.call('EXPIRE', key, ttl)
			return {1, max - count - 1}
		else
			return {0, 0}
		end
	`

	cmd := l.rdb.Eval(ctx, luaScript, []string{fullKey}, nowMs, windowMs, l.config.MaxRequests, ttlSec)
	if err := cmd.Err(); err != nil {
		return nil, fmt.Errorf("rate limit check failed: %w", err)
	}

	result, ok := cmd.Val().([]interface{})
	if !ok || len(result) != 2 {
		return nil, fmt.Errorf("unexpected Lua script result: %v", cmd.Val())
	}

	allowedInt, ok1 := result[0].(int64)
	remainingInt, ok2 := result[1].(int64)
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("invalid Lua script result types: %v", result)
	}

	return &Result{
		Allowed:   allowedInt == 1,
		Remaining: remainingInt,
		ResetAt:   now.Add(l.config.WindowSize),
	}, nil
}

func (l *Limiter) nowTime() time.Time {
	if l.clock != nil {
		return l.clock()
	}
	return time.Now()
}

// --- Key generation helpers ---

// KeyIP generates a rate limit key for IP-based limiting.
func KeyIP(ip string) string {
	return "ip:" + ip
}

// KeyUser generates a rate limit key for user-based limiting.
func KeyUser(userID string) string {
	return "user:" + userID
}

// KeyTarget generates a rate limit key for target-based limiting (e.g., domain).
func KeyTarget(target string) string {
	return "target:" + target
}