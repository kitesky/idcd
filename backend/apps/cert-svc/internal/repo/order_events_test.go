package repo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newOrderEventsRepo(t *testing.T) (*OrderEventsRepo, pgxmock.PgxPoolIface) {
	t.Helper()
	pool := newMockPool(t)
	return &OrderEventsRepo{pool: pool}, pool
}

func TestOrderEventsRepo_Append_Success(t *testing.T) {
	r, mock := newOrderEventsRepo(t)
	now := time.Now().UTC()

	e := &OrderEvent{OrderID: 1, ActionSeq: 1, Action: "authz_created", Payload: []byte(`{"k":1}`)}
	mock.ExpectQuery(`INSERT INTO cert\.order_events`).
		WithArgs(int64(1), 1, "authz_created", []byte(`{"k":1}`)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "occurred_at"}).AddRow(int64(7), now))

	require.NoError(t, r.Append(context.Background(), e))
	assert.Equal(t, int64(7), e.ID)
	assert.Equal(t, now, e.OccurredAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderEventsRepo_Append_NilPayload(t *testing.T) {
	r, mock := newOrderEventsRepo(t)
	now := time.Now().UTC()

	e := &OrderEvent{OrderID: 1, ActionSeq: 2, Action: "polled"}
	mock.ExpectQuery(`INSERT INTO cert\.order_events`).
		WithArgs(int64(1), 2, "polled", nil).
		WillReturnRows(pgxmock.NewRows([]string{"id", "occurred_at"}).AddRow(int64(8), now))

	require.NoError(t, r.Append(context.Background(), e))
}

func TestOrderEventsRepo_Append_UniqueViolation(t *testing.T) {
	r, mock := newOrderEventsRepo(t)

	mock.ExpectQuery(`INSERT INTO cert\.order_events`).
		WithArgs(int64(1), 1, "x", nil).
		WillReturnError(&pgconn.PgError{Code: pgUniqueViolation})

	err := r.Append(context.Background(), &OrderEvent{OrderID: 1, ActionSeq: 1, Action: "x"})
	assert.ErrorIs(t, err, ErrConflict)
}

func TestOrderEventsRepo_Append_DBError(t *testing.T) {
	r, mock := newOrderEventsRepo(t)

	mock.ExpectQuery(`INSERT INTO cert\.order_events`).
		WithArgs(int64(1), 1, "x", nil).
		WillReturnError(errors.New("conn refused"))

	err := r.Append(context.Background(), &OrderEvent{OrderID: 1, ActionSeq: 1, Action: "x"})
	require.Error(t, err)
}

func TestOrderEventsRepo_ListByOrder(t *testing.T) {
	r, mock := newOrderEventsRepo(t)
	now := time.Now().UTC()

	mock.ExpectQuery(`SELECT .+ FROM cert\.order_events\s+WHERE order_id`).
		WithArgs(int64(1)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "order_id", "action_seq", "action", "payload_jsonb", "occurred_at"}).
			AddRow(int64(1), int64(1), 1, "authz_created", []byte(`{"a":1}`), now).
			AddRow(int64(2), int64(1), 2, "polled", []byte(nil), now))

	out, err := r.ListByOrder(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, out, 2)
	assert.Equal(t, "authz_created", out[0].Action)
}

func TestOrderEventsRepo_ListByOrder_DBError(t *testing.T) {
	r, mock := newOrderEventsRepo(t)
	mock.ExpectQuery(`SELECT .+ FROM cert\.order_events`).
		WithArgs(int64(1)).
		WillReturnError(errors.New("io"))
	_, err := r.ListByOrder(context.Background(), 1)
	require.Error(t, err)
}

func TestOrderEventsRepo_NextActionSeq(t *testing.T) {
	r, mock := newOrderEventsRepo(t)

	mock.ExpectQuery(`SELECT COALESCE\(MAX\(action_seq\), 0\) \+ 1\s+FROM cert\.order_events`).
		WithArgs(int64(1)).
		WillReturnRows(pgxmock.NewRows([]string{"next"}).AddRow(4))

	n, err := r.NextActionSeq(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 4, n)
}

func TestOrderEventsRepo_NextActionSeq_DBError(t *testing.T) {
	r, mock := newOrderEventsRepo(t)
	mock.ExpectQuery(`SELECT COALESCE`).
		WithArgs(int64(1)).
		WillReturnError(errors.New("io"))
	_, err := r.NextActionSeq(context.Background(), 1)
	require.Error(t, err)
}
