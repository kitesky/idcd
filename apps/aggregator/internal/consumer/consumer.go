// Package consumer provides a Redis Stream XREADGROUP consumer for probe results.
package consumer

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// Processor handles a single stream message.
type Processor interface {
	Process(ctx context.Context, msgID string, values map[string]any) error
}

// Consumer reads messages from a Redis Stream consumer group and delegates
// to Processor. Guarantees at-least-once delivery via XACK on success.
type Consumer struct {
	rdb           redis.Cmdable
	stream        string
	group         string
	consumerName  string
	batchSize     int64
	blockTimeout  time.Duration
	claimMinIdle  time.Duration // min idle time before reclaiming pending msgs
	processor     Processor
	logger        *slog.Logger
}

// Config holds consumer configuration.
type Config struct {
	Stream       string
	Group        string
	ConsumerName string
	BatchSize    int64
	BlockTimeout time.Duration
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
	return &Consumer{
		rdb:          rdb,
		stream:       cfg.Stream,
		group:        cfg.Group,
		consumerName: cfg.ConsumerName,
		batchSize:    cfg.BatchSize,
		blockTimeout: cfg.BlockTimeout,
		claimMinIdle: 30 * time.Second,
		processor:    proc,
		logger:       logger,
	}
}

// Run starts the consumer loop. Blocks until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context) error {
	if err := c.ensureGroup(ctx); err != nil {
		return fmt.Errorf("consumer: ensure group: %w", err)
	}

	c.logger.Info("consumer started", "stream", c.stream, "group", c.group, "consumer", c.consumerName)

	// Reclaim any pending messages from previous runs on startup.
	if err := c.reclaimPending(ctx); err != nil {
		c.logger.Warn("consumer: reclaim pending failed (non-fatal)", "err", err)
	}

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("consumer stopping")
			return nil
		default:
		}

		msgs, err := c.readGroup(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			c.logger.Warn("consumer: XREADGROUP failed", "err", err)
			time.Sleep(time.Second)
			continue
		}

		for _, msg := range msgs {
			if err := c.processor.Process(ctx, msg.ID, msg.Values); err != nil {
				c.logger.Error("consumer: process failed", "msg_id", msg.ID, "err", err)
				// Do not ACK — message stays in PEL for reclaim.
				continue
			}
			if err := c.ack(ctx, msg.ID); err != nil {
				c.logger.Warn("consumer: ACK failed", "msg_id", msg.ID, "err", err)
			}
		}
	}
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
// idle for longer than claimMinIdle (previous consumer crashed).
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
		c.logger.Info("reclaiming pending message", "msg_id", msg.ID)
		if err := c.processor.Process(ctx, msg.ID, msg.Values); err != nil {
			c.logger.Error("consumer: reclaim process failed", "msg_id", msg.ID, "err", err)
			continue
		}
		if err := c.ack(ctx, msg.ID); err != nil {
			c.logger.Warn("consumer: reclaim ACK failed", "msg_id", msg.ID, "err", err)
		}
	}
	return nil
}
