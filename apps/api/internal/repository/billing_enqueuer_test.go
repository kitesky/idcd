package repository

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/api/internal/handler"
)

// stubAsynqClient is an in-memory recorder for asynqClient.EnqueueContext.
// Lets us assert task type, queue, payload, and the ProcessIn delay without
// needing a real Redis instance.
type stubAsynqClient struct {
	calls []recordedEnqueue
	err   error
}

type recordedEnqueue struct {
	taskType string
	payload  []byte
	queue    string
	delay    time.Duration
}

func (s *stubAsynqClient) EnqueueContext(_ context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	if s.err != nil {
		return nil, s.err
	}
	rec := recordedEnqueue{
		taskType: task.Type(),
		payload:  append([]byte(nil), task.Payload()...),
	}
	// asynq.Option's concrete types are unexported, but the interface exposes
	// Type() + Value() — enough to recover what we passed in.
	for _, opt := range opts {
		switch opt.Type() {
		case asynq.QueueOpt:
			if v, ok := opt.Value().(string); ok {
				rec.queue = v
			}
		case asynq.ProcessInOpt:
			if v, ok := opt.Value().(time.Duration); ok {
				rec.delay = v
			}
		}
	}
	s.calls = append(s.calls, rec)
	return &asynq.TaskInfo{ID: "stub", Queue: rec.queue, Type: rec.taskType}, nil
}

func samplePayload() handler.RefundRetryPayload {
	return handler.RefundRetryPayload{
		PaymentID:    "pay_1",
		ExtTxnID:     "ext_1",
		UserID:       "u_1",
		UserEmail:    "a@b.com",
		AmountCents:  1999,
		Currency:     "USD",
		Provider:     "paddle",
		Reason:       "user_request",
		AttemptCount: 0,
	}
}

func TestBillingEnqueuer_EnqueueRefundRetry_NoDelay(t *testing.T) {
	stub := &stubAsynqClient{}
	enq := newBillingEnqueuerWithClient(stub)

	err := enq.EnqueueRefundRetry(context.Background(), samplePayload(), 0)
	require.NoError(t, err)
	require.Len(t, stub.calls, 1)

	call := stub.calls[0]
	assert.Equal(t, handler.TaskRefundRetry, call.taskType)
	assert.Equal(t, handler.QueueBilling, call.queue)
	assert.Equal(t, time.Duration(0), call.delay, "delay 0 must not schedule ProcessIn")

	var decoded handler.RefundRetryPayload
	require.NoError(t, json.Unmarshal(call.payload, &decoded))
	assert.Equal(t, samplePayload(), decoded)
}

func TestBillingEnqueuer_EnqueueRefundRetry_WithDelay(t *testing.T) {
	stub := &stubAsynqClient{}
	enq := newBillingEnqueuerWithClient(stub)

	err := enq.EnqueueRefundRetry(context.Background(), samplePayload(), 5*time.Minute)
	require.NoError(t, err)
	require.Len(t, stub.calls, 1)
	assert.Equal(t, 5*time.Minute, stub.calls[0].delay)
}

func TestBillingEnqueuer_EnqueueRefundRetry_SecondRetryDelay(t *testing.T) {
	stub := &stubAsynqClient{}
	enq := newBillingEnqueuerWithClient(stub)

	err := enq.EnqueueRefundRetry(context.Background(), samplePayload(), 25*time.Minute)
	require.NoError(t, err)
	require.Len(t, stub.calls, 1)
	assert.Equal(t, 25*time.Minute, stub.calls[0].delay)
}

func TestBillingEnqueuer_EnqueueRefundRetry_ClientError(t *testing.T) {
	sentinel := errors.New("redis down")
	stub := &stubAsynqClient{err: sentinel}
	enq := newBillingEnqueuerWithClient(stub)

	err := enq.EnqueueRefundRetry(context.Background(), samplePayload(), 0)
	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel)
}

func TestBillingEnqueuer_EnqueueRefundRetry_NilClient(t *testing.T) {
	enq := &BillingEnqueuer{client: nil, queue: handler.QueueBilling}
	err := enq.EnqueueRefundRetry(context.Background(), samplePayload(), 0)
	require.Error(t, err)
}

func TestBillingEnqueuer_EnqueueRefundRetry_NilReceiver(t *testing.T) {
	var enq *BillingEnqueuer
	err := enq.EnqueueRefundRetry(context.Background(), samplePayload(), 0)
	require.Error(t, err)
}

func TestNewBillingEnqueuer_ConcreteClient(t *testing.T) {
	// Smoke-test the public constructor. We don't enqueue anything (would
	// require Redis) — just verify the queue is wired correctly.
	c := asynq.NewClient(asynq.RedisClientOpt{Addr: "127.0.0.1:0"})
	defer c.Close()
	enq := NewBillingEnqueuer(c)
	require.NotNil(t, enq)
	assert.Equal(t, handler.QueueBilling, enq.queue)
}
