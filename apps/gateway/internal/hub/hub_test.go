package hub

import (
	"context"
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
		SendCh:  make(chan []byte, 256),
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
		SendCh:  make(chan []byte, 256),
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
		SendCh:  make(chan []byte, 256),
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
		if string(received) != string(msg) {
			t.Errorf("expected message %q, got %q", msg, received)
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
			SendCh:  make(chan []byte, 256),
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
			if string(received) != string(msg) {
				t.Errorf("node %s: expected message %q, got %q", c.NodeID, msg, received)
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
		SendCh:  make(chan []byte, 256),
		closeCh: make(chan struct{}),
	}
	h.connections["node-stale"] = &Connection{
		NodeID:  "node-stale",
		LastHB:  time.Now().Add(-2 * time.Second), // stale
		SendCh:  make(chan []byte, 256),
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
		SendCh:  make(chan []byte, 256),
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
		SendCh:  make(chan []byte, 256),
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
			SendCh:  make(chan []byte, 256),
			closeCh: make(chan struct{}),
		}
		h.mu.Unlock()
	}

	if h.Count() != 5 {
		t.Errorf("expected 5 connections, got %d", h.Count())
	}
}
