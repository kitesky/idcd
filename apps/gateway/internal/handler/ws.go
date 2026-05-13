// Package handler provides HTTP handlers for the Gateway service.
package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/kite365/idcd/apps/gateway/internal/hub"
	"github.com/kite365/idcd/lib/auth/apikey"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/stream"
)

// allowedOrigins lists the origins permitted to open WebSocket connections.
// Empty origin (non-browser clients) is always allowed.
var allowedOrigins = map[string]bool{
	"https://idcd.com":     true,
	"https://app.idcd.com": true,
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // non-browser / server-to-server
		}
		return allowedOrigins[origin]
	},
}

// Message types
const (
	MsgTypeHeartbeat = "heartbeat"
	MsgTypeResult    = "result"
	MsgTypeAck       = "ack"
	MsgTypeDrain     = "drain"
	MsgTypeTask      = "task"
)

// Message is the WebSocket message format.
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// WSHandler handles WebSocket connections from agent nodes.
type WSHandler struct {
	hub       *hub.Hub
	queries   *idcdmain.Queries
	streamCli *stream.Client
	logger    *slog.Logger
}

// NewWSHandler creates a new WebSocket handler.
func NewWSHandler(h *hub.Hub, q *idcdmain.Queries, streamCli *stream.Client, logger *slog.Logger) *WSHandler {
	return &WSHandler{
		hub:       h,
		queries:   q,
		streamCli: streamCli,
		logger:    logger,
	}
}

// ServeWS handles WebSocket connection requests from agent nodes.
// GET /agent/ws?api_key=xxx
func (h *WSHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract and verify API key from query parameter
	apiKey := r.URL.Query().Get("api_key")
	if apiKey == "" {
		h.logger.Warn("missing api_key in WebSocket request", "remote_addr", r.RemoteAddr)
		http.Error(w, "missing api_key", http.StatusUnauthorized)
		return
	}

	// Verify API key and get node_id
	nodeID, err := h.verifyAPIKey(ctx, apiKey)
	if err != nil {
		h.logger.Warn("invalid api_key", "error", err, "remote_addr", r.RemoteAddr)
		http.Error(w, "invalid api_key", http.StatusUnauthorized)
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("failed to upgrade connection", "error", err, "node_id", nodeID)
		return
	}

	// Register connection in hub
	c := h.hub.Register(nodeID, conn)

	h.logger.Info("agent connected", "node_id", nodeID, "remote_addr", r.RemoteAddr)

	// Start read and write pumps
	go h.writePump(c)
	h.readPump(c) // blocks until connection closes
}

// verifyAPIKey verifies the API key and returns the associated node_id.
func (h *WSHandler) verifyAPIKey(ctx context.Context, key string) (string, error) {
	// Extract prefix for lookup (will be used for DB query)
	_ = apikey.ExtractPrefix(key)

	// Query database for API key by prefix
	// TODO: Implement proper API key lookup in database
	// For now, we'll use a simple verification approach
	// In production, you'd query the api_keys table

	// Placeholder: In real implementation, query database:
	// prefix := apikey.ExtractPrefix(key)
	// row, err := h.queries.GetAPIKeyByPrefix(ctx, prefix)
	// if err != nil { return "", apperr.Unauthorized("invalid API key") }
	// if !apikey.Verify(key, row.Hash) { return "", apperr.Unauthorized("invalid API key") }
	// return row.NodeID, nil

	// For now, return a mock node_id based on key
	// This will be replaced with real DB lookup
	if len(key) < 16 {
		return "", apperr.Unauthorized("invalid API key format")
	}

	// Mock implementation: use last 8 chars as node_id
	// In production, this must be replaced with DB lookup
	nodeID := "node_" + key[len(key)-8:]

	return nodeID, nil
}

// readPump reads messages from the WebSocket connection.
func (h *WSHandler) readPump(c *hub.Connection) {
	defer func() {
		h.hub.Unregister(c.NodeID, "connection_closed")
	}()

	// Set read deadline
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, msgBytes, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.logger.Error("websocket read error", "error", err, "node_id", c.NodeID)
			}
			break
		}

		// Parse message
		var msg Message
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			h.logger.Warn("invalid message format", "error", err, "node_id", c.NodeID)
			continue
		}

		// Handle message based on type
		if err := h.handleMessage(c, &msg); err != nil {
			h.logger.Error("failed to handle message", "error", err, "node_id", c.NodeID, "msg_type", msg.Type)
		}
	}
}

// writePump writes messages to the WebSocket connection.
func (h *WSHandler) writePump(c *hub.Connection) {
	ticker := time.NewTicker(54 * time.Second) // Ping interval
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.SendCh:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Hub closed the channel
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				h.logger.Error("websocket write error", "error", err, "node_id", c.NodeID)
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}

		// Check if connection is closed
		if c.IsClosed() {
			return
		}
	}
}

// handleMessage processes incoming WebSocket messages.
func (h *WSHandler) handleMessage(c *hub.Connection, msg *Message) error {
	switch msg.Type {
	case MsgTypeHeartbeat:
		return h.handleHeartbeat(c)

	case MsgTypeResult:
		return h.handleResult(c, msg.Payload)

	case MsgTypeAck:
		return h.handleAck(c, msg.Payload)

	default:
		h.logger.Warn("unknown message type", "type", msg.Type, "node_id", c.NodeID)
		return nil
	}
}

// handleHeartbeat processes a heartbeat message.
func (h *WSHandler) handleHeartbeat(c *hub.Connection) error {
	h.hub.UpdateHeartbeat(c.NodeID)
	h.logger.Debug("heartbeat received", "node_id", c.NodeID)
	return nil
}

// handleResult processes a probe result message.
func (h *WSHandler) handleResult(c *hub.Connection, payload json.RawMessage) error {
	// Parse result payload
	var result struct {
		TaskID string         `json:"task_id"`
		Data   map[string]any `json:"data"`
	}
	if err := json.Unmarshal(payload, &result); err != nil {
		return apperr.Validation("invalid result payload", err.Error())
	}

	// Push to probe.results Redis Stream
	ctx := context.Background()
	streamID, err := h.streamCli.AddProbeResult(ctx, result.TaskID, c.NodeID, result.Data)
	if err != nil {
		return apperr.Internal("failed to write to stream", err)
	}

	h.logger.Info("probe result received", "node_id", c.NodeID, "task_id", result.TaskID, "stream_id", streamID)

	// Send ack back to node
	ackPayload, _ := json.Marshal(map[string]string{"task_id": result.TaskID})
	ackMsg := Message{
		Type:    MsgTypeAck,
		Payload: json.RawMessage(ackPayload),
	}
	ackBytes, _ := json.Marshal(ackMsg)
	h.hub.Broadcast(c.NodeID, ackBytes)

	return nil
}

// handleAck processes an acknowledgment message.
func (h *WSHandler) handleAck(c *hub.Connection, payload json.RawMessage) error {
	var ack struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(payload, &ack); err != nil {
		return apperr.Validation("invalid ack payload", err.Error())
	}

	h.logger.Debug("ack received", "node_id", c.NodeID, "task_id", ack.TaskID)
	return nil
}
