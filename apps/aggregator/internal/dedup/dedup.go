// Package dedup provides idempotency for aggregator processing using Redis.
// Prevents duplicate processing of probe results by tracking task_id with 24h TTL.
package dedup

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Dedup TTL - how long to remember processed task_ids
	defaultTTL = 24 * time.Hour

	// Redis key prefix for dedup tracking
	keyPrefix = "agg:dedup:"
)

// Deduper handles idempotency checking and marking for aggregator processing.
type Deduper struct {
	rdb redis.Cmdable
	ttl time.Duration
}

// New creates a Deduper with the default 24h TTL.
func New(rdb redis.Cmdable) *Deduper {
	return &Deduper{
		rdb: rdb,
		ttl: defaultTTL,
	}
}

// NewWithTTL creates a Deduper with a custom TTL.
func NewWithTTL(rdb redis.Cmdable, ttl time.Duration) *Deduper {
	return &Deduper{
		rdb: rdb,
		ttl: ttl,
	}
}

// IsDuplicate checks if a task_id has already been processed.
// Returns true if the task_id is already in Redis, false otherwise.
func (d *Deduper) IsDuplicate(ctx context.Context, taskID string) (bool, error) {
	key := keyPrefix + taskID

	result, err := d.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("dedup.IsDuplicate: %w", err)
	}

	return result > 0, nil
}

// MarkProcessed marks a task_id as processed by setting it in Redis with TTL.
// Uses SET NX to ensure atomic operation - returns error if key already exists.
func (d *Deduper) MarkProcessed(ctx context.Context, taskID string) error {
	key := keyPrefix + taskID

	// Use SET NX EX to atomically set only if not exists with expiration
	result, err := d.rdb.SetNX(ctx, key, "1", d.ttl).Result()
	if err != nil {
		return fmt.Errorf("dedup.MarkProcessed: %w", err)
	}

	if !result {
		return fmt.Errorf("dedup.MarkProcessed: task_id %q already processed", taskID)
	}

	return nil
}

// IsProcessedAndMark atomically checks if a task_id is already processed
// and marks it as processed if not. Returns (was_duplicate, error).
// This is a more efficient single-operation version for common use cases.
func (d *Deduper) IsProcessedAndMark(ctx context.Context, taskID string) (bool, error) {
	key := keyPrefix + taskID

	// Use SET NX EX to atomically set only if not exists with expiration
	result, err := d.rdb.SetNX(ctx, key, "1", d.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("dedup.IsProcessedAndMark: %w", err)
	}

	// If SetNX returned false, key already existed (duplicate)
	// If SetNX returned true, key was set successfully (not duplicate)
	return !result, nil
}

// Clear removes a task_id from the dedup set (for testing or error recovery).
func (d *Deduper) Clear(ctx context.Context, taskID string) error {
	key := keyPrefix + taskID

	if err := d.rdb.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("dedup.Clear: %w", err)
	}

	return nil
}

// Stats returns statistics about the dedup cache.
func (d *Deduper) Stats(ctx context.Context) (DedupStats, error) {
	pattern := keyPrefix + "*"

	keys, err := d.rdb.Keys(ctx, pattern).Result()
	if err != nil {
		return DedupStats{}, fmt.Errorf("dedup.Stats: %w", err)
	}

	return DedupStats{
		TrackedTaskIDs: int64(len(keys)),
		TTL:            d.ttl,
	}, nil
}

// DedupStats provides visibility into the dedup cache state.
type DedupStats struct {
	TrackedTaskIDs int64         `json:"tracked_task_ids"`
	TTL            time.Duration `json:"ttl"`
}