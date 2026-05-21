// Package streamconsumer is a minimal Redis Streams XREADGROUP consumer
// helper used by the attest-generator binary (and any future attest-svc
// worker that consumes a single stream).
//
// It deliberately does NOT implement the full set of features that
// apps/aggregator/internal/consumer.Consumer exposes (XAUTOCLAIM reclaim,
// DLQ sweep, per-stream metrics). The attest-generator's retry safety is
// already provided by the attestation_record WAL (D4) — every externally
// effecting step in service.GenerateVerdict is idempotent, so a message
// that fails Handler is simply left un-ACKed and redelivered on the next
// XREADGROUP cycle.
//
// Lifecycle:
//
//  1. Run() ensures the consumer group exists (XGROUP CREATE MKSTREAM,
//     treating BUSYGROUP as success).
//  2. Loop: XREADGROUP with BLOCK=BlockTime, COUNT=Count, ">".
//  3. For each message, call Handler. On nil error → XACK; on non-nil →
//     log + leave un-ACKed (Redis will redeliver after the message goes
//     idle in the PEL on the next consumer reconnect, or to the same
//     consumer on the next XREADGROUP if no other consumer claims it).
//  4. Exit cleanly when ctx is Done.
package streamconsumer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Default tuning. Match the aggregator consumer's defaults (5s / 10) so
// operators can reason about timeouts uniformly.
const (
	DefaultBlockTime = 5 * time.Second
	DefaultCount     = int64(10)
)

// Handler processes one stream message. fields is the XREADGROUP
// "values" map (string keys, string-or-bytes values — go-redis decodes
// to map[string]interface{} where each value is typically a string).
//
// Return nil to ACK the message. Return a non-nil error to leave the
// message pending; Redis will redeliver it on the next XREADGROUP cycle
// after the PEL idle window elapses.
type Handler func(ctx context.Context, fields map[string]any) error

// Config bundles every input the consumer needs. Redis / Stream / Group
// / Consumer / Handler are required; the rest have safe defaults.
type Config struct {
	// Redis is the go-redis client to use. Must already be configured
	// (Addr, password, TLS, etc.) — Consumer does not own the client.
	Redis redis.UniversalClient

	// Stream is the Redis Streams key to consume from.
	Stream string

	// Group is the consumer-group name. The group is auto-created on
	// first Run if it does not exist (BUSYGROUP is treated as success).
	Group string

	// Consumer is the per-process consumer name. Should be unique
	// across replicas (typically hostname or pod name) so XPENDING
	// summaries split correctly.
	Consumer string

	// Handler runs once per message. See Handler doc for ack semantics.
	Handler Handler

	// BlockTime is the XREADGROUP BLOCK duration. Defaults to
	// DefaultBlockTime when zero. Use a positive value so the goroutine
	// can periodically observe ctx.Done(); a value of 0 would block
	// forever (until a message arrives) and delay clean shutdown.
	BlockTime time.Duration

	// Count is the max number of messages to fetch per XREADGROUP. Defaults
	// to DefaultCount when zero or negative.
	Count int64

	// Logger is optional. nil falls back to slog.Default().
	Logger *slog.Logger
}

// Consumer drives the XREADGROUP loop for one stream.
type Consumer struct {
	cfg Config
}

// New validates cfg and returns a Consumer. Required fields:
// Redis, Stream, Group, Consumer, Handler. Missing required fields
// trigger a panic — misconfigured wiring should fail at process start.
func New(cfg Config) *Consumer {
	if cfg.Redis == nil {
		panic("streamconsumer: Redis is required")
	}
	if strings.TrimSpace(cfg.Stream) == "" {
		panic("streamconsumer: Stream is required")
	}
	if strings.TrimSpace(cfg.Group) == "" {
		panic("streamconsumer: Group is required")
	}
	if strings.TrimSpace(cfg.Consumer) == "" {
		panic("streamconsumer: Consumer is required")
	}
	if cfg.Handler == nil {
		panic("streamconsumer: Handler is required")
	}
	if cfg.BlockTime <= 0 {
		cfg.BlockTime = DefaultBlockTime
	}
	if cfg.Count <= 0 {
		cfg.Count = DefaultCount
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Consumer{cfg: cfg}
}

// busygroupSubstr is what go-redis surfaces from the Redis BUSYGROUP
// reply. We treat it as success so re-deploys don't crash on a
// pre-existing group.
const busygroupSubstr = "BUSYGROUP"

// Run starts the consumer loop. It blocks until ctx is Done, returning
// nil once the loop exits cleanly. The only non-nil return is from the
// initial XGROUP CREATE MKSTREAM when that fails for a non-BUSYGROUP
// reason — a transient redis hiccup at that point is treated as fatal
// because we have not yet processed any work.
func (c *Consumer) Run(ctx context.Context) error {
	if err := c.ensureGroup(ctx); err != nil {
		return fmt.Errorf("streamconsumer: ensure group: %w", err)
	}

	c.cfg.Logger.Info("streamconsumer: started",
		slog.String("stream", c.cfg.Stream),
		slog.String("group", c.cfg.Group),
		slog.String("consumer", c.cfg.Consumer),
		slog.Duration("block", c.cfg.BlockTime),
		slog.Int64("count", c.cfg.Count),
	)

	for {
		if ctx.Err() != nil {
			c.cfg.Logger.Info("streamconsumer: stopping",
				slog.String("consumer", c.cfg.Consumer),
			)
			return nil
		}

		msgs, err := c.readGroup(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			// Transient Redis errors are logged and re-tried after a
			// short backoff. Logging at warn (not error) avoids paging
			// for momentary network blips.
			c.cfg.Logger.Warn("streamconsumer: XREADGROUP failed",
				slog.String("consumer", c.cfg.Consumer),
				slog.String("err", err.Error()),
			)
			if !sleepCtx(ctx, time.Second) {
				return nil
			}
			continue
		}

		for _, msg := range msgs {
			if ctx.Err() != nil {
				return nil
			}
			if err := c.cfg.Handler(ctx, msg.Values); err != nil {
				c.cfg.Logger.Error("streamconsumer: handler failed; leaving message unacked",
					slog.String("consumer", c.cfg.Consumer),
					slog.String("msg_id", msg.ID),
					slog.String("err", err.Error()),
				)
				continue
			}
			if ackErr := c.cfg.Redis.XAck(ctx, c.cfg.Stream, c.cfg.Group, msg.ID).Err(); ackErr != nil {
				// XACK failure is logged but not propagated — the
				// message will be redelivered and the handler is
				// expected to be idempotent (WAL D4).
				c.cfg.Logger.Warn("streamconsumer: XACK failed",
					slog.String("consumer", c.cfg.Consumer),
					slog.String("msg_id", msg.ID),
					slog.String("err", ackErr.Error()),
				)
			}
		}
	}
}

// ensureGroup creates the consumer group with MKSTREAM so the first
// deploy on an empty Redis works. BUSYGROUP (group already exists) is
// treated as success.
func (c *Consumer) ensureGroup(ctx context.Context) error {
	err := c.cfg.Redis.XGroupCreateMkStream(ctx, c.cfg.Stream, c.cfg.Group, "0").Err()
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), busygroupSubstr) {
		return nil
	}
	return err
}

// readGroup runs one XREADGROUP. Returns nil messages on BLOCK timeout
// (redis.Nil); other errors are propagated.
func (c *Consumer) readGroup(ctx context.Context) ([]redis.XMessage, error) {
	streams, err := c.cfg.Redis.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.cfg.Group,
		Consumer: c.cfg.Consumer,
		Streams:  []string{c.cfg.Stream, ">"},
		Count:    c.cfg.Count,
		Block:    c.cfg.BlockTime,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(streams) == 0 {
		return nil, nil
	}
	return streams[0].Messages, nil
}

// sleepCtx sleeps for d but wakes early when ctx is cancelled. Returns
// true if the full sleep elapsed, false if ctx was cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
