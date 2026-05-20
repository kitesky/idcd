// Package queue defines the probe task data type and priority levels used by
// the scheduler.
//
// Historical note: this package previously also implemented a Redis sorted-set
// priority queue (`scheduler:tasks` ZSET) consumed by a scheduler worker pool.
// Production never produced into that ZSET — every ad-hoc tool probe writes
// directly to the `probe.tasks` stream via the API, and monitor checks are
// dispatched by the monitor poller inside the scheduler. The worker pool was
// dead code that contributed to a leader split-brain window, so it and the
// ZSET helpers were removed (see docs/REVIEW-FINDINGS-2026-05-16.md P0#8).
//
// Only the data types remain: NodeSelector implementations and the monitor
// poller still need ProbeTask + the priority constants.
package queue

import "time"

const (
	// Priority levels (lower number = higher priority)
	P0 = 0 // Critical
	P1 = 1 // High
	P2 = 2 // Normal
	P3 = 3 // Low
	P4 = 4 // Very low
	P5 = 5 // Lowest
)

// ProbeTask represents a task to be scheduled to a node.
type ProbeTask struct {
	ID         string            `json:"id"`          // probe_task.id
	Type       string            `json:"type"`        // http/tcp/ping/dns/udp
	Target     string            `json:"target"`      // URL or host
	NodeID     string            `json:"node_id"`     // assigned node ID
	Priority   int               `json:"priority"`    // P0-P5
	Params     map[string]string `json:"params"`      // probe-specific parameters
	MonitorID  string            `json:"monitor_id"`  // non-empty for monitor-originated tasks
	EnqueuedAt time.Time         `json:"enqueued_at"`
}
