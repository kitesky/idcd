// Package queue implements a priority queue for probe tasks using Redis sorted sets.
// Priority levels P0-P5 are encoded in the score as: priority*1e12 + timestamp_ms
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Priority levels (lower number = higher priority)
	P0 = 0 // Critical
	P1 = 1 // High
	P2 = 2 // Normal
	P3 = 3 // Low
	P4 = 4 // Very low
	P5 = 5 // Lowest
)

// ProbeTask represents a task to be scheduled to a node.
type ProbeTask struct {
	ID       string            `json:"id"`        // probe_task.id
	Type     string            `json:"type"`      // http/tcp/ping/dns/udp
	Target   string            `json:"target"`    // URL or host
	NodeID   string            `json:"node_id"`   // assigned node ID
	Priority int               `json:"priority"`  // P0-P5
	Params   map[string]string `json:"params"`    // probe-specific parameters
	EnqueuedAt time.Time       `json:"enqueued_at"`
}

// Queue wraps a Redis client for priority queue operations.
type Queue struct {
	rdb       redis.Cmdable
	queueName string
}

// New creates a Queue instance.
// queueName is the Redis sorted set key (e.g., "scheduler:tasks").
func New(rdb redis.Cmdable, queueName string) *Queue {
	return &Queue{
		rdb:       rdb,
		queueName: queueName,
	}
}

// Enqueue adds a task to the priority queue.
// Score = priority*1e12 + timestamp_ms (ensures FIFO within same priority).
func (q *Queue) Enqueue(ctx context.Context, task *ProbeTask) error {
	task.EnqueuedAt = time.Now()

	// Serialize task as JSON
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("queue.Enqueue: marshal: %w", err)
	}

	// Calculate score: priority*1e12 + timestamp_ms
	score := float64(task.Priority)*1e12 + float64(task.EnqueuedAt.UnixMilli())

	// ZADD: add task to sorted set
	if err := q.rdb.ZAdd(ctx, q.queueName, redis.Z{
		Score:  score,
		Member: data,
	}).Err(); err != nil {
		return fmt.Errorf("queue.Enqueue: ZADD: %w", err)
	}

	return nil
}

// Dequeue removes and returns the highest priority task from the queue.
// Returns nil if the queue is empty.
func (q *Queue) Dequeue(ctx context.Context) (*ProbeTask, error) {
	// ZPOPMIN: remove and return the member with the lowest score (highest priority)
	result, err := q.rdb.ZPopMin(ctx, q.queueName).Result()
	if err == redis.Nil {
		return nil, nil // queue empty
	}
	if err != nil {
		return nil, fmt.Errorf("queue.Dequeue: ZPOPMIN: %w", err)
	}

	if len(result) == 0 {
		return nil, nil // queue empty
	}

	// Deserialize task
	data, ok := result[0].Member.(string)
	if !ok {
		return nil, fmt.Errorf("queue.Dequeue: member is not string")
	}

	var task ProbeTask
	if err := json.Unmarshal([]byte(data), &task); err != nil {
		return nil, fmt.Errorf("queue.Dequeue: unmarshal: %w", err)
	}

	return &task, nil
}

// Len returns the number of tasks in the queue.
func (q *Queue) Len(ctx context.Context) (int64, error) {
	n, err := q.rdb.ZCard(ctx, q.queueName).Result()
	if err != nil {
		return 0, fmt.Errorf("queue.Len: ZCARD: %w", err)
	}
	return n, nil
}

// Peek returns the highest priority task without removing it.
// Returns nil if the queue is empty.
func (q *Queue) Peek(ctx context.Context) (*ProbeTask, error) {
	// ZRANGE: get the member with the lowest score without removing
	result, err := q.rdb.ZRangeWithScores(ctx, q.queueName, 0, 0).Result()
	if err != nil {
		return nil, fmt.Errorf("queue.Peek: ZRANGE: %w", err)
	}

	if len(result) == 0 {
		return nil, nil // queue empty
	}

	// Deserialize task
	data, ok := result[0].Member.(string)
	if !ok {
		return nil, fmt.Errorf("queue.Peek: member is not string")
	}

	var task ProbeTask
	if err := json.Unmarshal([]byte(data), &task); err != nil {
		return nil, fmt.Errorf("queue.Peek: unmarshal: %w", err)
	}

	return &task, nil
}

// Clear removes all tasks from the queue (for testing).
func (q *Queue) Clear(ctx context.Context) error {
	if err := q.rdb.Del(ctx, q.queueName).Err(); err != nil {
		return fmt.Errorf("queue.Clear: DEL: %w", err)
	}
	return nil
}
