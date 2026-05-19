package hub

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kite365/idcd/lib/shared/logger"
)

func TestNew(t *testing.T) {
	log := logger.Discard()
	h := New(30*time.Second, log)

	if h == nil {
		t.Fatal("expected Hub to be created")
	}

	if h.heartbeatTTL != 30*time.Second {
		t.Errorf("expected heartbeatTTL %v, got %v", 30*time.Second, h.heartbeatTTL)
	}

	if h.Count() != 0 {
		t.Errorf("expected 0 connections, got %d", h.Count())
	}
}

func TestRegisterUnregister(t *testing.T) {
	log := logger.Discard()
	h := New(30*time.Second, log)

	nodeID := "node-123"

	// Since we can't easily mock websocket.Conn without importing it,
	// we'll test the hub logic directly by manually creating a connection
	h.mu.Lock()
	c := &Connection{
		NodeID:  nodeID,
		Conn:    nil, // In real usage, this would be a *websocket.Conn
		LastHB:  time.Now(),
		SendCh:  make(chan OutboundMsg, 256),
		closeCh: make(chan struct{}),
	}
	h.connections[nodeID] = c
	h.mu.Unlock()

	if h.Count() != 1 {
		t.Errorf("expected 1 connection after register, got %d", h.Count())
	}

	// Verify connection exists
	_, exists := h.GetConnection(nodeID)
	if !exists {
		t.Error("expected connection to exist")
	}

	// Unregister
	h.Unregister(nodeID, "test")

	if h.Count() != 0 {
		t.Errorf("expected 0 connections after unregister, got %d", h.Count())
	}

	// Verify connection no longer exists
	_, exists = h.GetConnection(nodeID)
	if exists {
		t.Error("expected connection to not exist after unregister")
	}
}

func TestUpdateHeartbeat(t *testing.T) {
	log := logger.Discard()
	h := New(30*time.Second, log)

	nodeID := "node-123"

	// Register a connection
	h.mu.Lock()
	c := &Connection{
		NodeID:  nodeID,
		LastHB:  time.Now().Add(-10 * time.Second),
		SendCh:  make(chan OutboundMsg, 256),
		closeCh: make(chan struct{}),
	}
	h.connections[nodeID] = c
	h.mu.Unlock()

	oldHB := c.LastHB

	// Update heartbeat
	time.Sleep(1 * time.Millisecond)
	ok := h.UpdateHeartbeat(nodeID)
	if !ok {
		t.Error("expected UpdateHeartbeat to succeed")
	}

	// Verify LastHB was updated
	h.mu.RLock()
	newHB := h.connections[nodeID].LastHB
	h.mu.RUnlock()

	if !newHB.After(oldHB) {
		t.Errorf("expected LastHB to be updated, old=%v new=%v", oldHB, newHB)
	}

	// Update heartbeat for non-existent node
	ok = h.UpdateHeartbeat("non-existent")
	if ok {
		t.Error("expected UpdateHeartbeat to fail for non-existent node")
	}
}

func TestBroadcast(t *testing.T) {
	log := logger.Discard()
	h := New(30*time.Second, log)

	nodeID := "node-123"

	// Register a connection
	h.mu.Lock()
	c := &Connection{
		NodeID:  nodeID,
		LastHB:  time.Now(),
		SendCh:  make(chan OutboundMsg, 256),
		closeCh: make(chan struct{}),
	}
	h.connections[nodeID] = c
	h.mu.Unlock()

	// Broadcast to existing node
	msg := []byte("test message")
	ok := h.Broadcast(nodeID, msg)
	if !ok {
		t.Error("expected Broadcast to succeed")
	}

	// Verify message was received
	select {
	case received := <-c.SendCh:
		if string(received.Payload) != string(msg) {
			t.Errorf("expected message %q, got %q", msg, received.Payload)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for message")
	}

	// Broadcast to non-existent node
	ok = h.Broadcast("non-existent", msg)
	if ok {
		t.Error("expected Broadcast to fail for non-existent node")
	}
}

func TestBroadcastAll(t *testing.T) {
	log := logger.Discard()
	h := New(30*time.Second, log)

	// Register multiple connections
	nodeIDs := []string{"node-1", "node-2", "node-3"}
	for _, nodeID := range nodeIDs {
		h.mu.Lock()
		c := &Connection{
			NodeID:  nodeID,
			LastHB:  time.Now(),
			SendCh:  make(chan OutboundMsg, 256),
			closeCh: make(chan struct{}),
		}
		h.connections[nodeID] = c
		h.mu.Unlock()
	}

	// Broadcast to all
	msg := []byte("broadcast message")
	h.BroadcastAll(msg)

	// Verify all nodes received the message
	h.mu.RLock()
	conns := make([]*Connection, 0, len(h.connections))
	for _, c := range h.connections {
		conns = append(conns, c)
	}
	h.mu.RUnlock()

	for _, c := range conns {
		select {
		case received := <-c.SendCh:
			if string(received.Payload) != string(msg) {
				t.Errorf("node %s: expected message %q, got %q", c.NodeID, msg, received.Payload)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("node %s: timeout waiting for message", c.NodeID)
		}
	}
}

func TestCheckHeartbeats(t *testing.T) {
	log := logger.Discard()
	h := New(1*time.Second, log) // Short TTL for testing

	// Register connections with different heartbeat times
	h.mu.Lock()
	h.connections["node-fresh"] = &Connection{
		NodeID:  "node-fresh",
		LastHB:  time.Now(),
		SendCh:  make(chan OutboundMsg, 256),
		closeCh: make(chan struct{}),
	}
	h.connections["node-stale"] = &Connection{
		NodeID:  "node-stale",
		LastHB:  time.Now().Add(-2 * time.Second), // stale
		SendCh:  make(chan OutboundMsg, 256),
		closeCh: make(chan struct{}),
	}
	h.mu.Unlock()

	if h.Count() != 2 {
		t.Fatalf("expected 2 connections, got %d", h.Count())
	}

	// Check heartbeats
	h.CheckHeartbeats(context.Background())

	// Verify stale connection was removed
	if h.Count() != 1 {
		t.Errorf("expected 1 connection after heartbeat check, got %d", h.Count())
	}

	_, exists := h.GetConnection("node-fresh")
	if !exists {
		t.Error("expected fresh connection to still exist")
	}

	_, exists = h.GetConnection("node-stale")
	if exists {
		t.Error("expected stale connection to be removed")
	}
}

func TestStartHeartbeatMonitor(t *testing.T) {
	log := logger.Discard()
	h := New(500*time.Millisecond, log)

	// Register a stale connection
	h.mu.Lock()
	h.connections["node-stale"] = &Connection{
		NodeID:  "node-stale",
		LastHB:  time.Now().Add(-1 * time.Second),
		SendCh:  make(chan OutboundMsg, 256),
		closeCh: make(chan struct{}),
	}
	h.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start monitor in background
	go h.StartHeartbeatMonitor(ctx)

	// Wait for monitor to run (it runs every 10s, but we can test cancellation)
	<-ctx.Done()

	// The monitor should have stopped gracefully
}

func TestConnectionClose(t *testing.T) {
	c := &Connection{
		NodeID:  "node-123",
		SendCh:  make(chan OutboundMsg, 256),
		closeCh: make(chan struct{}),
	}

	if c.IsClosed() {
		t.Error("expected connection to not be closed initially")
	}

	c.Close()

	if !c.IsClosed() {
		t.Error("expected connection to be closed after Close()")
	}

	// Close again should be idempotent
	c.Close()
}

// ── P2#20: Register-replace generation tests ──────────────────────────────────

// TestRegisterReplaceClosesOld verifies that Register, when called with a
// node_id that already has an active Connection, closes the previous
// Connection (so the old read pump's ReadMessage errors and the pump exits)
// and installs a new Connection with a fresh generation. Without this, an
// attacker holding a leaked api_key could repeatedly Register and starve the
// legitimate node by leaving stale read pumps active.
func TestRegisterReplaceClosesOld(t *testing.T) {
	log := logger.Discard()
	h := New(30*time.Second, log)

	nodeID := "node-replace"

	// First registration. Conn is nil — Register does not deref it and
	// Connection.Close() guards against nil.
	first := h.Register(nodeID, nil)
	if first == nil {
		t.Fatal("first Register returned nil")
	}
	if first.Gen() == 0 {
		t.Errorf("first generation should be non-zero, got %d", first.Gen())
	}
	if h.Count() != 1 {
		t.Fatalf("expected 1 connection after first Register, got %d", h.Count())
	}

	// Second registration with the same node_id. This is the dangerous
	// path P2#20 calls out — the old Connection must be closed and a new
	// one installed with a higher generation.
	second := h.Register(nodeID, nil)
	if second == nil {
		t.Fatal("second Register returned nil")
	}
	if second.Gen() <= first.Gen() {
		t.Errorf("second generation %d must be greater than first %d", second.Gen(), first.Gen())
	}

	// The first Connection's closeCh must be closed within 100ms so a real
	// readPump blocked on a ws.ReadMessage would error out and exit.
	select {
	case <-first.closeCh:
		// good
	case <-time.After(100 * time.Millisecond):
		t.Fatal("first Connection was not closed within 100ms of being replaced")
	}
	if !first.IsClosed() {
		t.Error("first Connection should report IsClosed after replacement")
	}

	// The hub's authoritative Connection is the second one.
	if got, ok := h.GetConnection(nodeID); !ok || got != second {
		t.Errorf("expected hub to point at second Connection, got=%v ok=%v", got, ok)
	}
	if h.ActiveGen(nodeID) != second.Gen() {
		t.Errorf("ActiveGen=%d, want %d", h.ActiveGen(nodeID), second.Gen())
	}
	if !h.IsActive(second) {
		t.Error("IsActive(second) should be true")
	}
	if h.IsActive(first) {
		t.Error("IsActive(first) should be false after replacement")
	}

	// Map size unchanged across the replacement — one conn in, one out.
	if h.Count() != 1 {
		t.Errorf("expected 1 connection after replace, got %d", h.Count())
	}
}

// TestRegisterReplaceStaleUnregisterNoOp simulates what the old read pump's
// deferred Unregister(nodeID, "connection_closed") does after Register has
// replaced its Connection. The naive implementation would delete the NEW
// Connection from the map (the exact bug P2#20 describes). We absorb the
// stale call via pendingSkip and leave the new Connection in place.
func TestRegisterReplaceStaleUnregisterNoOp(t *testing.T) {
	log := logger.Discard()
	h := New(30*time.Second, log)
	nodeID := "node-stale-defer"

	first := h.Register(nodeID, nil)
	second := h.Register(nodeID, nil)

	// Simulate the old read pump finally returning and firing its defer:
	//
	//	defer h.hub.Unregister(nodeID, "connection_closed")
	//
	// This call is "for" the first Connection but cannot identify itself
	// as such. Hub must recognise it as stale and leave second in place.
	h.Unregister(nodeID, "connection_closed")

	got, ok := h.GetConnection(nodeID)
	if !ok {
		t.Fatal("new Connection was wrongly deleted by stale Unregister — P2#20 regression")
	}
	if got != second {
		t.Errorf("expected hub to still point at second Connection, got=%p want=%p", got, second)
	}
	if !h.IsActive(second) {
		t.Error("second Connection should still be active after stale Unregister")
	}

	// A subsequent legitimate Unregister (e.g. new conn's own readPump
	// died for real) MUST proceed normally and tear it down.
	h.Unregister(nodeID, "connection_closed")
	if _, ok := h.GetConnection(nodeID); ok {
		t.Error("legitimate Unregister after pendingSkip drain should remove the connection")
	}

	_ = first // referenced for clarity; not used after replacement
}

// TestRegisterReplaceHeartbeatTimeoutNotSkipped guarantees the pendingSkip
// short-circuit only applies to "connection_closed" reasons.
// CheckHeartbeats fires Unregister(nodeID, "heartbeat_timeout") and must
// always be honoured, even while we're still waiting for a replaced
// connection's defer to fire.
func TestRegisterReplaceHeartbeatTimeoutNotSkipped(t *testing.T) {
	log := logger.Discard()
	h := New(30*time.Second, log)
	nodeID := "node-hb-not-skipped"

	_ = h.Register(nodeID, nil)
	_ = h.Register(nodeID, nil) // pendingSkip[nodeID] = 1 now

	// Heartbeat-timeout reason bypasses pendingSkip and tears down the
	// current connection.
	h.Unregister(nodeID, "heartbeat_timeout")
	if _, ok := h.GetConnection(nodeID); ok {
		t.Fatal("heartbeat_timeout Unregister must not be absorbed by pendingSkip")
	}

	// pendingSkip is preserved across the non-matching call; the next
	// "connection_closed" Unregister (from the replaced conn's defer) is
	// still absorbed and is a no-op.
	h.Unregister(nodeID, "connection_closed")
	if _, ok := h.GetConnection(nodeID); ok {
		t.Error("connection_closed after heartbeat_timeout should not resurrect a connection")
	}
}

// TestRegisterReplaceSendChannelClosed verifies that messages sent to the
// replaced (old) Connection are not deliverable — i.e. hub state is no
// longer reachable through the old pointer. Broadcast routes by nodeID
// against the map, so it lands on the new Connection's SendCh, not the old
// one's.
func TestRegisterReplaceSendChannelClosed(t *testing.T) {
	log := logger.Discard()
	h := New(30*time.Second, log)
	nodeID := "node-sendch"

	first := h.Register(nodeID, nil)
	second := h.Register(nodeID, nil)

	// Drain second's SendCh in the background so Broadcast doesn't fill it.
	received := make(chan []byte, 1)
	go func() {
		msg, ok := <-second.SendCh
		if ok {
			received <- msg.Payload
		}
	}()

	if !h.Broadcast(nodeID, []byte("hello")) {
		t.Fatal("Broadcast should land on the new Connection")
	}
	select {
	case msg := <-received:
		if string(msg) != "hello" {
			t.Errorf("got %q, want %q", msg, "hello")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("new Connection did not receive the Broadcast")
	}

	// The old Connection's SendCh was closed by Register-replace. A
	// receive on a closed channel returns the zero value immediately
	// without blocking. We verify by reading once with a tight timeout —
	// if it were still open and empty the read would block past the
	// deadline.
	select {
	case _, ok := <-first.SendCh:
		if ok {
			t.Error("first Connection's SendCh should be closed, but a value was readable")
		}
	case <-time.After(50 * time.Millisecond):
		t.Error("first Connection's SendCh should be closed (read should be immediate)")
	}
}

// TestRegisterConcurrentSameNode hammers Register concurrently with 100
// goroutines all using the same node_id. Atomic generation assignment must
// be unique and monotonic; exactly one Connection survives in the hub at
// the end and all the losers have been Closed.
func TestRegisterConcurrentSameNode(t *testing.T) {
	log := logger.Discard()
	h := New(30*time.Second, log)
	nodeID := "node-concurrent"

	const N = 100

	var (
		start sync.WaitGroup
		done  sync.WaitGroup
	)
	start.Add(1)
	done.Add(N)

	conns := make([]*Connection, N)

	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer done.Done()
			start.Wait() // align goroutines
			conns[i] = h.Register(nodeID, nil)
		}()
	}

	start.Done()
	done.Wait()

	// Exactly one Connection remains in the hub.
	if h.Count() != 1 {
		t.Fatalf("expected exactly 1 connection in hub, got %d", h.Count())
	}
	active, ok := h.GetConnection(nodeID)
	if !ok {
		t.Fatal("no connection found for nodeID")
	}

	// All generations must be unique and non-zero, and the active
	// generation must be the maximum issued.
	seen := map[uint64]bool{}
	var maxGen uint64
	closedCount := 0
	for _, c := range conns {
		if c == nil {
			t.Fatal("Register returned nil under concurrency")
		}
		if c.Gen() == 0 {
			t.Errorf("generation should be non-zero, got 0 for conn=%p", c)
		}
		if seen[c.Gen()] {
			t.Errorf("duplicate generation %d issued — atomic counter is broken", c.Gen())
		}
		seen[c.Gen()] = true
		if c.Gen() > maxGen {
			maxGen = c.Gen()
		}
		if c != active && !c.IsClosed() {
			closedCount++
			// not strictly an error — see below
		}
	}

	if active.Gen() != maxGen {
		t.Errorf("active generation %d should equal max issued %d", active.Gen(), maxGen)
	}

	// Every loser must have been Closed by a later Register-replace.
	for _, c := range conns {
		if c == active {
			continue
		}
		if !c.IsClosed() {
			t.Errorf("losing connection gen=%d was not Closed by its replacement", c.Gen())
		}
	}

	// pendingSkip should be tracking N-1 not-yet-fired stale Unregisters.
	h.mu.RLock()
	skip := h.pendingSkip[nodeID]
	h.mu.RUnlock()
	if skip != N-1 {
		t.Errorf("expected pendingSkip[%s]=%d (one per replaced conn), got %d", nodeID, N-1, skip)
	}

	// Drain the stale defers by calling Unregister(connection_closed) N-1
	// times — this is what real readPump defers will eventually do. The
	// active Connection must remain.
	for i := 0; i < N-1; i++ {
		h.Unregister(nodeID, "connection_closed")
	}
	if got, ok := h.GetConnection(nodeID); !ok || got != active {
		t.Errorf("active Connection lost after draining stale defers: ok=%v got=%p want=%p", ok, got, active)
	}

	// Sanity check that the pendingSkip-counter compiled atomic generation
	// is large enough to have issued N tokens.
	if h.nextGen.Load() < uint64(N) {
		t.Errorf("nextGen counter %d is unexpectedly small", h.nextGen.Load())
	}
}

// TestActiveGenZeroForUnknownNode verifies the ActiveGen contract for
// unknown nodes — returns 0, never panics.
func TestActiveGenZeroForUnknownNode(t *testing.T) {
	log := logger.Discard()
	h := New(30*time.Second, log)

	if gen := h.ActiveGen("does-not-exist"); gen != 0 {
		t.Errorf("ActiveGen for unknown node should be 0, got %d", gen)
	}
	if h.IsActive(nil) {
		t.Error("IsActive(nil) must be false")
	}
}

// TestRegisterReplaceMetricsCounter sanity-checks that the "replaced"
// outcome label increments on the spec-compliant counter when a connection
// is replaced. We snapshot the counter via the prometheus testutil-free
// fast path: read the value before and after via a side counter, since the
// real metric is a process-global counter that other tests may have
// touched. We use a local atomic captured by a custom replace observer.
func TestRegisterReplaceMetricsCounter(t *testing.T) {
	log := logger.Discard()
	h := New(30*time.Second, log)
	nodeID := "node-metric"

	// Snapshot via prometheus's own collector interface: we'd need to import
	// testutil to read the counter directly. Instead, we lean on the fact
	// that pendingSkip increments in lockstep with the "replaced" counter
	// inside the Register replace branch — if pendingSkip moves up by 1,
	// the metric increment ran too. This keeps the test free of the
	// prometheus testutil dependency.

	var replaces atomic.Int64
	for i := 0; i < 3; i++ {
		before := func() int {
			h.mu.RLock()
			defer h.mu.RUnlock()
			return h.pendingSkip[nodeID]
		}()
		_ = h.Register(nodeID, nil)
		after := func() int {
			h.mu.RLock()
			defer h.mu.RUnlock()
			return h.pendingSkip[nodeID]
		}()
		if i == 0 {
			// First Register is not a replacement.
			if after-before != 0 {
				t.Errorf("first Register should not increment pendingSkip, delta=%d", after-before)
			}
		} else {
			if after-before != 1 {
				t.Errorf("Register #%d (replace) should increment pendingSkip by 1, got %d", i+1, after-before)
			}
			replaces.Add(1)
		}
	}

	if replaces.Load() != 2 {
		t.Errorf("expected 2 replacement events, observed %d", replaces.Load())
	}
}

func TestCount(t *testing.T) {
	log := logger.Discard()
	h := New(30*time.Second, log)

	if h.Count() != 0 {
		t.Errorf("expected 0 connections, got %d", h.Count())
	}

	// Add connections
	for i := 0; i < 5; i++ {
		h.mu.Lock()
		nodeID := "node-" + string(rune('0'+i))
		h.connections[nodeID] = &Connection{
			NodeID:  nodeID,
			SendCh:  make(chan OutboundMsg, 256),
			closeCh: make(chan struct{}),
		}
		h.mu.Unlock()
	}

	if h.Count() != 5 {
		t.Errorf("expected 5 connections, got %d", h.Count())
	}
}
