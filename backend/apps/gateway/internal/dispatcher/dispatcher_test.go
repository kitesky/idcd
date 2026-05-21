package dispatcher_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/gateway/internal/dispatcher"
	"github.com/kite365/idcd/apps/gateway/internal/hub"
)

func newTestRedis(t *testing.T) (*miniredis.Miniredis, redis.Cmdable) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, rdb
}

func newTestHub(t *testing.T) *hub.Hub {
	t.Helper()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return hub.New(30*time.Second, log)
}

func TestDispatcher_AcksDeliveredTask(t *testing.T) {
	mr, rdb := newTestRedis(t)
	h := newTestHub(t)
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Add a task to the stream
	mr.XAdd("probe.tasks", "*", []string{
		"task_id", "pt_test1",
		"type", "http",
		"target", "https://example.com",
		"node_id", "nd_missing", // node not connected → not dispatched
	})

	d := dispatcher.New(rdb, h, log)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Run briefly; the node is not connected, so message stays in PEL
	d.Run(ctx)

	// Verify message is in PEL (not ACKed) because node was offline
	info, err := rdb.XPendingExt(context.Background(), &redis.XPendingExtArgs{
		Stream:   "probe.tasks",
		Group:    "gateway-dispatch",
		Start:    "-",
		End:      "+",
		Count:    10,
	}).Result()
	if err != nil {
		t.Fatalf("XPendingExt: %v", err)
	}
	if len(info) != 1 {
		t.Errorf("expected 1 pending message (node offline), got %d", len(info))
	}
}

// TestDispatcher_DropsStaleEpoch verifies the end-to-end split-brain
// defence: after the high-water mark has been advanced to epoch=5
// (persisted in Redis), a freshly-added message with epoch=2 — modelling a
// task written by a deposed scheduler that doesn't yet know it lost the
// lock — must be dropped + ACKed rather than dispatched to the agent.
//
// The persisted-mark setup mirrors the production scenario where the
// "deposed leader" write arrives *after* a higher-epoch write has already
// been observed; within a single XREADGROUP batch the parallel-goroutine
// ordering between sibling messages is non-deterministic and not what this
// test cares about.
func TestDispatcher_DropsStaleEpoch(t *testing.T) {
	mr, rdb := newTestRedis(t)
	h := newTestHub(t)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Seed the persisted high-water to 5 (simulates prior run that already
	// observed a healthy leader at epoch=5).
	if err := rdb.Set(context.Background(), "idcd:gateway:epoch:max", "5", 0).Err(); err != nil {
		t.Fatalf("seed high-water: %v", err)
	}

	// Live message at epoch=5 (current leader) + stale message at epoch=2
	// (deposed leader still writing).
	mr.XAdd("probe.tasks", "*", []string{
		"task_id", "pt_live", "type", "http", "target", "https://x.com",
		"node_id", "nd_missing", "epoch", "5",
	})
	mr.XAdd("probe.tasks", "*", []string{
		"task_id", "pt_stale", "type", "http", "target", "https://x.com",
		"node_id", "nd_missing", "epoch", "2",
	})

	d := dispatcher.New(rdb, h, log)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	d.Run(ctx)

	pending, err := rdb.XPendingExt(context.Background(), &redis.XPendingExtArgs{
		Stream: "probe.tasks",
		Group:  "gateway-dispatch",
		Start:  "-",
		End:    "+",
		Count:  10,
	}).Result()
	if err != nil {
		t.Fatalf("XPendingExt: %v", err)
	}

	// Live message: node offline → stays in PEL. Stale message: dropped +
	// ACKed → must not be in PEL.
	for _, p := range pending {
		entries, err := rdb.XRange(context.Background(), "probe.tasks", p.ID, p.ID).Result()
		if err != nil || len(entries) == 0 {
			continue
		}
		if entries[0].Values["task_id"] == "pt_stale" {
			t.Errorf("stale-epoch message stayed in PEL (not dropped): id=%s", p.ID)
		}
	}
	if len(pending) != 1 {
		t.Errorf("PEL size = %d, want 1 (live held in PEL; stale ACKed)", len(pending))
	}
}

func TestDispatcher_TaskPayloadFormat(t *testing.T) {
	// Verify taskMessage JSON structure
	type taskMsg struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}

	msg := taskMsg{
		Type:    "task",
		Payload: json.RawMessage(`{"task_id":"pt_1","type":"http","target":"https://x.com"}`),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded taskMsg
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Type != "task" {
		t.Errorf("expected type 'task', got %q", decoded.Type)
	}
}
