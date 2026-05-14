package quota

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisIncrClient is the minimal Redis interface required by APIRateLimiter.
// *redis.Client satisfies this interface.
type RedisIncrClient interface {
	Incr(ctx context.Context, key string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	Get(ctx context.Context, key string) *redis.StringCmd
}

// APIRateLimiter enforces per-user daily API call limits using a Redis INCR
// counter that expires after 24 hours.
//
// Redis key scheme: quota:api:{user_id}:{YYYY-MM-DD}
// The date component naturally resets the counter each calendar day (UTC).
type APIRateLimiter struct {
	rdb   RedisIncrClient
	clock func() time.Time // injectable for testing
}

// NewAPIRateLimiter creates an APIRateLimiter backed by the given Redis client.
func NewAPIRateLimiter(rdb RedisIncrClient) *APIRateLimiter {
	return &APIRateLimiter{rdb: rdb, clock: time.Now}
}

// apiKey builds the Redis key for the given user on the current UTC date.
func (r *APIRateLimiter) apiKey(userID string, now time.Time) string {
	return fmt.Sprintf("quota:api:%s:%s", userID, now.UTC().Format("2006-01-02"))
}

// Allow checks whether the user has capacity for one more API call under their
// plan's daily limit, and atomically increments the counter.
//
// Returns:
//   - allowed: true if the call is within quota
//   - used: number of calls used (including this one) after incrementing
//   - limit: the plan's daily limit (0 = unlimited)
//   - err: non-nil only for Redis errors
func (r *APIRateLimiter) Allow(ctx context.Context, userID string, plan string) (allowed bool, used int, limit int, err error) {
	l := Limits(plan)
	limit = l.MaxAPIDailyReqs

	now := r.now()
	key := r.apiKey(userID, now)

	// Atomically increment the counter.
	count, err := r.rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, 0, limit, fmt.Errorf("quota: redis incr failed: %w", err)
	}

	// Set expiry on the first increment (count == 1) to ensure automatic
	// cleanup. On subsequent calls the key already has a TTL.
	if count == 1 {
		// Expire in 25h to give a little slack across day boundaries.
		if expErr := r.rdb.Expire(ctx, key, 25*time.Hour).Err(); expErr != nil {
			// Best-effort; log but don't fail the request.
			_ = expErr
		}
	}

	used = int(count)

	// 0 = unlimited, always allow.
	if limit == 0 {
		return true, used, limit, nil
	}

	if used > limit {
		return false, used, limit, nil
	}

	return true, used, limit, nil
}

// CurrentUsage returns the current API call count for the user on the current
// UTC day without modifying the counter.
func (r *APIRateLimiter) CurrentUsage(ctx context.Context, userID string) (used int, err error) {
	now := r.now()
	key := r.apiKey(userID, now)

	val, err := r.rdb.Get(ctx, key).Int()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("quota: redis get failed: %w", err)
	}
	return val, nil
}

func (r *APIRateLimiter) now() time.Time {
	if r.clock != nil {
		return r.clock()
	}
	return time.Now()
}
