// Package processor writes probe results from Redis Stream messages to TimescaleDB.
package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kite365/idcd/apps/aggregator/internal/dedup"
	"github.com/kite365/idcd/lib/shared/idgen"
)

// Processor parses stream messages and persists probe results.
type Processor struct {
	pool     *pgxpool.Pool
	dedupr   *dedup.Deduper
	trigger  *AlertTrigger
	baseline *BaselineUpdater
	anchor   *AnchorChecker
}

// New creates a Processor. dedupr may be nil to disable dedup (for testing).
func New(pool *pgxpool.Pool, dedupr *dedup.Deduper) *Processor {
	return &Processor{
		pool:     pool,
		dedupr:   dedupr,
		baseline: newBaselineUpdater(pool),
		anchor:   newAnchorChecker(pool),
	}
}

// NewWithTrigger creates a Processor with alert triggering enabled.
func NewWithTrigger(pool *pgxpool.Pool, dedupr *dedup.Deduper, trigger *AlertTrigger) *Processor {
	return &Processor{
		pool:     pool,
		dedupr:   dedupr,
		trigger:  trigger,
		baseline: newBaselineUpdater(pool),
		anchor:   newAnchorChecker(pool),
	}
}

// MonitorCheckStatus maps a probe result to a monitor check status.
func probeSuccessToCheckStatus(success bool, errMsg string) string {
	if success {
		return "up"
	}
	if errMsg != "" {
		return "down"
	}
	return "degraded"
}

// Process handles a single stream message:
//  1. Dedup by task_id+node_id composite key.
//  2. Insert into probe_result hypertable.
//  3. Transition probe_task to "completed" if applicable.
//  4. If monitor_id is non-empty, write a monitor_checks row and update monitors.next_check_at.
func (p *Processor) Process(ctx context.Context, msgID string, values map[string]any) error {
	taskID, _ := values["task_id"].(string)
	nodeID, _ := values["node_id"].(string)
	if taskID == "" || nodeID == "" {
		return fmt.Errorf("processor: missing task_id or node_id in msg %s", msgID)
	}

	dedupKey := taskID + ":" + nodeID

	// 先做 fast-path check（命中即跳过，避免无谓 DB 写）；但只在 DB 写入成功后再
	// MarkProcessed，否则瞬时 DB 故障会把 dedup key 锁死 24h、消息进入 PEL 后
	// 永远被 "duplicate, skip" 静默丢弃。
	if p.dedupr != nil {
		dup, err := p.dedupr.IsDuplicate(ctx, dedupKey)
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

	summaryJSON, _ := json.Marshal(result.summary)
	if err := p.completeProbeTask(ctx, taskID, summaryJSON); err != nil {
		return fmt.Errorf("processor: complete probe_task: %w", err)
	}

	// DB 已落库，标记 dedup（并发 consumer 已通过 ON CONFLICT 收敛，这里 SetNX 返
	// false 也无碍——业务上已完成）。失败不影响主流程，只多一次后续 reclaim 时
	// 的幂等 INSERT。
	if p.dedupr != nil {
		if err := p.dedupr.MarkProcessed(ctx, dedupKey); err != nil {
			slog.Warn("processor: mark dedup failed (non-fatal)", "key", dedupKey, "err", err)
		}
	}

	// If this result belongs to a monitor-originated task, write monitor_checks
	// and advance the monitor's next_check_at schedule.
	if monitorID, _ := values["monitor_id"].(string); monitorID != "" {
		if err := p.writeMonitorCheck(ctx, monitorID, nodeID, result); err != nil {
			// Non-fatal: log and continue — don't fail the whole message.
			slog.Warn("processor: write monitor_check failed", "monitor_id", monitorID, "err", err)
		}
		if err := p.advanceMonitorSchedule(ctx, monitorID, taskID); err != nil {
			slog.Warn("processor: advance monitor schedule failed", "monitor_id", monitorID, "err", err)
		}
		if p.trigger != nil {
			checkStatus := probeSuccessToCheckStatus(result.success, result.errMsg)
			p.trigger.CheckAndTrigger(ctx, monitorID, checkStatus)
		}
		if p.baseline != nil {
			_ = p.baseline.UpdateBaseline(ctx, monitorID)
		}
		if p.anchor != nil {
			_ = p.anchor.CheckDeviation(ctx, monitorID, float64(result.durationMs), result.success)
		}
	}

	return nil
}

// writeMonitorCheck inserts a row into monitor_checks.
func (p *Processor) writeMonitorCheck(ctx context.Context, monitorID, nodeID string, r probeResultData) error {
	if p.pool == nil {
		return nil
	}
	status := probeSuccessToCheckStatus(r.success, r.errMsg)
	latencyMS := int(r.durationMs)

	metadataJSON, _ := json.Marshal(r.summary)

	var errPtr *string
	if r.errMsg != "" {
		errPtr = &r.errMsg
	}

	_, err := p.pool.Exec(ctx, `
		INSERT INTO monitor_checks (check_at, monitor_id, node_id, status, latency_ms, error, metadata)
		VALUES (NOW(), $1, $2, $3, $4, $5, $6)
	`, monitorID, nodeID, status, latencyMS, errPtr, metadataJSON)
	return err
}

// advanceMonitorSchedule updates monitors.last_check_at and next_check_at.
// It reads the interval_s from the monitors table to compute the next schedule.
func (p *Processor) advanceMonitorSchedule(ctx context.Context, monitorID, taskID string) error {
	if p.pool == nil {
		return nil
	}
	_, err := p.pool.Exec(ctx, `
		UPDATE monitors
		SET last_check_at = NOW(),
		    next_check_at = NOW() + make_interval(secs => interval_s::float8),
		    updated_at = NOW()
		WHERE id = $1 AND status = 'active'
	`, monitorID)
	return err
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

// completeProbeTask transitions a probe_task from "running" to "completed" and writes the result.
func (p *Processor) completeProbeTask(ctx context.Context, taskID string, resultJSON []byte) error {
	_, err := p.pool.Exec(ctx, `
		UPDATE probe_task
		SET status = 'completed', completed_at = NOW(), result = $2
		WHERE id = $1 AND status = 'running'
	`, taskID, resultJSON)
	return err
}
