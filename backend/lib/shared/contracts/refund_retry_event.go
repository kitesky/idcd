package contracts

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// RefundRetryEvent is the standard payload of the `refund_retry_queue`
// Redis stream (钱相关 — PaymentHub webhook 处理失败后进入 D5 retry ladder).
//
// 生产者: apps/attest/internal/handler/paymenthub.Handler.enqueueRetry
//
//	(PaymentHub webhook 进来后 lookup/UpdateStatus 失败时)
//
// 消费者: apps/attest/cmd/refund-worker (refund.Handler.HandleRetryEnqueue)
//
//	把 stream entry 翻译为 delay-zone ZSET 成员, 由 30s tick 驱动 5min/30min
//	ladder + apology email (DECISIONS.md §M D5).
//
// 业务关键度:
//   - 钱 — 拼错字段名 → tick worker 收到 bad entry → 静默丢退款重试 → 用户
//     付了钱但 PaymentHub 5xx → 没收到道歉邮箱 → 客诉 / 投诉
//   - retry ladder 第一档 (attempt=1, 5min delay)
//
// 字段对应关系 (与历史 raw XAdd 写入的兼容性):
//
//	旧 wire layout (P0-4 W3 之前):
//	  - order_id / ext_event_id / attempt / scheduled_at  顶层平铺
//	  - attempt 用字符串 "1" (历史 producer 直接传 string)
//	  - scheduled_at 用 RFC3339Nano 字符串
//
//	新 wire layout (P0-4 W3 后, 由 ToStreamValues 写出):
//	  - schema_ver / order_id / ext_event_id / attempt / scheduled_at  顶层平铺
//	  - attempt 仍然 wire 成字符串 (strconv.Itoa), 保持向后兼容
//	  - scheduled_at 仍然 RFC3339Nano
//
// 没有灰度兼容期 — 该流流量极小, producer + consumer 同时升级.
// ParseRefundRetryEvent 容忍 schema_ver 缺失 (SchemaVer=0) 以应对 in-flight
// 旧消息.
type RefundRetryEvent struct {
	SchemaVer int `json:"schema_ver"   stream:"schema_ver"`

	// OrderID 是 verdict_order ID (string, 不假设 int 形态). 历史 producer
	// 直接传 fakeLookup 返回的 orderID 字符串.
	OrderID string `json:"order_id"     stream:"order_id"`

	// ExtEventID 是 PaymentHub 端的事件 id, 仅用作日志关联, 退款 idempotency
	// 由 OrderID + Attempt 决定.
	ExtEventID string `json:"ext_event_id" stream:"ext_event_id"`

	// Attempt 是当前重试次数, 1-based. enqueueRetry 写第一次入队时永远填
	// 1; 后续 5min/30min ladder 由 tick worker 自己管理, 不再 XAdd 进流.
	Attempt int `json:"attempt"      stream:"attempt"`

	// ScheduledAt 是预期下次重试的 UTC 时间. 历史 wire 用 RFC3339Nano.
	// zero 视为非法 — Parse 会返回错误 (生产者必须传).
	ScheduledAt time.Time `json:"scheduled_at" stream:"scheduled_at"`
}

// RefundRetryEventSchemaV1 is the current wire format version.
// Increment only on breaking field changes (rename / remove / retype),
// not for additive optional fields.
const RefundRetryEventSchemaV1 = 1

// ToStreamValues converts the struct into a Redis XAdd values map.
//
// All four business fields are required and always written. SchemaVer
// auto-defaults to RefundRetryEventSchemaV1 when zero. ScheduledAt is
// NOT auto-defaulted (zero would be an obvious bug); the caller must
// pass an explicit time.
//
// attempt is encoded as a decimal string to preserve the historical
// wire format (raw XAdd put it as "1"); switching to a native int here
// would silently break the consumer-side string parse.
func (e RefundRetryEvent) ToStreamValues() map[string]any {
	schemaVer := e.SchemaVer
	if schemaVer == 0 {
		schemaVer = RefundRetryEventSchemaV1
	}
	return map[string]any{
		"schema_ver":   strconv.Itoa(schemaVer),
		"order_id":     e.OrderID,
		"ext_event_id": e.ExtEventID,
		"attempt":      strconv.Itoa(e.Attempt),
		"scheduled_at": e.ScheduledAt.UTC().Format(time.RFC3339Nano),
	}
}

// ParseRefundRetryEvent decodes a Redis stream values map into a
// RefundRetryEvent.
//
// Strict requirements (return error):
//   - order_id must be non-empty
//   - attempt must be parseable as int >= 1
//   - scheduled_at must be present and a valid RFC3339Nano (or RFC3339) string
//   - schema_ver, if present and > RefundRetryEventSchemaV1, returns
//     ErrUnknownSchemaVer
//
// Lenient:
//   - missing ext_event_id → empty string (just a log handle, not required)
//   - schema_ver missing → SchemaVer=0 (legacy message tolerated)
func ParseRefundRetryEvent(vals map[string]any) (RefundRetryEvent, error) {
	e := RefundRetryEvent{}

	switch v := vals["schema_ver"].(type) {
	case nil:
		e.SchemaVer = 0
	case int:
		e.SchemaVer = v
	case int64:
		e.SchemaVer = int(v)
	case float64:
		e.SchemaVer = int(v)
	case string:
		n, err := strconv.Atoi(v)
		if err != nil {
			return RefundRetryEvent{}, fmt.Errorf("contracts.ParseRefundRetryEvent: schema_ver=%q: %w", v, err)
		}
		e.SchemaVer = n
	default:
		return RefundRetryEvent{}, fmt.Errorf("contracts.ParseRefundRetryEvent: schema_ver has unexpected type %T", v)
	}
	if e.SchemaVer > RefundRetryEventSchemaV1 {
		return RefundRetryEvent{}, fmt.Errorf("%w: refund_retry_event schema_ver=%d (known max=%d)",
			ErrUnknownSchemaVer, e.SchemaVer, RefundRetryEventSchemaV1)
	}

	orderID, _ := vals["order_id"].(string)
	if orderID == "" {
		return RefundRetryEvent{}, errors.New("contracts.ParseRefundRetryEvent: order_id is required")
	}
	e.OrderID = orderID

	e.ExtEventID, _ = vals["ext_event_id"].(string)

	if v, ok := vals["attempt"]; ok {
		n, err := parseInt64(v)
		if err != nil {
			return RefundRetryEvent{}, fmt.Errorf("contracts.ParseRefundRetryEvent: attempt: %w", err)
		}
		if n < 1 {
			return RefundRetryEvent{}, fmt.Errorf("contracts.ParseRefundRetryEvent: attempt must be >= 1, got %d", n)
		}
		e.Attempt = int(n)
	} else {
		return RefundRetryEvent{}, errors.New("contracts.ParseRefundRetryEvent: attempt is required")
	}

	s, ok := vals["scheduled_at"].(string)
	if !ok || s == "" {
		return RefundRetryEvent{}, errors.New("contracts.ParseRefundRetryEvent: scheduled_at is required")
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		e.ScheduledAt = t
	} else if t, err := time.Parse(time.RFC3339, s); err == nil {
		e.ScheduledAt = t
	} else {
		return RefundRetryEvent{}, fmt.Errorf("contracts.ParseRefundRetryEvent: scheduled_at=%q is not RFC3339(Nano)", s)
	}

	return e, nil
}
