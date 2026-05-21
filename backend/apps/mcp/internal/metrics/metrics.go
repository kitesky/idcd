// Package metrics defines idcd-mcp Prometheus business metrics
// (P1-11 Phase 1).
//
// The MCP server's previous /metrics surface was just Go runtime metrics
// — this package adds the three signals the D13 capacity decision is
// built on:
//
//   - sse_connections (gauge)        — current active SSE connections
//                                       per process. D13 capacity bound:
//                                       10k connections per instance.
//   - tool_invocations_total{tool, outcome} — per-tool call counter.
//   - tool_duration_seconds{tool}    — per-tool latency histogram.
//
// Naming convention: idcd_mcp_<subsystem>_<noun>_<unit>.
//
// Cardinality discipline:
//
//   - "tool" — bounded by the static list of registered tools in
//     internal/tools/registry. New tools must be reviewed for cardinality
//     impact (currently ≤ 50).
//   - "outcome" — bounded enum.
//   - never label by user / session id / arg payload.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// SSEConnections is the current count of active SSE connections held by
// the MCP HTTP transport. Incremented on connection accept, decremented
// on disconnect / context cancellation.
var SSEConnections = promauto.NewGauge(prometheus.GaugeOpts{
	Namespace: "idcd_mcp",
	Subsystem: "sse",
	Name:      "connections",
	Help:      "当前活跃的 SSE 连接数 (D13 容量监控, 10k/instance 上限)",
})

// ToolInvocations counts every tools/call request.
//
//	tool    — registered tool name (e.g. "monitors.list").
//	outcome — "ok" | "tool_failure" | "internal_error" | "method_not_found"
var ToolInvocations = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "idcd_mcp",
	Subsystem: "tool",
	Name:      "invocations_total",
	Help:      "MCP tool 调用次数 (按 tool + outcome)",
}, []string{"tool", "outcome"})

// ToolDuration observes wall-clock time per tool invocation. Buckets
// favour sub-second responses (most MCP tools are thin proxies onto the
// idcd API).
var ToolDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "idcd_mcp",
	Subsystem: "tool",
	Name:      "duration_seconds",
	Help:      "MCP tool 调用延迟分布 (按 tool)",
	Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
}, []string{"tool"})

// RecordToolInvocation is the single hook the JSON-RPC dispatcher calls
// after each handler return. duration may be zero (e.g. early-rejection
// paths) — the histogram observation is skipped in that case so the
// bucket distribution isn't polluted with 0s samples.
func RecordToolInvocation(tool, outcome string, durationSeconds float64) {
	t := tool
	if t == "" {
		t = "unknown"
	}
	o := outcome
	if o == "" {
		o = "unknown"
	}
	ToolInvocations.WithLabelValues(t, o).Inc()
	if durationSeconds > 0 {
		ToolDuration.WithLabelValues(t).Observe(durationSeconds)
	}
}
