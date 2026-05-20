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
	"github.com/kite365/idcd/lib/shared/stream"
)

// Dispatcher tuning constants. 这些是 dispatcher 内部的批处理 / 超时参数，
// 与具体业务规则无关 (与 scheduler 的探测周期、agent 的心跳无关)，
// 出问题时只需在这里调整：
//
//   - batchSize: 单次 XREADGROUP 拉取的任务条数。
//     上调可提升吞吐但增大单实例突发负载；下调使任务派发更平滑。
//   - blockTimeout: 无任务时 XREADGROUP 的阻塞时长。短则空轮询多 CPU 高；
//     长则关闭/重载时 shutdown 延迟变大。
//   - claimMinIdle: PEL 中超过此时长仍未 ACK 的任务允许被其他 dispatcher
//     XAutoClaim 接管，应大于单次任务处理 P99 时延。
const (
	batchSize    = 20
	blockTimeout = 2 * time.Second
	claimMinIdle = 60 * time.Second

	// deliveryTimeout caps how long a worker waits for writePump to commit a
	// dispatched frame to the OS socket buffer before giving up and leaving
	// the stream message in PEL for reclaimPending to pick up. Should be a
	// few-second budget — long enough to ride out a backlog on a busy agent's
	// SendCh, short enough that a wedged agent doesn't pin one stream slot
	// indefinitely.
	deliveryTimeout = 5 * time.Second

	// redisBusyGroup is the error string Redis returns when the consumer group already exists.
	redisBusyGroup = "BUSYGROUP Consumer Group name already exists"
)

// probeTasksStream 是被 dispatcher 消费的探测任务流名，跨服务一致命名，
// 集中登记在 lib/shared/stream，不要在此就地写字符串字面量。
const probeTasksStream = stream.ProbeTasks

// taskMessage is the WebSocket payload sent to the agent.
type taskMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Dispatcher reads from the probe.tasks stream and forwards tasks to agents.
type Dispatcher struct {
	rdb          redis.Cmdable
	hub          *hub.Hub
	group        string
	consumerName string
	logger       *slog.Logger

	// reclaimCursor tracks progressive XAutoClaim scanning so large PELs drain
	// across multiple 30s ticks rather than only reading the first batchSize entries.
	reclaimCursor string
}

// New creates a Dispatcher. consumerName should be unique per gateway instance
// (e.g. hostname). Call Run(ctx) to start.
//
// The consumer-group name comes from the GATEWAY_DISPATCH_GROUP env var when
// set, otherwise defaults to "gateway-dispatch". Override is needed for dev
// setups that share a redis with a production gateway: using the same group
// means redis load-balances each task to exactly one consumer (dev or prod),
// so half the time a tool-page probe lands on the *other* gateway and stalls
// in PEL because that gateway doesn't have the target agent's ws. Setting a
// dev-only group gives each gateway its own copy of the stream and lets it
// only ACK the tasks whose node_id it can actually serve.
func New(rdb redis.Cmdable, h *hub.Hub, logger *slog.Logger) *Dispatcher {
	group := os.Getenv("GATEWAY_DISPATCH_GROUP")
	if group == "" {
		group = "gateway-dispatch"
	}
	return &Dispatcher{
		rdb:          rdb,
		hub:          h,
		group:        group,
		consumerName: consumerName(),
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
		"group", d.group,
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

		// Each message is dispatched in its own goroutine so a slow agent's
		// deliveryTimeout doesn't serialise the rest of the batch. Worst-case
		// throughput on a batch of 20 fully-offline nodes drops from
		// 20*deliveryTimeout to one deliveryTimeout — and the messages stay
		// in PEL either way for reclaimPending to retry.
		for _, msg := range msgs {
			go d.dispatchAndAck(ctx, msg)
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

// dispatchAndAck forwards a single stream message to the target agent and
// XACKs the stream entry only after writePump has confirmed the frame hit the
// OS socket. Three outcomes:
//
//  1. Garbage message (no node_id / unmarshal-able) — ACK to clear it.
//  2. Successful delivery (delivered chan closed) — ACK.
//  3. Node offline, SendCh full, connection died mid-write, or delivery
//     timeout — do NOT ACK; reclaimPending will pick it up on the next 30s
//     tick. This is the at-least-once safety net the bare hub.Broadcast
//     return value couldn't provide on its own (that only signalled "queued
//     in process buffer", not "wrote to socket").
func (d *Dispatcher) dispatchAndAck(ctx context.Context, msg redis.XMessage) {
	nodeID, ok := msg.Values["node_id"].(string)
	if !ok || nodeID == "" {
		d.logger.Warn("dispatcher: message missing node_id, discarding", "msg_id", msg.ID)
		d.ack(ctx, msg.ID)
		return
	}

	// Build the task payload from stream fields
	payload := make(map[string]any, len(msg.Values))
	maps.Copy(payload, msg.Values)

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		d.logger.Error("dispatcher: marshal payload", "err", err, "msg_id", msg.ID)
		d.ack(ctx, msg.ID) // corrupt — ACK to avoid infinite retry
		return
	}

	wsMsg := taskMessage{
		Type:    "task",
		Payload: json.RawMessage(payloadJSON),
	}
	wsMsgJSON, _ := json.Marshal(wsMsg)

	fut, sent := d.hub.BroadcastWithDelivery(nodeID, wsMsgJSON)
	if !sent {
		// Node not connected (or SendCh full) — leave in PEL for reclaimPending.
		d.logger.Info("dispatcher: node offline, task held in PEL",
			"node_id", nodeID,
			"task_id", msg.Values["task_id"],
			"msg_id", msg.ID)
		return
	}

	select {
	case <-fut.Delivered:
		d.logger.Debug("dispatcher: task delivered",
			"node_id", nodeID,
			"task_id", msg.Values["task_id"],
			"msg_id", msg.ID)
		d.ack(ctx, msg.ID)
	case <-fut.Closed:
		// Connection torn down before writePump could commit the frame —
		// leave the message un-ACKed so the next dispatcher reclaim sweeps it.
		d.logger.Info("dispatcher: connection closed before delivery, task held in PEL",
			"node_id", nodeID,
			"task_id", msg.Values["task_id"],
			"msg_id", msg.ID)
	case <-time.After(deliveryTimeout):
		d.logger.Warn("dispatcher: delivery timeout, task held in PEL",
			"node_id", nodeID,
			"task_id", msg.Values["task_id"],
			"msg_id", msg.ID,
			"timeout", deliveryTimeout)
	case <-ctx.Done():
		return
	}
}

// ensureGroup creates the consumer group (MKSTREAM creates the stream if absent).
// The default group ("gateway-dispatch") starts from "0" so a brand-new
// production deployment will replay any tasks that landed before it came up.
// Custom dev groups (GATEWAY_DISPATCH_GROUP override) start from "$" instead
// — they only fire on tasks queued *after* the dev gateway connects, which
// avoids ingesting the full historical backlog of node_ids that this dev
// gateway can't serve and would just churn PEL forever.
func (d *Dispatcher) ensureGroup(ctx context.Context) error {
	startID := "0"
	if d.group != "gateway-dispatch" {
		startID = "$"
	}
	err := d.rdb.XGroupCreateMkStream(ctx, probeTasksStream, d.group, startID).Err()
	if err != nil && err.Error() != redisBusyGroup {
		return fmt.Errorf("XGroupCreateMkStream: %w", err)
	}
	return nil
}

// readGroup reads up to batchSize new messages from the consumer group.
func (d *Dispatcher) readGroup(ctx context.Context) ([]redis.XMessage, error) {
	streams, err := d.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    d.group,
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
	if err := d.rdb.XAck(ctx, probeTasksStream, d.group, msgID).Err(); err != nil {
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
		Group:    d.group,
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
		go d.dispatchAndAck(ctx, msg)
	}
}

func consumerName() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return "gateway-" + h
	}
	return "gateway-default"
}
