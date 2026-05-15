// Package task defines core data structures for agent tasks.
package task

import (
	"time"

	"github.com/kite365/idcd/apps/agent/internal/probe"
)

// Task represents a probe task to be executed by the agent.
type Task struct {
	ID      string               `json:"task_id"`
	Type    probe.TaskType       `json:"type"`
	Target  string               `json:"target"`      // domain or IP
	Options map[string]any       `json:"options"`     // type-specific opts
	Timeout time.Duration        `json:"timeout_ms"`
	NodeID  string               `json:"node_id"`     // injected to each task
}

// Re-export types from probe package for convenience
type TaskType = probe.TaskType
type Result = probe.Result

const (
	TaskHTTP       = probe.TaskHTTP
	TaskPing       = probe.TaskPing
	TaskTCP        = probe.TaskTCP
	TaskDNS        = probe.TaskDNS
	TaskTraceroute = probe.TaskTraceroute
	TaskSMTP       = probe.TaskSMTP
	TaskNTP        = probe.TaskNTP
)

// IsValidTaskType checks if the given task type is supported.
func IsValidTaskType(t probe.TaskType) bool {
	switch t {
	case probe.TaskHTTP, probe.TaskPing, probe.TaskTCP, probe.TaskDNS, probe.TaskTraceroute,
		probe.TaskSMTP, probe.TaskNTP:
		return true
	default:
		return false
	}
}