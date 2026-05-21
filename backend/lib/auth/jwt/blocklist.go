// Package jwt — JTI blocklist support.
//
// A JTIBlocklist stores revoked token IDs (the "jti" RegisteredClaim) with
// a TTL bounded by the original token's remaining lifetime. Once a token's
// jti is revoked, Verify rejects it even though the signature and expiry
// are still valid.
//
// Two implementations ship in this package:
//
//   - InMemoryBlocklist: process-local map + TTL pruning. Used in tests
//     and single-process deployments. NOT safe across multiple API replicas.
//   - RedisBlocklist: backed by go-redis. Production default. Survives
//     restarts and is shared across all API replicas.
//
// Callers wire an implementation into the JWT Service via WithBlocklist.
// When no blocklist is configured the Service skips revocation checks
// entirely (legacy / opt-in behavior).
package jwt

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrTokenRevoked is returned by Verify when the token's jti is on the
// blocklist (or when the blocklist lookup itself failed — see fail-closed
// note in the package docs).
var ErrTokenRevoked = errors.New("token revoked")

// JTIBlocklist tracks revoked JWT IDs.
//
// Implementations MUST be safe for concurrent use. TTL is the maximum time
// the entry needs to live — typically the token's remaining lifetime. After
// TTL the entry may be reaped because the token would be rejected by its
// own expiry check anyway.
type JTIBlocklist interface {
	// Revoke marks jti as revoked for at most ttl. A non-positive ttl
	// is treated as already-expired and is a no-op (the token would
	// already fail signature/expiry checks).
	Revoke(ctx context.Context, jti string, ttl time.Duration) error

	// IsRevoked reports whether jti is on the blocklist. The boolean is
	// only meaningful when err == nil. On error the caller MUST decide
	// fail-open vs fail-closed; the JWT Service is fail-closed by
	// default (treats lookup errors as "revoked").
	IsRevoked(ctx context.Context, jti string) (bool, error)
}

// ---------------------------------------------------------------------------
// In-memory implementation (tests, single-process deployments).
// ---------------------------------------------------------------------------

// InMemoryBlocklist is a thread-safe in-memory JTIBlocklist with lazy TTL
// reaping. Suitable for tests and small single-replica deployments.
type InMemoryBlocklist struct {
	mu      sync.RWMutex
	entries map[string]time.Time // jti -> expiresAt
	now     func() time.Time     // injectable for tests; defaults to time.Now
}

// NewInMemoryBlocklist returns a fresh in-memory blocklist.
func NewInMemoryBlocklist() *InMemoryBlocklist {
	return &InMemoryBlocklist{
		entries: make(map[string]time.Time),
		now:     time.Now,
	}
}

// Revoke records jti with the given ttl. Negative or zero ttl is a no-op.
func (b *InMemoryBlocklist) Revoke(_ context.Context, jti string, ttl time.Duration) error {
	if jti == "" {
		return nil
	}
	if ttl <= 0 {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries[jti] = b.now().Add(ttl)
	return nil
}

// IsRevoked reports whether jti is currently revoked. Expired entries are
// removed lazily on read.
func (b *InMemoryBlocklist) IsRevoked(_ context.Context, jti string) (bool, error) {
	if jti == "" {
		return false, nil
	}
	b.mu.RLock()
	exp, ok := b.entries[jti]
	b.mu.RUnlock()
	if !ok {
		return false, nil
	}
	if b.now().After(exp) {
		// Lazy GC: drop the entry so the map doesn't grow unbounded.
		b.mu.Lock()
		if exp2, still := b.entries[jti]; still && !b.now().Before(exp2) {
			delete(b.entries, jti)
		}
		b.mu.Unlock()
		return false, nil
	}
	return true, nil
}

// Len returns the (best-effort) entry count. Test helper; counts expired
// entries that haven't been reaped yet.
func (b *InMemoryBlocklist) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.entries)
}

// ---------------------------------------------------------------------------
// Redis-backed implementation (production).
// ---------------------------------------------------------------------------

const redisBlocklistPrefix = "jwt:bl:" // jti

// RedisBlocklist stores revoked JTIs in Redis with native key TTLs.
type RedisBlocklist struct {
	client redis.UniversalClient
}

// NewRedisBlocklist returns a Redis-backed blocklist. The caller owns the
// client lifecycle.
func NewRedisBlocklist(client redis.UniversalClient) *RedisBlocklist {
	return &RedisBlocklist{client: client}
}

// Revoke writes jwt:bl:<jti> with the given ttl. A non-positive ttl is a
// no-op (matches InMemoryBlocklist).
func (b *RedisBlocklist) Revoke(ctx context.Context, jti string, ttl time.Duration) error {
	if jti == "" {
		return nil
	}
	if ttl <= 0 {
		return nil
	}
	if err := b.client.Set(ctx, b.key(jti), "1", ttl).Err(); err != nil {
		return fmt.Errorf("redis blocklist revoke: %w", err)
	}
	return nil
}

// IsRevoked reports whether jwt:bl:<jti> exists. Returns (false, nil) when
// the key is missing. Wraps redis errors so the caller can fail-closed.
func (b *RedisBlocklist) IsRevoked(ctx context.Context, jti string) (bool, error) {
	if jti == "" {
		return false, nil
	}
	_, err := b.client.Get(ctx, b.key(jti)).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("redis blocklist lookup: %w", err)
	}
	return true, nil
}

func (b *RedisBlocklist) key(jti string) string {
	return redisBlocklistPrefix + jti
}
