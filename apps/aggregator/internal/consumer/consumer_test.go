package consumer

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// mockProcessor captures processed messages for assertions.
type mockProcessor struct {
	processed []string
	failIDs   map[string]bool
}

func newMockProcessor(failIDs ...string) *mockProcessor {
	m := &mockProcessor{failIDs: make(map[string]bool)}
	for _, id := range failIDs {
		m.failIDs[id] = true
	}
	return m
}

func (m *mockProcessor) Process(_ context.Context, msgID string, _ map[string]any) error {
	if m.failIDs[msgID] {
		return errors.New("mock process error")
	}
	m.processed = append(m.processed, msgID)
	return nil
}

func newTestConsumer(t *testing.T, s *miniredis.Miniredis, proc Processor) *Consumer {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	return New(rdb, Config{
		Stream:       "test.stream",
		Group:        "test-group",
		ConsumerName: "test-consumer",
		BatchSize:    10,
		BlockTimeout: 100 * time.Millisecond,
	}, proc, slog.Default())
}

func seedMessage(t *testing.T, s *miniredis.Miniredis, values map[string]any) string {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	id, err := rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "test.stream",
		ID:     "*",
		Values: values,
	}).Result()
	if err != nil {
		t.Fatalf("seed message: %v", err)
	}
	return id
}

func TestConsumer_ensureGroup(t *testing.T) {
	s := miniredis.RunT(t)
	proc := newMockProcessor()
	c := newTestConsumer(t, s, proc)

	ctx := context.Background()

	// First call should succeed (create group).
	if err := c.ensureGroup(ctx); err != nil {
		t.Fatalf("first ensureGroup: %v", err)
	}

	// Second call should be idempotent (BUSYGROUP).
	if err := c.ensureGroup(ctx); err != nil {
		t.Fatalf("second ensureGroup: %v", err)
	}
}

func TestConsumer_readGroup_noMessages(t *testing.T) {
	s := miniredis.RunT(t)
	proc := newMockProcessor()
	c := newTestConsumer(t, s, proc)

	ctx := context.Background()
	if err := c.ensureGroup(ctx); err != nil {
		t.Fatal(err)
	}

	msgs, err := c.readGroup(ctx)
	if err != nil {
		t.Fatalf("readGroup: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

func TestConsumer_processAndAck(t *testing.T) {
	s := miniredis.RunT(t)
	proc := newMockProcessor()
	c := newTestConsumer(t, s, proc)

	ctx := context.Background()
	if err := c.ensureGroup(ctx); err != nil {
		t.Fatal(err)
	}

	// Seed a message.
	msgID := seedMessage(t, s, map[string]any{"task_id": "pt_001", "node_id": "nd_jp_01"})

	msgs, err := c.readGroup(ctx)
	if err != nil {
		t.Fatalf("readGroup: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	if err := c.processor.Process(ctx, msgs[0].ID, msgs[0].Values); err != nil {
		t.Fatalf("process: %v", err)
	}
	if err := c.ack(ctx, msgs[0].ID); err != nil {
		t.Fatalf("ack: %v", err)
	}

	// After ACK, PEL should be empty.
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	pending, err := rdb.XPending(ctx, c.stream, c.group).Result()
	if err != nil {
		t.Fatalf("XPending: %v", err)
	}
	if pending.Count != 0 {
		t.Errorf("expected 0 pending after ACK, got %d", pending.Count)
	}

	// Processor should have seen the message.
	if len(proc.processed) != 1 || proc.processed[0] != msgID {
		t.Errorf("processor saw %v, want [%s]", proc.processed, msgID)
	}
}

func TestConsumer_run_processesMessages(t *testing.T) {
	s := miniredis.RunT(t)
	proc := newMockProcessor()
	c := newTestConsumer(t, s, proc)

	// Seed messages before starting consumer.
	seedMessage(t, s, map[string]any{"task_id": "pt_001", "node_id": "nd_jp_01"})
	seedMessage(t, s, map[string]any{"task_id": "pt_002", "node_id": "nd_sg_01"})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- c.Run(ctx)
	}()

	// Wait for messages to be processed.
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(proc.processed) >= 2 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	cancel()
	<-done

	if len(proc.processed) != 2 {
		t.Errorf("expected 2 processed messages, got %d: %v", len(proc.processed), proc.processed)
	}
}

func TestConsumer_run_noACKOnProcessFailure(t *testing.T) {
	s := miniredis.RunT(t)
	proc := newMockProcessor() // will fail for dynamic ID

	// Seed a message first to get its ID.
	msgID := seedMessage(t, s, map[string]any{"task_id": "pt_fail", "node_id": "nd_jp_01"})
	proc.failIDs[msgID] = true

	c := newTestConsumer(t, s, proc)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	c.Run(ctx)

	// Message should still be pending (not ACKed). Use background context since
	// the run context is already cancelled.
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	pending, err := rdb.XPending(context.Background(), c.stream, c.group).Result()
	if err != nil {
		t.Fatalf("XPending: %v", err)
	}
	// Message remains in PEL because process failed.
	if pending.Count != 1 {
		t.Errorf("expected 1 pending (unACKed after failure), got %d", pending.Count)
	}
}
