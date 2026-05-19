package task

import (
	"testing"
	"time"

	"github.com/kite365/idcd/apps/agent/internal/probe"
)

// MockPingSender for testing
type MockPingSender struct {
	stats probe.PingStats
	err   error
}

func (m *MockPingSender) SendPing(target string, timeout time.Duration, count int) (probe.PingStats, error) {
	if m.err != nil {
		return probe.PingStats{}, m.err
	}
	return m.stats, nil
}

// MockDNSResolver for testing
type MockDNSResolver struct {
	hostResults []string
	mxResults   []probe.MXRecord
	txtResults  []string
	cnameResult string
	nsResults   []string
	err         error
}

func (m *MockDNSResolver) LookupHost(name string) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.hostResults, nil
}

func (m *MockDNSResolver) LookupMX(name string) ([]probe.MXRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.mxResults, nil
}

func (m *MockDNSResolver) LookupTXT(name string) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.txtResults, nil
}

func (m *MockDNSResolver) LookupCNAME(name string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.cnameResult, nil
}

func (m *MockDNSResolver) LookupNS(name string) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.nsResults, nil
}

func TestNewExecutor(t *testing.T) {
	secretKey := []byte("test-secret-key")
	executor := NewExecutor(secretKey, nil)

	if executor == nil {
		t.Fatal("Expected non-nil executor")
	}

	if executor.httpProbe == nil {
		t.Error("Expected HTTP probe to be initialized")
	}

	if executor.pingProbe == nil {
		t.Error("Expected ping probe to be initialized")
	}

	if executor.tcpProbe == nil {
		t.Error("Expected TCP probe to be initialized")
	}

	if executor.dnsProbe == nil {
		t.Error("Expected DNS probe to be initialized")
	}

	if executor.tracerouteProbe == nil {
		t.Error("Expected traceroute probe to be initialized")
	}
}

func TestExecutor_Execute(t *testing.T) {
	secretKey := []byte("test-secret-key")
	executor := NewExecutor(secretKey, nil)

	// Mock the ping probe
	mockPingProbe := &probe.PingProbe{
		Sender: &MockPingSender{
			stats: probe.PingStats{
				PacketsSent:     5,
				PacketsReceived: 5,
				PacketLoss:      0.0,
				MinRTT:          10 * time.Millisecond,
				AvgRTT:          15 * time.Millisecond,
				MaxRTT:          20 * time.Millisecond,
			},
		},
	}
	executor.SetPingProbe(mockPingProbe)

	// Mock the DNS probe
	mockDNSProbe := &probe.DNSProbe{
		Resolver: &MockDNSResolver{
			hostResults: []string{"192.168.1.1"},
		},
	}
	executor.SetDNSProbe(mockDNSProbe)

	tests := []struct {
		name         string
		task         Task
		expectSuccess bool
		expectType    TaskType
	}{
		{
			name: "HTTP task",
			task: Task{
				ID:      "http-task-1",
				Type:    TaskHTTP,
				Target:  "example.com",
				Options: map[string]any{"method": "GET"},
				Timeout: 30 * time.Second,
				NodeID:  "test-node",
			},
			expectSuccess: false, // Will fail without real server
			expectType:    TaskHTTP,
		},
		{
			name: "Ping task",
			task: Task{
				ID:      "ping-task-1",
				Type:    TaskPing,
				Target:  "example.com",
				Options: map[string]any{"count": 5},
				Timeout: 30 * time.Second,
				NodeID:  "test-node",
			},
			expectSuccess: true, // Mock will succeed
			expectType:    TaskPing,
		},
		{
			name: "TCP task",
			task: Task{
				ID:      "tcp-task-1",
				Type:    TaskTCP,
				Target:  "example.com:80",
				Options: map[string]any{},
				Timeout: 5 * time.Second,
				NodeID:  "test-node",
			},
			expectSuccess: false, // Will fail without real connection
			expectType:    TaskTCP,
		},
		{
			name: "DNS task",
			task: Task{
				ID:      "dns-task-1",
				Type:    TaskDNS,
				Target:  "example.com",
				Options: map[string]any{},
				Timeout: 30 * time.Second,
				NodeID:  "test-node",
			},
			expectSuccess: true, // Mock will succeed
			expectType:    TaskDNS,
		},
		{
			name: "Traceroute task",
			task: Task{
				ID:      "traceroute-task-1",
				Type:    TaskTraceroute,
				Target:  "example.com",
				Options: map[string]any{},
				Timeout: 30 * time.Second,
				NodeID:  "test-node",
			},
			expectSuccess: false, // Will likely fail without privileges
			expectType:    TaskTraceroute,
		},
		{
			name: "Invalid task type",
			task: Task{
				ID:      "invalid-task-1",
				Type:    TaskType("invalid"),
				Target:  "example.com",
				Options: map[string]any{},
				Timeout: 30 * time.Second,
				NodeID:  "test-node",
			},
			expectSuccess: false,
			expectType:    TaskType("invalid"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.Execute(tt.task)

			// Check basic result structure
			if result == nil {
				t.Fatal("Expected non-nil result")
			}

			if result.TaskID != tt.task.ID {
				t.Errorf("Expected TaskID %s, got %s", tt.task.ID, result.TaskID)
			}

			if result.NodeID != tt.task.NodeID {
				t.Errorf("Expected NodeID %s, got %s", tt.task.NodeID, result.NodeID)
			}

			if result.Type != tt.expectType {
				t.Errorf("Expected Type %s, got %s", tt.expectType, result.Type)
			}

			if result.Target != tt.task.Target {
				t.Errorf("Expected Target %s, got %s", tt.task.Target, result.Target)
			}

			// Check that watermark is generated
			if result.Watermark == "" {
				t.Error("Expected watermark to be generated")
			}

			// Check timestamp
			if result.Timestamp.IsZero() {
				t.Error("Expected non-zero timestamp")
			}

			// Check duration
			if result.DurationMs < 0 {
				t.Error("Expected non-negative duration")
			}

			// Check data structure
			if result.Data == nil {
				t.Error("Expected non-nil data map")
			}

			// For invalid task type, should have error
			if tt.task.Type == TaskType("invalid") {
				if result.Success {
					t.Error("Expected failure for invalid task type")
				}
				if result.Error == "" {
					t.Error("Expected error message for invalid task type")
				}
			}

			// Check expected success/failure
			if result.Success != tt.expectSuccess {
				t.Logf("Task %s: success=%t, error=%s", tt.name, result.Success, result.Error)
			}
		})
	}
}

func TestExecutor_ExecuteWithDefaults(t *testing.T) {
	secretKey := []byte("test-secret-key")
	executor := NewExecutor(secretKey, nil)

	// Test with zero timeout (should get default)
	task := Task{
		ID:      "test-task",
		Type:    TaskHTTP,
		Target:  "example.com",
		Options: map[string]any{},
		Timeout: 0, // Should get default
		NodeID:  "test-node",
	}

	result := executor.Execute(task)

	// Should not panic and should have some duration
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Should have attempted the request (even if it fails)
	if result.TaskID != task.ID {
		t.Errorf("Expected TaskID %s, got %s", task.ID, result.TaskID)
	}
}

func TestExecutor_ExecuteBatch(t *testing.T) {
	secretKey := []byte("test-secret-key")
	executor := NewExecutor(secretKey, nil)

	// Mock the ping probe for consistent results
	mockPingProbe := &probe.PingProbe{
		Sender: &MockPingSender{
			stats: probe.PingStats{
				PacketsSent:     5,
				PacketsReceived: 5,
				PacketLoss:      0.0,
			},
		},
	}
	executor.SetPingProbe(mockPingProbe)

	tasks := []Task{
		{
			ID:      "task-1",
			Type:    TaskPing,
			Target:  "example1.com",
			Timeout: 5 * time.Second,
			NodeID:  "test-node",
		},
		{
			ID:      "task-2",
			Type:    TaskPing,
			Target:  "example2.com",
			Timeout: 5 * time.Second,
			NodeID:  "test-node",
		},
		{
			ID:      "task-3",
			Type:    TaskType("invalid"),
			Target:  "example3.com",
			Timeout: 5 * time.Second,
			NodeID:  "test-node",
		},
	}

	results := executor.ExecuteBatch(tasks)

	if len(results) != len(tasks) {
		t.Errorf("Expected %d results, got %d", len(tasks), len(results))
	}

	for i, result := range results {
		if result.TaskID != tasks[i].ID {
			t.Errorf("Result %d: expected TaskID %s, got %s", i, tasks[i].ID, result.TaskID)
		}

		if result.NodeID != tasks[i].NodeID {
			t.Errorf("Result %d: expected NodeID %s, got %s", i, tasks[i].NodeID, result.NodeID)
		}

		if result.Watermark == "" {
			t.Errorf("Result %d: expected watermark to be generated", i)
		}
	}

	// First two should succeed (mock ping), last should fail (invalid type)
	if !results[0].Success {
		t.Error("Expected first task to succeed")
	}

	if !results[1].Success {
		t.Error("Expected second task to succeed")
	}

	if results[2].Success {
		t.Error("Expected third task to fail (invalid type)")
	}
}

func TestExecutor_SetProbes(t *testing.T) {
	secretKey := []byte("test-secret-key")
	executor := NewExecutor(secretKey, nil)

	// Test SetPingProbe
	customPingProbe := &probe.PingProbe{
		Sender: &MockPingSender{
			stats: probe.PingStats{PacketsSent: 1, PacketsReceived: 1},
		},
	}

	executor.SetPingProbe(customPingProbe)
	if executor.pingProbe != customPingProbe {
		t.Error("Expected ping probe to be set")
	}

	// Test SetDNSProbe
	customDNSProbe := &probe.DNSProbe{
		Resolver: &MockDNSResolver{
			hostResults: []string{"1.2.3.4"},
		},
	}

	executor.SetDNSProbe(customDNSProbe)
	if executor.dnsProbe != customDNSProbe {
		t.Error("Expected DNS probe to be set")
	}
}