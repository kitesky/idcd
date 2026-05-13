package stream_test

import (
	"context"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/packages/shared/stream"
)

// newTestClient spins up a miniredis instance and returns a stream.Client.
// The miniredis server is automatically stopped when the test ends.
func newTestClient(t *testing.T) *stream.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return stream.New(rdb)
}

func TestNewFromConfig(t *testing.T) {
	mr := miniredis.RunT(t)
	c, rdb := stream.NewFromConfig(mr.Addr(), "", 0)
	defer rdb.Close()
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("NewFromConfig: Ping failed: %v", err)
	}
}

func TestPing(t *testing.T) {
	c := newTestClient(t)
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestAdd_returnsID(t *testing.T) {
	c := newTestClient(t)
	id, err := c.Add(context.Background(), "test.stream", map[string]any{"k": "v"})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if id == "" {
		t.Error("Add should return a non-empty message ID")
	}
	// Redis stream IDs are "<ms>-<seq>"
	if !strings.Contains(id, "-") {
		t.Errorf("unexpected ID format: %q", id)
	}
}

func TestLen_empty(t *testing.T) {
	c := newTestClient(t)
	n, err := c.Len(context.Background(), "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestLen_afterAdd(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	for range 5 {
		if _, err := c.Add(ctx, "test.len", map[string]any{"x": "y"}); err != nil {
			t.Fatal(err)
		}
	}
	n, err := c.Len(ctx, "test.len")
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Errorf("expected 5, got %d", n)
	}
}

func TestAddProbeResult(t *testing.T) {
	c := newTestClient(t)
	id, err := c.AddProbeResult(context.Background(), "pt_abc", "nd_jp_01", map[string]any{
		"duration_ms": 42,
		"success":     true,
	})
	if err != nil {
		t.Fatalf("AddProbeResult failed: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty ID")
	}
	// Verify stream has 1 entry
	n, _ := c.Len(context.Background(), stream.Probe)
	if n != 1 {
		t.Errorf("expected 1 message in %s, got %d", stream.Probe, n)
	}
}

func TestAddMonitorEvent(t *testing.T) {
	c := newTestClient(t)
	_, err := c.AddMonitorEvent(context.Background(), "m_abc", "down", map[string]any{
		"reason": "timeout",
	})
	if err != nil {
		t.Fatalf("AddMonitorEvent failed: %v", err)
	}
	n, _ := c.Len(context.Background(), stream.Monitor)
	if n != 1 {
		t.Errorf("expected 1 message in %s, got %d", stream.Monitor, n)
	}
}

func TestAddAlertEvent(t *testing.T) {
	c := newTestClient(t)
	_, err := c.AddAlertEvent(context.Background(), "ae_xyz", "m_abc", "down")
	if err != nil {
		t.Fatalf("AddAlertEvent failed: %v", err)
	}
	n, _ := c.Len(context.Background(), stream.Alert)
	if n != 1 {
		t.Errorf("expected 1 message in %s, got %d", stream.Alert, n)
	}
}

func TestAddAuditEvent(t *testing.T) {
	c := newTestClient(t)
	_, err := c.AddAuditEvent(context.Background(), map[string]any{
		"actor_user_id": "u_abc",
		"action":        "login",
		"result":        "ok",
	})
	if err != nil {
		t.Fatalf("AddAuditEvent failed: %v", err)
	}
	n, _ := c.Len(context.Background(), stream.Audit)
	if n != 1 {
		t.Errorf("expected 1 message in %s, got %d", stream.Audit, n)
	}
}

// newTestClientWithRDB is like newTestClient but also returns the underlying
// *redis.Client so tests can read back stream entries for assertion.
func newTestClientWithRDB(t *testing.T) (*stream.Client, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return stream.New(rdb), rdb
}

// newStoppedClient returns a client pointing at a stopped Redis server.
func newStoppedClient(t *testing.T) *stream.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c := stream.New(rdb)
	mr.Close()
	return c
}

func TestErrorPaths(t *testing.T) {
	c := newStoppedClient(t)
	ctx := context.Background()
	t.Run("Add", func(t *testing.T) {
		_, err := c.Add(ctx, "test.stream", map[string]any{"k": "v"})
		if err == nil {
			t.Error("expected error when Redis is unavailable")
		}
	})
	t.Run("Ping", func(t *testing.T) {
		if err := c.Ping(ctx); err == nil {
			t.Error("expected error when Redis is unavailable")
		}
	})
	t.Run("Len", func(t *testing.T) {
		_, err := c.Len(ctx, "some.stream")
		if err == nil {
			t.Error("expected error when Redis is unavailable")
		}
	})
}

func TestAddProbeResult_requiredFieldsNotOverridden(t *testing.T) {
	c, rdb := newTestClientWithRDB(t)
	ctx := context.Background()
	_, err := c.AddProbeResult(ctx, "pt_real", "nd_real", map[string]any{
		"task_id": "OVERRIDE",
		"node_id": "OVERRIDE",
		"extra":   "value",
	})
	if err != nil {
		t.Fatalf("AddProbeResult failed: %v", err)
	}
	msgs, err := rdb.XRange(ctx, stream.Probe, "-", "+").Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if got := msgs[0].Values["task_id"]; got != "pt_real" {
		t.Errorf("task_id: expected %q, got %v", "pt_real", got)
	}
	if got := msgs[0].Values["node_id"]; got != "nd_real" {
		t.Errorf("node_id: expected %q, got %v", "nd_real", got)
	}
}

func TestAddMonitorEvent_requiredFieldsNotOverridden(t *testing.T) {
	c, rdb := newTestClientWithRDB(t)
	ctx := context.Background()
	_, err := c.AddMonitorEvent(ctx, "m_real", "up", map[string]any{
		"monitor_id": "OVERRIDE",
		"event":      "OVERRIDE",
	})
	if err != nil {
		t.Fatalf("AddMonitorEvent failed: %v", err)
	}
	msgs, err := rdb.XRange(ctx, stream.Monitor, "-", "+").Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if got := msgs[0].Values["monitor_id"]; got != "m_real" {
		t.Errorf("monitor_id: expected %q, got %v", "m_real", got)
	}
	if got := msgs[0].Values["event"]; got != "up" {
		t.Errorf("event: expected %q, got %v", "up", got)
	}
}

func TestStreamConstants(t *testing.T) {
	// Ensure constants haven't been accidentally changed — these names are
	// shared across services and changing them would break consumers.
	cases := map[string]string{
		"Probe":   stream.Probe,
		"Monitor": stream.Monitor,
		"Alert":   stream.Alert,
		"Audit":   stream.Audit,
		"Usage":   stream.Usage,
	}
	expected := map[string]string{
		"Probe":   "probe.results",
		"Monitor": "monitor.events",
		"Alert":   "alert.events",
		"Audit":   "audit.events",
		"Usage":   "usage.events",
	}
	for name, got := range cases {
		if got != expected[name] {
			t.Errorf("%s constant: expected %q, got %q", name, expected[name], got)
		}
	}
}
