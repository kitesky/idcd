package processor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/aggregator/internal/dedup"
)

// --- monitor check helper tests ---

func TestProbeSuccessToCheckStatus(t *testing.T) {
	tests := []struct {
		success bool
		errMsg  string
		want    string
	}{
		{true, "", "up"},
		{false, "timeout", "down"},
		{false, "", "degraded"},
	}
	for _, tc := range tests {
		got := probeSuccessToCheckStatus(tc.success, tc.errMsg)
		if got != tc.want {
			t.Errorf("probeSuccessToCheckStatus(%v, %q) = %q, want %q", tc.success, tc.errMsg, got, tc.want)
		}
	}
}

func TestProcessor_Process_withMonitorID_nilPool(t *testing.T) {
	// When pool is nil, writeMonitorCheck and advanceMonitorSchedule should no-op.
	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	dedupr := dedup.New(rdb)

	p := New(nil, dedupr)
	ctx := context.Background()

	// With nil pool, insertProbeResult will fail — use a deduped key to skip it.
	dedupKey := "pt_mon_01:nd_jp_01"
	if err := dedupr.MarkProcessed(ctx, dedupKey); err != nil {
		t.Fatalf("pre-mark: %v", err)
	}

	err := p.Process(ctx, "1-1", map[string]any{
		"task_id":    "pt_mon_01",
		"node_id":    "nd_jp_01",
		"monitor_id": "mon_001",
		"success":    "true",
		"duration_ms": "200",
	})
	// Should be nil because message is deduped (no DB ops attempted).
	if err != nil {
		t.Errorf("expected nil (dedup skip with monitor_id), got %v", err)
	}
}

func TestParseResult(t *testing.T) {
	t.Run("string JSON fields", func(t *testing.T) {
		raw := map[string]any{"status": 200}
		rawJSON, _ := json.Marshal(raw)
		summary := map[string]any{"latency_ms": 42}
		summaryJSON, _ := json.Marshal(summary)

		values := map[string]any{
			"task_id":     "pt_001",
			"node_id":     "nd_jp_01",
			"raw":         string(rawJSON),
			"summary":     string(summaryJSON),
			"duration_ms": "150",
			"success":     "true",
			"error":       "",
			"signature":   "abc123",
		}

		r := parseResult(values)

		if r.durationMs != 150 {
			t.Errorf("durationMs: got %d, want 150", r.durationMs)
		}
		if !r.success {
			t.Error("success: expected true")
		}
		if r.signature != "abc123" {
			t.Errorf("signature: got %q", r.signature)
		}
		if _, ok := r.raw["status"]; !ok {
			t.Error("raw.status not parsed")
		}
		if _, ok := r.summary["latency_ms"]; !ok {
			t.Error("summary.latency_ms not parsed")
		}
	})

	t.Run("native map fields", func(t *testing.T) {
		values := map[string]any{
			"raw":         map[string]any{"code": 200},
			"summary":     map[string]any{"ok": true},
			"duration_ms": int64(250),
			"success":     true,
		}

		r := parseResult(values)

		if r.durationMs != 250 {
			t.Errorf("durationMs: got %d, want 250", r.durationMs)
		}
		if !r.success {
			t.Error("success: expected true")
		}
	})

	t.Run("missing optional fields default safely", func(t *testing.T) {
		values := map[string]any{
			"task_id": "pt_001",
			"node_id": "nd_jp_01",
		}

		r := parseResult(values)

		if r.durationMs != 0 {
			t.Errorf("durationMs: expected 0, got %d", r.durationMs)
		}
		if r.success {
			t.Error("success: expected false for missing field")
		}
		if r.raw == nil {
			t.Error("raw: expected non-nil map")
		}
		if r.summary == nil {
			t.Error("summary: expected non-nil map")
		}
	})
}

func TestProcessor_Process_missingTaskID(t *testing.T) {
	p := New(nil, nil)

	err := p.Process(context.Background(), "1-1", map[string]any{
		"node_id": "nd_jp_01",
	})

	if err == nil {
		t.Error("expected error for missing task_id, got nil")
	}
}

func TestProcessor_Process_missingNodeID(t *testing.T) {
	p := New(nil, nil)

	err := p.Process(context.Background(), "1-1", map[string]any{
		"task_id": "pt_001",
	})

	if err == nil {
		t.Error("expected error for missing node_id, got nil")
	}
}

func TestProcessor_Process_dedupSkips(t *testing.T) {
	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	dedupr := dedup.New(rdb)

	ctx := context.Background()
	key := "pt_001:nd_jp_01"

	// Pre-mark the key as already processed.
	if err := dedupr.MarkProcessed(ctx, key); err != nil {
		t.Fatalf("pre-mark: %v", err)
	}

	// With nil pool: if dedup fires first (returns duplicate=true), Process
	// returns nil without touching the pool.
	p := New(nil, dedupr)
	err := p.Process(ctx, "1-1", map[string]any{
		"task_id": "pt_001",
		"node_id": "nd_jp_01",
	})
	if err != nil {
		t.Errorf("expected nil (dedup skip), got %v", err)
	}
}
