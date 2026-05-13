// Package leader implements Redis-based leader election for the scheduler.
// S1 uses Redis SETNX; S2 will migrate to etcd.
package leader

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Leader handles leader election using Redis SETNX.
// Only one scheduler instance can hold the leader lock at a time.
type Leader struct {
	rdb redis.Cmdable
	key string
	ttl time.Duration

	isLeader bool
	nodeID   string // unique identifier for this scheduler instance
}

// New creates a Leader instance.
// nodeID should be unique per scheduler instance (e.g., hostname+pid).
func New(rdb redis.Cmdable, key string, ttl time.Duration, nodeID string) *Leader {
	return &Leader{
		rdb:    rdb,
		key:    key,
		ttl:    ttl,
		nodeID: nodeID,
	}
}

// Acquire attempts to acquire the leader lock.
// Returns true if this instance became the leader, false otherwise.
func (l *Leader) Acquire(ctx context.Context) (bool, error) {
	// SETNX returns true if the key was set (we became leader)
	ok, err := l.rdb.SetNX(ctx, l.key, l.nodeID, l.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("leader.Acquire: %w", err)
	}
	l.isLeader = ok
	return ok, nil
}

// Renew extends the leader lock TTL.
// Should be called periodically (every TTL/2) by the current leader.
// Returns error if we're not the leader or renewal failed.
func (l *Leader) Renew(ctx context.Context) error {
	if !l.isLeader {
		return fmt.Errorf("leader.Renew: not the leader")
	}

	// Use Lua script to atomically check owner and extend TTL
	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("PEXPIRE", KEYS[1], ARGV[2])
		else
			return 0
		end
	`)

	ttlMs := l.ttl.Milliseconds()
	result, err := script.Run(ctx, l.rdb, []string{l.key}, l.nodeID, ttlMs).Int()
	if err != nil {
		return fmt.Errorf("leader.Renew: %w", err)
	}

	if result == 0 {
		// We lost leadership (someone else took the lock)
		l.isLeader = false
		return fmt.Errorf("leader.Renew: lost leadership")
	}

	return nil
}

// Release releases the leader lock.
// Only releases if this instance is the current owner.
func (l *Leader) Release(ctx context.Context) error {
	if !l.isLeader {
		return nil // not leader, nothing to release
	}

	// Use Lua script to atomically check owner and delete
	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`)

	result, err := script.Run(ctx, l.rdb, []string{l.key}, l.nodeID).Int()
	if err != nil {
		return fmt.Errorf("leader.Release: %w", err)
	}

	l.isLeader = false

	if result == 0 {
		return fmt.Errorf("leader.Release: not the owner")
	}

	return nil
}

// IsLeader reports whether this instance is currently the leader.
func (l *Leader) IsLeader() bool {
	return l.isLeader
}
