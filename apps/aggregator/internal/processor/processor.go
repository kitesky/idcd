// Package processor writes probe results from Redis Stream messages to TimescaleDB.
package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kite365/idcd/apps/aggregator/internal/dedup"
	"github.com/kite365/idcd/lib/shared/idgen"
)

// Processor parses stream messages and persists probe results.
type Processor struct {
	pool   *pgxpool.Pool
	dedupr *dedup.Deduper
}

// New creates a Processor. dedupr may be nil to disable dedup (for testing).
func New(pool *pgxpool.Pool, dedupr *dedup.Deduper) *Processor {
	return &Processor{pool: pool, dedupr: dedupr}
}

// Process handles a single stream message:
//  1. Dedup by task_id+node_id composite key.
//  2. Insert into probe_result hypertable.
//  3. Transition probe_task to "completed" if applicable.
func (p *Processor) Process(ctx context.Context, msgID string, values map[string]any) error {
	taskID, _ := values["task_id"].(string)
	nodeID, _ := values["node_id"].(string)
	if taskID == "" || nodeID == "" {
		return fmt.Errorf("processor: missing task_id or node_id in msg %s", msgID)
	}

	dedupKey := taskID + ":" + nodeID

	if p.dedupr != nil {
		dup, err := p.dedupr.IsProcessedAndMark(ctx, dedupKey)
		if err != nil {
			return fmt.Errorf("processor: dedup check for %s: %w", dedupKey, err)
		}
		if dup {
			return nil // already processed, skip silently
		}
	}

	result := parseResult(values)

	if err := p.insertProbeResult(ctx, taskID, nodeID, result); err != nil {
		return fmt.Errorf("processor: insert probe_result: %w", err)
	}

	if err := p.completeProbeTask(ctx, taskID); err != nil {
		return fmt.Errorf("processor: complete probe_task: %w", err)
	}

	return nil
}

// probeResultData holds parsed values from the stream message.
type probeResultData struct {
	raw        map[string]any
	summary    map[string]any
	durationMs int32
	success    bool
	errMsg     string
	signature  string
}

func parseResult(values map[string]any) probeResultData {
	r := probeResultData{}

	if raw, ok := values["raw"]; ok {
		switch v := raw.(type) {
		case string:
			_ = json.Unmarshal([]byte(v), &r.raw)
		case map[string]any:
			r.raw = v
		}
	}
	if r.raw == nil {
		r.raw = make(map[string]any)
	}

	if summary, ok := values["summary"]; ok {
		switch v := summary.(type) {
		case string:
			_ = json.Unmarshal([]byte(v), &r.summary)
		case map[string]any:
			r.summary = v
		}
	}
	if r.summary == nil {
		r.summary = make(map[string]any)
	}

	if d, ok := values["duration_ms"]; ok {
		switch v := d.(type) {
		case int64:
			r.durationMs = int32(v)
		case float64:
			r.durationMs = int32(v)
		case string:
			if n, err := strconv.ParseInt(v, 10, 32); err == nil {
				r.durationMs = int32(n)
			}
		}
	}

	if s, ok := values["success"]; ok {
		switch v := s.(type) {
		case bool:
			r.success = v
		case string:
			r.success = v == "true" || v == "1"
		}
	}

	if e, ok := values["error"]; ok {
		r.errMsg, _ = e.(string)
	}

	if sig, ok := values["signature"]; ok {
		r.signature, _ = sig.(string)
	}

	return r
}

func (p *Processor) insertProbeResult(ctx context.Context, taskID, nodeID string, r probeResultData) error {
	rawJSON, _ := json.Marshal(r.raw)
	summaryJSON, _ := json.Marshal(r.summary)

	id := idgen.New("pr")
	now := time.Now().UTC()

	_, err := p.pool.Exec(ctx, `
		INSERT INTO probe_result
			(id, task_id, node_id, raw, summary, duration_ms, success, error, signature, created_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (task_id, node_id, created_at) DO NOTHING
	`, id, taskID, nodeID, rawJSON, summaryJSON, r.durationMs, r.success, r.errMsg, r.signature, now)

	return err
}

// completeProbeTask transitions a probe_task from "running" to "completed".
// For S1, we mark completed on the first result received (single-node tasks).
func (p *Processor) completeProbeTask(ctx context.Context, taskID string) error {
	_, err := p.pool.Exec(ctx, `
		UPDATE probe_task
		SET status = 'completed', completed_at = NOW()
		WHERE id = $1 AND status = 'running'
	`, taskID)
	return err
}
