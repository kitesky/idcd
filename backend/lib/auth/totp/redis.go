package totp

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisReplayStore implements ReplayStore using Redis SET NX for atomic
// single-use enforcement of TOTP codes within the replay window.
type RedisReplayStore struct {
	client *redis.Client
}

// NewRedisReplayStore returns a replay store backed by Redis.
func NewRedisReplayStore(client *redis.Client) *RedisReplayStore {
	return &RedisReplayStore{client: client}
}

// Mark atomically claims key via SET NX. Returns true on first use, false on
// replay (key already existed). The single Redis round-trip makes replay
// detection atomic — unlike the non-atomic Exists+Set pattern it replaces.
func (r *RedisReplayStore) Mark(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	ok, err := r.client.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("totp replay store: %w", err)
	}
	return ok, nil
}
