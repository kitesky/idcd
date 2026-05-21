// Package stream provides a Redis Streams write helper for idcd event buses.
//
// D18 decision: all XADD calls use MAXLEN ~ 500000 to cap Redis memory.
// Consumers (Aggregator, Notifier, etc.) must handle at-least-once delivery;
// producers do not wait for consumer ack.
package stream

import (
	"context"
	"fmt"
	"maps"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/lib/shared/contracts"
	"github.com/kite365/idcd/lib/shared/telemetry"
)

// Default MAXLEN for all streams (approximate trim, efficient).
// Override per stream via Options.MaxLen.
const DefaultMaxLen = 500_000

// Well-known stream / pubsub names used across idcd services.
//
// 集中管理位置：跨服务的 Redis Stream / Key 名称 "魔法字符串" 一律在这里登记，
// 调用方通过 stream.<Name> 引用，避免散落多处后改名困难。新增条目请同时给一句
// 业务用途说明 + 生产者/消费者关系。
const (
	// ProbeResults — 节点上报的探测结果流。
	// 生产者: agent (via gateway hub)；消费者: aggregator。
	Probe = "probe.results"

	// MonitorEvents — 监控状态机变化流（up/down/degraded）。
	// 生产者: aggregator；消费者: notifier、SSE fanout。
	Monitor = "monitor.events"

	// AlertEvents — 告警触发事件流。
	// 生产者: monitor evaluator；消费者: notifier、SSE fanout。
	Alert = "alert.events"

	// AuditEvents — 审计日志事件流。
	// 生产者: API handlers (中间件)；消费者: auditlog writer。
	Audit = "audit.events"

	// UsageEvents — API 配额 / 计费用量事件流。
	// 生产者: API handlers；消费者: billing、quota enforcer。
	Usage = "usage.events"

	// ProbeTasks — 调度器分发的探测任务流。
	// 生产者: scheduler；消费者: gateway dispatcher → agent WebSocket。
	// 命名沿用历史 "." 分隔约定（与 probe.results 对应）。
	ProbeTasks = "probe.tasks"

	// CertNotifications — 证书相关通知事件流（issued/failed/expiring/renewal_failed/revoked）。
	// 生产者: cert-svc NotificationWatcher；消费者: notifier cert consumer。
	// 命名沿用 cert-svc 服务前缀（":" 分隔约定）。
	CertNotifications = "cert:notifications"
)

// Well-known Redis key names (非 stream，但同样需要跨服务一致命名).
const (
	// CertNotificationsCursor — NotificationWatcher 的 last-event-id 游标键，
	// 重启后从此恢复，避免重复发送通知。TTL: 无（持久）。
	CertNotificationsCursor = "cert:notifications:cursor"
)

// Client wraps a Redis client for Streams operations.
type Client struct {
	rdb    redis.Cmdable
	maxLen int64
}

// New creates a Client using the provided redis.Cmdable
// (accepts *redis.Client or *redis.ClusterClient).
func New(rdb redis.Cmdable) *Client {
	return &Client{rdb: rdb, maxLen: DefaultMaxLen}
}

// NewFromConfig creates a *redis.Client and wraps it in a Client.
func NewFromConfig(addr, password string, db int) (*Client, *redis.Client) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
	})
	return New(rdb), rdb
}

// Add appends a message to stream with MAXLEN ~ DefaultMaxLen (D18).
// Returns the message ID assigned by Redis.
//
// P1-5: 在写入 Redis 前把 OTel trace context (W3C traceparent/tracestate)
// inject 进 values, 让消费端 (aggregator / gateway dispatcher) Extract 出来后
// 子 span 自动挂回原 trace, 跨进程异步链路保持同一个 trace_id。
// 业务字段不需要改, 也不需要在 contracts.ProbeResult/MonitorEvent 里加 trace 字段
// (trace 是 stream-level 元数据, 不进 typed contract)。
// 没有 active span 时 Inject 是 no-op, 不破坏 vals。
func (c *Client) Add(ctx context.Context, stream string, values map[string]any) (string, error) {
	telemetry.InjectStream(ctx, values)
	id, err := c.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		MaxLen: c.maxLen,
		Approx: true, // ~ flag: efficient probabilistic trimming
		ID:     "*",  // auto-generate ID
		Values: values,
	}).Result()
	if err != nil {
		return "", fmt.Errorf("stream.Add %q: %w", stream, err)
	}
	return id, nil
}

// AddProbeResult writes a probe result to the probe.results stream.
//
// Deprecated: 使用 AddProbeResultTyped(ctx, contracts.ProbeResult{...}) 替代。
// 跨 stream 边界禁止用 map[string]any 自由发挥字段名 (P0-4 决策, 见
// docs/prd/ARCHITECTURE-REVIEW-2026-05-21.md). 旧实现保留以兼容存量调用方,
// 渐进迁移完成后将移除。
func (c *Client) AddProbeResult(ctx context.Context, taskID, nodeID string, payload map[string]any) (string, error) {
	vals := make(map[string]any, len(payload)+2)
	maps.Copy(vals, payload)
	vals["task_id"] = taskID
	vals["node_id"] = nodeID
	return c.Add(ctx, Probe, vals)
}

// AddMonitorEvent writes a monitor state-change event.
//
// Deprecated: 使用 AddMonitorEventTyped(ctx, contracts.MonitorEvent{...}) 替代。
// 注意 *Typed 版本将 extra 折叠成单 JSON 字段 (而非平铺), 是有意的契约破坏,
// 见 contracts/doc.go. 旧实现保留以兼容存量调用方。
func (c *Client) AddMonitorEvent(ctx context.Context, monitorID, event string, extra map[string]any) (string, error) {
	vals := make(map[string]any, len(extra)+3)
	maps.Copy(vals, extra)
	vals["monitor_id"] = monitorID
	vals["event"] = event
	vals["ts"] = time.Now().UnixMilli()
	return c.Add(ctx, Monitor, vals)
}

// AddProbeResultTyped writes a probe result with a strongly-typed payload.
//
// 新代码必须使用此 API. 拼错字段名会编译期失败, 不会运行时静默丢数据 (P0-4).
// SchemaVer 自动注入为 contracts.ProbeResultSchemaV1, 无需调用方设置。
func (c *Client) AddProbeResultTyped(ctx context.Context, r contracts.ProbeResult) (string, error) {
	if r.SchemaVer == 0 {
		r.SchemaVer = contracts.ProbeResultSchemaV1
	}
	return c.Add(ctx, Probe, r.ToStreamValues())
}

// AddMonitorEventTyped writes a monitor state-change event with a typed payload.
//
// 与旧 AddMonitorEvent 的兼容性差异: extra 字段被合并成单个 JSON-encoded
// stream key "extra", 消费端必须用 contracts.ParseMonitorEvent 反序列化。
// SchemaVer / TsMs 自动注入。
func (c *Client) AddMonitorEventTyped(ctx context.Context, e contracts.MonitorEvent) (string, error) {
	if e.SchemaVer == 0 {
		e.SchemaVer = contracts.MonitorEventSchemaV1
	}
	return c.Add(ctx, Monitor, e.ToStreamValues())
}

// AddAlertEvent writes an alert event.
func (c *Client) AddAlertEvent(ctx context.Context, alertEventID, monitorID, kind string) (string, error) {
	return c.Add(ctx, Alert, map[string]any{
		"alert_event_id": alertEventID,
		"monitor_id":     monitorID,
		"kind":           kind,
		"ts":             time.Now().UnixMilli(),
	})
}

// AddAuditEvent writes an audit entry to the audit.events stream.
func (c *Client) AddAuditEvent(ctx context.Context, vals map[string]any) (string, error) {
	return c.Add(ctx, Audit, vals)
}

// Len returns the current length of a stream (for monitoring / alerts).
func (c *Client) Len(ctx context.Context, stream string) (int64, error) {
	n, err := c.rdb.XLen(ctx, stream).Result()
	if err != nil {
		return 0, fmt.Errorf("stream.Len %q: %w", stream, err)
	}
	return n, nil
}

// Ping checks the Redis connection is alive.
func (c *Client) Ping(ctx context.Context) error {
	if _, err := c.rdb.Ping(ctx).Result(); err != nil {
		return fmt.Errorf("stream.Ping: %w", err)
	}
	return nil
}
