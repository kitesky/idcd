package handler

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/kite365/idcd/apps/gateway/internal/hub"
	"github.com/kite365/idcd/lib/shared/logger"
)

// MockNodeAuthPool implements NodeAuthPool for tests.
type MockNodeAuthPool struct {
	nodeID string
	err    error
}

func (m *MockNodeAuthPool) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &mockRow{nodeID: m.nodeID, err: m.err}
}

func (m *MockNodeAuthPool) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

type mockRow struct {
	nodeID string
	err    error
}

func (r *mockRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) > 0 {
		*(dest[0].(*string)) = r.nodeID
	}
	return nil
}

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
	handler := NewWSHandler(h, nil, nil, nil, log)
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

func TestHandleMessage_UnknownType(t *testing.T) {
	log := logger.Discard()
	h := hub.New(30*time.Second, log)
	handler := NewWSHandler(h, nil, nil, nil, log)
	c := &hub.Connection{NodeID: "node-123"}
	msg := &Message{Type: "unknown"}
	err := handler.handleMessage(c, msg)
	if err != nil {
		t.Errorf("unexpected error for unknown message type: %v", err)
	}
}

func TestHandleAck(t *testing.T) {
	log := logger.Discard()
	h := hub.New(30*time.Second, log)
	handler := NewWSHandler(h, nil, nil, nil, log)
	c := &hub.Connection{NodeID: "node-123"}
	payload := json.RawMessage(`{"task_id":"task-456"}`)
	if err := handler.handleAck(c, payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleAck_InvalidPayload(t *testing.T) {
	log := logger.Discard()
	h := hub.New(30*time.Second, log)
	handler := NewWSHandler(h, nil, nil, nil, log)
	c := &hub.Connection{NodeID: "node-123"}
	if err := handler.handleAck(c, json.RawMessage(`invalid json`)); err == nil {
		t.Error("expected error for invalid payload")
	}
}

func TestVerifyAPIKey_TooShort(t *testing.T) {
	log := logger.Discard()
	h := hub.New(30*time.Second, log)
	handler := NewWSHandler(h, nil, nil, nil, log)
	_, err := handler.verifyAPIKey(context.Background(), "short")
	if err == nil {
		t.Error("expected error for short api_key")
	}
}

func TestVerifyAPIKey_NoPool(t *testing.T) {
	log := logger.Discard()
	h := hub.New(30*time.Second, log)
	handler := NewWSHandler(h, nil, nil, nil, log)
	_, err := handler.verifyAPIKey(context.Background(), "a-valid-length-key-here!")
	if err == nil {
		t.Error("expected error when no pool configured")
	}
}

func TestVerifyAPIKey_DBHit(t *testing.T) {
	log := logger.Discard()
	h := hub.New(30*time.Second, log)
	pool := &MockNodeAuthPool{nodeID: "nd_testnode"}
	handler := NewWSHandler(h, nil, pool, nil, log)

	nodeID, err := handler.verifyAPIKey(context.Background(), "valid-secret-key-that-is-long-enough")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nodeID != "nd_testnode" {
		t.Errorf("expected nd_testnode, got %q", nodeID)
	}
}

func TestVerifyAPIKey_NotFound(t *testing.T) {
	log := logger.Discard()
	h := hub.New(30*time.Second, log)
	pool := &MockNodeAuthPool{err: pgx.ErrNoRows}
	handler := NewWSHandler(h, nil, pool, nil, log)

	_, err := handler.verifyAPIKey(context.Background(), "valid-secret-key-that-is-long-enough")
	if err == nil {
		t.Error("expected error when node not found")
	}
}
