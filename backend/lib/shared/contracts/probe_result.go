package contracts

import (
	"errors"
	"fmt"
	"strconv"
)

// ProbeResult is the standard payload of the `probe.results` Redis stream.
//
// 生产者: gateway WebSocket handler (apps/gateway/internal/handler/ws.go)
// 消费者: aggregator processor (apps/aggregator/internal/processor/processor.go)
//
// 字段对应关系 (与历史 map[string]any 写入完全一致, 不引入新 wire format):
//
//   - task_id, node_id   AddProbeResult 在 stream 层自动注入
//   - raw, summary       探测原始 / 概要 JSON, stream 中以字符串编码存储
//   - duration_ms        探测耗时 (毫秒)
//   - success            探测是否成功
//   - error              失败时的错误信息 (与 sql 列名一致, 不是 error_code)
//   - signature          agent 端水印 / 签名, 用于防伪验证
//   - monitor_id         可选, 仅当 task 由 monitor 触发时才存在
//
// schema_ver 是新增字段, 旧消息没有此字段时按 ProbeResultSchemaV1 兜底。
type ProbeResult struct {
	SchemaVer  int    `json:"schema_ver"           stream:"schema_ver"`
	TaskID     string `json:"task_id"              stream:"task_id"`
	NodeID     string `json:"node_id"              stream:"node_id"`
	RawJSON    string `json:"raw,omitempty"        stream:"raw,omitempty"`
	SummaryJSON string `json:"summary,omitempty"   stream:"summary,omitempty"`
	DurationMs int64  `json:"duration_ms"          stream:"duration_ms"`
	Success    bool   `json:"success"              stream:"success"`
	Error      string `json:"error,omitempty"      stream:"error,omitempty"`
	Signature  string `json:"signature,omitempty"  stream:"signature,omitempty"`
	MonitorID  string `json:"monitor_id,omitempty" stream:"monitor_id,omitempty"`
}

// ProbeResultSchemaV1 is the current ProbeResult wire format version.
// Increment only on breaking field changes (rename / remove / retype),
// not for additive optional fields.
const ProbeResultSchemaV1 = 1

// ErrUnknownSchemaVer is returned by Parse* when the message carries a
// schema_ver newer than this package knows about. Callers should decide
// whether to drop the message, route it to a DLQ, or alert ops.
var ErrUnknownSchemaVer = errors.New("contracts: unknown schema version")

// ToStreamValues converts the struct into a Redis XAdd values map.
//
// Required scalar fields are always written. Optional fields (RawJSON,
// SummaryJSON, Error, Signature, MonitorID) are omitted when zero-valued
// to keep the wire format identical to the historical hand-written map.
func (r ProbeResult) ToStreamValues() map[string]any {
	schemaVer := r.SchemaVer
	if schemaVer == 0 {
		schemaVer = ProbeResultSchemaV1
	}
	vals := map[string]any{
		"schema_ver":  strconv.Itoa(schemaVer),
		"task_id":     r.TaskID,
		"node_id":     r.NodeID,
		"duration_ms": strconv.FormatInt(r.DurationMs, 10),
		"success":     formatBool(r.Success),
	}
	if r.RawJSON != "" {
		vals["raw"] = r.RawJSON
	}
	if r.SummaryJSON != "" {
		vals["summary"] = r.SummaryJSON
	}
	if r.Error != "" {
		vals["error"] = r.Error
	}
	if r.Signature != "" {
		vals["signature"] = r.Signature
	}
	if r.MonitorID != "" {
		vals["monitor_id"] = r.MonitorID
	}
	return vals
}

// ParseProbeResult decodes a Redis stream values map (as returned by
// XReadGroup / XRange) into a ProbeResult.
//
// Strict requirements (return error):
//   - task_id and node_id must be non-empty
//   - schema_ver, if present and >= 2, returns ErrUnknownSchemaVer
//
// Lenient (best-effort decode, no error):
//   - missing optional fields → zero value
//   - duration_ms as int64 / float64 / string all accepted
//   - success as bool / "true" / "false" / "1" / "0" all accepted
//
// Returning a partial ProbeResult is intentional: stream consumers should
// be liberal in what they accept, conservative in what they emit.
func ParseProbeResult(vals map[string]any) (ProbeResult, error) {
	r := ProbeResult{}

	// schema_ver
	switch v := vals["schema_ver"].(type) {
	case nil:
		r.SchemaVer = 0 // legacy message
	case int:
		r.SchemaVer = v
	case int64:
		r.SchemaVer = int(v)
	case float64:
		r.SchemaVer = int(v)
	case string:
		n, err := strconv.Atoi(v)
		if err != nil {
			return ProbeResult{}, fmt.Errorf("contracts.ParseProbeResult: schema_ver=%q: %w", v, err)
		}
		r.SchemaVer = n
	default:
		return ProbeResult{}, fmt.Errorf("contracts.ParseProbeResult: schema_ver has unexpected type %T", v)
	}
	if r.SchemaVer > ProbeResultSchemaV1 {
		return ProbeResult{}, fmt.Errorf("%w: probe_result schema_ver=%d (known max=%d)",
			ErrUnknownSchemaVer, r.SchemaVer, ProbeResultSchemaV1)
	}

	// required strings
	taskID, _ := vals["task_id"].(string)
	if taskID == "" {
		return ProbeResult{}, errors.New("contracts.ParseProbeResult: task_id is required")
	}
	r.TaskID = taskID

	nodeID, _ := vals["node_id"].(string)
	if nodeID == "" {
		return ProbeResult{}, errors.New("contracts.ParseProbeResult: node_id is required")
	}
	r.NodeID = nodeID

	// optional strings
	r.RawJSON, _ = vals["raw"].(string)
	r.SummaryJSON, _ = vals["summary"].(string)
	r.Error, _ = vals["error"].(string)
	r.Signature, _ = vals["signature"].(string)
	r.MonitorID, _ = vals["monitor_id"].(string)

	// duration_ms
	if v, ok := vals["duration_ms"]; ok {
		ms, err := parseInt64(v)
		if err != nil {
			return ProbeResult{}, fmt.Errorf("contracts.ParseProbeResult: duration_ms: %w", err)
		}
		r.DurationMs = ms
	}

	// success
	if v, ok := vals["success"]; ok {
		s, err := parseBool(v)
		if err != nil {
			return ProbeResult{}, fmt.Errorf("contracts.ParseProbeResult: success: %w", err)
		}
		r.Success = s
	}

	return r, nil
}
