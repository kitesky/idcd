package queue

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupRedis(t *testing.T) (*miniredis.Miniredis, redis.Cmdable) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, rdb
}

func TestEnqueueDequeue(t *testing.T) {
	_, rdb := setupRedis(t)
	ctx := context.Background()
	q := New(rdb, "test:queue")

	task := &ProbeTask{
		ID:       "pt_abc123",
		Type:     "http",
		Target:   "https://example.com",
		NodeID:   "nd_us_ny_01_aws",
		Priority: P2,
		Params:   map[string]string{"method": "GET"},
	}

	// Enqueue
	err := q.Enqueue(ctx, task)
	if err != nil {
		t.Fatalf("q.Enqueue: %v", err)
	}

	// Check length
	n, err := q.Len(ctx)
	if err != nil {
		t.Fatalf("q.Len: %v", err)
	}
	if n != 1 {
		t.Errorf("q.Len() = %d, want 1", n)
	}

	// Dequeue
	dequeued, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("q.Dequeue: %v", err)
	}
	if dequeued == nil {
		t.Fatalf("q.Dequeue() = nil, want task")
	}
	if dequeued.ID != task.ID {
		t.Errorf("dequeued.ID = %q, want %q", dequeued.ID, task.ID)
	}
	if dequeued.Type != task.Type {
		t.Errorf("dequeued.Type = %q, want %q", dequeued.Type, task.Type)
	}

	// Queue should be empty now
	n, err = q.Len(ctx)
	if err != nil {
		t.Fatalf("q.Len: %v", err)
	}
	if n != 0 {
		t.Errorf("q.Len() = %d after dequeue, want 0", n)
	}

	// Dequeue from empty queue should return nil
	dequeued, err = q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("q.Dequeue (empty): %v", err)
	}
	if dequeued != nil {
		t.Errorf("q.Dequeue() from empty queue = %v, want nil", dequeued)
	}
}

func TestPriorityOrder(t *testing.T) {
	_, rdb := setupRedis(t)
	ctx := context.Background()
	q := New(rdb, "test:queue")

	// Enqueue tasks in reverse priority order
	tasks := []*ProbeTask{
		{ID: "task_p5", Type: "http", Target: "p5.com", Priority: P5},
		{ID: "task_p3", Type: "http", Target: "p3.com", Priority: P3},
		{ID: "task_p0", Type: "http", Target: "p0.com", Priority: P0},
		{ID: "task_p1", Type: "http", Target: "p1.com", Priority: P1},
		{ID: "task_p2", Type: "http", Target: "p2.com", Priority: P2},
	}

	for _, task := range tasks {
		if err := q.Enqueue(ctx, task); err != nil {
			t.Fatalf("q.Enqueue: %v", err)
		}
	}

	// Dequeue should return tasks in priority order: P0, P1, P2, P3, P5
	expectedOrder := []string{"task_p0", "task_p1", "task_p2", "task_p3", "task_p5"}
	for i, expectedID := range expectedOrder {
		task, err := q.Dequeue(ctx)
		if err != nil {
			t.Fatalf("q.Dequeue[%d]: %v", i, err)
		}
		if task == nil {
			t.Fatalf("q.Dequeue[%d] = nil, want task", i)
		}
		if task.ID != expectedID {
			t.Errorf("q.Dequeue[%d].ID = %q, want %q", i, task.ID, expectedID)
		}
	}

	// Queue should be empty
	n, err := q.Len(ctx)
	if err != nil {
		t.Fatalf("q.Len: %v", err)
	}
	if n != 0 {
		t.Errorf("q.Len() = %d after all dequeues, want 0", n)
	}
}

func TestFIFOWithinSamePriority(t *testing.T) {
	mr, rdb := setupRedis(t)
	ctx := context.Background()
	q := New(rdb, "test:queue")

	// Enqueue three tasks with same priority at different times
	tasks := []string{"task1", "task2", "task3"}
	for _, id := range tasks {
		task := &ProbeTask{
			ID:       id,
			Type:     "http",
			Target:   "example.com",
			Priority: P2,
		}
		if err := q.Enqueue(ctx, task); err != nil {
			t.Fatalf("q.Enqueue(%s): %v", id, err)
		}
		// Advance time to ensure different timestamps
		mr.FastForward(10 * time.Millisecond)
	}

	// Dequeue should return in FIFO order within same priority
	for i, expectedID := range tasks {
		task, err := q.Dequeue(ctx)
		if err != nil {
			t.Fatalf("q.Dequeue[%d]: %v", i, err)
		}
		if task == nil {
			t.Fatalf("q.Dequeue[%d] = nil, want task", i)
		}
		if task.ID != expectedID {
			t.Errorf("q.Dequeue[%d].ID = %q, want %q (FIFO within P2)", i, task.ID, expectedID)
		}
	}
}

func TestPeek(t *testing.T) {
	_, rdb := setupRedis(t)
	ctx := context.Background()
	q := New(rdb, "test:queue")

	// Peek empty queue should return nil
	task, err := q.Peek(ctx)
	if err != nil {
		t.Fatalf("q.Peek (empty): %v", err)
	}
	if task != nil {
		t.Errorf("q.Peek() from empty queue = %v, want nil", task)
	}

	// Enqueue tasks
	task1 := &ProbeTask{ID: "task_p1", Priority: P1}
	task0 := &ProbeTask{ID: "task_p0", Priority: P0}

	if err := q.Enqueue(ctx, task1); err != nil {
		t.Fatalf("q.Enqueue(task1): %v", err)
	}
	if err := q.Enqueue(ctx, task0); err != nil {
		t.Fatalf("q.Enqueue(task0): %v", err)
	}

	// Peek should return highest priority task (P0)
	peeked, err := q.Peek(ctx)
	if err != nil {
		t.Fatalf("q.Peek: %v", err)
	}
	if peeked == nil {
		t.Fatalf("q.Peek() = nil, want task")
	}
	if peeked.ID != "task_p0" {
		t.Errorf("q.Peek().ID = %q, want task_p0", peeked.ID)
	}

	// Queue length should still be 2 (peek doesn't remove)
	n, err := q.Len(ctx)
	if err != nil {
		t.Fatalf("q.Len: %v", err)
	}
	if n != 2 {
		t.Errorf("q.Len() = %d after peek, want 2", n)
	}

	// Dequeue should still return task_p0
	dequeued, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("q.Dequeue: %v", err)
	}
	if dequeued.ID != "task_p0" {
		t.Errorf("q.Dequeue().ID = %q, want task_p0", dequeued.ID)
	}
}

func TestClear(t *testing.T) {
	_, rdb := setupRedis(t)
	ctx := context.Background()
	q := New(rdb, "test:queue")

	// Enqueue some tasks
	for i := 0; i < 5; i++ {
		task := &ProbeTask{
			ID:       "task_" + string(rune('a'+i)),
			Priority: P2,
		}
		if err := q.Enqueue(ctx, task); err != nil {
			t.Fatalf("q.Enqueue: %v", err)
		}
	}

	// Check length
	n, err := q.Len(ctx)
	if err != nil {
		t.Fatalf("q.Len: %v", err)
	}
	if n != 5 {
		t.Errorf("q.Len() = %d, want 5", n)
	}

	// Clear
	if err := q.Clear(ctx); err != nil {
		t.Fatalf("q.Clear: %v", err)
	}

	// Check length after clear
	n, err = q.Len(ctx)
	if err != nil {
		t.Fatalf("q.Len: %v", err)
	}
	if n != 0 {
		t.Errorf("q.Len() = %d after clear, want 0", n)
	}
}
