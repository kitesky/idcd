// Package hub manages WebSocket connections to agent nodes.
package hub

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Connection represents a single agent node WebSocket connection.
type Connection struct {
	NodeID    string
	Conn      *websocket.Conn
	LastHB    time.Time     // last heartbeat timestamp
	SendCh    chan []byte   // outbound message channel
	closeCh   chan struct{} // signal connection close
	closeOnce sync.Once
	// generation is a monotonically increasing token assigned at Register
	// time. P2#20: a leaked api_key could be used to repeatedly re-connect
	// the same node_id and knock the legitimate node offline; we close the
	// previous Connection and tag the new one with a fresh generation so any
	// in-flight messages from the old socket cannot mutate the new
	// Connection's hub state. See Hub.ActiveGen / Hub.IsActive.
	generation uint64
}

// Hub manages all active agent connections.
type Hub struct {
	mu           sync.RWMutex
	connections  map[string]*Connection // nodeID -> Connection
	heartbeatTTL time.Duration
	logger       *slog.Logger

	// nextGen issues monotonically increasing generation tokens to
	// Connections so callers can distinguish "this is still the active
	// Connection for this node" from "I am the leftover from a replaced
	// connection". atomic so Register can read/increment without taking
	// the write lock.
	nextGen atomic.Uint64

	// pendingSkip tracks how many forthcoming "connection_closed"
	// Unregister calls we expect to receive from the read pumps of
	// Connections we already replaced via Register. Without this counter,
	// the old readPump's deferred Unregister(nodeID, "connection_closed")
	// would race in and delete the NEW Connection from the map, exactly
	// the attack vector P2#20 describes. Keyed by nodeID, decremented per
	// matching Unregister call. Only consulted when reason ==
	// "connection_closed" — legitimate teardown reasons (e.g.
	// "heartbeat_timeout" from CheckHeartbeats) bypass this.
	pendingSkip map[string]int

	// Metrics
	connGauge       prometheus.Gauge
	heartbeatTotal  prometheus.Counter
	disconnectTotal *prometheus.CounterVec
}

var (
	connGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gateway_connections_active",
		Help: "Number of active agent connections",
	})

	heartbeatTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gateway_heartbeats_total",
		Help: "Total number of heartbeats received",
	})

	disconnectTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gateway_disconnects_total",
		Help: "Total number of disconnections",
	}, []string{"reason"})
)

// New creates a Hub with the given heartbeat timeout.
func New(heartbeatTTL time.Duration, logger *slog.Logger) *Hub {
	return &Hub{
		connections:     make(map[string]*Connection),
		pendingSkip:     make(map[string]int),
		heartbeatTTL:    heartbeatTTL,
		logger:          logger,
		connGauge:       connGauge,
		heartbeatTotal:  heartbeatTotal,
		disconnectTotal: disconnectTotal,
	}
}

// Register adds a new connection to the hub.
//
// If a Connection with the same nodeID is already registered, the existing
// Connection is closed (websocket + close/send channels) and the new
// Connection takes its place. The new Connection is tagged with a fresh
// monotonically-increasing generation token, accessible via ActiveGen /
// IsActive, so callers reading messages off the OLD socket can detect that
// they are no longer authoritative and exit before mutating shared state.
//
// This is the P2#20 fix: a leaked api_key could otherwise be used to
// repeatedly re-connect the same node_id and starve the legitimate node by
// leaving stale read pumps active.
func (h *Hub) Register(nodeID string, conn *websocket.Conn) *Connection {
	h.mu.Lock()
	defer h.mu.Unlock()

	gen := h.nextGen.Add(1)
	c := &Connection{
		NodeID:     nodeID,
		Conn:       conn,
		LastHB:     time.Now(),
		SendCh:     make(chan []byte, 256),
		closeCh:    make(chan struct{}),
		generation: gen,
	}

	if old, exists := h.connections[nodeID]; exists {
		// Replace path. We do NOT bump the active-connections gauge — one
		// connection leaves, one joins, net zero.
		//
		// We close the OLD Connection here (ws + closeCh + SendCh). Its
		// reader goroutine will see ReadMessage return an error and fall
		// through to its defer, which calls Unregister(nodeID,
		// "connection_closed"). That deferred call is the dangerous one
		// — by the time it lands the map already points at the NEW
		// Connection. We bump pendingSkip so Unregister will absorb the
		// stale call and leave the new Connection in place.
		h.pendingSkip[nodeID]++
		h.disconnectTotal.WithLabelValues("replaced").Inc()
		// Spec-compliant metric: gateway_ws_connections_total{outcome="replaced"}
		// counts the close-old half of a connection-replacement event.
		MetricsWSConnections.WithLabelValues("replaced").Inc()
		old.Close()
		h.logger.Info("connection replaced",
			"node_id", nodeID,
			"old_generation", old.generation,
			"new_generation", gen,
		)
	} else {
		// Brand-new node — bump active gauges. Replacement does not touch
		// these gauges because the count of distinct active node_ids does
		// not change.
		h.connGauge.Inc()
		MetricsActiveConnections.Inc()
	}

	h.connections[nodeID] = c
	// P1#19: spec-compliant metrics mirror the legacy counters/gauge so
	// dashboards can migrate without losing data.
	MetricsActiveNodes.Set(float64(len(h.connections)))
	MetricsWSConnections.WithLabelValues("accepted").Inc()

	h.logger.Info("node registered",
		"node_id", nodeID,
		"generation", gen,
		"total_connections", len(h.connections),
	)
	return c
}

// Unregister removes a connection from the hub.
//
// If a Register-replace recently closed a previous Connection for this
// nodeID, the closed Connection's read pump will eventually fire a deferred
// Unregister(nodeID, "connection_closed") — we absorb that stale call here
// (via pendingSkip) so the NEW Connection installed by Register is not
// accidentally removed. Non-"connection_closed" reasons (e.g.
// "heartbeat_timeout") always proceed with the normal teardown path so
// CheckHeartbeats can still evict a stale-but-current connection.
func (h *Hub) Unregister(nodeID string, reason string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if reason == "connection_closed" && h.pendingSkip[nodeID] > 0 {
		h.pendingSkip[nodeID]--
		if h.pendingSkip[nodeID] == 0 {
			delete(h.pendingSkip, nodeID)
		}
		h.logger.Debug("ignoring stale unregister from replaced connection",
			"node_id", nodeID,
		)
		return
	}

	c, exists := h.connections[nodeID]
	if !exists {
		return
	}

	delete(h.connections, nodeID)
	h.connGauge.Dec()
	h.disconnectTotal.WithLabelValues(reason).Inc()
	// P1#19: keep the spec-compliant gauges + counter in sync with the legacy
	// ones so Grafana can switch dashboard sources transparently.
	MetricsActiveConnections.Dec()
	MetricsActiveNodes.Set(float64(len(h.connections)))
	MetricsWSConnections.WithLabelValues("disconnected").Inc()

	// Close the connection
	c.Close()

	h.logger.Info("node unregistered", "node_id", nodeID, "reason", reason, "total_connections", len(h.connections))
}

// ActiveGen returns the generation token of the Connection currently
// registered for nodeID, or 0 if no Connection exists. A read pump (or any
// caller holding a *Connection) can compare its own Connection.Gen() against
// this value to detect that it has been superseded by a more recent
// Register call and should exit before mutating shared state.
func (h *Hub) ActiveGen(nodeID string) uint64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if c, ok := h.connections[nodeID]; ok {
		return c.generation
	}
	return 0
}

// IsActive reports whether c is still the authoritative Connection for its
// nodeID. Returns false if the Connection has been replaced by a newer
// Register call or unregistered entirely. Cheap enough to call after every
// read in a read pump.
func (h *Hub) IsActive(c *Connection) bool {
	if c == nil {
		return false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	cur, ok := h.connections[c.NodeID]
	return ok && cur == c
}

// Gen returns the Connection's generation token. Useful for callers that
// want to compare their captured generation against Hub.ActiveGen at a
// later point.
func (c *Connection) Gen() uint64 {
	return c.generation
}

// UpdateHeartbeat updates the last heartbeat time for a node.
func (h *Hub) UpdateHeartbeat(nodeID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	c, exists := h.connections[nodeID]
	if !exists {
		return false
	}

	c.LastHB = time.Now()
	h.heartbeatTotal.Inc()
	// P1#19: spec-compliant per-message-type counter. The hub only directly
	// observes heartbeats (other message types are dispatched in the WS
	// handler), so heartbeat is the only label populated from this path.
	MetricsNodeMessages.WithLabelValues("heartbeat").Inc()
	return true
}

// Broadcast sends a message to a specific node.
// Returns false if the node is not connected.
func (h *Hub) Broadcast(nodeID string, msg []byte) bool {
	h.mu.RLock()
	c, exists := h.connections[nodeID]
	h.mu.RUnlock()

	if !exists {
		return false
	}

	select {
	case c.SendCh <- msg:
		// P1#19: outbound dispatch counted under the "broadcast" type so the
		// gateway_node_messages_total series tracks both inbound and outbound
		// traffic without needing ws.go instrumentation.
		MetricsNodeMessages.WithLabelValues("broadcast").Inc()
		return true
	default:
		// Send channel full, log and skip
		MetricsNodeMessages.WithLabelValues("broadcast_dropped").Inc()
		h.logger.Warn("send channel full, dropping message", "node_id", nodeID)
		return false
	}
}

// BroadcastAll sends a message to all connected nodes.
func (h *Hub) BroadcastAll(msg []byte) {
	h.mu.RLock()
	conns := make([]*Connection, 0, len(h.connections))
	for _, c := range h.connections {
		conns = append(conns, c)
	}
	h.mu.RUnlock()

	for _, c := range conns {
		select {
		case c.SendCh <- msg:
		default:
			h.logger.Warn("send channel full during broadcast", "node_id", c.NodeID)
		}
	}
}

// GetConnection returns a connection by nodeID.
func (h *Hub) GetConnection(nodeID string) (*Connection, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	c, exists := h.connections[nodeID]
	return c, exists
}

// GetAllNodeIDs returns the IDs of all currently connected nodes.
func (h *Hub) GetAllNodeIDs() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]string, 0, len(h.connections))
	for id := range h.connections {
		ids = append(ids, id)
	}
	return ids
}

// Count returns the number of active connections.
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

// CheckHeartbeats scans all connections and disconnects stale ones.
// Should be called periodically (e.g., every 10 seconds).
func (h *Hub) CheckHeartbeats(ctx context.Context) {
	h.mu.RLock()
	staleNodes := make([]string, 0)
	now := time.Now()
	for nodeID, c := range h.connections {
		if now.Sub(c.LastHB) > h.heartbeatTTL {
			staleNodes = append(staleNodes, nodeID)
		}
	}
	h.mu.RUnlock()

	// Unregister stale nodes
	for _, nodeID := range staleNodes {
		h.Unregister(nodeID, "heartbeat_timeout")
	}
}

// StartHeartbeatMonitor starts a goroutine that periodically checks for stale connections.
func (h *Hub) StartHeartbeatMonitor(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	h.logger.Info("heartbeat monitor started", "interval", "10s", "ttl", h.heartbeatTTL)

	for {
		select {
		case <-ctx.Done():
			h.logger.Info("heartbeat monitor stopped")
			return
		case <-ticker.C:
			h.CheckHeartbeats(ctx)
		}
	}
}

// Close closes a connection and signals the close channel.
func (c *Connection) Close() {
	c.closeOnce.Do(func() {
		close(c.closeCh)
		close(c.SendCh)
		if c.Conn != nil {
			c.Conn.Close()
		}
	})
}

// IsClosed returns true if the connection has been closed.
func (c *Connection) IsClosed() bool {
	select {
	case <-c.closeCh:
		return true
	default:
		return false
	}
}

// UpdateNodeHeartbeat updates the last_heartbeat_at timestamp in the database
// and sets the node status to 'active'.
func (h *Hub) UpdateNodeHeartbeat(ctx context.Context, pool *pgxpool.Pool, nodeID string) error {
	query := `UPDATE enrolled_nodes SET last_seen_at = NOW(), status = 'active' WHERE node_id = $1`
	_, err := pool.Exec(ctx, query, nodeID)
	return err
}
