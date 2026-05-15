// Package hub manages WebSocket connections to agent nodes.
package hub

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Connection represents a single agent node WebSocket connection.
type Connection struct {
	NodeID      string
	Conn        *websocket.Conn
	LastHB      time.Time       // last heartbeat timestamp
	SendCh      chan []byte     // outbound message channel
	closeCh     chan struct{}   // signal connection close
	closeOnce   sync.Once
}

// Hub manages all active agent connections.
type Hub struct {
	mu              sync.RWMutex
	connections     map[string]*Connection  // nodeID -> Connection
	heartbeatTTL    time.Duration
	logger          *slog.Logger

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
		heartbeatTTL:    heartbeatTTL,
		logger:          logger,
		connGauge:       connGauge,
		heartbeatTotal:  heartbeatTotal,
		disconnectTotal: disconnectTotal,
	}
}

// Register adds a new connection to the hub.
func (h *Hub) Register(nodeID string, conn *websocket.Conn) *Connection {
	h.mu.Lock()
	defer h.mu.Unlock()

	c := &Connection{
		NodeID:  nodeID,
		Conn:    conn,
		LastHB:  time.Now(),
		SendCh:  make(chan []byte, 256),
		closeCh: make(chan struct{}),
	}

	h.connections[nodeID] = c
	h.connGauge.Inc()

	h.logger.Info("node registered", "node_id", nodeID, "total_connections", len(h.connections))
	return c
}

// Unregister removes a connection from the hub.
func (h *Hub) Unregister(nodeID string, reason string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	c, exists := h.connections[nodeID]
	if !exists {
		return
	}

	delete(h.connections, nodeID)
	h.connGauge.Dec()
	h.disconnectTotal.WithLabelValues(reason).Inc()

	// Close the connection
	c.Close()

	h.logger.Info("node unregistered", "node_id", nodeID, "reason", reason, "total_connections", len(h.connections))
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
		return true
	default:
		// Send channel full, log and skip
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
