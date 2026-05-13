package task

import (
	"testing"
	"time"
)

func TestIsValidTaskType(t *testing.T) {
	tests := []struct {
		name     string
		taskType TaskType
		want     bool
	}{
		{"HTTP task", TaskHTTP, true},
		{"Ping task", TaskPing, true},
		{"TCP task", TaskTCP, true},
		{"DNS task", TaskDNS, true},
		{"Traceroute task", TaskTraceroute, true},
		{"Invalid task", TaskType("invalid"), false},
		{"Empty task", TaskType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidTaskType(tt.taskType); got != tt.want {
				t.Errorf("IsValidTaskType(%v) = %v, want %v", tt.taskType, got, tt.want)
			}
		})
	}
}

func TestTaskStructure(t *testing.T) {
	task := Task{
		ID:      "test-task-123",
		Type:    TaskHTTP,
		Target:  "example.com",
		Options: map[string]any{"method": "GET"},
		Timeout: 30 * time.Second,
		NodeID:  "node-456",
	}

	if task.ID != "test-task-123" {
		t.Errorf("Expected task ID %s, got %s", "test-task-123", task.ID)
	}

	if task.Type != TaskHTTP {
		t.Errorf("Expected task type %s, got %s", TaskHTTP, task.Type)
	}

	if task.Target != "example.com" {
		t.Errorf("Expected target %s, got %s", "example.com", task.Target)
	}

	if task.NodeID != "node-456" {
		t.Errorf("Expected node ID %s, got %s", "node-456", task.NodeID)
	}
}

func TestResultStructure(t *testing.T) {
	now := time.Now()
	result := Result{
		TaskID:     "test-task-123",
		NodeID:     "node-456",
		Type:       TaskHTTP,
		Target:     "example.com",
		Success:    true,
		Data:       map[string]any{"status_code": 200},
		Watermark:  "test-watermark",
		Timestamp:  now,
		DurationMs: 150,
	}

	if result.TaskID != "test-task-123" {
		t.Errorf("Expected task ID %s, got %s", "test-task-123", result.TaskID)
	}

	if result.Success != true {
		t.Errorf("Expected success %t, got %t", true, result.Success)
	}

	if result.DurationMs != 150 {
		t.Errorf("Expected duration %d, got %d", 150, result.DurationMs)
	}

	if len(result.Data) != 1 {
		t.Errorf("Expected 1 data item, got %d", len(result.Data))
	}

	if result.Data["status_code"] != 200 {
		t.Errorf("Expected status_code 200, got %v", result.Data["status_code"])
	}
}