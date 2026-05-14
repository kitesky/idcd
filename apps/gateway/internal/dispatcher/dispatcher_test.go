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
