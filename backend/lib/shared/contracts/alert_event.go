package contracts

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// AlertEvent describes one alert lifecycle transition emitted on the
// `alert.events` Redis Stream.
//
// 生产者: monitor evaluator (TBD — alert.events 流当前无 production producer,
// 本契约作为 P0-4 W4 的预防性准备, 未来 evaluator 接入时直接用 Typed API).
// 消费者: notifier alert fanout, SSE fanout (TBD).
//
// 字段对应关系 (与现有 stream.Client.AddAlertEvent 写入的 wire 完全一致, 仅新增 schema_ver):
//
//   - alert_event_id   全局唯一 ID (idgen.AlertEvent() 产出)
//   - monitor_id       触发告警的 monitor
//   - kind             告警类型 ("down" / "up" / "degraded" / "recovered" 等,
//                      具体枚举由后续接入方按 D 决策落细)
//   - ts               触发时间, int64 millisecond Unix timestamp
//   - schema_ver       (新增) wire format 版本号
//
// 没有 active producer 时, 契约本身也作为 "未来接入方该读什么字段" 的文档。
// 字段名拼写错编译期发现, 不会出现 W1 那种 "duration_ms" 拼成 "duraion_ms" 的静默丢数据。
type AlertEvent struct {
	SchemaVer    int       `json:"schema_ver"     stream:"schema_ver"`
	AlertEventID string    `json:"alert_event_id" stream:"alert_event_id"`
	MonitorID    string    `json:"monitor_id"     stream:"monitor_id"`
	Kind         string    `json:"kind"           stream:"kind"`

	// TsMs 是触发时间. zero (time.Time{}) 时 ToStreamValues 自动填 time.Now(),
	// 与旧 AddAlertEvent 行为一致. wire 上以 int64 millisecond 编码 (字段名 "ts").
	TsMs time.Time `json:"ts" stream:"ts"`
}

// AlertEventSchemaV1 is the current wire format version.
// Increment only on breaking field changes (rename / remove / retype),
// not for additive optional fields.
const AlertEventSchemaV1 = 1

// nowAlert is the timestamp source for TsMs fallback, overridable in tests.
var nowAlert = func() time.Time { return time.Now() }

// ToStreamValues converts the struct into a Redis XAdd values map.
//
// 所有字段都是必填 (alert_event_id / monitor_id / kind), 始终写入 stream.
// ts 字段在 wire 上是 int64 millisecond (与旧 AddAlertEvent 完全一致), 不是
// RFC3339 字符串 — 该流的消费端历史上就按数字 ms 读。
//
// SchemaVer 自动默认为 AlertEventSchemaV1.
// TsMs zero 时自动填 time.Now().
func (e AlertEvent) ToStreamValues() map[string]any {
	schemaVer := e.SchemaVer
	if schemaVer == 0 {
		schemaVer = AlertEventSchemaV1
	}
	ts := e.TsMs
	if ts.IsZero() {
		ts = nowAlert()
	}
	return map[string]any{
		"schema_ver":     strconv.Itoa(schemaVer),
		"alert_event_id": e.AlertEventID,
		"monitor_id":     e.MonitorID,
		"kind":           e.Kind,
		"ts":             strconv.FormatInt(ts.UnixMilli(), 10),
	}
}

// ParseAlertEvent decodes a Redis stream values map into an AlertEvent.
//
// Strict requirements (return error):
//   - alert_event_id, monitor_id, kind 全部必填非空
//   - schema_ver > AlertEventSchemaV1 → ErrUnknownSchemaVer
//   - ts 字段如存在但类型不可解析为 int64 → 错误
//
// Lenient:
//   - schema_ver 缺失 → 视为 legacy 消息 (SchemaVer=0), 不报错
//   - ts 缺失 → TsMs zero time.Time (调用方自行判断是否兜底)
func ParseAlertEvent(vals map[string]any) (AlertEvent, error) {
	e := AlertEvent{}

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
			return AlertEvent{}, fmt.Errorf("contracts.ParseAlertEvent: schema_ver=%q: %w", v, err)
		}
		e.SchemaVer = n
	default:
		return AlertEvent{}, fmt.Errorf("contracts.ParseAlertEvent: schema_ver has unexpected type %T", v)
	}
	if e.SchemaVer > AlertEventSchemaV1 {
		return AlertEvent{}, fmt.Errorf("%w: alert_event schema_ver=%d (known max=%d)",
			ErrUnknownSchemaVer, e.SchemaVer, AlertEventSchemaV1)
	}

	alertEventID, _ := vals["alert_event_id"].(string)
	if alertEventID == "" {
		return AlertEvent{}, errors.New("contracts.ParseAlertEvent: alert_event_id is required")
	}
	e.AlertEventID = alertEventID

	monitorID, _ := vals["monitor_id"].(string)
	if monitorID == "" {
		return AlertEvent{}, errors.New("contracts.ParseAlertEvent: monitor_id is required")
	}
	e.MonitorID = monitorID

	kind, _ := vals["kind"].(string)
	if kind == "" {
		return AlertEvent{}, errors.New("contracts.ParseAlertEvent: kind is required")
	}
	e.Kind = kind

	if v, ok := vals["ts"]; ok && v != nil {
		ms, err := parseInt64(v)
		if err != nil {
			return AlertEvent{}, fmt.Errorf("contracts.ParseAlertEvent: ts: %w", err)
		}
		e.TsMs = time.UnixMilli(ms).UTC()
	}

	return e, nil
}
