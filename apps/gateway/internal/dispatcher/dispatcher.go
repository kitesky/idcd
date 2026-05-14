// Package dispatcher reads probe tasks from the probe.tasks Redis Stream
// and forwards them to the appropriate connected agent via the hub.
//
// Flow:
//
//	Scheduler ──XADD──► probe.tasks
//	                          │
//	                    Dispatcher (XREADGROUP)
//	                          │
//	                    hub.Broadcast(node_id, task_msg)
//	                          │
//	                    Agent WebSocket
package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/gateway/internal/hub"
)

const (
	probeTasksStream = "probe.tasks"
	consumerGroup    = "gateway-dispatch"
	batchSize        = 20
	blockTimeout     = 2 * time.Second
	claimMinIdle     = 60 * time.Second // reclaim tasks stuck > 60s in PEL

	// redisBusyGroup is the error string Redis returns when the consumer group already exists.
	redisBusyGroup = "BUSYGROUP Consumer Group name already exists"
)

// taskMessage is the WebSocket payload sent to the agent.
type taskMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Dispatcher reads from the probe.tasks stream and forwards tasks to agents.
type Dispatcher struct {
	rdb          redis.Cmdable
	hub          *hub.Hub
	consumerName string
	logger       *slog.Logger

	// reclaimCursor tracks progressive XAutoClaim scanning so large PELs drain
	// across multiple 30s ticks rather than only reading the first batchSize entries.
	reclaimCursor string
}

// New creates a Dispatcher. consumerName should be unique per gateway instance
// (e.g. hostname). Call Run(ctx) to start.
func New(rdb redis.Cmdable, h *hub.Hub, logger *slog.Logger) *Dispatcher {
	name := consumerName()
	return &Dispatcher{
		rdb:          rdb,
		hub:          h,
		consumerName: name,
		logger:       logger,
	}
}

// Run starts the dispatch loop. Blocks until ctx is cancelled.
func (d *Dispatcher) Run(ctx context.Context) {
	if err := d.ensureGroup(ctx); err != nil {
		d.logger.Error("dispatcher: failed to create consumer group", "err", err)
		return
	}

	d.logger.Info("dispatcher started",
		"stream", probeTasksStream,
		"group", consumerGroup,
		"consumer", d.consumerName)

	// Reclaim any messages abandoned by a crashed previous instance.
	d.reclaimPending(ctx)

	// Periodic reclaim ticker (catches messages that re-enter PEL).
	reclaimTick := time.NewTicker(30 * time.Second)
	defer reclaimTick.Stop()

	for {
		// readGroup blocks up to blockTimeout; check for shutdown/reclaim after it returns.
		msgs, err := d.readGroup(ctx)
		if ctx.Err() != nil {
			d.logger.Info("dispatcher stopped")
			return
		}
		if err != nil {
			d.logger.Warn("dispatcher: XREADGROUP error", "err", err)
			time.Sleep(time.Second)
			continue
		}

		for _, msg := range msgs {
			if dispatched := d.dispatch(ctx, msg); dispatched {
				d.ack(ctx, msg.ID)
			}
			// If not dispatched (node offline), leave in PEL for reclaimPending.
		}

		// Non-blocking check: fire reclaim only when the 30s ticker has elapsed.
		select {
		case <-reclaimTick.C:
			d.reclaimPending(ctx)
		case <-ctx.Done():
			d.logger.Info("dispatcher stopped")
			return
		default:
		}
	}
}

// dispatch forwards a single stream message to the target agent.
// Returns true if successfully sent (should ACK), false if node offline (should not ACK).
func (d *Dispatcher) dispatch(_ context.Context, msg redis.XMessage) bool {
	nodeID, ok := msg.Values["node_id"].(string)
	if !ok || nodeID == "" {
		d.logger.Warn("dispatcher: message missing node_id, discarding", "msg_id", msg.ID)
		return true // ACK to clear garbage
	}

	// Build the task payload from stream fields
	payload := make(map[string]any, len(msg.Values))
	maps.Copy(payload, msg.Values)

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		d.logger.Error("dispatcher: marshal payload", "err", err, "msg_id", msg.ID)
		return true // ACK to avoid infinite retry on corrupt message
	}

	wsMsg := taskMessage{
		Type:    "task",
		Payload: json.RawMessage(payloadJSON),
	}
	wsMsgJSON, _ := json.Marshal(wsMsg)

	sent := d.hub.Broadcast(nodeID, wsMsgJSON)
	if sent {
		d.logger.Debug("dispatcher: task dispatched",
			"node_id", nodeID,
			"task_id", msg.Values["task_id"],
			"msg_id", msg.ID)
		return true
	}

	// Node not connected — leave in PEL, reclaimPending will retry.
	d.logger.Info("dispatcher: node offline, task held in PEL",
		"node_id", nodeID,
		"task_id", msg.Values["task_id"],
		"msg_id", msg.ID)
	return false
}

// ensureGroup creates the consumer group (MKSTREAM creates the stream if absent).
func (d *Dispatcher) ensureGroup(ctx context.Context) error {
	err := d.rdb.XGroupCreateMkStream(ctx, probeTasksStream, consumerGroup, "0").Err()
	if err != nil && err.Error() != redisBusyGroup {
		return fmt.Errorf("XGroupCreateMkStream: %w", err)
	}
	return nil
}

// readGroup reads up to batchSize new messages from the consumer group.
func (d *Dispatcher) readGroup(ctx context.Context) ([]redis.XMessage, error) {
	streams, err := d.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    consumerGroup,
		Consumer: d.consumerName,
		Streams:  []string{probeTasksStream, ">"},
		Count:    batchSize,
		Block:    blockTimeout,
	}).Result()
	if err == redis.Nil {
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

// ack acknowledges successful processing of a message.
func (d *Dispatcher) ack(ctx context.Context, msgID string) {
	if err := d.rdb.XAck(ctx, probeTasksStream, consumerGroup, msgID).Err(); err != nil {
		d.logger.Warn("dispatcher: XAck failed", "msg_id", msgID, "err", err)
	}
}

// reclaimPending uses XAUTOCLAIM to retry messages that have been idle > claimMinIdle.
// It advances a cursor across ticks so large PELs drain progressively rather than
// stalling at the first batchSize entries on every tick.
func (d *Dispatcher) reclaimPending(ctx context.Context) {
	start := d.reclaimCursor
	if start == "" {
		start = "0-0"
	}

	msgs, next, err := d.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   probeTasksStream,
		Group:    consumerGroup,
		Consumer: d.consumerName,
		MinIdle:  claimMinIdle,
		Start:    start,
		Count:    batchSize,
	}).Result()
	if err != nil {
		d.logger.Debug("dispatcher: XAutoClaim error (non-fatal)", "err", err)
		return
	}

	// Advance cursor; reset to beginning when the scan completes ("0-0" means done).
	if next == "0-0" {
		d.reclaimCursor = ""
	} else {
		d.reclaimCursor = next
	}

	for _, msg := range msgs {
		d.logger.Info("dispatcher: reclaiming pending task", "msg_id", msg.ID, "node_id", msg.Values["node_id"])
		if dispatched := d.dispatch(ctx, msg); dispatched {
			d.ack(ctx, msg.ID)
		}
	}
}

func consumerName() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return "gateway-" + h
	}
	return "gateway-default"
}
