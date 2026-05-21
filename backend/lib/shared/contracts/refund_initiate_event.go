package contracts

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// RefundInitiateEvent is the standard payload of the `refund_initiate_queue`
// Redis stream (钱相关 / 合规相关 — Self-Verify 命中 bad PDF 触发的退款流).
//
// 生产者: apps/attest/cmd/verifier (redisRefundEnqueuer.EnqueueRefund)
// 消费者: apps/attest/cmd/refund-worker (refund.Handler.HandleInitiate)
//
// 业务关键度 (DECISIONS.md §M D5):
//   - 钱 — 拼错字段名 → refund-worker 收不到退款单 → 用户付了钱但 PDF 是
//     bad sig → 没退款 → 客诉 / 投诉合规
//   - 30min retry ladder 起点; D5 强制 30min 内必须给用户发道歉邮箱
//
// 字段对应关系 (与历史 raw XAdd 写入的兼容性):
//
//	旧 wire layout (P0-4 W3 之前):
//	  - report_id / reason / enqueued_at  顶层平铺
//
//	新 wire layout (P0-4 W3 后, 由 ToStreamValues 写出):
//	  - schema_ver / report_id / reason / enqueued_at  顶层平铺
//	  - reason 可空 (Self-Verify 不一定填); 空时 omit
//	  - enqueued_at 缺省时 ToStreamValues 兜底为 time.Now().UTC(), 行为
//	    与旧 redisRefundEnqueuer.EnqueueRefund 一致
//
// 没有灰度兼容期 — 该流流量极小 (S2 期, 每天个位数退款), producer + consumer
// 同时升级. ParseRefundInitiateEvent 仍然容忍 schema_ver 缺失的旧消息
// (SchemaVer=0), 这样 in-flight 旧消息不会被消费端直接拒绝.
type RefundInitiateEvent struct {
	SchemaVer int    `json:"schema_ver"           stream:"schema_ver"`
	ReportID  string `json:"report_id"            stream:"report_id"`
	Reason    string `json:"reason,omitempty"     stream:"reason,omitempty"`

	// EnqueuedAt 是生产者标注的入队时间. zero 时 ToStreamValues 兜底为
	// time.Now().UTC(). 历史 wire 用 RFC3339Nano, 我们沿用以保持精度
	// (Self-Verify 高峰可能同毫秒多次入队).
	EnqueuedAt time.Time `json:"enqueued_at"     stream:"enqueued_at"`
}

// RefundInitiateEventSchemaV1 is the current wire format version.
// Increment only on breaking field changes (rename / remove / retype),
// not for additive optional fields.
const RefundInitiateEventSchemaV1 = 1

// ToStreamValues converts the struct into a Redis XAdd values map.
//
// Required fields (report_id, enqueued_at) are always written. Reason
// is omitted when empty so the wire format stays compact and matches
// the producer's historical behaviour (only set when Self-Verify has
// a concrete reason string).
//
// SchemaVer auto-defaults to RefundInitiateEventSchemaV1 when zero.
// EnqueuedAt auto-defaults to time.Now().UTC() when zero.
func (e RefundInitiateEvent) ToStreamValues() map[string]any {
	schemaVer := e.SchemaVer
	if schemaVer == 0 {
		schemaVer = RefundInitiateEventSchemaV1
	}
	enqueued := e.EnqueuedAt
	if enqueued.IsZero() {
		enqueued = nowUTC()
	}

	vals := map[string]any{
		"schema_ver":  strconv.Itoa(schemaVer),
		"report_id":   e.ReportID,
		"enqueued_at": enqueued.UTC().Format(time.RFC3339Nano),
	}
	if e.Reason != "" {
		vals["reason"] = e.Reason
	}
	return vals
}

// ParseRefundInitiateEvent decodes a Redis stream values map into a
// RefundInitiateEvent.
//
// Strict requirements (return error):
//   - report_id must be non-empty
//   - schema_ver, if present and > RefundInitiateEventSchemaV1, returns
//     ErrUnknownSchemaVer
//
// Lenient (best-effort decode, no error):
//   - missing reason → empty string
//   - enqueued_at missing or malformed → zero time.Time
//   - schema_ver missing → SchemaVer=0 (legacy message tolerated)
func ParseRefundInitiateEvent(vals map[string]any) (RefundInitiateEvent, error) {
	e := RefundInitiateEvent{}

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
			return RefundInitiateEvent{}, fmt.Errorf("contracts.ParseRefundInitiateEvent: schema_ver=%q: %w", v, err)
		}
		e.SchemaVer = n
	default:
		return RefundInitiateEvent{}, fmt.Errorf("contracts.ParseRefundInitiateEvent: schema_ver has unexpected type %T", v)
	}
	if e.SchemaVer > RefundInitiateEventSchemaV1 {
		return RefundInitiateEvent{}, fmt.Errorf("%w: refund_initiate_event schema_ver=%d (known max=%d)",
			ErrUnknownSchemaVer, e.SchemaVer, RefundInitiateEventSchemaV1)
	}

	reportID, _ := vals["report_id"].(string)
	if reportID == "" {
		return RefundInitiateEvent{}, errors.New("contracts.ParseRefundInitiateEvent: report_id is required")
	}
	e.ReportID = reportID

	e.Reason, _ = vals["reason"].(string)

	if s, ok := vals["enqueued_at"].(string); ok && s != "" {
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			e.EnqueuedAt = t
		} else if t, err := time.Parse(time.RFC3339, s); err == nil {
			e.EnqueuedAt = t
		}
	}

	return e, nil
}
