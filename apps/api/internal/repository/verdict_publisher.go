package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/api/internal/handler"
)

// VerdictStreamName is the Redis Stream consumed by apps/attest/cmd/generator.
// The literal must stay in lockstep with the const of the same value in
// apps/attest/cmd/generator/main.go — both producer and consumer key off it.
const VerdictStreamName = "verdict_generation_queue"

// verdictRedisClient is the minimal subset of *redis.Client the publisher
// needs. Declared as an interface so unit tests can swap in a thin stub /
// miniredis-backed client without depending on the full go-redis surface.
type verdictRedisClient interface {
	XAdd(ctx context.Context, args *redis.XAddArgs) *redis.StringCmd
}

// VerdictPublisher publishes paid verdict_order rows onto the
// `verdict_generation_queue` Redis Stream so apps/attest can pick them up.
//
// Implements handler.VerdictStreamPublisher. Safe for concurrent use —
// *redis.Client is goroutine-safe and the publisher carries no mutable state.
type VerdictPublisher struct {
	client verdictRedisClient
	stream string
}

// NewVerdictPublisher constructs a VerdictPublisher writing to the canonical
// `verdict_generation_queue` stream on the given *redis.Client.
func NewVerdictPublisher(client *redis.Client) *VerdictPublisher {
	return &VerdictPublisher{
		client: client,
		stream: VerdictStreamName,
	}
}

// newVerdictPublisherWithClient is the test seam — accepts the interface
// directly so callers can pass an in-memory stub. Kept package-private so the
// public API (NewVerdictPublisher) stays tied to the concrete *redis.Client
// type that production wiring uses.
func newVerdictPublisherWithClient(client verdictRedisClient) *VerdictPublisher {
	return &VerdictPublisher{
		client: client,
		stream: VerdictStreamName,
	}
}

// EnqueueVerdict XAdds {order_id, owner_id, enqueued_at} onto the stream.
// Field names match what apps/attest/internal/streamconsumer expects.
func (p *VerdictPublisher) EnqueueVerdict(ctx context.Context, orderID, ownerID string) error {
	if p == nil || p.client == nil {
		return errors.New("verdict publisher not configured")
	}
	if orderID == "" {
		return errors.New("verdict publisher: empty order_id")
	}

	if _, err := p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: p.stream,
		Values: map[string]any{
			"order_id":    orderID,
			"owner_id":    ownerID,
			"enqueued_at": time.Now().UTC().Format(time.RFC3339Nano),
		},
	}).Result(); err != nil {
		return fmt.Errorf("xadd verdict_generation_queue: %w", err)
	}
	return nil
}

// Compile-time interface check.
var _ handler.VerdictStreamPublisher = (*VerdictPublisher)(nil)
