package handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kite365/idcd/apps/gateway/internal/hub"
	"github.com/kite365/idcd/packages/shared/logger"
)

func TestMessage_Marshal(t *testing.T) {
	msg := Message{
		Type:    MsgTypeHeartbeat,
		Payload: json.RawMessage(`{"foo":"bar"}`),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal message: %v", err)
	}

	var parsed Message
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal message: %v", err)
	}

	if parsed.Type != MsgTypeHeartbeat {
		t.Errorf("expected type %q, got %q", MsgTypeHeartbeat, parsed.Type)
	}
}

func TestNewWSHandler(t *testing.T) {
	log := logger.Discard()
	h := hub.New(30*time.Second, log)

	handler := NewWSHandler(h, nil, nil, log)

	if handler == nil {
		t.Fatal("expected handler to be created")
	}

	if handler.hub != h {
		t.Error("expected hub to be set")
	}

	if handler.logger == nil {
		t.Error("expected logger to be set")
	}
}

func TestHandleHeartbeat(t *testing.T) {
	log := logger.Discard()
	h := hub.New(30*time.Second, log)

	_ = NewWSHandler(h, nil, nil, log)

	// The actual heartbeat update logic is tested in hub_test.go
	// Testing WebSocket handler logic requires a real WebSocket connection,
	// which is difficult to mock properly in unit tests.
	// Integration tests would be more appropriate for testing the full WebSocket flow.
}

func TestHandleMessage_UnknownType(t *testing.T) {
	log := logger.Discard()
	h := hub.New(30*time.Second, log)

	handler := NewWSHandler(h, nil, nil, log)

	c := &hub.Connection{
		NodeID: "node-123",
	}

	msg := &Message{
		Type: "unknown",
	}

	// Should not return error for unknown type, just log warning
	err := handler.handleMessage(c, msg)
	if err != nil {
		t.Errorf("unexpected error for unknown message type: %v", err)
	}
}

func TestHandleAck(t *testing.T) {
	log := logger.Discard()
	h := hub.New(30*time.Second, log)

	handler := NewWSHandler(h, nil, nil, log)

	c := &hub.Connection{
		NodeID: "node-123",
	}

	payload := json.RawMessage(`{"task_id":"task-456"}`)

	err := handler.handleAck(c, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleAck_InvalidPayload(t *testing.T) {
	log := logger.Discard()
	h := hub.New(30*time.Second, log)

	handler := NewWSHandler(h, nil, nil, log)

	c := &hub.Connection{
		NodeID: "node-123",
	}

	payload := json.RawMessage(`invalid json`)

	err := handler.handleAck(c, payload)
	if err == nil {
		t.Error("expected error for invalid payload")
	}
}

func TestVerifyAPIKey_InvalidFormat(t *testing.T) {
	log := logger.Discard()
	h := hub.New(30*time.Second, log)

	handler := NewWSHandler(h, nil, nil, log)

	// Test with short key
	_, err := handler.verifyAPIKey(nil, "short")
	if err == nil {
		t.Error("expected error for short API key")
	}
}

func TestVerifyAPIKey_MockImplementation(t *testing.T) {
	log := logger.Discard()
	h := hub.New(30*time.Second, log)

	handler := NewWSHandler(h, nil, nil, log)

	// Test with valid-length key (mock implementation)
	key := "sk_live_1234567890abcdef"
	nodeID, err := handler.verifyAPIKey(nil, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if nodeID == "" {
		t.Error("expected node_id to be returned")
	}

	// Should contain "node_" prefix
	if len(nodeID) < 5 || nodeID[:5] != "node_" {
		t.Errorf("expected node_id to start with 'node_', got %q", nodeID)
	}
}
