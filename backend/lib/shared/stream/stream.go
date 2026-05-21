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

	// RefundInitiateQueue — Self-Verify 命中 bad PDF 时入队的退款流。
	// 生产者: apps/attest/cmd/verifier (redisRefundEnqueuer);
	// 消费者: apps/attest/cmd/refund-worker (refund.Handler.HandleInitiate).
	// 命名沿用 attest 历史 "_queue" 后缀 (与 refund_retry_queue 一致)。
	RefundInitiateQueue = "refund_initiate_queue"

	// RefundRetryQueue — PaymentHub webhook 处理失败后进入的 retry 流。
	// 生产者: apps/attest/internal/handler/paymenthub.Handler.enqueueRetry;
	// 消费者: apps/attest/cmd/refund-worker (refund.Handler.HandleRetryEnqueue).
	// D5 5min/30min retry ladder 第一档入口。
	RefundRetryQueue = "refund_retry_queue"
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

// AddCertNotificationTyped writes a cert notification event with a typed payload.
//
// 新代码必须使用此 API. P0-4 W2 — cert:notifications 是钱相关 / 合规相关流,
// 字段拼写错会导致用户收不到证书签发邮件、续签失败漏报 → 编译期发现 > 运行时
// 静默. SchemaVer 自动注入为 contracts.CertNotificationEventSchemaV1。
//
// 与历史 cert-svc.NotificationWatcher 直接 rdb.XAdd 的 wire 兼容性差异:
// 旧 layout 用 `payload` 顶层字段塞 JSON 二层 (内含 sans/ca/days/...), 新 layout
// 把所有字段平铺为 stream values 顶层 (sans 仍 JSON 单字段, 因为 stream value
// 必须是 scalar). 没有灰度兼容期 — 该流流量极小, producer 与 consumer 同时升级。
func (c *Client) AddCertNotificationTyped(ctx context.Context, e contracts.CertNotificationEvent) (string, error) {
	if e.SchemaVer == 0 {
		e.SchemaVer = contracts.CertNotificationEventSchemaV1
	}
	return c.Add(ctx, CertNotifications, e.ToStreamValues())
}

// AddRefundInitiateTyped writes a refund-initiate event with a typed payload.
//
// 新代码必须使用此 API. P0-4 W3 — refund_initiate_queue 是钱相关 / 合规相关流,
// 字段拼写错会导致 refund-worker 收不到退款单 → 用户付钱后没退款 → D5 道歉
// 邮箱也发不出去 → 客诉 → 投诉合规. SchemaVer 自动注入为
// contracts.RefundInitiateEventSchemaV1。
//
// 与历史 verifier-side redisRefundEnqueuer.EnqueueRefund 直接 rdb.XAdd 的
// wire 兼容性差异: 旧 layout 只写 report_id / reason / enqueued_at 三字段,
// 新 layout 加了 schema_ver 顶层。
func (c *Client) AddRefundInitiateTyped(ctx context.Context, e contracts.RefundInitiateEvent) (string, error) {
	if e.SchemaVer == 0 {
		e.SchemaVer = contracts.RefundInitiateEventSchemaV1
	}
	return c.Add(ctx, RefundInitiateQueue, e.ToStreamValues())
}

// AddRefundRetryTyped writes a refund-retry-enqueue event with a typed payload.
//
// 新代码必须使用此 API. P0-4 W3 — refund_retry_queue 是钱相关流, 字段拼写错
// 会导致 tick worker 收不到 retry → 用户付钱但 PaymentHub 5xx → 没收到道歉
// 邮箱. SchemaVer 自动注入为 contracts.RefundRetryEventSchemaV1。
//
// 与历史 paymenthub.Handler.enqueueRetry 直接 rdb.XAdd 的 wire 兼容性差异:
// 旧 layout 只写 order_id / ext_event_id / attempt / scheduled_at 四字段,
// 新 layout 加了 schema_ver 顶层; attempt wire 仍是 string (strconv.Itoa)
// 与旧 producer 字面量 "1" 一致, scheduled_at 仍是 RFC3339Nano。
func (c *Client) AddRefundRetryTyped(ctx context.Context, e contracts.RefundRetryEvent) (string, error) {
	if e.SchemaVer == 0 {
		e.SchemaVer = contracts.RefundRetryEventSchemaV1
	}
	return c.Add(ctx, RefundRetryQueue, e.ToStreamValues())
}

// AddProbeTaskTyped writes a probe task with a strongly-typed payload.
//
// 新代码必须使用此 API. P0-4 W5 — probe.tasks 是 scheduler→gateway 派发任务的
// 关键流, 且承载 P0-2 split-brain 防御所需的 "epoch" 字段. 字段拼错 →
// fencing token 校验静默失效 → 旧 leader 派的任务被当作合法任务转发给 agent.
// 编译期发现字段错误 > 运行时静默静默丢失安全语义.
// SchemaVer 自动注入为 contracts.ProbeTaskSchemaV1.
//
// Wire 兼容性: ToStreamValues 输出的 stream key 集合是历史 scheduler /
// api/probe.go 手写 map 的并集 (按 producer 角色取子集 — scheduler 写 epoch
// 但不写 target_normalized, api 反过来), 字段名一字未改. 加上 schema_ver
// 顶层后旧 gateway dispatcher 仍能直接 read msg.Values["node_id"] / ["epoch"]
// — 不需要 consumer 端代码变更.
func (c *Client) AddProbeTaskTyped(ctx context.Context, t contracts.ProbeTask) (string, error) {
	if t.SchemaVer == 0 {
		t.SchemaVer = contracts.ProbeTaskSchemaV1
	}
	return c.Add(ctx, ProbeTasks, t.ToStreamValues())
}

// AddAlertEvent writes an alert event.
//
// Deprecated: 使用 AddAlertEventTyped(ctx, contracts.AlertEvent{...}) 替代.
// 内部仍用 map[string]any 是 P0-4 W4 之前的实现; 新代码必须用 Typed 版本.
// 该 stream 当前无 production caller, 旧 API 保留以备未来流量上线后渐进迁移。
func (c *Client) AddAlertEvent(ctx context.Context, alertEventID, monitorID, kind string) (string, error) {
	return c.Add(ctx, Alert, map[string]any{
		"alert_event_id": alertEventID,
		"monitor_id":     monitorID,
		"kind":           kind,
		"ts":             time.Now().UnixMilli(),
	})
}

// AddAlertEventTyped writes an alert event with a strongly-typed payload.
//
// 新代码必须用此 API. P0-4 W4 — alert.events 流字段拼写错 → 用户该收到的
// 告警通知静默丢失. 编译期发现.
// SchemaVer 自动注入为 contracts.AlertEventSchemaV1.
// TsMs zero 时 ToStreamValues 自动填 time.Now() (与旧 AddAlertEvent 行为一致).
func (c *Client) AddAlertEventTyped(ctx context.Context, e contracts.AlertEvent) (string, error) {
	if e.SchemaVer == 0 {
		e.SchemaVer = contracts.AlertEventSchemaV1
	}
	return c.Add(ctx, Alert, e.ToStreamValues())
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
