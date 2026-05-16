// Package hub — Prometheus metrics for the gateway service (P1#19).
//
// These metrics complement the existing connection / heartbeat counters in
// hub.go (gateway_connections_active, gateway_heartbeats_total,
// gateway_disconnects_total). They follow the docs/REVIEW-FINDINGS-2026-05-16.md
// naming and are exposed alongside the legacy ones via the default
// Prometheus registry — Grafana dashboards can adopt the new names
// incrementally without breaking the existing ones.
//
// Naming follows the spec:
//   - gateway_ws_connections_total{outcome}
//   - gateway_node_messages_total{type}
//   - gateway_active_connections (gauge)
//   - gateway_active_nodes       (gauge)
package hub

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MetricsWSConnections counts WebSocket connection lifecycle events.
//
//	outcome — "accepted"     handshake succeeded, connection registered
//	          "rejected"     handshake rejected (auth / capacity / etc.)
//	          "disconnected" connection torn down (any reason)
var MetricsWSConnections = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "gateway_ws_connections_total",
	Help: "Total number of WebSocket connection events, partitioned by outcome.",
}, []string{"outcome"})

// MetricsNodeMessages counts messages crossing the gateway, labelled by the
// message type field of the protocol envelope (e.g. "heartbeat",
// "probe_result", "probe_task", "unknown").
var MetricsNodeMessages = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "gateway_node_messages_total",
	Help: "Total number of gateway-routed messages, partitioned by message type.",
}, []string{"type"})

// MetricsActiveConnections mirrors gateway_connections_active under the
// spec-compliant name. Updated alongside the legacy gauge so both move in
// lockstep — once dashboards migrate, the legacy gauge can be retired.
var MetricsActiveConnections = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "gateway_active_connections",
	Help: "Current number of active agent WebSocket connections.",
})

// MetricsActiveNodes is the count of distinct registered nodes. Today this is
// identical to gateway_active_connections (one connection per node), but is
// exposed separately so multi-connection-per-node deployments can diverge
// without breaking dashboards.
var MetricsActiveNodes = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "gateway_active_nodes",
	Help: "Current number of registered agent nodes.",
})
