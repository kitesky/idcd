// Package scheduler implements the main scheduling loop for probe tasks.
//
// As of 2026-05-16 the only active path is the monitor poller: it queries the
// database for monitors that are due and pushes probe_task entries directly to
// the `probe.tasks` Redis Stream. Ad-hoc tool probes are pushed to the same
// stream by the API handler, bypassing the scheduler entirely.
//
// The Scheduler also owns leader election. When leadership is lost the work
// context is cancelled immediately so the monitor poller stops in <1s rather
// than waiting for the next 30s tick (avoids a split-brain window where two
// scheduler instances could both be polling).
package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kite365/idcd/apps/scheduler/internal/leader"
	"github.com/kite365/idcd/apps/scheduler/internal/queue"
	"github.com/kite365/idcd/lib/shared/idgen"
	"github.com/kite365/idcd/lib/shared/stream"
)

// ProbeTasksStream is re-exported here for backward compatibility with existing
// callers / tests. The canonical name lives in lib/shared/stream — new code
// should reference stream.ProbeTasks directly.
const ProbeTasksStream = stream.ProbeTasks

const (
	// monitorPollInterval is how often the monitor poller queries for due
	// monitors. Kept short enough that a leader transition + the next poll
	// stays within the user-perceived monitor cadence.
	monitorPollInterval = 30 * time.Second

	// leaderRenewInterval controls how often the current leader extends its
	// Redis lock. Must be < leaderTTL by a comfortable margin.
	leaderRenewInterval = 2 * time.Second
)

// DueMonitor is the minimal projection of a monitors row needed by the poller.
type DueMonitor struct {
	ID        string
	Type      string
	Target    string
	IntervalS int32
	NodeCount int32
	Config    json.RawMessage
}

// MonitorStore is the interface for querying due monitors.
// Implemented by the real DB and by fakes in tests.
type MonitorStore interface {
	// ListActiveMonitorsDue returns monitors whose next_check_at <= NOW().
	ListActiveMonitorsDue(ctx context.Context) ([]DueMonitor, error)
}

// NodeSelector selects a node to execute a task.
// S1 implementation: random selection from a static list.
// S2: will query DB for online nodes with capacity.
type NodeSelector interface {
	SelectNode(ctx context.Context, task *queue.ProbeTask) (string, error)
}

// Scheduler orchestrates task scheduling.
//
// Today the scheduler owns two responsibilities:
//   - Leader election via Redis SETNX (only the leader does any work).
//   - Monitor polling: query the DB every monitorPollInterval and push
//     probe_tasks to the `probe.tasks` Redis Stream.
type Scheduler struct {
	leader       *leader.Leader
	selector     NodeSelector
	stream       *stream.Client
	pool         *pgxpool.Pool
	monitorStore MonitorStore
	nodeID       string // optional, used to label scheduler_is_leader{node}
	logger       *slog.Logger

	// epoch is the fencing token claimed at scheduler startup (see
	// leader.AcquireEpoch). Tagged onto every probe.tasks stream message so
	// the gateway dispatcher can reject writes from a "deposed" leader that
	// hasn't yet noticed it lost the Redis lock (split-brain defence —
	// docs/prd/ARCHITECTURE-REVIEW-2026-05-21.md P0-2).
	//
	// Zero is treated as "not set" by downstream consumers (backward-compat
	// window for older schedulers that haven't been redeployed yet). Once all
	// schedulers in a cluster have been upgraded, the consumer-side
	// "missing epoch = accept" branch can be tightened to "missing epoch =
	// reject" — track via the idcd_gateway_stale_epoch_total{reason="missing"}
	// counter dropping to zero.
	epoch leader.FencingToken
}

// Config holds Scheduler configuration.
type Config struct {
	Leader       *leader.Leader
	Selector     NodeSelector
	Stream       *stream.Client
	Pool         *pgxpool.Pool
	MonitorStore MonitorStore // optional; set to enable monitor polling
	// NodeID identifies this scheduler replica in metric labels. Optional —
	// when empty the leader gauge falls back to "unknown" so the label cardinality
	// stays bounded if the wiring isn't fully plumbed yet.
	NodeID string
	// Logger is the structured logger used by the scheduler. Optional — when
	// nil it falls back to slog.Default() so existing callers (and tests) keep
	// working without forcing them to wire a logger.
	Logger *slog.Logger
	// Epoch is the fencing token claimed by main.go via leader.AcquireEpoch
	// before constructing the Scheduler. Optional — when zero the scheduler
	// will still run but stream messages will be tagged with epoch=0, which
	// downstream consumers treat as "legacy, accept + warn" during the
	// backward-compat window.
	Epoch leader.FencingToken
}

// New creates a Scheduler instance.
func New(cfg Config) *Scheduler {
	lg := cfg.Logger
	if lg == nil {
		lg = slog.Default().With("component", "scheduler")
	}
	return &Scheduler{
		leader:       cfg.Leader,
		selector:     cfg.Selector,
		stream:       cfg.Stream,
		pool:         cfg.Pool,
		monitorStore: cfg.MonitorStore,
		nodeID:       cfg.NodeID,
		logger:       lg,
		epoch:        cfg.Epoch,
	}
}

// SetLogger swaps the scheduler's structured logger. Useful for tests or for
// late-binding a request-scoped logger; safe to call before Run.
func (s *Scheduler) SetLogger(l *slog.Logger) {
	if l == nil {
		return
	}
	s.logger = l
}

// metricNode returns the label value to use for scheduler_is_leader{node}.
// Bounded fallback prevents an empty-string label series leaking into Prometheus.
func (s *Scheduler) metricNode() string {
	if s.nodeID == "" {
		return "unknown"
	}
	return s.nodeID
}

// Run starts the scheduler loop.
// Only runs if this instance is the leader.
// Blocks until ctx is cancelled (or leadership is lost).
func (s *Scheduler) Run(ctx context.Context) error {
	s.logger.Info("starting scheduler loop")

	// Try to acquire leadership
	isLeader, err := s.leader.Acquire(ctx)
	if err != nil {
		MetricsIsLeader.WithLabelValues(s.metricNode()).Set(0)
		return fmt.Errorf("scheduler.Run: acquire leadership: %w", err)
	}
	if !isLeader {
		MetricsIsLeader.WithLabelValues(s.metricNode()).Set(0)
		s.logger.Info("not the leader, exiting")
		return nil
	}

	MetricsIsLeader.WithLabelValues(s.metricNode()).Set(1)
	s.logger.Info("acquired leadership, starting work goroutines")

	// workCtx is cancelled either when the parent ctx is cancelled OR when
	// renewLeadership detects we lost the lock. All work goroutines (today:
	// the monitor poller) must select on workCtx.Done() so they stop
	// immediately on leader loss — this is what closes the split-brain
	// window.
	workCtx, cancelWork := context.WithCancel(ctx)
	defer cancelWork()

	// Start leader renewal goroutine; it cancels workCtx on renewal failure.
	go s.renewLeadership(workCtx, cancelWork)

	// Start monitor poller goroutine if a MonitorStore was provided.
	if s.monitorStore != nil {
		go s.monitorPoller(workCtx)
	}

	// Block until either the parent ctx is cancelled (shutdown signal) or
	// leadership is lost (renewLeadership cancels workCtx).
	<-workCtx.Done()
	if ctx.Err() != nil {
		s.logger.Info("context cancelled, releasing leadership")
	} else {
		s.logger.Info("leadership lost, stopping")
	}

	// Release leadership best-effort. Use a fresh background ctx because
	// workCtx is already cancelled.
	releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.leader.Release(releaseCtx); err != nil {
		s.logger.Error("failed to release leadership", "err", err)
	}
	MetricsIsLeader.WithLabelValues(s.metricNode()).Set(0)

	// If the parent context was cancelled, surface that error. If we exited
	// purely because leadership was lost, return nil (clean exit, the orchestrator
	// can restart us and we'll re-attempt acquisition).
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

// renewLeadership periodically renews the leader lock. On any renewal failure
// it cancels workCtx via cancelWork, which signals every other goroutine to
// stop. The previous implementation only returned, which left the monitor
// poller running on its 30s ticker for up to 30s after leadership was lost —
// during which a second instance could acquire the lock and both would poll
// in parallel (split brain).
func (s *Scheduler) renewLeadership(ctx context.Context, cancelWork context.CancelFunc) {
	ticker := time.NewTicker(leaderRenewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.leader.Renew(ctx); err != nil {
				MetricsLeaderRenewals.WithLabelValues("fail").Inc()
				MetricsIsLeader.WithLabelValues(s.metricNode()).Set(0)
				s.logger.Error("failed to renew leadership, cancelling work", "err", err)
				cancelWork()
				return
			}
			MetricsLeaderRenewals.WithLabelValues("ok").Inc()
		}
	}
}

// monitorPoller polls the DB every monitorPollInterval for monitors that are
// due for their next check. For each due monitor it generates probe_tasks and
// pushes them to the probe.tasks Redis Stream.
//
// Returns as soon as ctx is cancelled (parent shutdown OR leader loss).
func (s *Scheduler) monitorPoller(ctx context.Context) {
	s.logger.Info("monitor poller started")
	defer s.logger.Info("monitor poller stopped")

	ticker := time.NewTicker(monitorPollInterval)
	defer ticker.Stop()

	// Run immediately on startup, then on ticker cadence. Check ctx first
	// so we never poll if we were already cancelled.
	if ctx.Err() != nil {
		return
	}
	s.pollMonitors(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !s.leader.IsLeader() {
				return
			}
			s.pollMonitors(ctx)
		}
	}
}

// pollMonitors fetches due monitors and dispatches probe tasks for each.
func (s *Scheduler) pollMonitors(ctx context.Context) {
	// Cheap fail-fast before hitting the DB.
	if err := ctx.Err(); err != nil {
		return
	}

	start := time.Now()
	defer func() {
		MetricsPollDuration.Observe(time.Since(start).Seconds())
	}()

	monitors, err := s.monitorStore.ListActiveMonitorsDue(ctx)
	if err != nil {
		MetricsMonitorPolls.WithLabelValues("error").Inc()
		s.logger.Error("monitor poller: list due monitors failed", "err", err)
		return
	}
	MetricsMonitorPolls.WithLabelValues("ok").Inc()

	for _, m := range monitors {
		// Bail out mid-loop if leadership was lost while we were dispatching.
		if err := ctx.Err(); err != nil {
			return
		}
		if err := s.dispatchMonitorTask(ctx, m); err != nil {
			s.logger.Error("monitor poller: dispatch monitor failed", "monitor_id", m.ID, "err", err)
		}
	}
}

// dispatchMonitorTask creates probe tasks for a due monitor — one per requested node.
// We push directly to the probe.tasks stream; monitor tasks should be dispatched immediately.
func (s *Scheduler) dispatchMonitorTask(ctx context.Context, m DueMonitor) error {
	// Determine how many nodes to use (minimum 1).
	count := int(m.NodeCount)
	if count < 1 {
		count = 1
	}

	// Build base params from monitor config. Parse failure means the row
	// is corrupt — log it so we can see persistent decoding issues instead
	// of silently dispatching the monitor with an empty params object.
	baseParams := map[string]any{}
	if len(m.Config) > 0 {
		if err := json.Unmarshal(m.Config, &baseParams); err != nil {
			s.logger.Error("dispatch monitor task: malformed config", "monitor_id", m.ID, "err", err)
		}
	}

	// Map monitor type to probe type (some types share probe mechanics).
	probeType := monitorTypeToProbeType(m.Type)

	paramsJSON, err := json.Marshal(baseParams)
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}

	for i := 0; i < count; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		taskID := idgen.ProbeTask()

		// Select a node via the configured NodeSelector.
		task := &queue.ProbeTask{
			ID:        taskID,
			Type:      probeType,
			Target:    m.Target,
			Priority:  queue.P2,
			MonitorID: m.ID,
		}
		nodeID, err := s.selector.SelectNode(ctx, task)
		if err != nil {
			s.logger.Error("dispatch monitor task: select node failed", "monitor_id", m.ID, "err", err)
			continue
		}
		task.NodeID = nodeID

		// LINT-IGNORE: stream-payload-legacy
		// probe.tasks has no typed contract yet (only ProbeResult / MonitorEvent
		// are typed under lib/shared/contracts). When that contract lands we
		// will fold "epoch" into the strongly-typed payload — for now it
		// rides as a plain string field next to the other map values.
		// "epoch" is the scheduler fencing token; see leader.AcquireEpoch and
		// the consumer-side check in gateway dispatcher.
		vals := map[string]any{
			"task_id":    taskID,
			"type":       probeType,
			"target":     m.Target,
			"node_id":    nodeID,
			"priority":   queue.P2,
			"monitor_id": m.ID,
			"params":     string(paramsJSON),
			"epoch":      s.epoch.String(),
		}
		if _, err := s.stream.Add(ctx, ProbeTasksStream, vals); err != nil {
			s.logger.Error("dispatch monitor task: push to stream failed", "monitor_id", m.ID, "err", err)
		}
	}
	return nil
}

// monitorTypeToProbeType maps a monitor type to the corresponding probe type.
func monitorTypeToProbeType(monType string) string {
	switch monType {
	case "http", "https", "keyword", "ssl_expiry", "domain_expiry", "icp_change":
		return "http"
	case "ping":
		return "ping"
	case "tcp":
		return "tcp"
	case "dns":
		return "dns"
	default:
		return "http"
	}
}

// --- S1 Simple Node Selector ---

// StaticNodeSelector selects nodes from a static list (S1 implementation).
type StaticNodeSelector struct {
	nodes []string
	rnd   *rand.Rand
}

// NewStaticNodeSelector creates a StaticNodeSelector with the given node list.
func NewStaticNodeSelector(nodes []string) *StaticNodeSelector {
	return &StaticNodeSelector{
		nodes: nodes,
		rnd:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SelectNode randomly selects a node from the static list.
func (s *StaticNodeSelector) SelectNode(ctx context.Context, task *queue.ProbeTask) (string, error) {
	if len(s.nodes) == 0 {
		return "", fmt.Errorf("no nodes available")
	}
	idx := s.rnd.Intn(len(s.nodes))
	return s.nodes[idx], nil
}
