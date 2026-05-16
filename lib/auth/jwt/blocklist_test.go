package jwt

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// InMemoryBlocklist
// ---------------------------------------------------------------------------

func TestInMemoryBlocklist_RevokeAndIsRevoked(t *testing.T) {
	bl := NewInMemoryBlocklist()
	ctx := context.Background()

	t.Run("unrevoked jti returns false", func(t *testing.T) {
		got, err := bl.IsRevoked(ctx, "j_never_revoked")
		require.NoError(t, err)
		assert.False(t, got)
	})

	t.Run("revoke then IsRevoked returns true", func(t *testing.T) {
		require.NoError(t, bl.Revoke(ctx, "j_alpha", time.Minute))
		got, err := bl.IsRevoked(ctx, "j_alpha")
		require.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("empty jti is no-op", func(t *testing.T) {
		require.NoError(t, bl.Revoke(ctx, "", time.Minute))
		got, err := bl.IsRevoked(ctx, "")
		require.NoError(t, err)
		assert.False(t, got)
	})

	t.Run("non-positive ttl is no-op", func(t *testing.T) {
		require.NoError(t, bl.Revoke(ctx, "j_zero", 0))
		require.NoError(t, bl.Revoke(ctx, "j_neg", -time.Second))
		zeroGot, _ := bl.IsRevoked(ctx, "j_zero")
		negGot, _ := bl.IsRevoked(ctx, "j_neg")
		assert.False(t, zeroGot)
		assert.False(t, negGot)
	})
}

func TestInMemoryBlocklist_TTLExpiry(t *testing.T) {
	bl := NewInMemoryBlocklist()
	ctx := context.Background()

	// Inject a controllable clock.
	now := time.Now()
	bl.now = func() time.Time { return now }

	require.NoError(t, bl.Revoke(ctx, "j_short", 5*time.Second))

	// Within TTL: revoked.
	got, err := bl.IsRevoked(ctx, "j_short")
	require.NoError(t, err)
	assert.True(t, got)

	// Advance past TTL.
	now = now.Add(10 * time.Second)

	got, err = bl.IsRevoked(ctx, "j_short")
	require.NoError(t, err)
	assert.False(t, got, "expired entries must not be reported as revoked")

	// Lazy GC: expired entry should have been dropped.
	assert.Equal(t, 0, bl.Len(), "expired entries should be reaped on read")
}

func TestInMemoryBlocklist_Concurrent(t *testing.T) {
	bl := NewInMemoryBlocklist()
	ctx := context.Background()

	// Simple race-free smoke test — gofmt's race detector will catch
	// concurrent map access if Revoke / IsRevoked aren't locked.
	done := make(chan struct{}, 100)
	for i := 0; i < 50; i++ {
		go func(i int) {
			_ = bl.Revoke(ctx, "j_"+itoa(i), time.Minute)
			done <- struct{}{}
		}(i)
		go func(i int) {
			_, _ = bl.IsRevoked(ctx, "j_"+itoa(i))
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 100; i++ {
		<-done
	}
}

// itoa is a tiny local helper to avoid importing strconv just for tests.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// ---------------------------------------------------------------------------
// RedisBlocklist
// ---------------------------------------------------------------------------

func newRedisBlocklist(t *testing.T) (*RedisBlocklist, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return NewRedisBlocklist(client), mr
}

func TestRedisBlocklist_RevokeAndIsRevoked(t *testing.T) {
	bl, _ := newRedisBlocklist(t)
	ctx := context.Background()

	t.Run("unrevoked jti returns false", func(t *testing.T) {
		got, err := bl.IsRevoked(ctx, "j_redis_unknown")
		require.NoError(t, err)
		assert.False(t, got)
	})

	t.Run("revoke then IsRevoked returns true", func(t *testing.T) {
		require.NoError(t, bl.Revoke(ctx, "j_redis_alpha", time.Minute))
		got, err := bl.IsRevoked(ctx, "j_redis_alpha")
		require.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("empty / non-positive ttl is no-op", func(t *testing.T) {
		require.NoError(t, bl.Revoke(ctx, "", time.Minute))
		require.NoError(t, bl.Revoke(ctx, "j_zero_ttl", 0))
		got, err := bl.IsRevoked(ctx, "j_zero_ttl")
		require.NoError(t, err)
		assert.False(t, got)
	})
}

func TestRedisBlocklist_TTLExpiry(t *testing.T) {
	bl, mr := newRedisBlocklist(t)
	ctx := context.Background()

	require.NoError(t, bl.Revoke(ctx, "j_redis_ttl", 5*time.Second))

	got, err := bl.IsRevoked(ctx, "j_redis_ttl")
	require.NoError(t, err)
	assert.True(t, got)

	// miniredis supports fake clock advancement.
	mr.FastForward(10 * time.Second)

	got, err = bl.IsRevoked(ctx, "j_redis_ttl")
	require.NoError(t, err)
	assert.False(t, got, "expired entries must not be reported as revoked")
}

func TestRedisBlocklist_LookupError(t *testing.T) {
	// Closing miniredis simulates a Redis outage. IsRevoked must surface
	// the error so the JWT Service can fail-closed.
	bl, mr := newRedisBlocklist(t)
	ctx := context.Background()

	mr.Close() // kill the backend

	_, err := bl.IsRevoked(ctx, "j_anything")
	require.Error(t, err, "blocklist lookup against a dead Redis must error so callers fail-closed")
	assert.Contains(t, err.Error(), "redis blocklist lookup")
}

func TestRedisBlocklist_KeyFormat(t *testing.T) {
	bl, mr := newRedisBlocklist(t)
	ctx := context.Background()

	require.NoError(t, bl.Revoke(ctx, "j_keyfmt", time.Minute))
	// The Redis blocklist must use a namespaced key prefix so it doesn't
	// collide with session / other keys.
	assert.True(t, mr.Exists("jwt:bl:j_keyfmt"), "expected prefixed key in Redis")
}

// ---------------------------------------------------------------------------
// errBlocklist — used by jwt_test.go to exercise the fail-closed Verify path.
// ---------------------------------------------------------------------------

type errBlocklist struct{}

func (errBlocklist) Revoke(_ context.Context, _ string, _ time.Duration) error {
	return errors.New("simulated revoke failure")
}

func (errBlocklist) IsRevoked(_ context.Context, _ string) (bool, error) {
	return false, errors.New("simulated lookup failure")
}
