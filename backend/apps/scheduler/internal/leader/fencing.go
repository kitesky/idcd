// Package leader — fencing token (epoch) support for scheduler leader election.
//
// Background — why we need a fencing token on top of Redis SETNX:
//
// SETNX alone is vulnerable to the classic distributed lock split-brain
// scenario laid out in Martin Kleppmann's "How to do distributed locking":
//
//  1. scheduler-A acquires the lock with TTL=30s.
//  2. scheduler-A's process suffers a 40s GC stall (or a network partition).
//  3. The Redis TTL expires; scheduler-B acquires the lock and starts
//     dispatching probe tasks.
//  4. scheduler-A's GC ends. From its own perspective nothing has changed —
//     it still believes it is the leader. It happily continues dispatching
//     tasks to the probe.tasks stream.
//  5. The same monitor is now dispatched twice (once by A, once by B), and
//     agents receive duplicate tasks.
//
// Even our Lua-scripted Renew() doesn't help here — A only notices it lost
// leadership at the next renewal tick, which can be seconds after it has
// already written stale tasks to the stream.
//
// The fix is a fencing token: a monotonically increasing counter that each
// new leader claims atomically from Redis (INCR scheduler:epoch) before doing
// any work. Each stream write tags the message with the epoch. The consumer
// (gateway dispatcher) tracks the highest epoch it has ever seen and drops
// any message whose epoch is strictly lower — i.e. a message from a
// "deposed" leader that doesn't yet know it has been deposed.
//
// This makes the split-brain window safe at the consumer side regardless of
// whether the producer side ever realises it has lost leadership.
//
// Persistence note — Redis must be configured with AOF (or RDB + frequent
// snapshots) so an unplanned restart doesn't reset the epoch counter to 0,
// which would resurrect stale messages. The scheduler's deployment runbook
// captures this; this comment is the canonical in-tree pointer to that
// requirement.
package leader

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// DefaultEpochKey is the Redis key used to allocate monotonically increasing
// epoch numbers for scheduler leaders. Lives next to the leader lock
// ("scheduler:leader") with a parallel naming convention.
const DefaultEpochKey = "scheduler:epoch"

// FencingToken is a monotonically increasing leader epoch number.
//
// Every scheduler process must claim an epoch at startup (via AcquireEpoch)
// and tag every probe.tasks stream message with it. Consumers reject any
// message whose epoch is lower than the highest epoch they have already
// processed, defeating the split-brain scenario described in the package doc.
//
// FencingToken intentionally fits comfortably in an int64 — the int64 space
// gives us ~9e18 distinct epochs, enough to issue one per scheduler restart
// every second for ~292 billion years.
type FencingToken int64

// String renders the token as a base-10 decimal string. Useful for stream
// payload encoding where everything is text.
func (t FencingToken) String() string {
	return fmt.Sprintf("%d", int64(t))
}

// Int64 returns the underlying int64 representation.
func (t FencingToken) Int64() int64 {
	return int64(t)
}

// AcquireEpoch claims the next monotonically increasing epoch number by
// running Redis INCR against key. The returned FencingToken is unique across
// all scheduler instances connected to this Redis (INCR is atomic).
//
// MUST be called once per scheduler process, BEFORE Run/Acquire, and the
// returned token must be plumbed into every stream write the scheduler
// performs. Failure to acquire is fatal: a scheduler that doesn't know its
// own epoch can't tag its writes and would silently re-enable the
// split-brain it's supposed to prevent.
//
// Concurrency: INCR is atomic in Redis, so multiple scheduler replicas
// starting up simultaneously are guaranteed to receive distinct tokens
// without any external coordination.
func AcquireEpoch(ctx context.Context, rdb redis.Cmdable, key string) (FencingToken, error) {
	if key == "" {
		key = DefaultEpochKey
	}
	n, err := rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("leader.AcquireEpoch: INCR %q: %w", key, err)
	}
	return FencingToken(n), nil
}
