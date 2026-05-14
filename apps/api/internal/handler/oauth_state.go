package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisStateStore implements OAuthStateStore using Redis.
type redisStateStore struct {
	redis *redis.Client
}

// NewRedisStateStore creates an OAuthStateStore backed by Redis.
func NewRedisStateStore(r *redis.Client) OAuthStateStore {
	return &redisStateStore{redis: r}
}

func (s *redisStateStore) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if err := s.redis.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("state store set: %w", err)
	}
	return nil
}

func (s *redisStateStore) Get(ctx context.Context, key string) (string, error) {
	val, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("state store get: %w", err)
	}
	return val, nil
}

func (s *redisStateStore) Del(ctx context.Context, key string) error {
	if err := s.redis.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("state store del: %w", err)
	}
	return nil
}
