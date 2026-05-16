// Package scheduler — Prometheus metrics for the scheduler service (P1#19).
//
// The scheduler exposes leadership and monitor-polling state for Grafana
// dashboards. Metrics are registered against the default Prometheus registry
// via promauto so cmd/scheduler/main.go's /metrics handler picks them up
// without explicit wiring.
//
// Naming follows the docs/REVIEW-FINDINGS-2026-05-16.md spec:
//   - scheduler_monitor_polls_total{outcome}
//   - scheduler_leader_renewals_total{outcome}
//   - scheduler_is_leader{node}
//   - scheduler_poll_duration_seconds
package scheduler

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MetricsMonitorPolls counts each call to pollMonitors.
//
//	outcome — "ok"     pollMonitors returned without listing error
//	          "error"  monitorStore.ListActiveMonitorsDue returned an error
var MetricsMonitorPolls = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "scheduler_monitor_polls_total",
	Help: "Total number of monitor-polling iterations, partitioned by outcome.",
}, []string{"outcome"})

// MetricsLeaderRenewals counts every leader.Renew attempt.
//
//	outcome — "ok"    renew succeeded
//	          "fail"  renew returned an error (workCtx is cancelled)
var MetricsLeaderRenewals = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "scheduler_leader_renewals_total",
	Help: "Total number of Redis leader-lock renewal attempts, partitioned by outcome.",
}, []string{"outcome"})

// MetricsIsLeader is 1 when this scheduler instance currently holds the
// leader lock, 0 otherwise. The "node" label carries the scheduler's nodeID
// (set by main.go) so Grafana can chart the leader handoff between replicas.
var MetricsIsLeader = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "scheduler_is_leader",
	Help: "1 if this scheduler instance holds the leader lock, 0 otherwise.",
}, []string{"node"})

// MetricsPollDuration observes wall-clock time of each pollMonitors call.
// The single-bucket histogram surfaces tail latency from the DB query plus
// downstream stream pushes.
var MetricsPollDuration = promauto.NewHistogram(prometheus.HistogramOpts{
	Name:    "scheduler_poll_duration_seconds",
	Help:    "Wall-clock duration of scheduler.pollMonitors invocations.",
	Buckets: prometheus.DefBuckets,
})
