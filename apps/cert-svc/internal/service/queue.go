package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Queue field names on the Redis Stream entries.
const (
	queueFieldOrderID = "order_id"
)

// reclaimInterval / reclaimMinIdle drive XAUTOCLAIM in RunConsumer.
const (
	reclaimInterval = 1 * time.Minute
	reclaimMinIdle  = 5 * time.Minute
)

// EnqueueOrder publishes one order to the Redis Stream the worker reads.
// Producers (HTTP handler, retry path) call this after the order row is
// persisted; the worker picks it up via RunConsumer.
func (s *Service) EnqueueOrder(ctx context.Context, orderID int64) error {
	if s.cfg.Redis == nil {
		return fmt.Errorf("enqueue: redis client not configured")
	}
	_, err := s.cfg.Redis.XAdd(ctx, &redis.XAddArgs{
		Stream: s.cfg.Stream,
		Values: map[string]any{queueFieldOrderID: strconv.FormatInt(orderID, 10)},
	}).Result()
	if err != nil {
		return fmt.Errorf("xadd: %w", err)
	}
	return nil
}

// RunConsumer reads orders from the Redis Stream and drives each one.
// It blocks until ctx is cancelled. A maintenance ticker reclaims
// abandoned messages every minute.
func (s *Service) RunConsumer(ctx context.Context) error {
	if s.cfg.Redis == nil {
		return fmt.Errorf("run consumer: redis client not configured")
	}

	if err := s.ensureGroup(ctx); err != nil {
		return fmt.Errorf("ensure group: %w", err)
	}

	consumer := s.cfg.ConsumerName
	if consumer == "" {
		host, _ := os.Hostname()
		if host == "" {
			host = "cert-worker"
		}
		consumer = host
	}

	go s.runReclaimer(ctx, consumer)

	for {
		if ctx.Err() != nil {
			return nil
		}
		msgs, err := s.readGroup(ctx, consumer)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			s.cfg.Logger.Warn("xreadgroup error", "err", err)
			// Brief backoff so a sustained Redis outage does not spin.
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
			}
			continue
		}

		for _, m := range msgs {
			s.handleMessage(ctx, m)
		}
	}
}

// ensureGroup creates the stream + consumer group if missing.
func (s *Service) ensureGroup(ctx context.Context) error {
	err := s.cfg.Redis.XGroupCreateMkStream(ctx, s.cfg.Stream, s.cfg.Group, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil
}

// readGroup pulls a batch from the stream. err is redis.Nil when the
// block timeout expires with no messages.
func (s *Service) readGroup(ctx context.Context, consumer string) ([]redis.XMessage, error) {
	streams, err := s.cfg.Redis.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    s.cfg.Group,
		Consumer: consumer,
		Streams:  []string{s.cfg.Stream, ">"},
		Count:    10,
		Block:    s.cfg.BlockTimeout,
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

// handleMessage drives one order and ACKs the message regardless of
// outcome. RetryOrder is the explicit retry path; this consumer does not
// rely on Redis redelivery for business failures.
func (s *Service) handleMessage(ctx context.Context, m redis.XMessage) {
	orderID, err := parseOrderID(m.Values)
	if err != nil {
		s.cfg.Logger.Error("invalid stream message", "msg_id", m.ID, "err", err)
		s.ack(ctx, m.ID)
		return
	}
	if err := s.DriveOrder(ctx, orderID); err != nil {
		s.cfg.Logger.Error("drive order failed",
			"order_id", orderID, "msg_id", m.ID, "err", err)
	}
	s.ack(ctx, m.ID)
}

func (s *Service) ack(ctx context.Context, msgID string) {
	if err := s.cfg.Redis.XAck(ctx, s.cfg.Stream, s.cfg.Group, msgID).Err(); err != nil {
		s.cfg.Logger.Warn("xack failed", "msg_id", msgID, "err", err)
	}
}

// runReclaimer ticks every reclaimInterval until ctx cancels, taking
// over messages whose previous owner has been idle past reclaimMinIdle.
// Runs in its own goroutine so a blocking XReadGroup in the main loop
// never starves reclamation.
func (s *Service) runReclaimer(ctx context.Context, consumer string) {
	t := time.NewTicker(reclaimInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.reclaimAbandoned(ctx, consumer)
		}
	}
}

// reclaimAbandoned claims messages whose previous owner has been idle
// longer than reclaimMinIdle and re-processes them.
func (s *Service) reclaimAbandoned(ctx context.Context, consumer string) {
	msgs, _, err := s.cfg.Redis.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   s.cfg.Stream,
		Group:    s.cfg.Group,
		Consumer: consumer,
		MinIdle:  reclaimMinIdle,
		Start:    "0-0",
		Count:    50,
	}).Result()
	if err != nil {
		s.cfg.Logger.Debug("xautoclaim error", "err", err)
		return
	}
	for _, m := range msgs {
		s.cfg.Logger.Info("reclaiming pending order", "msg_id", m.ID)
		s.handleMessage(ctx, m)
	}
}

// parseOrderID reads order_id out of a stream message map, tolerating
// either int64-typed values (miniredis) or string values (real Redis).
func parseOrderID(values map[string]any) (int64, error) {
	v, ok := values[queueFieldOrderID]
	if !ok {
		return 0, fmt.Errorf("missing %q field", queueFieldOrderID)
	}
	switch x := v.(type) {
	case string:
		return strconv.ParseInt(x, 10, 64)
	case int64:
		return x, nil
	case int:
		return int64(x), nil
	default:
		return 0, fmt.Errorf("unexpected %q type %T", queueFieldOrderID, v)
	}
}
