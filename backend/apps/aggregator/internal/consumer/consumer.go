// Package consumer provides a Redis Stream XREADGROUP consumer for probe results.
//
// Each Consumer instance owns a unique consumer name and runs an independent
// XREADGROUP loop. A separate maintenance goroutine periodically reclaims
// pending messages (XAUTOCLAIM) and forwards messages exceeding the redelivery
// threshold to a dead-letter stream ("<stream>:dlq").
package consumer

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/aggregator/internal/metrics"
)

// Default tunables for periodic reclaim + DLQ behaviour. Exported so tests can
// override them without reaching into unexported fields.
const (
	// DefaultClaimMinIdle is how long a message must sit idle in the PEL before
	// XAUTOCLAIM steals it for a different consumer.
	DefaultClaimMinIdle = 5 * time.Minute
	// DefaultReclaimInterval is how often the reclaim goroutine runs.
	DefaultReclaimInterval = 1 * time.Minute
	// DefaultDLQDeliveryThreshold caps how many times a single message may be
	// (re)delivered before it is shunted to the DLQ. delivery_count > threshold
	// → DLQ + ACK.
	DefaultDLQDeliveryThreshold = 5
	// DLQStreamSuffix is appended to the source stream name to form the DLQ
	// stream key, e.g. "probe.results:dlq".
	DLQStreamSuffix = ":dlq"
)

// Processor handles a single stream message.
type Processor interface {
	Process(ctx context.Context, msgID string, values map[string]any) error
}

// Consumer reads messages from a Redis Stream consumer group and delegates
// to Processor. Guarantees at-least-once delivery via XACK on success.
type Consumer struct {
	rdb                  redis.Cmdable
	stream               string
	dlqStream            string
	group                string
	consumerName         string
	batchSize            int64
	blockTimeout         time.Duration
	claimMinIdle         time.Duration
	dlqDeliveryThreshold int64
	processor            Processor
	logger               *slog.Logger
}

// Config holds consumer configuration.
type Config struct {
	Stream       string
	Group        string
	ConsumerName string
	BatchSize    int64
	BlockTimeout time.Duration
	// ClaimMinIdle overrides DefaultClaimMinIdle when non-zero.
	ClaimMinIdle time.Duration
	// DLQDeliveryThreshold overrides DefaultDLQDeliveryThreshold when > 0.
	DLQDeliveryThreshold int64
}

// New creates a Consumer. Call Run to start processing.
func New(rdb redis.Cmdable, cfg Config, proc Processor, logger *slog.Logger) *Consumer {
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 10
	}
	if cfg.BlockTimeout == 0 {
		cfg.BlockTimeout = 5 * time.Second
	}
	if cfg.ConsumerName == "" {
		cfg.ConsumerName = "aggregator-0"
	}
	if cfg.ClaimMinIdle == 0 {
		cfg.ClaimMinIdle = DefaultClaimMinIdle
	}
	if cfg.DLQDeliveryThreshold == 0 {
		cfg.DLQDeliveryThreshold = DefaultDLQDeliveryThreshold
	}
	return &Consumer{
		rdb:                  rdb,
		stream:               cfg.Stream,
		dlqStream:            cfg.Stream + DLQStreamSuffix,
		group:                cfg.Group,
		consumerName:         cfg.ConsumerName,
		batchSize:            cfg.BatchSize,
		blockTimeout:         cfg.BlockTimeout,
		claimMinIdle:         cfg.ClaimMinIdle,
		dlqDeliveryThreshold: cfg.DLQDeliveryThreshold,
		processor:            proc,
		logger:               logger,
	}
}

// Name returns the consumer name (unique per worker within a consumer group).
func (c *Consumer) Name() string { return c.consumerName }

// Stream returns the source stream name (without the :dlq suffix).
func (c *Consumer) Stream() string { return c.stream }

// Run starts the consumer loop. Blocks until ctx is cancelled.
//
// Run only owns the foreground XREADGROUP loop; periodic PEL maintenance
// (reclaim + DLQ) is driven by RunMaintenance, which should be started as a
// separate goroutine — typically only once per process across all consumers.
func (c *Consumer) Run(ctx context.Context) error {
	if err := c.ensureGroup(ctx); err != nil {
		return fmt.Errorf("consumer: ensure group: %w", err)
	}

	c.logger.Info("consumer started", "stream", c.stream, "group", c.group, "consumer", c.consumerName)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("consumer stopping", "consumer", c.consumerName)
			return nil
		default:
		}

		msgs, err := c.readGroup(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			c.logger.Warn("consumer: XREADGROUP failed", "consumer", c.consumerName, "err", err)
			time.Sleep(time.Second)
			continue
		}

		for _, msg := range msgs {
			if err := c.processor.Process(ctx, msg.ID, msg.Values); err != nil {
				c.logger.Error("consumer: process failed",
					"consumer", c.consumerName, "msg_id", msg.ID, "err", err)
				metrics.MessagesFailed.WithLabelValues(c.stream, c.consumerName).Inc()
				// Do not ACK — message stays in PEL for reclaim.
				continue
			}
			if err := c.ack(ctx, msg.ID); err != nil {
				c.logger.Warn("consumer: ACK failed",
					"consumer", c.consumerName, "msg_id", msg.ID, "err", err)
			}
			metrics.MessagesProcessed.WithLabelValues(c.stream, c.consumerName).Inc()
		}
	}
}

// RunMaintenance runs the periodic reclaim + DLQ + PEL-gauge sampling loop.
// Should be started as a goroutine. Blocks until ctx is cancelled.
//
// Only a single maintenance loop is needed per process — running multiple in
// parallel is wasteful but safe (XAUTOCLAIM is idempotent on the PEL).
func (c *Consumer) RunMaintenance(ctx context.Context, interval time.Duration) {
	if interval == 0 {
		interval = DefaultReclaimInterval
	}
	if err := c.ensureGroup(ctx); err != nil {
		c.logger.Warn("consumer: maintenance ensure group failed",
			"consumer", c.consumerName, "err", err)
	}
	// Run once at startup so a fresh deploy picks up messages abandoned by a
	// crashed predecessor without waiting `interval`.
	c.maintenanceTick(ctx)

	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.maintenanceTick(ctx)
		}
	}
}

// maintenanceTick performs one DLQ sweep + reclaim cycle + PEL gauge sample.
// Order matters: DLQ sweep runs first so poison messages don't get reclaimed
// only to fail again.
func (c *Consumer) maintenanceTick(ctx context.Context) {
	c.sweepDLQ(ctx)
	if err := c.reclaimPending(ctx); err != nil {
		c.logger.Warn("consumer: reclaim pending failed (non-fatal)",
			"consumer", c.consumerName, "err", err)
	}
	c.samplePELSize(ctx)
}

// ensureGroup creates the consumer group if it does not already exist.
func (c *Consumer) ensureGroup(ctx context.Context) error {
	err := c.rdb.XGroupCreateMkStream(ctx, c.stream, c.group, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil
}

// readGroup reads up to batchSize messages from the consumer group.
func (c *Consumer) readGroup(ctx context.Context) ([]redis.XMessage, error) {
	streams, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.group,
		Consumer: c.consumerName,
		Streams:  []string{c.stream, ">"},
		Count:    c.batchSize,
		Block:    c.blockTimeout,
	}).Result()
	if err == redis.Nil {
		return nil, nil // block timeout, no messages
	}
	if err != nil {
		return nil, err
	}
	if len(streams) == 0 {
		return nil, nil
	}
	return streams[0].Messages, nil
}

// ack acknowledges a message.
func (c *Consumer) ack(ctx context.Context, msgID string) error {
	return c.rdb.XAck(ctx, c.stream, c.group, msgID).Err()
}

// reclaimPending uses XAUTOCLAIM to take ownership of messages that have been
// idle for longer than claimMinIdle (previous consumer crashed). After claiming,
// the processor is invoked synchronously — successful messages are ACKed.
// Failures stay in the PEL until the next pass or until the DLQ sweep evicts
// them.
func (c *Consumer) reclaimPending(ctx context.Context) error {
	msgs, _, err := c.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   c.stream,
		Group:    c.group,
		Consumer: c.consumerName,
		MinIdle:  c.claimMinIdle,
		Start:    "0-0",
		Count:    c.batchSize,
	}).Result()
	if err != nil {
		return err
	}

	for _, msg := range msgs {
		metrics.ReclaimedMessages.WithLabelValues(c.stream, c.consumerName).Inc()
		c.logger.Info("reclaiming pending message",
			"consumer", c.consumerName, "msg_id", msg.ID)
		if err := c.processor.Process(ctx, msg.ID, msg.Values); err != nil {
			c.logger.Error("consumer: reclaim process failed",
				"consumer", c.consumerName, "msg_id", msg.ID, "err", err)
			metrics.MessagesFailed.WithLabelValues(c.stream, c.consumerName).Inc()
			continue
		}
		if err := c.ack(ctx, msg.ID); err != nil {
			c.logger.Warn("consumer: reclaim ACK failed",
				"consumer", c.consumerName, "msg_id", msg.ID, "err", err)
		}
		metrics.MessagesProcessed.WithLabelValues(c.stream, c.consumerName).Inc()
	}
	return nil
}

// sweepDLQ scans the PEL for messages whose delivery_count exceeds the
// configured threshold, copies them to "<stream>:dlq" and ACKs the original so
// the poison message stops circulating in the main consumer group.
//
// Uses XPENDING (group, start, end, count) which returns delivery counts.
func (c *Consumer) sweepDLQ(ctx context.Context) {
	infos, err := c.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: c.stream,
		Group:  c.group,
		Idle:   c.claimMinIdle,
		Start:  "-",
		End:    "+",
		Count:  c.batchSize,
	}).Result()
	if err != nil {
		c.logger.Debug("consumer: XPendingExt failed (non-fatal)",
			"consumer", c.consumerName, "err", err)
		return
	}

	for _, info := range infos {
		if info.RetryCount <= c.dlqDeliveryThreshold {
			continue
		}
		// Re-read the message body from the stream so we can copy fields into
		// the DLQ. XRANGE with the message ID as both start + end returns the
		// single entry (or empty if the entry has been trimmed away).
		entries, err := c.rdb.XRange(ctx, c.stream, info.ID, info.ID).Result()
		if err != nil || len(entries) == 0 {
			// Body gone (trimmed). Still ACK so the PEL slot frees.
			if ackErr := c.ack(ctx, info.ID); ackErr != nil {
				c.logger.Warn("consumer: DLQ ack-only failed",
					"consumer", c.consumerName, "msg_id", info.ID, "err", ackErr)
			}
			continue
		}
		values := entries[0].Values
		values["_dlq_reason"] = "max_deliveries_exceeded"
		values["_dlq_delivery_count"] = info.RetryCount
		values["_dlq_original_id"] = info.ID

		if err := c.rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: c.dlqStream,
			Values: values,
		}).Err(); err != nil {
			c.logger.Warn("consumer: XADD to DLQ failed",
				"consumer", c.consumerName, "msg_id", info.ID, "err", err)
			continue
		}
		if err := c.ack(ctx, info.ID); err != nil {
			c.logger.Warn("consumer: DLQ ack failed",
				"consumer", c.consumerName, "msg_id", info.ID, "err", err)
			continue
		}
		metrics.DLQMessages.WithLabelValues(c.stream).Inc()
		c.logger.Warn("consumer: message moved to DLQ",
			"consumer", c.consumerName,
			"msg_id", info.ID,
			"delivery_count", info.RetryCount,
			"dlq_stream", c.dlqStream)
	}
}

// samplePELSize updates the PELSize gauge for this consumer.
func (c *Consumer) samplePELSize(ctx context.Context) {
	pending, err := c.rdb.XPending(ctx, c.stream, c.group).Result()
	if err != nil {
		return
	}
	// Per-consumer count isn't returned by XPENDING summary directly; the
	// summary's Consumers map carries the breakdown.
	if n, ok := pending.Consumers[c.consumerName]; ok {
		metrics.PELSize.WithLabelValues(c.stream, c.consumerName).Set(float64(n))
		return
	}
	metrics.PELSize.WithLabelValues(c.stream, c.consumerName).Set(0)
}
