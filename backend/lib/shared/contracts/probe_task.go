package contracts

import (
	"errors"
	"fmt"
	"strconv"
)

// ProbeTask is the standard payload of the `probe.tasks` Redis stream.
//
// 生产者:
//   - apps/scheduler — monitor poller (writes Epoch fencing token)
//   - apps/api — ad-hoc tool probe handler (no Epoch; writes target_normalized
//     / node_selection / created_at instead)
//
// 消费者: apps/gateway dispatcher (epoch 校验 + forward 给 agent)
type ProbeTask struct {
	SchemaVer int    `json:"schema_ver"                  stream:"schema_ver"`
	TaskID    string `json:"task_id"                     stream:"task_id"`
	Type      string `json:"type"                        stream:"type"`
	Target    string `json:"target"                      stream:"target"`
	NodeID    string `json:"node_id,omitempty"           stream:"node_id,omitempty"`
	MonitorID string `json:"monitor_id,omitempty"        stream:"monitor_id,omitempty"`

	Priority int `json:"priority,omitempty"             stream:"priority,omitempty"`

	// ParamsJSON is the JSON-encoded per-probe parameters blob. Both producers
	// json.Marshal a map[string]any before writing; we keep the encoded string
	// to avoid double-marshalling on the consumer side.
	ParamsJSON string `json:"params,omitempty"           stream:"params,omitempty"`

	// Epoch is the scheduler-side fencing token (see
	// apps/scheduler/internal/leader/fencing.go). nil means the producer did
	// not run under leader election (api ad-hoc probes); the gateway
	// dispatcher treats nil/missing as the legacy-compat "missing" branch
	// and increments a counter. Use a pointer so the zero value (no epoch)
	// is distinct from an explicit epoch=0.
	Epoch *int64 `json:"epoch,omitempty"             stream:"epoch,omitempty"`

	// api-only fields. The scheduler poller doesn't write these; they ride
	// straight through dispatcher to the agent so probe handlers can apply
	// the operator's node-selection policy and the original input target.
	TargetNormalized  string `json:"target_normalized,omitempty" stream:"target_normalized,omitempty"`
	NodeSelectionJSON string `json:"node_selection,omitempty"    stream:"node_selection,omitempty"`
	CreatedAt         int64  `json:"created_at,omitempty"        stream:"created_at,omitempty"`
}

// ProbeTaskSchemaV1 is the current ProbeTask wire format version.
const ProbeTaskSchemaV1 = 1

// ToStreamValues converts the struct into a Redis XAdd values map.
//
// Wire compatibility: only required fields (task_id, type, target) plus the
// schema marker are always written. Optional fields are omitted when zero so
// the wire shape stays identical to what scheduler / api/probe.go used to
// hand-write — no consumer changes required.
//
// Priority omits zero on purpose: P0 (Critical = 0) is technically valid but
// no producer writes P0, and omitting matches the historical scheduler
// behaviour where P2 was always the literal in the map. If a future producer
// needs to emit P0 explicitly, switch Priority to *int.
func (t ProbeTask) ToStreamValues() map[string]any {
	schemaVer := t.SchemaVer
	if schemaVer == 0 {
		schemaVer = ProbeTaskSchemaV1
	}
	// Pre-size for the worst case (scheduler shape: 9 keys, api shape: 9 keys).
	vals := make(map[string]any, 11)
	vals["schema_ver"] = strconv.Itoa(schemaVer)
	vals["task_id"] = t.TaskID
	vals["type"] = t.Type
	vals["target"] = t.Target
	if t.NodeID != "" {
		vals["node_id"] = t.NodeID
	}
	if t.MonitorID != "" {
		vals["monitor_id"] = t.MonitorID
	}
	if t.Priority != 0 {
		vals["priority"] = t.Priority
	}
	if t.ParamsJSON != "" {
		vals["params"] = t.ParamsJSON
	}
	if t.Epoch != nil {
		vals["epoch"] = strconv.FormatInt(*t.Epoch, 10)
	}
	if t.TargetNormalized != "" {
		vals["target_normalized"] = t.TargetNormalized
	}
	if t.NodeSelectionJSON != "" {
		vals["node_selection"] = t.NodeSelectionJSON
	}
	if t.CreatedAt != 0 {
		vals["created_at"] = strconv.FormatInt(t.CreatedAt, 10)
	}
	return vals
}

// ParseProbeTask decodes a Redis stream values map into a ProbeTask.
//
// Lenient by design — the dispatcher must accept tasks from both the
// scheduler (monitor poller, has epoch) and the api ad-hoc handler (no
// epoch, but has target_normalized / node_selection). A required field
// missing is a hard error; optional fields silently default.
func ParseProbeTask(vals map[string]any) (ProbeTask, error) {
	t := ProbeTask{}

	switch v := vals["schema_ver"].(type) {
	case nil:
		t.SchemaVer = 0
	case int:
		t.SchemaVer = v
	case int64:
		t.SchemaVer = int(v)
	case float64:
		t.SchemaVer = int(v)
	case string:
		n, err := strconv.Atoi(v)
		if err != nil {
			return ProbeTask{}, fmt.Errorf("contracts.ParseProbeTask: schema_ver=%q: %w", v, err)
		}
		t.SchemaVer = n
	default:
		return ProbeTask{}, fmt.Errorf("contracts.ParseProbeTask: schema_ver has unexpected type %T", v)
	}
	if t.SchemaVer > ProbeTaskSchemaV1 {
		return ProbeTask{}, fmt.Errorf("%w: probe_task schema_ver=%d (known max=%d)",
			ErrUnknownSchemaVer, t.SchemaVer, ProbeTaskSchemaV1)
	}

	taskID, _ := vals["task_id"].(string)
	if taskID == "" {
		return ProbeTask{}, errors.New("contracts.ParseProbeTask: task_id is required")
	}
	t.TaskID = taskID

	probeType, _ := vals["type"].(string)
	if probeType == "" {
		return ProbeTask{}, errors.New("contracts.ParseProbeTask: type is required")
	}
	t.Type = probeType

	target, _ := vals["target"].(string)
	if target == "" {
		return ProbeTask{}, errors.New("contracts.ParseProbeTask: target is required")
	}
	t.Target = target

	t.NodeID, _ = vals["node_id"].(string)
	t.MonitorID, _ = vals["monitor_id"].(string)
	t.ParamsJSON, _ = vals["params"].(string)
	t.TargetNormalized, _ = vals["target_normalized"].(string)
	t.NodeSelectionJSON, _ = vals["node_selection"].(string)

	if v, ok := vals["priority"]; ok {
		p, err := parseInt64(v)
		if err != nil {
			return ProbeTask{}, fmt.Errorf("contracts.ParseProbeTask: priority: %w", err)
		}
		t.Priority = int(p)
	}

	if v, ok := vals["epoch"]; ok {
		epoch, err := parseInt64(v)
		if err != nil {
			return ProbeTask{}, fmt.Errorf("contracts.ParseProbeTask: epoch: %w", err)
		}
		t.Epoch = &epoch
	}

	if v, ok := vals["created_at"]; ok {
		ts, err := parseInt64(v)
		if err != nil {
			return ProbeTask{}, fmt.Errorf("contracts.ParseProbeTask: created_at: %w", err)
		}
		t.CreatedAt = ts
	}

	return t, nil
}
