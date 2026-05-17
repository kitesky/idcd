// Package ws provides a resilient WebSocket client for the idcd agent.
// It maintains a persistent connection to the gateway with automatic
// exponential-backoff reconnection.
package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/kite365/idcd/apps/agent/internal/fingerprint"
)

const (
	heartbeatInterval = 30 * time.Second
	writeTimeout      = 10 * time.Second
	pongTimeout       = 60 * time.Second
	pingInterval      = 54 * time.Second

	backoffMin = 1 * time.Second
	backoffMax = 60 * time.Second
)

// MessageHandler is called when the gateway sends a control message.
type MessageHandler func(payload json.RawMessage) error

// Message is the wire format shared with the gateway.
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Client is a resilient WebSocket client connected to the gateway.
type Client struct {
	gatewayURL string
	secretKey  string
	nodeID     string

	mu       sync.Mutex
	conn     *websocket.Conn
	handlers map[string]MessageHandler

	fp     *fingerprint.Fingerprint
	fpMu   sync.RWMutex

	doneCh chan struct{}
	logger *slog.Logger
}

// New creates a Client. Call Run(ctx) to start the connection loop.
func New(gatewayURL, secretKey, nodeID string, logger *slog.Logger) *Client {
	return &Client{
		gatewayURL: gatewayURL,
		secretKey:  secretKey,
		nodeID:     nodeID,
		handlers:   make(map[string]MessageHandler),
		doneCh:     make(chan struct{}),
		logger:     logger,
	}
}

// Handle registers a handler for a specific message type.
// Must be called before Run.
func (c *Client) Handle(msgType string, h MessageHandler) {
	c.handlers[msgType] = h
}

// Send sends an outbound message to the gateway.
// It is safe to call from any goroutine; c.mu serializes all writes.
func (c *Client) Send(msgType string, payload any) error {
	var raw json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("ws: marshal payload: %w", err)
		}
		raw = b
	}

	msg := Message{Type: msgType, Payload: raw}
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("ws: marshal message: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("ws: not connected")
	}
	c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// UpdateFingerprint refreshes the cached fingerprint (e.g. after config reload).
func (c *Client) UpdateFingerprint(fp *fingerprint.Fingerprint) {
	c.fpMu.Lock()
	c.fp = fp
	c.fpMu.Unlock()
}

// Run starts the connection loop. It blocks until ctx is cancelled.
// On disconnect it reconnects with exponential backoff.
func (c *Client) Run(ctx context.Context) {
	backoff := backoffMin

	for {
		select {
		case <-ctx.Done():
			c.closeConn()
			return
		default:
		}

		if err := c.connect(ctx); err != nil {
			c.logger.Warn("ws: connection failed", "err", err, "retry_in", backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, backoffMax)
			continue
		}

		backoff = backoffMin // reset on successful connect
		c.logger.Info("ws: connected to gateway", "url", c.gatewayURL)

		// Run read/write loops until disconnected
		c.runSession(ctx)

		c.logger.Info("ws: disconnected, will reconnect")
		c.closeConn()
	}
}

// connect establishes the WebSocket connection.
func (c *Client) connect(ctx context.Context) error {
	_, err := url.Parse(c.gatewayURL)
	if err != nil {
		return fmt.Errorf("parse gateway URL: %w", err)
	}

	// Pass the secret key in a request header, not in the URL, so it is never
	// written to proxy/access logs.
	handshakeHeaders := http.Header{}
	handshakeHeaders.Set("Authorization", "Bearer "+c.secretKey)
	handshakeHeaders.Set("X-Node-ID", c.nodeID)

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.DialContext(ctx, c.gatewayURL, handshakeHeaders)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	return nil
}

// runSession runs the heartbeat sender and message reader until the connection dies.
func (c *Client) runSession(ctx context.Context) {
	conn := c.getConn()
	if conn == nil {
		return
	}

	conn.SetReadDeadline(time.Now().Add(pongTimeout))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongTimeout))
		return nil
	})

	// heartbeat + ping goroutine
	hbDone := make(chan struct{})
	go func() {
		defer close(hbDone)
		hbTick := time.NewTicker(heartbeatInterval)
		pingTick := time.NewTicker(pingInterval)
		defer hbTick.Stop()
		defer pingTick.Stop()

		// Send initial heartbeat immediately
		c.sendHeartbeat()

		for {
			select {
			case <-ctx.Done():
				return
			case <-hbTick.C:
				c.sendHeartbeat()
			case <-pingTick.C:
				if err := c.writePing(); err != nil {
					c.logger.Warn("ws: ping failed", "err", err)
					return
				}
			}
		}
	}()

	// message reader (blocks)
	c.readLoop(conn)
	<-hbDone
}

// readLoop reads and dispatches incoming messages.
func (c *Client) readLoop(conn *websocket.Conn) {
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.logger.Warn("ws: read error", "err", err)
			}
			return
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			c.logger.Warn("ws: invalid message JSON", "err", err)
			continue
		}

		c.dispatch(msg)
	}
}

func (c *Client) dispatch(msg Message) {
	handler, ok := c.handlers[msg.Type]
	if !ok {
		c.logger.Debug("ws: no handler for message type", "type", msg.Type)
		return
	}
	if err := handler(msg.Payload); err != nil {
		c.logger.Error("ws: message handler error", "type", msg.Type, "err", err)
	}
}

// sendHeartbeat sends a heartbeat message with the current fingerprint.
func (c *Client) sendHeartbeat() {
	c.fpMu.RLock()
	fp := c.fp
	c.fpMu.RUnlock()

	payload := struct {
		NodeID      string               `json:"node_id"`
		Fingerprint *fingerprint.Fingerprint `json:"fingerprint,omitempty"`
		Timestamp   int64                `json:"ts"`
	}{
		NodeID:      c.nodeID,
		Fingerprint: fp,
		Timestamp:   time.Now().Unix(),
	}

	if err := c.Send("heartbeat", payload); err != nil {
		c.logger.Warn("ws: heartbeat failed", "err", err)
	}
}

func (c *Client) writePing() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("ws: not connected")
	}
	c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	return c.conn.WriteMessage(websocket.PingMessage, nil)
}

func (c *Client) getConn() *websocket.Conn {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn
}

func (c *Client) closeConn() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		_ = c.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseGoingAway, ""))
		c.conn.Close()
		c.conn = nil
	}
}

