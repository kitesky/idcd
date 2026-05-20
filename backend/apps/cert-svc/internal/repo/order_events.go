package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// OrderEventsRepo is the WAL writer/reader for cert.order_events.
//
// Per PRD §6, each event has a (order_id, action_seq) UNIQUE constraint;
// the ACME worker is expected to compute the next action_seq with
// NextActionSeq, then Append. A conflict on Append means another worker
// already recorded that step — the caller should treat it as idempotent
// success after reading back via ListByOrder.
type OrderEventsRepo struct {
	pool Pool
}

const orderEventsColumns = `id, order_id, action_seq, action, payload_jsonb, occurred_at`

const orderEventsInsertSQL = `
	INSERT INTO cert.order_events (order_id, action_seq, action, payload_jsonb)
	VALUES ($1, $2, $3, $4)
	RETURNING id, occurred_at
`

// Append writes one WAL entry. Returns ErrConflict if (order_id,
// action_seq) is already present.
func (r *OrderEventsRepo) Append(ctx context.Context, e *OrderEvent) error {
	var payloadArg any
	if len(e.Payload) > 0 {
		payloadArg = e.Payload
	}
	err := r.pool.QueryRow(ctx, orderEventsInsertSQL,
		e.OrderID,
		e.ActionSeq,
		e.Action,
		payloadArg,
	).Scan(&e.ID, &e.OccurredAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("order_events append: %w", err)
	}
	return nil
}

const orderEventsListByOrderSQL = `
	SELECT ` + orderEventsColumns + `
	FROM cert.order_events
	WHERE order_id = $1
	ORDER BY action_seq ASC
`

// ListByOrder returns every WAL entry for an order in action_seq order.
func (r *OrderEventsRepo) ListByOrder(ctx context.Context, orderID int64) ([]*OrderEvent, error) {
	rows, err := r.pool.Query(ctx, orderEventsListByOrderSQL, orderID)
	if err != nil {
		return nil, fmt.Errorf("order_events list: %w", err)
	}
	defer rows.Close()

	out := make([]*OrderEvent, 0)
	for rows.Next() {
		e, scanErr := scanOrderEvent(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("order_events list scan: %w", scanErr)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("order_events list rows: %w", err)
	}
	return out, nil
}

const orderEventsNextSeqSQL = `
	SELECT COALESCE(MAX(action_seq), 0) + 1
	FROM cert.order_events
	WHERE order_id = $1
`

// NextActionSeq returns the next action_seq for the given order. Returns
// 1 when the order has no events yet. NOTE: this is advisory only — two
// concurrent callers will read the same value and one Append will trip
// ErrConflict. The worker contract handles that as idempotent success.
func (r *OrderEventsRepo) NextActionSeq(ctx context.Context, orderID int64) (int, error) {
	var next int
	err := r.pool.QueryRow(ctx, orderEventsNextSeqSQL, orderID).Scan(&next)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// MAX over empty set returns NULL, but COALESCE folds it to 0;
			// QueryRow should not return ErrNoRows here. Defensive only.
			return 1, nil
		}
		return 0, fmt.Errorf("order_events next seq: %w", err)
	}
	return next, nil
}

func scanOrderEvent(r rowScanner) (*OrderEvent, error) {
	var (
		e       OrderEvent
		payload []byte
	)
	if err := r.Scan(
		&e.ID,
		&e.OrderID,
		&e.ActionSeq,
		&e.Action,
		&payload,
		&e.OccurredAt,
	); err != nil {
		return nil, err
	}
	e.Payload = payload
	return &e, nil
}
