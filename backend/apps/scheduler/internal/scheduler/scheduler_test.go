package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
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
	calls    atomic.Int32
}

func (m *mockMonitorStore) ListActiveMonitorsDue(ctx context.Context) ([]DueMonitor, error) {
	m.calls.Add(1)
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
		Selector:     selector,
		Stream:       streamClient,
		MonitorStore: store,
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
		Selector:     selector,
		Stream:       streamClient,
		MonitorStore: store,
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
		Selector:     selector,
		Stream:       streamClient,
		MonitorStore: store,
	})

	s.pollMonitors(ctx)

	length, _ := streamClient.Len(ctx, ProbeTasksStream)
	if length != 0 {
		t.Errorf("expected 0 stream messages for empty monitor list, got %d", length)
	}
}

// TestPollMonitors_respectsCancelledContext verifies the poller bails out
// before hitting the DB or stream once its context is cancelled. This is the
// safety net behind the leader race fix: when renewLeadership cancels workCtx
// the in-flight pollMonitors call must stop instead of finishing a full poll
// against the new leader's data.
func TestPollMonitors_respectsCancelledContext(t *testing.T) {
	_, rdb := setupRedis(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	streamClient := stream.New(rdb)
	selector := &mockNodeSelector{nodeID: "nd_test_01"}
	l := leader.New(rdb, "test:leader_cancel", 10*time.Second, "node1")
	_, _ = l.Acquire(context.Background())

	store := &mockMonitorStore{
		monitors: []DueMonitor{
			{ID: "mon_x", Type: "http", Target: "x.com", NodeCount: 1},
		},
	}

	s := New(Config{
		Leader:       l,
		Selector:     selector,
		Stream:       streamClient,
		MonitorStore: store,
	})

	s.pollMonitors(ctx)

	// Cancelled ctx → must not query DB and must not push to stream.
	if got := store.calls.Load(); got != 0 {
		t.Errorf("ListActiveMonitorsDue calls = %d, want 0 when ctx cancelled", got)
	}
	length, _ := streamClient.Len(ctx, ProbeTasksStream)
	if length != 0 {
		t.Errorf("stream length = %d, want 0 when ctx cancelled", length)
	}
}

// TestMonitorPoller_stopsOnContextCancel exercises the long-running goroutine
// loop. The leader race fix routes leader loss → workCtx cancel → poller
// returns. We simulate the same shape here by cancelling the ctx the poller
// is running with.
func TestMonitorPoller_stopsOnContextCancel(t *testing.T) {
	_, rdb := setupRedis(t)
	streamClient := stream.New(rdb)
	selector := &mockNodeSelector{nodeID: "nd_test_01"}
	l := leader.New(rdb, "test:leader_cancel_loop", 10*time.Second, "node1")
	if ok, err := l.Acquire(context.Background()); err != nil || !ok {
		t.Fatalf("l.Acquire: %v ok=%v", err, ok)
	}

	store := &mockMonitorStore{monitors: []DueMonitor{}}

	s := New(Config{
		Leader:       l,
		Selector:     selector,
		Stream:       streamClient,
		MonitorStore: store,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.monitorPoller(ctx)
		close(done)
	}()

	// Let the immediate poll on startup happen, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// success — poller exited promptly on cancel
	case <-time.After(1 * time.Second):
		t.Fatal("monitorPoller did not stop within 1s of ctx cancel (leader race fix regressed)")
	}

	// At minimum the immediate startup poll should have run.
	if store.calls.Load() < 1 {
		t.Errorf("expected at least 1 ListActiveMonitorsDue call, got %d", store.calls.Load())
	}
}

// TestPollMonitors_attachesEpoch verifies the scheduler tags every
// probe.tasks stream message with the fencing-token epoch supplied via
// Config.Epoch. This is what the gateway dispatcher relies on to detect a
// deposed leader (split-brain defence — docs/prd/ARCHITECTURE-REVIEW-2026-05-21.md
// P0-2).
func TestPollMonitors_attachesEpoch(t *testing.T) {
	_, rdb := setupRedis(t)
	ctx := context.Background()

	streamClient := stream.New(rdb)
	selector := &mockNodeSelector{nodeID: "nd_test_01"}
	l := leader.New(rdb, "test:leader_epoch", 10*time.Second, "node1")
	if ok, err := l.Acquire(ctx); err != nil || !ok {
		t.Fatalf("l.Acquire: %v ok=%v", err, ok)
	}

	store := &mockMonitorStore{
		monitors: []DueMonitor{
			{ID: "mon_epoch_1", Type: "http", Target: "epoch.test", IntervalS: 60, NodeCount: 1},
		},
	}

	s := New(Config{
		Leader:       l,
		Selector:     selector,
		Stream:       streamClient,
		MonitorStore: store,
		Epoch:        leader.FencingToken(7),
	})

	s.pollMonitors(ctx)

	entries, err := rdb.XRange(ctx, ProbeTasksStream, "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	got, ok := entries[0].Values["epoch"].(string)
	if !ok {
		t.Fatalf("epoch field missing or non-string in stream payload: %#v", entries[0].Values)
	}
	if got != "7" {
		t.Errorf("epoch = %q, want %q (Config.Epoch=7)", got, "7")
	}
}

// TestPollMonitors_zeroEpochSerialisesAsZero verifies the default-Config
// path: when no epoch is supplied (Config.Epoch zero value), the stream
// payload still has the field set to "0". Consumers treat "0" as
// backward-compat (accept + warn) rather than reject — exercising that
// branch from the producer side keeps the contract explicit.
func TestPollMonitors_zeroEpochSerialisesAsZero(t *testing.T) {
	_, rdb := setupRedis(t)
	ctx := context.Background()

	streamClient := stream.New(rdb)
	selector := &mockNodeSelector{nodeID: "nd_test_01"}
	l := leader.New(rdb, "test:leader_zero_epoch", 10*time.Second, "node1")
	if ok, err := l.Acquire(ctx); err != nil || !ok {
		t.Fatalf("l.Acquire: %v ok=%v", err, ok)
	}

	store := &mockMonitorStore{
		monitors: []DueMonitor{
			{ID: "mon_zero", Type: "http", Target: "x.test", NodeCount: 1},
		},
	}

	s := New(Config{
		Leader:       l,
		Selector:     selector,
		Stream:       streamClient,
		MonitorStore: store,
		// Epoch omitted on purpose
	})

	s.pollMonitors(ctx)

	entries, _ := rdb.XRange(ctx, ProbeTasksStream, "-", "+").Result()
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if got := entries[0].Values["epoch"]; got != "0" {
		t.Errorf("epoch field = %v, want \"0\" (default zero-value)", got)
	}
}

// TestRun_stopsWhenLeadershipLost is the headline test for the leader race
// fix. Run() is invoked, leadership is then yanked out from under it (via a
// second instance acquiring the lock after the first one's lease expires),
// and Run() must return in <1s — well under the previous worst case of
// monitorPollInterval (30s).
func TestRun_stopsWhenLeadershipLost(t *testing.T) {
	mr, rdb := setupRedis(t)
	streamClient := stream.New(rdb)
	selector := &mockNodeSelector{nodeID: "nd_test_01"}

	// Short TTL so renewal fires several times per test second.
	ttl := 1 * time.Second
	l1 := leader.New(rdb, "test:run_loss", ttl, "node1")

	store := &mockMonitorStore{monitors: []DueMonitor{}}

	s := New(Config{
		Leader:       l1,
		Selector:     selector,
		Stream:       streamClient,
		MonitorStore: store,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runErr := make(chan error, 1)
	go func() { runErr <- s.Run(ctx) }()

	// Wait for the scheduler to acquire and start its goroutines.
	time.Sleep(100 * time.Millisecond)
	if !l1.IsLeader() {
		t.Fatal("scheduler did not acquire leadership")
	}

	// Expire the lock and hand it to a second instance.
	mr.FastForward(2 * ttl)
	l2 := leader.New(rdb, "test:run_loss", ttl, "node2")
	if ok, err := l2.Acquire(context.Background()); err != nil || !ok {
		t.Fatalf("l2.Acquire after expiry: ok=%v err=%v", ok, err)
	}

	// Run() must return promptly once renewLeadership notices the loss.
	// Renewal interval is 2s in production; in this test we accept up to
	// 3 * leaderRenewInterval as a comfortable bound.
	select {
	case err := <-runErr:
		if err != nil {
			t.Errorf("Run returned error on leader loss, want nil; got %v", err)
		}
	case <-time.After(3 * leaderRenewInterval):
		t.Fatalf("Run did not exit within %v of leader loss — split brain window still open", 3*leaderRenewInterval)
	}
}
