package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/scheduler/internal/leader"
	"github.com/kite365/idcd/apps/scheduler/internal/queue"
	"github.com/kite365/idcd/lib/shared/stream"
)

func setupRedis(t *testing.T) (*miniredis.Miniredis, redis.Cmdable) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, rdb
}

// mockNodeSelector always returns a fixed node ID.
type mockNodeSelector struct {
	nodeID string
	err    error
}

func (m *mockNodeSelector) SelectNode(ctx context.Context, task *queue.ProbeTask) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.nodeID, nil
}

func TestProcessTask(t *testing.T) {
	_, rdb := setupRedis(t)
	ctx := context.Background()

	// Setup
	q := queue.New(rdb, "test:queue")
	streamClient := stream.New(rdb)
	selector := &mockNodeSelector{nodeID: "nd_test_01"}
	l := leader.New(rdb, "test:leader", 10*time.Second, "node1")

	// Acquire leadership
	ok, err := l.Acquire(ctx)
	if err != nil {
		t.Fatalf("l.Acquire: %v", err)
	}
	if !ok {
		t.Fatalf("l.Acquire() = false, want true")
	}

	s := New(Config{
		Leader:      l,
		Queue:       q,
		Selector:    selector,
		Stream:      streamClient,
		Pool:        nil, // not used in S1
		WorkerCount: 2,
	})

	// Create task
	task := &queue.ProbeTask{
		ID:       "pt_test123",
		Type:     "http",
		Target:   "https://example.com",
		Priority: queue.P2,
		Params:   map[string]string{"method": "GET"},
	}

	// Process task
	err = s.processTask(ctx, task)
	if err != nil {
		t.Fatalf("s.processTask: %v", err)
	}

	// Verify task was assigned node
	if task.NodeID != "nd_test_01" {
		t.Errorf("task.NodeID = %q, want nd_test_01", task.NodeID)
	}

	// Verify task was added to stream
	length, err := streamClient.Len(ctx, ProbeTasksStream)
	if err != nil {
		t.Fatalf("streamClient.Len: %v", err)
	}
	if length != 1 {
		t.Errorf("stream length = %d, want 1", length)
	}

	// Read stream entry to verify content
	entries, err := rdb.XRange(ctx, ProbeTasksStream, "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	values := entries[0].Values
	if values["task_id"] != "pt_test123" {
		t.Errorf("values[task_id] = %v, want pt_test123", values["task_id"])
	}
	if values["type"] != "http" {
		t.Errorf("values[type] = %v, want http", values["type"])
	}
	if values["node_id"] != "nd_test_01" {
		t.Errorf("values[node_id] = %v, want nd_test_01", values["node_id"])
	}
	if values["param_method"] != "GET" {
		t.Errorf("values[param_method] = %v, want GET", values["param_method"])
	}
}

func TestStaticNodeSelector(t *testing.T) {
	ctx := context.Background()

	t.Run("select from multiple nodes", func(t *testing.T) {
		nodes := []string{"node1", "node2", "node3"}
		selector := NewStaticNodeSelector(nodes)

		task := &queue.ProbeTask{ID: "pt_test"}

		// Select multiple times and ensure we get valid nodes
		seen := make(map[string]bool)
		for i := 0; i < 20; i++ {
			nodeID, err := selector.SelectNode(ctx, task)
			if err != nil {
				t.Fatalf("selector.SelectNode: %v", err)
			}

			// Check it's a valid node
			valid := false
			for _, n := range nodes {
				if nodeID == n {
					valid = true
					break
				}
			}
			if !valid {
				t.Errorf("SelectNode() = %q, not in nodes list", nodeID)
			}

			seen[nodeID] = true
		}

		// With 20 selections from 3 nodes, we should see all nodes (statistically)
		// This is probabilistic, but failure is extremely unlikely
		if len(seen) < 2 {
			t.Errorf("Only saw %d unique nodes out of 3 after 20 selections", len(seen))
		}
	})

	t.Run("select from single node", func(t *testing.T) {
		nodes := []string{"only_node"}
		selector := NewStaticNodeSelector(nodes)

		task := &queue.ProbeTask{ID: "pt_test"}
		nodeID, err := selector.SelectNode(ctx, task)
		if err != nil {
			t.Fatalf("selector.SelectNode: %v", err)
		}
		if nodeID != "only_node" {
			t.Errorf("SelectNode() = %q, want only_node", nodeID)
		}
	})

	t.Run("no nodes available", func(t *testing.T) {
		selector := NewStaticNodeSelector([]string{})

		task := &queue.ProbeTask{ID: "pt_test"}
		_, err := selector.SelectNode(ctx, task)
		if err == nil {
			t.Errorf("SelectNode() with no nodes should return error")
		}
	})
}

// --- Monitor poller tests ---

// mockMonitorStore implements MonitorStore for testing.
type mockMonitorStore struct {
	monitors []DueMonitor
	err      error
}

func (m *mockMonitorStore) ListActiveMonitorsDue(ctx context.Context) ([]DueMonitor, error) {
	return m.monitors, m.err
}

func TestMonitorTypeToProbeType(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"http", "http"},
		{"https", "http"},
		{"keyword", "http"},
		{"ssl_expiry", "http"},
		{"domain_expiry", "http"},
		{"icp_change", "http"},
		{"ping", "ping"},
		{"tcp", "tcp"},
		{"dns", "dns"},
		{"unknown", "http"},
	}
	for _, tc := range tests {
		got := monitorTypeToProbeType(tc.in)
		if got != tc.want {
			t.Errorf("monitorTypeToProbeType(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPollMonitors_dispatchesTasks(t *testing.T) {
	_, rdb := setupRedis(t)
	ctx := context.Background()

	streamClient := stream.New(rdb)
	selector := &mockNodeSelector{nodeID: "nd_test_01"}
	l := leader.New(rdb, "test:leader2", 10*time.Second, "node1")
	ok, err := l.Acquire(ctx)
	if err != nil || !ok {
		t.Fatalf("l.Acquire: %v ok=%v", err, ok)
	}

	config := json.RawMessage(`{"timeout_ms": 5000}`)
	store := &mockMonitorStore{
		monitors: []DueMonitor{
			{ID: "mon_001", Type: "http", Target: "example.com", IntervalS: 300, NodeCount: 2, Config: config},
		},
	}

	s := New(Config{
		Leader:       l,
		Queue:        queue.New(rdb, "test:queue2"),
		Selector:     selector,
		Stream:       streamClient,
		MonitorStore: store,
		WorkerCount:  1,
	})

	s.pollMonitors(ctx)

	// NodeCount=2 → 2 tasks should be pushed
	length, err := streamClient.Len(ctx, ProbeTasksStream)
	if err != nil {
		t.Fatalf("streamClient.Len: %v", err)
	}
	if length != 2 {
		t.Errorf("stream length = %d, want 2 (one per node)", length)
	}

	// Verify monitor_id field is present
	entries, err := rdb.XRange(ctx, ProbeTasksStream, "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange: %v", err)
	}
	if len(entries) < 1 {
		t.Fatalf("no stream entries")
	}
	if entries[0].Values["monitor_id"] != "mon_001" {
		t.Errorf("monitor_id = %v, want mon_001", entries[0].Values["monitor_id"])
	}
	if entries[0].Values["type"] != "http" {
		t.Errorf("type = %v, want http", entries[0].Values["type"])
	}
}

func TestPollMonitors_storeError(t *testing.T) {
	_, rdb := setupRedis(t)
	ctx := context.Background()

	streamClient := stream.New(rdb)
	selector := &mockNodeSelector{nodeID: "nd_test_01"}
	l := leader.New(rdb, "test:leader3", 10*time.Second, "node1")
	_, _ = l.Acquire(ctx)

	store := &mockMonitorStore{err: errors.New("db error")}

	s := New(Config{
		Leader:       l,
		Queue:        queue.New(rdb, "test:queue3"),
		Selector:     selector,
		Stream:       streamClient,
		MonitorStore: store,
		WorkerCount:  1,
	})

	// Should not panic — just log and return.
	s.pollMonitors(ctx)

	length, _ := streamClient.Len(ctx, ProbeTasksStream)
	if length != 0 {
		t.Errorf("expected 0 stream messages after store error, got %d", length)
	}
}

func TestPollMonitors_emptyMonitors(t *testing.T) {
	_, rdb := setupRedis(t)
	ctx := context.Background()

	streamClient := stream.New(rdb)
	selector := &mockNodeSelector{nodeID: "nd_test_01"}
	l := leader.New(rdb, "test:leader4", 10*time.Second, "node1")
	_, _ = l.Acquire(ctx)

	store := &mockMonitorStore{monitors: []DueMonitor{}}

	s := New(Config{
		Leader:       l,
		Queue:        queue.New(rdb, "test:queue4"),
		Selector:     selector,
		Stream:       streamClient,
		MonitorStore: store,
		WorkerCount:  1,
	})

	s.pollMonitors(ctx)

	length, _ := streamClient.Len(ctx, ProbeTasksStream)
	if length != 0 {
		t.Errorf("expected 0 stream messages for empty monitor list, got %d", length)
	}
}

func TestWorkerStopsWhenLostLeadership(t *testing.T) {
	_, rdb := setupRedis(t)

	// Setup
	q := queue.New(rdb, "test:queue")
	streamClient := stream.New(rdb)
	selector := &mockNodeSelector{nodeID: "nd_test_01"}
	l := leader.New(rdb, "test:leader", 10*time.Second, "node1")

	// Acquire leadership
	ctx := context.Background()
	ok, err := l.Acquire(ctx)
	if err != nil {
		t.Fatalf("l.Acquire: %v", err)
	}
	if !ok {
		t.Fatalf("l.Acquire() = false, want true")
	}

	s := New(Config{
		Leader:      l,
		Queue:       q,
		Selector:    selector,
		Stream:      streamClient,
		Pool:        nil,
		WorkerCount: 1,
	})

	// Enqueue a task
	task := &queue.ProbeTask{
		ID:       "pt_test123",
		Type:     "http",
		Target:   "https://example.com",
		Priority: queue.P2,
	}
	if err := q.Enqueue(ctx, task); err != nil {
		t.Fatalf("q.Enqueue: %v", err)
	}

	// Start worker in goroutine
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go s.worker(workerCtx, 0)

	// Wait for task to be processed
	time.Sleep(1500 * time.Millisecond)

	// Verify task was processed
	length, err := streamClient.Len(ctx, ProbeTasksStream)
	if err != nil {
		t.Fatalf("streamClient.Len: %v", err)
	}
	if length != 1 {
		t.Errorf("stream length = %d, want 1 (task should be processed)", length)
	}

	// Simulate leadership loss
	if err := l.Release(ctx); err != nil {
		t.Fatalf("l.Release: %v", err)
	}

	// Enqueue another task
	task2 := &queue.ProbeTask{
		ID:       "pt_test456",
		Type:     "http",
		Target:   "https://example.com",
		Priority: queue.P2,
	}
	if err := q.Enqueue(ctx, task2); err != nil {
		t.Fatalf("q.Enqueue: %v", err)
	}

	// Wait a bit
	time.Sleep(1500 * time.Millisecond)

	// Second task should NOT be processed (worker should have stopped)
	length, err = streamClient.Len(ctx, ProbeTasksStream)
	if err != nil {
		t.Fatalf("streamClient.Len: %v", err)
	}
	if length != 1 {
		t.Errorf("stream length = %d, want 1 (second task should not be processed after losing leadership)", length)
	}

	// Cancel worker context
	cancel()
}
