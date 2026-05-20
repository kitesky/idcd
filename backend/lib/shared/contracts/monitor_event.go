package contracts

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// MonitorEvent is the standard payload of the `monitor.events` Redis stream.
//
// 生产者: aggregator monitor state machine
// 消费者: notifier, SSE fanout
//
// 字段对应关系 (与历史 AddMonitorEvent 写入大致一致, 但 extra 的处理方式变了):
//
//   - monitor_id, event, ts  由现有 AddMonitorEvent 自动注入
//   - severity, reason       常用扩展字段, 单独抽出来强类型化
//   - extra                  其它扩展字段聚合后 JSON-encode, 作为单 stream key
//
// 注意 (破坏性兼容变更):
//   - 旧 AddMonitorEvent(extra map[string]any) 把 extra 各 key 平铺到 stream values。
//   - MonitorEvent 把所有非常用字段塞进 ExtraJSON, 然后作为单 key "extra" 写入。
//   - 旧消费者读 "severity" / "reason" 等单 key 的代码必须改读 ParseMonitorEvent。
//   - 旧 API (AddMonitorEvent 不带 Typed 后缀) 保留 deprecation, 调用方渐进迁移。
type MonitorEvent struct {
	SchemaVer int    `json:"schema_ver"          stream:"schema_ver"`
	MonitorID string `json:"monitor_id"          stream:"monitor_id"`
	Event     string `json:"event"               stream:"event"`
	TsMs      int64  `json:"ts"                  stream:"ts"`
	Severity  string `json:"severity,omitempty"  stream:"severity,omitempty"`
	Reason    string `json:"reason,omitempty"    stream:"reason,omitempty"`
	ExtraJSON []byte `json:"extra,omitempty"     stream:"extra,omitempty"`
}

// MonitorEventSchemaV1 is the current MonitorEvent wire format version.
const MonitorEventSchemaV1 = 1

// nowMs is the timestamp source, overridable in tests.
var nowMs = func() int64 { return time.Now().UnixMilli() }

// ToStreamValues converts the struct into a Redis XAdd values map.
//
// TsMs defaults to time.Now().UnixMilli() when zero, matching the
// historical behavior of stream.Client.AddMonitorEvent.
func (e MonitorEvent) ToStreamValues() map[string]any {
	schemaVer := e.SchemaVer
	if schemaVer == 0 {
		schemaVer = MonitorEventSchemaV1
	}
	ts := e.TsMs
	if ts == 0 {
		ts = nowMs()
	}
	vals := map[string]any{
		"schema_ver": strconv.Itoa(schemaVer),
		"monitor_id": e.MonitorID,
		"event":      e.Event,
		"ts":         strconv.FormatInt(ts, 10),
	}
	if e.Severity != "" {
		vals["severity"] = e.Severity
	}
	if e.Reason != "" {
		vals["reason"] = e.Reason
	}
	if len(e.ExtraJSON) > 0 {
		vals["extra"] = string(e.ExtraJSON)
	}
	return vals
}

// ParseMonitorEvent decodes a Redis stream values map into a MonitorEvent.
//
// Strict requirements:
//   - monitor_id and event must be non-empty
//   - schema_ver > MonitorEventSchemaV1 → ErrUnknownSchemaVer
//
// Lenient:
//   - ts missing → 0 (caller decides whether to fill in)
//   - extra missing → nil ExtraJSON
//   - extra type validation: must be string or []byte
func ParseMonitorEvent(vals map[string]any) (MonitorEvent, error) {
	e := MonitorEvent{}

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
			return MonitorEvent{}, fmt.Errorf("contracts.ParseMonitorEvent: schema_ver=%q: %w", v, err)
		}
		e.SchemaVer = n
	default:
		return MonitorEvent{}, fmt.Errorf("contracts.ParseMonitorEvent: schema_ver has unexpected type %T", v)
	}
	if e.SchemaVer > MonitorEventSchemaV1 {
		return MonitorEvent{}, fmt.Errorf("%w: monitor_event schema_ver=%d (known max=%d)",
			ErrUnknownSchemaVer, e.SchemaVer, MonitorEventSchemaV1)
	}

	monitorID, _ := vals["monitor_id"].(string)
	if monitorID == "" {
		return MonitorEvent{}, errors.New("contracts.ParseMonitorEvent: monitor_id is required")
	}
	e.MonitorID = monitorID

	event, _ := vals["event"].(string)
	if event == "" {
		return MonitorEvent{}, errors.New("contracts.ParseMonitorEvent: event is required")
	}
	e.Event = event

	e.Severity, _ = vals["severity"].(string)
	e.Reason, _ = vals["reason"].(string)

	if v, ok := vals["ts"]; ok {
		ts, err := parseInt64(v)
		if err != nil {
			return MonitorEvent{}, fmt.Errorf("contracts.ParseMonitorEvent: ts: %w", err)
		}
		e.TsMs = ts
	}

	if v, ok := vals["extra"]; ok && v != nil {
		switch x := v.(type) {
		case string:
			if x != "" {
				// Validate JSON shape so downstream consumers can rely on it.
				if !json.Valid([]byte(x)) {
					return MonitorEvent{}, errors.New("contracts.ParseMonitorEvent: extra is not valid JSON")
				}
				e.ExtraJSON = []byte(x)
			}
		case []byte:
			if len(x) > 0 {
				if !json.Valid(x) {
					return MonitorEvent{}, errors.New("contracts.ParseMonitorEvent: extra is not valid JSON")
				}
				e.ExtraJSON = x
			}
		default:
			return MonitorEvent{}, fmt.Errorf("contracts.ParseMonitorEvent: extra has unexpected type %T", v)
		}
	}

	return e, nil
}
