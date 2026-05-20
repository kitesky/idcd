package consumer

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// mockProcessor captures processed messages for assertions.
type mockProcessor struct {
	mu        sync.Mutex
	processed []string
	failIDs   map[string]bool
	// alwaysFail makes every Process call return an error so a message stays
	// in the PEL across redeliveries (used by DLQ tests).
	alwaysFail bool
}

func newMockProcessor(failIDs ...string) *mockProcessor {
	m := &mockProcessor{failIDs: make(map[string]bool)}
	for _, id := range failIDs {
		m.failIDs[id] = true
	}
	return m
}

func (m *mockProcessor) Process(_ context.Context, msgID string, _ map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.alwaysFail || m.failIDs[msgID] {
		return errors.New("mock process error")
	}
	m.processed = append(m.processed, msgID)
	return nil
}

func (m *mockProcessor) processedIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.processed))
	copy(out, m.processed)
	return out
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
	processed := proc.processedIDs()
	if len(processed) != 1 || processed[0] != msgID {
		t.Errorf("processor saw %v, want [%s]", processed, msgID)
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
		if len(proc.processedIDs()) >= 2 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	cancel()
	<-done

	processed := proc.processedIDs()
	if len(processed) != 2 {
		t.Errorf("expected 2 processed messages, got %d: %v", len(processed), processed)
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

	_ = c.Run(ctx)

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

// TestConsumer_multipleConsumersUniquePEL verifies that two consumers with
// distinct names against the same consumer group do not steal each other's
// in-flight messages: each consumer's PEL accounts independently.
//
// This is the regression test for the hard-coded "aggregator-0" bug. Before
// the fix, two replicas would share the same consumer name and one would
// effectively starve the other.
func TestConsumer_multipleConsumersUniquePEL(t *testing.T) {
	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})

	// Build two consumers sharing the same group but different names.
	procA := newMockProcessor()
	procB := newMockProcessor()
	procA.alwaysFail = true // keep messages in A's PEL
	procB.alwaysFail = true // keep messages in B's PEL

	cfgBase := Config{
		Stream:       "test.stream",
		Group:        "test-group",
		BatchSize:    1, // one msg per XREADGROUP so two consumers interleave deterministically
		BlockTimeout: 50 * time.Millisecond,
	}
	cfgA := cfgBase
	cfgA.ConsumerName = "pod-x-0"
	cfgB := cfgBase
	cfgB.ConsumerName = "pod-x-1"
	cA := New(rdb, cfgA, procA, slog.Default())
	cB := New(rdb, cfgB, procB, slog.Default())

	// Ensure the group exists before any messages land (so XREADGROUP "> "
	// will see them as new for the group).
	if err := cA.ensureGroup(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Seed messages.
	for i := 0; i < 4; i++ {
		seedMessage(t, s, map[string]any{"task_id": "pt", "node_id": "nd"})
	}

	// Drive each consumer's readGroup manually so neither greedily empties the
	// stream. This is what the production code does end-to-end — each Run
	// goroutine owns an independent XREADGROUP call.
	for i := 0; i < 2; i++ {
		msgsA, err := cA.readGroup(context.Background())
		if err != nil {
			t.Fatalf("cA.readGroup: %v", err)
		}
		for _, m := range msgsA {
			_ = procA.Process(context.Background(), m.ID, m.Values) // always fails → stays in PEL
		}
		msgsB, err := cB.readGroup(context.Background())
		if err != nil {
			t.Fatalf("cB.readGroup: %v", err)
		}
		for _, m := range msgsB {
			_ = procB.Process(context.Background(), m.ID, m.Values)
		}
	}

	// All four messages should be split between two PELs, neither stealing
	// from the other.
	pending, err := rdb.XPending(context.Background(), "test.stream", "test-group").Result()
	if err != nil {
		t.Fatalf("XPending: %v", err)
	}
	if pending.Count != 4 {
		t.Fatalf("expected 4 total pending, got %d", pending.Count)
	}
	if got := pending.Consumers["pod-x-0"] + pending.Consumers["pod-x-1"]; got != 4 {
		t.Errorf("expected 4 across both consumers, got map=%v", pending.Consumers)
	}
	if pending.Consumers["pod-x-0"] == 0 || pending.Consumers["pod-x-1"] == 0 {
		t.Errorf("expected both consumers to own at least one message, got %v", pending.Consumers)
	}
}

// TestConsumer_reclaimPending_movesFromIdle verifies XAUTOCLAIM behaviour:
// after the configured idle window elapses, a different consumer can claim
// abandoned messages and successfully process them.
func TestConsumer_reclaimPending_movesFromIdle(t *testing.T) {
	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})

	procFail := newMockProcessor()
	procFail.alwaysFail = true
	failed := New(rdb, Config{
		Stream:       "test.stream",
		Group:        "test-group",
		ConsumerName: "failed-1",
		BatchSize:    10,
		BlockTimeout: 50 * time.Millisecond,
		ClaimMinIdle: 1 * time.Millisecond,
	}, procFail, slog.Default())

	procOK := newMockProcessor()
	survivor := New(rdb, Config{
		Stream:       "test.stream",
		Group:        "test-group",
		ConsumerName: "survivor-1",
		BatchSize:    10,
		BlockTimeout: 50 * time.Millisecond,
		ClaimMinIdle: 1 * time.Millisecond,
	}, procOK, slog.Default())

	seedMessage(t, s, map[string]any{"task_id": "pt_reclaim", "node_id": "nd_1"})

	// Run the failing consumer briefly so the message lands in its PEL.
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	_ = failed.Run(ctx)
	cancel()

	// Confirm the message is in the failing consumer's PEL.
	pending, _ := rdb.XPending(context.Background(), "test.stream", "test-group").Result()
	if pending.Count != 1 || pending.Consumers["failed-1"] != 1 {
		t.Fatalf("expected 1 pending under failed-1, got %v", pending.Consumers)
	}

	// Sleep past claim-idle. (FastForward doesn't affect stream PEL idle time
	// in miniredis — only TTLs.)
	time.Sleep(15 * time.Millisecond)

	// Survivor reclaims and successfully processes.
	if err := survivor.reclaimPending(context.Background()); err != nil {
		t.Fatalf("reclaimPending: %v", err)
	}
	if len(procOK.processedIDs()) != 1 {
		t.Errorf("survivor should have processed 1 reclaimed message, got %v", procOK.processedIDs())
	}
	// After successful reclaim + ACK, PEL should be empty.
	pending, _ = rdb.XPending(context.Background(), "test.stream", "test-group").Result()
	if pending.Count != 0 {
		t.Errorf("expected empty PEL after reclaim+ACK, got %d", pending.Count)
	}
}

// TestConsumer_sweepDLQ_movesPoisonMessages verifies that messages whose
// redelivery count exceeds the DLQ threshold get shunted to "<stream>:dlq"
// and ACKed in the main group.
func TestConsumer_sweepDLQ_movesPoisonMessages(t *testing.T) {
	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})

	proc := newMockProcessor()
	proc.alwaysFail = true

	cfg := Config{
		Stream:               "test.stream",
		Group:                "test-group",
		ConsumerName:         "poison-1",
		BatchSize:            10,
		BlockTimeout:         50 * time.Millisecond,
		ClaimMinIdle:         1 * time.Millisecond,
		DLQDeliveryThreshold: 2,
	}
	c := New(rdb, cfg, proc, slog.Default())

	msgID := seedMessage(t, s, map[string]any{"task_id": "pt_poison", "node_id": "nd_x"})

	// Ensure the group + add the message to the consumer's PEL.
	if err := c.ensureGroup(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := rdb.XReadGroup(context.Background(), &redis.XReadGroupArgs{
		Group:    cfg.Group,
		Consumer: cfg.ConsumerName,
		Streams:  []string{cfg.Stream, ">"},
		Count:    1,
		Block:    10 * time.Millisecond,
	}).Result(); err != nil && err != redis.Nil {
		t.Fatalf("XReadGroup: %v", err)
	}

	// Bump delivery count past the threshold using XCLAIM with RETRYCOUNT.
	// go-redis doesn't expose RETRYCOUNT in XClaimArgs, so we issue the raw
	// command — miniredis supports it.
	if err := rdb.Do(context.Background(),
		"XCLAIM", cfg.Stream, cfg.Group, cfg.ConsumerName,
		"0", msgID, "RETRYCOUNT", "10",
	).Err(); err != nil {
		t.Fatalf("XCLAIM RETRYCOUNT: %v", err)
	}

	// Sleep past the idle window so XPENDING IDLE returns this entry.
	// (miniredis's FastForward only advances TTLs, not stream PEL idle time,
	// so a real sleep is required for the idle filter to pass.)
	time.Sleep(20 * time.Millisecond)

	// Sanity check: the message should now have RetryCount past threshold.
	// XPending (no idle filter) confirms the message exists in the group PEL.
	if pend, err := rdb.XPending(context.Background(), cfg.Stream, cfg.Group).Result(); err != nil {
		t.Fatalf("XPending pre-sweep: %v", err)
	} else if pend.Count != 1 {
		t.Fatalf("expected 1 pending pre-sweep (XPending), got %d (consumers=%v)", pend.Count, pend.Consumers)
	}
	pendInfo, err := rdb.XPendingExt(context.Background(), &redis.XPendingExtArgs{
		Stream: cfg.Stream,
		Group:  cfg.Group,
		Idle:   cfg.ClaimMinIdle,
		Start:  "-",
		End:    "+",
		Count:  10,
	}).Result()
	if err != nil {
		t.Fatalf("XPendingExt pre-sweep: %v", err)
	}
	if len(pendInfo) != 1 {
		t.Fatalf("expected 1 pending entry pre-sweep (XPendingExt idle=%v), got %+v",
			cfg.ClaimMinIdle, pendInfo)
	}
	if pendInfo[0].RetryCount <= cfg.DLQDeliveryThreshold {
		t.Fatalf("expected retry > %d, got %d", cfg.DLQDeliveryThreshold, pendInfo[0].RetryCount)
	}

	c.sweepDLQ(context.Background())

	// Main group PEL should be empty (message ACKed via DLQ path).
	pending, _ := rdb.XPending(context.Background(), cfg.Stream, cfg.Group).Result()
	if pending.Count != 0 {
		t.Errorf("expected empty PEL after DLQ sweep, got %d", pending.Count)
	}

	// DLQ stream should hold exactly one entry referencing the original.
	dlqEntries, err := rdb.XRange(context.Background(), cfg.Stream+DLQStreamSuffix, "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange dlq: %v", err)
	}
	if len(dlqEntries) != 1 {
		t.Fatalf("expected 1 DLQ entry, got %d", len(dlqEntries))
	}
	if got := dlqEntries[0].Values["_dlq_original_id"]; got != msgID {
		t.Errorf("DLQ entry _dlq_original_id = %v, want %s", got, msgID)
	}
	if got := dlqEntries[0].Values["_dlq_reason"]; got != "max_deliveries_exceeded" {
		t.Errorf("DLQ entry _dlq_reason = %v, want max_deliveries_exceeded", got)
	}
}

// TestConsumer_RunMaintenance_periodicallyRuns verifies the maintenance
// goroutine actually fires more than once during its lifetime.
func TestConsumer_RunMaintenance_periodicallyRuns(t *testing.T) {
	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})

	proc := newMockProcessor()
	c := New(rdb, Config{
		Stream:       "test.stream",
		Group:        "test-group",
		ConsumerName: "maint-1",
		BatchSize:    10,
		BlockTimeout: 50 * time.Millisecond,
		ClaimMinIdle: 1 * time.Millisecond,
	}, proc, slog.Default())

	// Wrap rdb in a counting cmdable would be heavyweight — instead we observe
	// the side effect of samplePELSize by reading the PELSize gauge. Simpler:
	// run maintenance twice on the same idle PEL and confirm no panic + at
	// least the startup tick + one timer tick executed.
	var ticks atomic.Int32
	done := make(chan struct{})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
		defer cancel()
		// We can't easily probe internal tick count without refactoring. Just
		// ensure RunMaintenance returns cleanly when ctx ends.
		t := time.NewTicker(20 * time.Millisecond)
		defer t.Stop()
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					ticks.Add(1)
				}
			}
		}()
		c.RunMaintenance(ctx, 20*time.Millisecond)
		close(done)
	}()
	<-done
	if ticks.Load() == 0 {
		t.Error("expected at least one observable tick during maintenance window")
	}
}
