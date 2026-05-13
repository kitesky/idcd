// Package stream provides a Redis Streams write helper for idcd event buses.
//
// D18 decision: all XADD calls use MAXLEN ~ 500000 to cap Redis memory.
// Consumers (Aggregator, Notifier, etc.) must handle at-least-once delivery;
// producers do not wait for consumer ack.
package stream

import (
	"context"
	"fmt"
	"maps"
	"time"

	"github.com/redis/go-redis/v9"
)

// Default MAXLEN for all streams (approximate trim, efficient).
// Override per stream via Options.MaxLen.
const DefaultMaxLen = 500_000

// Well-known stream names used across idcd services.
const (
	Probe   = "probe.results"   // node probe results → Aggregator
	Monitor = "monitor.events"  // monitor state changes → Notifier
	Alert   = "alert.events"    // alert events → Notifier / SSE fanout
	Audit   = "audit.events"    // audit log entries → AuditLog writer
	Usage   = "usage.events"    // API usage events → billing/quota
)

// Client wraps a Redis client for Streams operations.
type Client struct {
	rdb    redis.Cmdable
	maxLen int64
}

// New creates a Client using the provided redis.Cmdable
// (accepts *redis.Client or *redis.ClusterClient).
func New(rdb redis.Cmdable) *Client {
	return &Client{rdb: rdb, maxLen: DefaultMaxLen}
}

// NewFromConfig creates a *redis.Client and wraps it in a Client.
func NewFromConfig(addr, password string, db int) (*Client, *redis.Client) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
	})
	return New(rdb), rdb
}

// Add appends a message to stream with MAXLEN ~ DefaultMaxLen (D18).
// Returns the message ID assigned by Redis.
func (c *Client) Add(ctx context.Context, stream string, values map[string]any) (string, error) {
	id, err := c.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		MaxLen: c.maxLen,
		Approx: true, // ~ flag: efficient probabilistic trimming
		ID:     "*",  // auto-generate ID
		Values: values,
	}).Result()
	if err != nil {
		return "", fmt.Errorf("stream.Add %q: %w", stream, err)
	}
	return id, nil
}

// AddProbeResult writes a probe result to the probe.results stream.
func (c *Client) AddProbeResult(ctx context.Context, taskID, nodeID string, payload map[string]any) (string, error) {
	vals := make(map[string]any, len(payload)+2)
	maps.Copy(vals, payload)
	vals["task_id"] = taskID
	vals["node_id"] = nodeID
	return c.Add(ctx, Probe, vals)
}

// AddMonitorEvent writes a monitor state-change event.
func (c *Client) AddMonitorEvent(ctx context.Context, monitorID, event string, extra map[string]any) (string, error) {
	vals := make(map[string]any, len(extra)+3)
	maps.Copy(vals, extra)
	vals["monitor_id"] = monitorID
	vals["event"] = event
	vals["ts"] = time.Now().UnixMilli()
	return c.Add(ctx, Monitor, vals)
}

// AddAlertEvent writes an alert event.
func (c *Client) AddAlertEvent(ctx context.Context, alertEventID, monitorID, kind string) (string, error) {
	return c.Add(ctx, Alert, map[string]any{
		"alert_event_id": alertEventID,
		"monitor_id":     monitorID,
		"kind":           kind,
		"ts":             time.Now().UnixMilli(),
	})
}

// AddAuditEvent writes an audit entry to the audit.events stream.
func (c *Client) AddAuditEvent(ctx context.Context, vals map[string]any) (string, error) {
	return c.Add(ctx, Audit, vals)
}

// Len returns the current length of a stream (for monitoring / alerts).
func (c *Client) Len(ctx context.Context, stream string) (int64, error) {
	n, err := c.rdb.XLen(ctx, stream).Result()
	if err != nil {
		return 0, fmt.Errorf("stream.Len %q: %w", stream, err)
	}
	return n, nil
}

// Ping checks the Redis connection is alive.
func (c *Client) Ping(ctx context.Context) error {
	if _, err := c.rdb.Ping(ctx).Result(); err != nil {
		return fmt.Errorf("stream.Ping: %w", err)
	}
	return nil
}
