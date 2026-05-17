package repo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newOrdersRepo(t *testing.T) (*VerdictOrdersRepo, pgxmock.PgxPoolIface) {
	t.Helper()
	pool := newMockPool(t)
	return &VerdictOrdersRepo{pool: pool}, pool
}

func sampleVerdictOrder() *Order {
	return &Order{
		ID:                 "v_abc123",
		OwnerID:            "u_owner1",
		Template:           "sla",
		Target:             "example.com",
		TimeWindowStart:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		TimeWindowEnd:      time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC),
		Status:             OrderStatusPending,
		PriceCNY:           199.00,
		RefundAttemptCount: 0,
	}
}

func verdictOrderRowColumns() []string {
	return []string{
		"id", "owner_id", "template", "target",
		"time_window_start", "time_window_end", "status",
		"price_cny", "price_paid_cny", "ext_order_id",
		"refund_reason", "refund_attempt_count", "refund_last_error",
		"refund_apology_sent_at",
		"created_at", "paid_at", "delivered_at", "failed_at", "refunded_at",
	}
}

func sampleVerdictOrderRow(id string) []any {
	now := time.Now().UTC()
	return []any{
		id, "u_owner1", "sla", "example.com",
		now, now.Add(24 * time.Hour), "pending",
		float64(199), (*float64)(nil), (*string)(nil),
		(*string)(nil), 0, (*string)(nil),
		(*time.Time)(nil),
		now, (*time.Time)(nil), (*time.Time)(nil), (*time.Time)(nil), (*time.Time)(nil),
	}
}

func TestVerdictOrdersRepo_Insert_Success(t *testing.T) {
	r, mock := newOrdersRepo(t)
	o := sampleVerdictOrder()

	mock.ExpectExec(`INSERT INTO idcd_attest\.verdict_order`).
		WithArgs(anyArgs(19)...).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err := r.Insert(context.Background(), o)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVerdictOrdersRepo_Insert_Conflict(t *testing.T) {
	r, mock := newOrdersRepo(t)
	o := sampleVerdictOrder()

	mock.ExpectExec(`INSERT INTO idcd_attest\.verdict_order`).
		WithArgs(anyArgs(19)...).
		WillReturnError(&pgconn.PgError{Code: pgUniqueViolation})

	err := r.Insert(context.Background(), o)
	assert.ErrorIs(t, err, ErrConflict)
}

func TestVerdictOrdersRepo_Insert_DBError(t *testing.T) {
	r, mock := newOrdersRepo(t)
	o := sampleVerdictOrder()
	sentinel := errors.New("conn refused")

	mock.ExpectExec(`INSERT INTO idcd_attest\.verdict_order`).
		WithArgs(anyArgs(19)...).
		WillReturnError(sentinel)

	err := r.Insert(context.Background(), o)
	assert.ErrorIs(t, err, sentinel)
}

func TestVerdictOrdersRepo_GetByID_Success(t *testing.T) {
	r, mock := newOrdersRepo(t)

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_order\s+WHERE id`).
		WithArgs("v_abc123").
		WillReturnRows(pgxmock.NewRows(verdictOrderRowColumns()).AddRow(sampleVerdictOrderRow("v_abc123")...))

	o, err := r.GetByID(context.Background(), "v_abc123")
	require.NoError(t, err)
	assert.Equal(t, "v_abc123", o.ID)
	assert.Equal(t, OrderStatusPending, o.Status)
	assert.InDelta(t, 199.0, o.PriceCNY, 0.001)
}

func TestVerdictOrdersRepo_GetByID_NotFound(t *testing.T) {
	r, mock := newOrdersRepo(t)

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_order\s+WHERE id`).
		WithArgs("missing").
		WillReturnError(pgx.ErrNoRows)

	o, err := r.GetByID(context.Background(), "missing")
	assert.Nil(t, o)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestVerdictOrdersRepo_GetByID_DBError(t *testing.T) {
	r, mock := newOrdersRepo(t)
	sentinel := errors.New("disk full")

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_order\s+WHERE id`).
		WithArgs("v_1").
		WillReturnError(sentinel)

	_, err := r.GetByID(context.Background(), "v_1")
	assert.ErrorIs(t, err, sentinel)
}

func TestVerdictOrdersRepo_ListByOwner_NoStatus(t *testing.T) {
	r, mock := newOrdersRepo(t)

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_order\s+WHERE owner_id = \$1\s+ORDER BY`).
		WithArgs("u_owner1", 10, 0).
		WillReturnRows(pgxmock.NewRows(verdictOrderRowColumns()).
			AddRow(sampleVerdictOrderRow("v_1")...).
			AddRow(sampleVerdictOrderRow("v_2")...))

	out, err := r.ListByOwner(context.Background(), "u_owner1", nil, 10, 0)
	require.NoError(t, err)
	require.Len(t, out, 2)
	assert.Equal(t, "v_1", out[0].ID)
}

func TestVerdictOrdersRepo_ListByOwner_WithStatus(t *testing.T) {
	r, mock := newOrdersRepo(t)
	status := OrderStatusPaid

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_order\s+WHERE owner_id = \$1 AND status = \$2`).
		WithArgs("u_owner1", status, 5, 0).
		WillReturnRows(pgxmock.NewRows(verdictOrderRowColumns()))

	out, err := r.ListByOwner(context.Background(), "u_owner1", &status, 5, 0)
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestVerdictOrdersRepo_ListByOwner_QueryError(t *testing.T) {
	r, mock := newOrdersRepo(t)
	sentinel := errors.New("boom")

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_order`).
		WithArgs("u_owner1", 10, 0).
		WillReturnError(sentinel)

	_, err := r.ListByOwner(context.Background(), "u_owner1", nil, 10, 0)
	assert.ErrorIs(t, err, sentinel)
}

func TestVerdictOrdersRepo_UpdateStatus_Success(t *testing.T) {
	r, mock := newOrdersRepo(t)
	reason := "paymenthub webhook"

	mock.ExpectExec(`UPDATE idcd_attest\.verdict_order\s+SET status`).
		WithArgs(OrderStatusPaid, &reason, "v_1", OrderStatusPending).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := r.UpdateStatus(context.Background(), "v_1", OrderStatusPending, OrderStatusPaid, &reason)
	require.NoError(t, err)
}

func TestVerdictOrdersRepo_UpdateStatus_InvalidTransition(t *testing.T) {
	r, mock := newOrdersRepo(t)

	mock.ExpectExec(`UPDATE idcd_attest\.verdict_order`).
		WithArgs(OrderStatusDelivered, (*string)(nil), "v_1", OrderStatusPending).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err := r.UpdateStatus(context.Background(), "v_1", OrderStatusPending, OrderStatusDelivered, nil)
	assert.ErrorIs(t, err, ErrInvalidStatus)
}

func TestVerdictOrdersRepo_IncrementRefundAttempt_Success(t *testing.T) {
	r, mock := newOrdersRepo(t)

	mock.ExpectExec(`UPDATE idcd_attest\.verdict_order\s+SET refund_attempt_count`).
		WithArgs("paymenthub 5xx", "v_1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := r.IncrementRefundAttempt(context.Background(), "v_1", "paymenthub 5xx")
	require.NoError(t, err)
}

func TestVerdictOrdersRepo_IncrementRefundAttempt_NotFound(t *testing.T) {
	r, mock := newOrdersRepo(t)

	mock.ExpectExec(`UPDATE idcd_attest\.verdict_order\s+SET refund_attempt_count`).
		WithArgs("err", "missing").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err := r.IncrementRefundAttempt(context.Background(), "missing", "err")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestVerdictOrdersRepo_StampRefundedAt_Success(t *testing.T) {
	r, mock := newOrdersRepo(t)
	now := time.Now().UTC()

	mock.ExpectExec(`UPDATE idcd_attest\.verdict_order\s+SET refunded_at`).
		WithArgs(now, "v_1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := r.StampRefundedAt(context.Background(), "v_1", now)
	require.NoError(t, err)
}

func TestVerdictOrdersRepo_StampRefundedAt_NotFound(t *testing.T) {
	r, mock := newOrdersRepo(t)
	now := time.Now().UTC()

	mock.ExpectExec(`UPDATE idcd_attest\.verdict_order\s+SET refunded_at`).
		WithArgs(now, "missing").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err := r.StampRefundedAt(context.Background(), "missing", now)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestVerdictOrdersRepo_StampRefundedAt_DBError(t *testing.T) {
	r, mock := newOrdersRepo(t)
	now := time.Now().UTC()
	sentinel := errors.New("disk full")

	mock.ExpectExec(`UPDATE idcd_attest\.verdict_order\s+SET refunded_at`).
		WithArgs(now, "v_1").
		WillReturnError(sentinel)

	err := r.StampRefundedAt(context.Background(), "v_1", now)
	assert.ErrorIs(t, err, sentinel)
}

func TestVerdictOrdersRepo_SetRefundApologySent_Success(t *testing.T) {
	r, mock := newOrdersRepo(t)
	now := time.Now().UTC()

	mock.ExpectExec(`UPDATE idcd_attest\.verdict_order\s+SET refund_apology_sent_at`).
		WithArgs(now, "v_1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := r.SetRefundApologySent(context.Background(), "v_1", now)
	require.NoError(t, err)
}

func TestVerdictOrdersRepo_SetRefundApologySent_NotFound(t *testing.T) {
	r, mock := newOrdersRepo(t)
	now := time.Now().UTC()

	mock.ExpectExec(`UPDATE idcd_attest\.verdict_order\s+SET refund_apology_sent_at`).
		WithArgs(now, "missing").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err := r.SetRefundApologySent(context.Background(), "missing", now)
	assert.ErrorIs(t, err, ErrNotFound)
}
