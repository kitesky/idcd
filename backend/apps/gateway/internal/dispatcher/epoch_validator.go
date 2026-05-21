// Package dispatcher — fencing-token (epoch) validation for probe.tasks
// stream messages.
//
// Background — why this exists:
//
// The scheduler tags every probe.tasks stream entry with an "epoch" field
// (a monotonically increasing fencing token claimed from Redis at scheduler
// startup; see backend/apps/scheduler/internal/leader/fencing.go). This
// dispatcher records the highest epoch it has ever observed and rejects any
// message whose epoch is strictly lower — i.e. a message from a "deposed"
// leader that doesn't yet know it lost the Redis lock.
//
// The high-water mark is persisted to Redis so that a gateway restart doesn't
// accidentally revive stale messages by resetting the in-memory mark to 0.
// On startup we read the persisted value; on every accepted message we
// atomically bump it via an EVAL "max(current, new)" script. Persistence
// key: idcd:gateway:epoch:max (TTL 7d — easily long enough to survive any
// realistic gateway outage, short enough that abandoned dev gateways
// eventually drop the key).
//
// Backward-compatibility window:
//
// Messages with no "epoch" field (or epoch==0) are accepted with a warning
// log + counter increment. This lets us deploy the gateway change before the
// scheduler change without breaking the pipeline. Once
// idcd_gateway_stale_epoch_total{reason="missing"} reaches zero across the
// fleet, the "missing → accept" branch can be tightened to "missing →
// reject". Target removal date: one minor release after this lands.
package dispatcher

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
)

// metricsStaleEpoch counts probe.tasks stream messages dropped because their
// epoch is lower than the gateway-wide high-water mark.
//
//	reason — "stale"   epoch < high-water (deposed leader)
//	         "missing" no epoch field on the message (backward-compat;
//	                   tracked separately so we can verify the field is
//	                   present on 100% of writes before tightening the
//	                   policy)
//	         "unparseable" epoch field present but not a base-10 int64
var metricsStaleEpoch = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "idcd_gateway_stale_epoch_total",
	Help: "Total probe.tasks messages dropped or flagged because their fencing-token epoch was stale, missing, or unparseable.",
}, []string{"reason"})

// epochHighWaterKey is the Redis key persisting the highest epoch seen by
// any gateway dispatcher. Survives gateway restarts; without it a fresh
// gateway with in-memory mark=0 would accept stale messages from a deposed
// scheduler. TTL is refreshed on every accepted message.
const epochHighWaterKey = "idcd:gateway:epoch:max"

// epochHighWaterTTL is how long the persisted high-water mark survives
// without updates. 7 days comfortably covers any realistic gateway outage
// (we'd have bigger problems before then); the TTL is mostly there so
// abandoned dev/test gateways don't pin the key forever.
const epochHighWaterTTL = 7 * 24 * time.Hour

// epochCASScript atomically updates the high-water mark to max(current, new)
// and refreshes the TTL. Returns the resulting high-water value. Using a Lua
// script keeps the read-compare-write atomic across multi-gateway races and
// avoids the lost-update window a naive GET/SET would have.
//
// KEYS[1] = high-water key
// ARGV[1] = candidate epoch (string-encoded int64)
// ARGV[2] = TTL in seconds
var epochCASScript = redis.NewScript(`
	local cur = tonumber(redis.call("GET", KEYS[1]) or "0")
	local cand = tonumber(ARGV[1])
	if cand > cur then
		redis.call("SET", KEYS[1], cand, "EX", ARGV[2])
		return cand
	end
	-- still refresh TTL so the key doesn't fall out under sustained writes
	-- of equal-or-lower epoch values
	redis.call("EXPIRE", KEYS[1], ARGV[2])
	return cur
`)

// epochValidator tracks the highest epoch observed across probe.tasks
// stream messages and decides whether a given incoming message should be
// accepted or dropped as stale.
//
// One epochValidator per Dispatcher. Safe for concurrent use — dispatchAndAck
// runs in a goroutine per message.
type epochValidator struct {
	rdb    redis.Cmdable
	logger *slog.Logger

	mu          sync.Mutex
	highWater   int64
	initialised bool
}

// newEpochValidator constructs a validator. The first Validate call after
// construction lazy-loads the persisted high-water mark from Redis; we
// don't do it eagerly in the constructor so test setups that never call
// Validate don't have to mock GET.
func newEpochValidator(rdb redis.Cmdable, logger *slog.Logger) *epochValidator {
	return &epochValidator{rdb: rdb, logger: logger}
}

// loadHighWater pulls the persisted high-water value from Redis. Called
// lazily on first Validate. Errors are logged but not fatal: a transient
// Redis blip during startup must not poison the in-memory state — we'll
// converge as soon as the first message comes in and the CAS script runs.
func (v *epochValidator) loadHighWater(ctx context.Context) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.initialised {
		return
	}
	v.initialised = true

	raw, err := v.rdb.Get(ctx, epochHighWaterKey).Result()
	if err == redis.Nil {
		v.logger.Info("epoch validator: no persisted high-water, starting from 0",
			"key", epochHighWaterKey)
		return
	}
	if err != nil {
		v.logger.Warn("epoch validator: failed to load persisted high-water",
			"key", epochHighWaterKey, "err", err)
		return
	}
	n, parseErr := strconv.ParseInt(raw, 10, 64)
	if parseErr != nil {
		v.logger.Warn("epoch validator: persisted high-water is not an int64, ignoring",
			"key", epochHighWaterKey, "value", raw, "err", parseErr)
		return
	}
	v.highWater = n
	v.logger.Info("epoch validator: restored high-water from Redis",
		"key", epochHighWaterKey, "epoch", n)
}

// epochDecision is the outcome of Validate.
type epochDecision int

const (
	// epochAccept means the message's epoch is >= our high-water mark and
	// the dispatcher should process it normally.
	epochAccept epochDecision = iota

	// epochAcceptMissing means the message has no epoch field (or epoch==0).
	// During the backward-compat window we still accept it but increment the
	// "missing" counter so we can track migration progress.
	epochAcceptMissing

	// epochDropStale means the message's epoch is strictly less than the
	// high-water mark we already observed — it came from a deposed leader.
	// The dispatcher must NOT forward this message to any agent; instead it
	// should XACK it to clear the entry from the consumer group's PEL and
	// move on (the same scheduler will re-issue the task on the next poll
	// tick under the current leader's epoch).
	epochDropStale
)

// validate inspects msgValues["epoch"] and returns a decision. observed is
// the parsed epoch (0 when missing/unparseable); useful for log lines and
// metrics.
//
// Side effects:
//   - On epochAccept, the Redis-persisted high-water mark is bumped to
//     max(current, observed) via an atomic Lua CAS, and the in-memory
//     highWater is updated to match.
//   - On epochAcceptMissing / epochDropStale, no Redis write happens (a
//     stale or epoch-less message must never advance the high-water).
//
// The Redis CAS write happens with a 5s timeout derived from the caller's
// ctx so a Redis blip can't wedge the entire dispatch loop; on timeout we
// still return epochAccept and log a warning — at-least-once delivery
// remains intact, we've only temporarily lost the persistence guarantee
// (a gateway crash within that window could let one stale message through,
// which is acceptable given the 5s budget).
func (v *epochValidator) validate(ctx context.Context, msgValues map[string]any) (epochDecision, int64) {
	v.loadHighWater(ctx)

	rawEpoch, ok := msgValues["epoch"]
	if !ok {
		return epochAcceptMissing, 0
	}
	epochStr, ok := rawEpoch.(string)
	if !ok || epochStr == "" {
		return epochAcceptMissing, 0
	}
	observed, err := strconv.ParseInt(epochStr, 10, 64)
	if err != nil {
		v.logger.Warn("epoch validator: unparseable epoch field, treating as missing",
			"raw", epochStr, "err", err)
		metricsStaleEpoch.WithLabelValues("unparseable").Inc()
		return epochAcceptMissing, 0
	}
	if observed <= 0 {
		// Treat <=0 the same as missing — pre-INCR sentinel, or a scheduler
		// that didn't claim a token. Accept under the backward-compat
		// window; metric will fire to surface the misconfig.
		return epochAcceptMissing, observed
	}

	v.mu.Lock()
	high := v.highWater
	if observed < high {
		v.mu.Unlock()
		return epochDropStale, observed
	}
	// Optimistic local bump; Redis CAS below makes it durable. The local
	// bump happens unconditionally for observed >= high so concurrent
	// validates with the same epoch don't all hammer Redis.
	if observed > v.highWater {
		v.highWater = observed
	}
	v.mu.Unlock()

	// Persist asynchronously-ish: a short timeout so Redis hiccups don't
	// stall the dispatcher hot path. The Lua script reconciles concurrent
	// writes from sibling dispatchers / gateway replicas.
	persistCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := epochCASScript.Run(persistCtx, v.rdb, []string{epochHighWaterKey},
		strconv.FormatInt(observed, 10),
		strconv.FormatInt(int64(epochHighWaterTTL/time.Second), 10),
	).Result(); err != nil {
		v.logger.Warn("epoch validator: failed to persist high-water, continuing in-memory",
			"key", epochHighWaterKey, "observed", observed, "err", err)
	}

	return epochAccept, observed
}
