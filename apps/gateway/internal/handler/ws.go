// Package handler provides HTTP handlers for the Gateway service.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/apps/gateway/internal/hub"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
	"github.com/kite365/idcd/lib/shared/stream"
)

// allowedOrigins lists the origins permitted to open WebSocket connections.
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
			return true
		}
		return allowedOrigins[origin]
	},
}

// Message types inbound (agent → gateway)
const (
	MsgTypeHeartbeat  = "heartbeat"
	MsgTypeResult     = "result"
	MsgTypeAck        = "ack"
	MsgTypeCmdAck     = "cmd_ack"
)

// Message types outbound (gateway → agent)
const (
	MsgTypeDrain        = "drain"
	MsgTypeTask         = "task"
	MsgTypeUpgrade      = "upgrade"
	MsgTypeReloadConfig = "reload_config"
)

const (
	wsReadDeadline  = 60 * time.Second
	wsWriteDeadline = 10 * time.Second
	wsPingInterval  = 54 * time.Second
)

// Message is the WebSocket wire format.
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// NodeAuthPool is the DB interface needed by WSHandler.
// *pgxpool.Pool satisfies this interface.
type NodeAuthPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// WSHandler handles WebSocket connections from agent nodes.
type WSHandler struct {
	hub       *hub.Hub
	queries   *idcdmain.Queries
	pool      NodeAuthPool
	streamCli *stream.Client
	logger    *slog.Logger
}

// NewWSHandler creates a new WebSocket handler.
func NewWSHandler(h *hub.Hub, q *idcdmain.Queries, pool NodeAuthPool, streamCli *stream.Client, logger *slog.Logger) *WSHandler {
	return &WSHandler{
		hub:       h,
		queries:   q,
		pool:      pool,
		streamCli: streamCli,
		logger:    logger,
	}
}

// ServeWS handles WebSocket upgrade requests from agent nodes.
// Accepts the secret key via the Authorization header (preferred: "Bearer <key>")
// or, for backward compatibility, via the api_key query parameter.
func (h *WSHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Prefer Authorization header so the key never appears in proxy/access logs.
	apiKey := ""
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		apiKey = strings.TrimPrefix(auth, "Bearer ")
	} else {
		// Fallback: query param supported for backward compatibility.
		apiKey = r.URL.Query().Get("api_key")
	}
	if apiKey == "" {
		http.Error(w, "missing api_key", http.StatusUnauthorized)
		return
	}

	nodeID, err := h.verifyAPIKey(ctx, apiKey)
	if err != nil {
		h.logger.Warn("invalid api_key", "err", err, "remote_addr", r.RemoteAddr)
		http.Error(w, "invalid api_key", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", "err", err, "node_id", nodeID)
		return
	}

	if h.pool != nil {
		_, _ = h.pool.Exec(ctx, `
			UPDATE enrolled_nodes
			SET status = 'active', last_seen_at = now()
			WHERE node_id = $1
		`, nodeID)
	}

	c := h.hub.Register(nodeID, conn)
	h.logger.Info("agent connected", "node_id", nodeID, "remote_addr", r.RemoteAddr)

	go h.writePump(c)
	h.readPump(c)
}

// ── auth ──────────────────────────────────────────────────────────────────────


func (h *WSHandler) verifyAPIKey(ctx context.Context, key string) (string, error) {
	if len(key) < 16 {
		return "", apperr.Unauthorized("invalid api_key format")
	}
	if h.pool == nil {
		return "", fmt.Errorf("gateway: no DB pool configured")
	}
	secretHash := idgen.SHA256Hex(key)
	var nodeID string
	err := h.pool.QueryRow(ctx, `
		SELECT node_id FROM enrolled_nodes
		WHERE secret_hash = $1
		  AND status != 'disabled'
	`, secretHash).Scan(&nodeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperr.Unauthorized("invalid or revoked api_key")
		}
		return "", fmt.Errorf("gateway: db lookup: %w", err)
	}
	return nodeID, nil
}


// ── read / write pumps ────────────────────────────────────────────────────────

func (h *WSHandler) readPump(c *hub.Connection) {
	nodeID := c.NodeID
	defer func() {
		h.hub.Unregister(nodeID, "connection_closed")
		if h.pool != nil {
			ctx := context.Background()
			_, _ = h.pool.Exec(ctx,
				`UPDATE enrolled_nodes SET status = 'offline' WHERE node_id = $1`, nodeID)
		}
	}()

	c.Conn.SetReadDeadline(time.Now().Add(wsReadDeadline))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(wsReadDeadline))
		return nil
	})

	for {
		_, msgBytes, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.logger.Warn("websocket read error", "err", err, "node_id", c.NodeID)
			}
			return
		}
		var msg Message
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			h.logger.Warn("invalid message JSON", "err", err, "node_id", c.NodeID)
			continue
		}
		if err := h.handleMessage(c, &msg); err != nil {
			h.logger.Error("message handler error", "err", err, "node_id", c.NodeID, "type", msg.Type)
		}
	}
}

func (h *WSHandler) writePump(c *hub.Connection) {
	ticker := time.NewTicker(wsPingInterval)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.SendCh:
			c.Conn.SetWriteDeadline(time.Now().Add(wsWriteDeadline))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				h.logger.Error("websocket write error", "err", err, "node_id", c.NodeID)
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(wsWriteDeadline))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
		if c.IsClosed() {
			return
		}
	}
}

// ── message dispatch ──────────────────────────────────────────────────────────

func (h *WSHandler) handleMessage(c *hub.Connection, msg *Message) error {
	switch msg.Type {
	case MsgTypeHeartbeat:
		return h.handleHeartbeat(c, msg.Payload)
	case MsgTypeResult:
		return h.handleResult(c, msg.Payload)
	case MsgTypeAck:
		return h.handleAck(c, msg.Payload)
	case MsgTypeCmdAck:
		return h.handleCmdAck(c, msg.Payload)
	default:
		h.logger.Debug("unknown message type", "type", msg.Type, "node_id", c.NodeID)
		return nil
	}
}

// ── heartbeat ─────────────────────────────────────────────────────────────────

// heartbeatPayload matches the agent's ws/client.go heartbeat struct.
type heartbeatPayload struct {
	NodeID      string           `json:"node_id"`
	Fingerprint *nodeFingerprint `json:"fingerprint,omitempty"`
	Timestamp   int64            `json:"ts"`
}

type nodeFingerprint struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Kernel   string `json:"kernel"`
	MAC      string `json:"mac"`
	CPUModel string `json:"cpu_model"`
}

func (h *WSHandler) handleHeartbeat(c *hub.Connection, payload json.RawMessage) error {
	h.hub.UpdateHeartbeat(c.NodeID)

	var hb heartbeatPayload
	if err := json.Unmarshal(payload, &hb); err != nil {
		// Tolerate missing/malformed payload — heartbeat itself is still valid.
		h.logger.Debug("heartbeat: could not parse payload", "node_id", c.NodeID, "err", err)
		return nil
	}

	ctx := context.Background()

	if h.pool != nil {
		h.processFingerprint(ctx, c.NodeID, hb.Fingerprint)
		h.dispatchPendingCommands(ctx, c.NodeID)
	}

	return nil
}

// processFingerprint compares the received fingerprint against the stored one
// and logs a warning if it has changed unexpectedly.
func (h *WSHandler) processFingerprint(ctx context.Context, nodeID string, fp *nodeFingerprint) {
	if fp == nil {
		// Still update last_seen_at even without fingerprint data.
		_, _ = h.pool.Exec(ctx,
			`UPDATE enrolled_nodes SET last_seen_at = now() WHERE node_id = $1`, nodeID)
		return
	}

	fpJSON, err := json.Marshal(fp)
	if err != nil {
		return
	}

	var stored []byte
	row := h.pool.QueryRow(ctx,
		`SELECT fingerprint FROM enrolled_nodes WHERE node_id = $1`, nodeID)
	_ = row.Scan(&stored)

	if len(stored) > 0 && string(stored) != "null" {
		var prev nodeFingerprint
		if err := json.Unmarshal(stored, &prev); err == nil {
			if fp.Hostname != prev.Hostname || fp.MAC != prev.MAC || fp.Kernel != prev.Kernel {
				h.logger.Warn("node fingerprint changed — possible host replacement",
					"node_id", nodeID,
					"hostname", fmt.Sprintf("%s→%s", prev.Hostname, fp.Hostname),
					"mac",      fmt.Sprintf("%s→%s", prev.MAC, fp.MAC),
					"kernel",   fmt.Sprintf("%s→%s", prev.Kernel, fp.Kernel),
				)
			}
		}
	}

	_, _ = h.pool.Exec(ctx, `
		UPDATE enrolled_nodes
		SET last_seen_at           = now(),
		    fingerprint            = $2::jsonb,
		    fingerprint_updated_at = now()
		WHERE node_id = $1
	`, nodeID, string(fpJSON))
}

// pendingCmd is a queued command from the node_commands table.
type pendingCmd struct {
	ID      string          `json:"id"`
	Command string          `json:"command"`
	Payload json.RawMessage `json:"payload"`
}

// dispatchPendingCommands fetches undelivered commands for this node and sends them.
func (h *WSHandler) dispatchPendingCommands(ctx context.Context, nodeID string) {
	rows, err := queryPendingCommands(ctx, h.pool, nodeID)
	if err != nil || len(rows) == 0 {
		return
	}

	for _, cmd := range rows {
		// Inject cmd_id into the payload so the agent can echo it back in the ack,
		// allowing the gateway to identify the exact command that was acknowledged.
		var payloadMap map[string]json.RawMessage
		if err := json.Unmarshal(cmd.Payload, &payloadMap); err != nil || payloadMap == nil {
			payloadMap = make(map[string]json.RawMessage)
		}
		cmdIDJSON, _ := json.Marshal(cmd.ID)
		payloadMap["cmd_id"] = cmdIDJSON
		enrichedPayload, err := json.Marshal(payloadMap)
		if err != nil {
			enrichedPayload = cmd.Payload
		}

		msg := Message{
			Type:    cmd.Command,
			Payload: enrichedPayload,
		}
		data, err := json.Marshal(msg)
		if err != nil {
			continue
		}

		sent := h.hub.Broadcast(nodeID, data)
		if sent {
			_, _ = h.pool.Exec(ctx,
				`UPDATE node_commands SET status = 'sent', sent_at = now() WHERE id = $1`,
				cmd.ID)
			h.logger.Info("command dispatched", "node_id", nodeID, "command", cmd.Command, "cmd_id", cmd.ID)
		}
	}
}

// queryPendingCommands uses json_agg to return multi-row results through the
// NodeAuthPool interface, which only exposes QueryRow (no Query method).
func queryPendingCommands(ctx context.Context, pool NodeAuthPool, nodeID string) ([]pendingCmd, error) {
	var raw []byte
	err := pool.QueryRow(ctx, `
		SELECT COALESCE(json_agg(row_to_json(t)), '[]'::json)
		FROM (
			SELECT id, command, payload
			FROM node_commands
			WHERE node_id = $1 AND status = 'pending'
			ORDER BY created_at
			LIMIT 5
		) t
	`, nodeID).Scan(&raw)
	if err != nil {
		return nil, err
	}

	var cmds []pendingCmd
	if err := json.Unmarshal(raw, &cmds); err != nil {
		return nil, err
	}
	return cmds, nil
}

// ── result / ack / cmd_ack ────────────────────────────────────────────────────

// probeResultItem is the minimal structure parsed from a probe result.
type probeResultItem struct {
	TaskID string         `json:"task_id"`
	Data   map[string]any `json:"data"`
}

func (h *WSHandler) handleResult(c *hub.Connection, payload json.RawMessage) error {
	// Agent sends either a single result object or an array of results.
	// Normalise both into a slice for uniform processing.
	var results []probeResultItem
	if len(payload) > 0 && payload[0] == '[' {
		if err := json.Unmarshal(payload, &results); err != nil {
			return apperr.Validation("invalid result payload (array)", err.Error())
		}
	} else {
		var single probeResultItem
		if err := json.Unmarshal(payload, &single); err != nil {
			return apperr.Validation("invalid result payload", err.Error())
		}
		results = []probeResultItem{single}
	}

	if h.streamCli == nil {
		h.logger.Warn("result received but stream client not configured, dropping", "node_id", c.NodeID)
		return nil
	}

	ctx := context.Background()
	for _, result := range results {
		if result.TaskID == "" {
			continue
		}

		// Verify that the task was assigned to this node, preventing a rogue agent
		// from submitting results for tasks belonging to other users' monitors.
		if h.pool != nil {
			var assignedNodeID string
			err := h.pool.QueryRow(ctx,
				`SELECT node_id FROM scheduler_tasks WHERE id = $1`, result.TaskID,
			).Scan(&assignedNodeID)
			if err == nil && assignedNodeID != "" && assignedNodeID != c.NodeID {
				h.logger.Warn("result rejected: task_id belongs to different node",
					"node_id", c.NodeID, "task_id", result.TaskID, "assigned_node", assignedNodeID)
				continue
			}
		}

		streamID, err := h.streamCli.AddProbeResult(ctx, result.TaskID, c.NodeID, result.Data)
		if err != nil {
			h.logger.Error("failed to write result to stream", "task_id", result.TaskID, "err", err)
			continue
		}
		h.logger.Info("probe result received", "node_id", c.NodeID, "task_id", result.TaskID, "stream_id", streamID)

		ackPayload, _ := json.Marshal(map[string]string{"task_id": result.TaskID})
		ackMsg := Message{Type: MsgTypeAck, Payload: json.RawMessage(ackPayload)}
		ackBytes, _ := json.Marshal(ackMsg)
		h.hub.Broadcast(c.NodeID, ackBytes)
	}
	return nil
}

func (h *WSHandler) handleAck(c *hub.Connection, payload json.RawMessage) error {
	var ack struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(payload, &ack); err != nil {
		return apperr.Validation("invalid ack payload", err.Error())
	}
	h.logger.Debug("task ack", "node_id", c.NodeID, "task_id", ack.TaskID)
	return nil
}

func (h *WSHandler) handleCmdAck(c *hub.Connection, payload json.RawMessage) error {
	var ack struct {
		Command string `json:"command"`
		CmdID   string `json:"cmd_id,omitempty"` // preferred; identifies the exact command
		Version string `json:"version,omitempty"`
	}
	if err := json.Unmarshal(payload, &ack); err != nil {
		return apperr.Validation("invalid cmd_ack payload", err.Error())
	}

	h.logger.Info("command acked by node", "node_id", c.NodeID, "command", ack.Command, "cmd_id", ack.CmdID)

	if h.pool != nil {
		ctx := context.Background()
		if ack.CmdID != "" {
			// Preferred path: ack by exact cmd_id so concurrent commands of the
			// same type do not cross-contaminate each other.
			_, _ = h.pool.Exec(ctx, `
				UPDATE node_commands
				SET status = 'acked', acked_at = now()
				WHERE id = $1 AND node_id = $2 AND status = 'sent'
			`, ack.CmdID, c.NodeID)
		} else {
			// Legacy fallback: agents that don't send cmd_id fall back to
			// acking the most-recently-sent command of that type.
			_, _ = h.pool.Exec(ctx, `
				UPDATE node_commands
				SET status = 'acked', acked_at = now()
				WHERE id = (
					SELECT id FROM node_commands
					WHERE node_id = $1 AND command = $2 AND status = 'sent'
					ORDER BY sent_at DESC
					LIMIT 1
				)
			`, c.NodeID, ack.Command)
		}
	}
	return nil
}

// ── SendCommand is called by the admin handler to push a command ──────────────

// SendCommandToNode writes a command to the queue and immediately tries to push it
// if the node is currently connected. Returns the command ID.
func (h *WSHandler) SendCommandToNode(ctx context.Context, nodeID, command string, payload any) (string, error) {
	if h.pool == nil {
		return "", fmt.Errorf("no DB pool")
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	cmdID := idgen.New("cmd_")
	_, err = h.pool.Exec(ctx, `
		INSERT INTO node_commands (id, node_id, command, payload)
		VALUES ($1, $2, $3, $4::jsonb)
	`, cmdID, nodeID, command, string(payloadJSON))
	if err != nil {
		return "", fmt.Errorf("insert command: %w", err)
	}

	// Try immediate delivery if the node is online
	msg := Message{Type: command, Payload: json.RawMessage(payloadJSON)}
	data, _ := json.Marshal(msg)
	if h.hub.Broadcast(nodeID, data) {
		_, _ = h.pool.Exec(ctx,
			`UPDATE node_commands SET status = 'sent', sent_at = now() WHERE id = $1`, cmdID)
		h.logger.Info("command sent immediately", "node_id", nodeID, "command", command, "cmd_id", cmdID)
	} else {
		h.logger.Info("node offline, command queued", "node_id", nodeID, "command", command, "cmd_id", cmdID)
	}

	return cmdID, nil
}
