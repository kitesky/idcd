// Package scheduler implements the main scheduling loop for probe tasks.
package scheduler

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kite365/idcd/apps/scheduler/internal/leader"
	"github.com/kite365/idcd/apps/scheduler/internal/queue"
	"github.com/kite365/idcd/packages/shared/stream"
)

const (
	// Stream names
	ProbeTasksStream = "probe.tasks" // tasks to be executed by agents
)

// NodeSelector selects a node to execute a task.
// S1 implementation: random selection from a static list.
// S2: will query DB for online nodes with capacity.
type NodeSelector interface {
	SelectNode(ctx context.Context, task *queue.ProbeTask) (string, error)
}

// Scheduler orchestrates task scheduling.
type Scheduler struct {
	leader   *leader.Leader
	queue    *queue.Queue
	selector NodeSelector
	stream   *stream.Client
	pool     *pgxpool.Pool

	workerCount int
}

// Config holds Scheduler configuration.
type Config struct {
	Leader      *leader.Leader
	Queue       *queue.Queue
	Selector    NodeSelector
	Stream      *stream.Client
	Pool        *pgxpool.Pool
	WorkerCount int
}

// New creates a Scheduler instance.
func New(cfg Config) *Scheduler {
	return &Scheduler{
		leader:      cfg.Leader,
		queue:       cfg.Queue,
		selector:    cfg.Selector,
		stream:      cfg.Stream,
		pool:        cfg.Pool,
		workerCount: cfg.WorkerCount,
	}
}

// Run starts the scheduler loop.
// Only runs if this instance is the leader.
// Blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	log.Println("[scheduler] Starting scheduler loop")

	// Try to acquire leadership
	isLeader, err := s.leader.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("scheduler.Run: acquire leadership: %w", err)
	}
	if !isLeader {
		log.Println("[scheduler] Not the leader, exiting")
		return nil
	}

	log.Println("[scheduler] Acquired leadership, starting workers")

	// Start leader renewal goroutine
	renewCtx, cancelRenew := context.WithCancel(ctx)
	defer cancelRenew()
	go s.renewLeadership(renewCtx)

	// Start worker goroutines
	for i := 0; i < s.workerCount; i++ {
		go s.worker(ctx, i)
	}

	// Wait for context cancellation
	<-ctx.Done()
	log.Println("[scheduler] Context cancelled, releasing leadership")

	// Release leadership
	releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.leader.Release(releaseCtx); err != nil {
		log.Printf("[scheduler] Failed to release leadership: %v", err)
	}

	return ctx.Err()
}

// renewLeadership periodically renews the leader lock.
func (s *Scheduler) renewLeadership(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second) // renew every 5s (TTL is 10s)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.leader.Renew(ctx); err != nil {
				log.Printf("[scheduler] Failed to renew leadership: %v", err)
				// Lost leadership, stop scheduling
				return
			}
		}
	}
}

// worker processes tasks from the queue.
func (s *Scheduler) worker(ctx context.Context, id int) {
	log.Printf("[scheduler] Worker %d started", id)
	defer log.Printf("[scheduler] Worker %d stopped", id)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !s.leader.IsLeader() {
				// Lost leadership, stop processing
				return
			}

			// Try to dequeue a task
			task, err := s.queue.Dequeue(ctx)
			if err != nil {
				log.Printf("[scheduler] Worker %d: dequeue error: %v", id, err)
				continue
			}
			if task == nil {
				// Queue empty, wait for next tick
				continue
			}

			// Process task
			if err := s.processTask(ctx, task); err != nil {
				log.Printf("[scheduler] Worker %d: process task %s error: %v", id, task.ID, err)
				// TODO: retry logic or dead-letter queue
			} else {
				log.Printf("[scheduler] Worker %d: task %s dispatched to node %s", id, task.ID, task.NodeID)
			}
		}
	}
}

// processTask selects a node and dispatches the task.
func (s *Scheduler) processTask(ctx context.Context, task *queue.ProbeTask) error {
	// Select node
	nodeID, err := s.selector.SelectNode(ctx, task)
	if err != nil {
		return fmt.Errorf("select node: %w", err)
	}
	task.NodeID = nodeID

	// Dispatch to probe.tasks stream
	vals := map[string]any{
		"task_id":  task.ID,
		"type":     task.Type,
		"target":   task.Target,
		"node_id":  task.NodeID,
		"priority": task.Priority,
	}
	// Add params
	for k, v := range task.Params {
		vals["param_"+k] = v
	}

	_, err = s.stream.Add(ctx, ProbeTasksStream, vals)
	if err != nil {
		return fmt.Errorf("add to stream: %w", err)
	}

	return nil
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
