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

func newOrdersRepo(t *testing.T) (*OrdersRepo, pgxmock.PgxPoolIface) {
	t.Helper()
	pool := newMockPool(t)
	return &OrdersRepo{pool: pool}, pool
}

func sampleOrder() *Order {
	key := "idem-123"
	csr := "-----BEGIN CERTIFICATE REQUEST-----\nMI...\n-----END CERTIFICATE REQUEST-----"
	return &Order{
		AccountID:      42,
		SANs:           []string{"example.com"},
		Tier:           "free-dv",
		CA:             "lets-encrypt",
		ValidityDays:   90,
		ChallengeType:  "dns-01",
		Status:         OrderStatusDraft,
		CSRPEM:         &csr,
		IdempotencyKey: &key,
	}
}

func orderColumns() []string {
	return []string{
		"id", "account_id", "sans", "sans_unicode", "common_name", "tier", "ca",
		"reseller_channel", "reseller_order_ref", "organization_id", "validity_days",
		"challenge_type", "dns_credential_id", "status", "csr_pem", "cert_id",
		"billing_invoice_id", "retry_count", "last_error", "idempotency_key",
		"created_at", "finalized_at",
	}
}

func sampleOrderRow(id int64) []any {
	return []any{
		id, int64(42), []string{"example.com"}, []string(nil), (*string)(nil), "free-dv", "lets-encrypt",
		(*string)(nil), (*string)(nil), (*int64)(nil), 90,
		"dns-01", (*int64)(nil), "draft", (*string)(nil), (*int64)(nil),
		(*string)(nil), 0, (*string)(nil), (*string)(nil),
		time.Now().UTC(), (*time.Time)(nil),
	}
}

func TestOrdersRepo_Insert_Success(t *testing.T) {
	r, mock := newOrdersRepo(t)
	o := sampleOrder()
	now := time.Now().UTC()

	mock.ExpectQuery(`INSERT INTO cert\.orders`).
		WithArgs(o.AccountID, o.SANs, o.SANsUnicode, o.CommonName, o.Tier, o.CA,
			o.ResellerChannel, o.ResellerOrderRef, o.OrganizationID, o.ValidityDays,
			o.ChallengeType, o.DNSCredentialID, "draft", o.CSRPEM,
			o.BillingInvoiceID, o.IdempotencyKey).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow(int64(101), now))

	id, err := r.Insert(context.Background(), o)
	require.NoError(t, err)
	assert.Equal(t, int64(101), id)
	assert.Equal(t, int64(101), o.ID)
	assert.Equal(t, now, o.CreatedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOrdersRepo_Insert_IdempotencyConflict_ReturnsExistingID(t *testing.T) {
	r, mock := newOrdersRepo(t)
	o := sampleOrder()

	pgErr := &pgconn.PgError{Code: pgUniqueViolation}
	mock.ExpectQuery(`INSERT INTO cert\.orders`).
		WithArgs(anyArgs(16)...).
		WillReturnError(pgErr)
	mock.ExpectQuery(`SELECT id\s+FROM cert\.orders\s+WHERE account_id`).
		WithArgs(o.AccountID, *o.IdempotencyKey).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int64(77)))

	id, err := r.Insert(context.Background(), o)
	assert.ErrorIs(t, err, ErrConflict)
	assert.Equal(t, int64(77), id)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOrdersRepo_Insert_UniqueViolationWithoutIdempotencyKey(t *testing.T) {
	r, mock := newOrdersRepo(t)
	o := sampleOrder()
	o.IdempotencyKey = nil

	mock.ExpectQuery(`INSERT INTO cert\.orders`).
		WithArgs(anyArgs(16)...).
		WillReturnError(&pgconn.PgError{Code: pgUniqueViolation})

	id, err := r.Insert(context.Background(), o)
	assert.ErrorIs(t, err, ErrConflict)
	assert.Equal(t, int64(0), id)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOrdersRepo_Insert_DBError(t *testing.T) {
	r, mock := newOrdersRepo(t)
	o := sampleOrder()
	sentinel := errors.New("conn refused")

	mock.ExpectQuery(`INSERT INTO cert\.orders`).
		WithArgs(anyArgs(16)...).
		WillReturnError(sentinel)

	id, err := r.Insert(context.Background(), o)
	assert.ErrorIs(t, err, sentinel)
	assert.Equal(t, int64(0), id)
}

func TestOrdersRepo_GetByID_Success(t *testing.T) {
	r, mock := newOrdersRepo(t)

	mock.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE id`).
		WithArgs(int64(101)).
		WillReturnRows(pgxmock.NewRows(orderColumns()).AddRow(sampleOrderRow(101)...))

	o, err := r.GetByID(context.Background(), 101)
	require.NoError(t, err)
	assert.Equal(t, int64(101), o.ID)
	assert.Equal(t, OrderStatusDraft, o.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOrdersRepo_GetByID_NotFound(t *testing.T) {
	r, mock := newOrdersRepo(t)

	mock.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE id`).
		WithArgs(int64(999)).
		WillReturnError(pgx.ErrNoRows)

	o, err := r.GetByID(context.Background(), 999)
	assert.Nil(t, o)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestOrdersRepo_GetByID_DBError(t *testing.T) {
	r, mock := newOrdersRepo(t)
	sentinel := errors.New("disk full")

	mock.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE id`).
		WithArgs(int64(1)).
		WillReturnError(sentinel)

	o, err := r.GetByID(context.Background(), 1)
	assert.Nil(t, o)
	assert.ErrorIs(t, err, sentinel)
}

func TestOrdersRepo_ListByAccount_NoStatus(t *testing.T) {
	r, mock := newOrdersRepo(t)

	mock.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE account_id = \$1\s+ORDER BY`).
		WithArgs(int64(42), 10, 0).
		WillReturnRows(pgxmock.NewRows(orderColumns()).
			AddRow(sampleOrderRow(1)...).
			AddRow(sampleOrderRow(2)...))

	out, err := r.ListByAccount(context.Background(), 42, nil, 10, 0)
	require.NoError(t, err)
	require.Len(t, out, 2)
	assert.Equal(t, int64(1), out[0].ID)
}

func TestOrdersRepo_ListByAccount_WithStatus(t *testing.T) {
	r, mock := newOrdersRepo(t)
	st := OrderStatusIssued

	mock.ExpectQuery(`WHERE account_id = \$1 AND status = \$2`).
		WithArgs(int64(42), "issued", 5, 0).
		WillReturnRows(pgxmock.NewRows(orderColumns()).AddRow(sampleOrderRow(7)...))

	out, err := r.ListByAccount(context.Background(), 42, &st, 5, 0)
	require.NoError(t, err)
	require.Len(t, out, 1)
}

func TestOrdersRepo_ListByAccount_DBError(t *testing.T) {
	r, mock := newOrdersRepo(t)
	mock.ExpectQuery(`SELECT .+ FROM cert\.orders`).
		WithArgs(int64(42), 10, 0).
		WillReturnError(errors.New("timeout"))

	_, err := r.ListByAccount(context.Background(), 42, nil, 10, 0)
	require.Error(t, err)
}

func TestOrdersRepo_UpdateStatus_OK(t *testing.T) {
	r, mock := newOrdersRepo(t)
	msg := "validation failed"

	mock.ExpectExec(`UPDATE cert\.orders\s+SET status`).
		WithArgs("failed", &msg, int64(101), "validating").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := r.UpdateStatus(context.Background(), 101, OrderStatusValidating, OrderStatusFailed, &msg)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOrdersRepo_UpdateStatus_OptimisticLockMiss(t *testing.T) {
	r, mock := newOrdersRepo(t)

	mock.ExpectExec(`UPDATE cert\.orders\s+SET status`).
		WithArgs("issued", (*string)(nil), int64(101), "issuing").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err := r.UpdateStatus(context.Background(), 101, OrderStatusIssuing, OrderStatusIssued, nil)
	assert.ErrorIs(t, err, ErrInvalidStatus)
}

func TestOrdersRepo_UpdateStatus_DBError(t *testing.T) {
	r, mock := newOrdersRepo(t)
	mock.ExpectExec(`UPDATE cert\.orders`).
		WithArgs("failed", (*string)(nil), int64(1), "draft").
		WillReturnError(errors.New("deadlock"))

	err := r.UpdateStatus(context.Background(), 1, OrderStatusDraft, OrderStatusFailed, nil)
	require.Error(t, err)
}

func TestOrdersRepo_IncrementRetryCount(t *testing.T) {
	r, mock := newOrdersRepo(t)

	mock.ExpectExec(`UPDATE cert\.orders\s+SET retry_count`).
		WithArgs(int64(101)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	require.NoError(t, r.IncrementRetryCount(context.Background(), 101))

	mock.ExpectExec(`UPDATE cert\.orders\s+SET retry_count`).
		WithArgs(int64(999)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	assert.ErrorIs(t, r.IncrementRetryCount(context.Background(), 999), ErrNotFound)

	mock.ExpectExec(`UPDATE cert\.orders\s+SET retry_count`).
		WithArgs(int64(1)).
		WillReturnError(errors.New("boom"))
	require.Error(t, r.IncrementRetryCount(context.Background(), 1))
}

func TestOrdersRepo_SetCertID(t *testing.T) {
	r, mock := newOrdersRepo(t)

	mock.ExpectExec(`UPDATE cert\.orders\s+SET cert_id`).
		WithArgs(int64(555), int64(101)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	require.NoError(t, r.SetCertID(context.Background(), 101, 555))

	mock.ExpectExec(`UPDATE cert\.orders\s+SET cert_id`).
		WithArgs(int64(555), int64(0)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	assert.ErrorIs(t, r.SetCertID(context.Background(), 0, 555), ErrNotFound)

	mock.ExpectExec(`UPDATE cert\.orders\s+SET cert_id`).
		WithArgs(int64(2), int64(1)).
		WillReturnError(errors.New("io"))
	require.Error(t, r.SetCertID(context.Background(), 1, 2))
}

func TestOrdersRepo_SetFinalizedAt(t *testing.T) {
	r, mock := newOrdersRepo(t)
	now := time.Now().UTC()

	mock.ExpectExec(`UPDATE cert\.orders\s+SET finalized_at`).
		WithArgs(now, int64(101)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	require.NoError(t, r.SetFinalizedAt(context.Background(), 101, now))

	mock.ExpectExec(`UPDATE cert\.orders\s+SET finalized_at`).
		WithArgs(now, int64(2)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	assert.ErrorIs(t, r.SetFinalizedAt(context.Background(), 2, now), ErrNotFound)

	mock.ExpectExec(`UPDATE cert\.orders\s+SET finalized_at`).
		WithArgs(now, int64(1)).
		WillReturnError(errors.New("net"))
	require.Error(t, r.SetFinalizedAt(context.Background(), 1, now))
}

func TestOrdersRepo_ListPickable(t *testing.T) {
	r, mock := newOrdersRepo(t)
	statuses := []OrderStatus{OrderStatusValidating, OrderStatusIssuing}

	mock.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE status = ANY`).
		WithArgs([]string{"validating", "issuing"}, 10).
		WillReturnRows(pgxmock.NewRows(orderColumns()).AddRow(sampleOrderRow(1)...))

	out, err := r.ListPickable(context.Background(), statuses, 10)
	require.NoError(t, err)
	require.Len(t, out, 1)
}

func TestOrdersRepo_ListPickable_EmptyStatuses(t *testing.T) {
	r, _ := newOrdersRepo(t)
	out, err := r.ListPickable(context.Background(), nil, 10)
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestOrdersRepo_ListPickable_DBError(t *testing.T) {
	r, mock := newOrdersRepo(t)
	mock.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE status = ANY`).
		WithArgs([]string{"draft"}, 10).
		WillReturnError(errors.New("io"))

	_, err := r.ListPickable(context.Background(), []OrderStatus{OrderStatusDraft}, 10)
	require.Error(t, err)
}

func TestOrdersRepo_AdminListOrders_NoFilters(t *testing.T) {
	r, mock := newOrdersRepo(t)

	mock.ExpectQuery(`SELECT .+ FROM cert\.orders ORDER BY created_at DESC LIMIT \$1 OFFSET \$2`).
		WithArgs(50, 0).
		WillReturnRows(pgxmock.NewRows(orderColumns()).
			AddRow(sampleOrderRow(1)...).
			AddRow(sampleOrderRow(2)...))

	out, err := r.AdminListOrders(context.Background(), AdminOrderFilter{})
	require.NoError(t, err)
	require.Len(t, out, 2)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOrdersRepo_AdminListOrders_AllFilters(t *testing.T) {
	r, mock := newOrdersRepo(t)
	st := OrderStatusIssued
	acct := int64(42)
	ca := "lets-encrypt"

	mock.ExpectQuery(`SELECT .+ FROM cert\.orders WHERE status = \$1 AND account_id = \$2 AND ca = \$3 ORDER BY created_at DESC LIMIT \$4 OFFSET \$5`).
		WithArgs("issued", int64(42), "lets-encrypt", 25, 10).
		WillReturnRows(pgxmock.NewRows(orderColumns()).AddRow(sampleOrderRow(7)...))

	out, err := r.AdminListOrders(context.Background(), AdminOrderFilter{
		Status: &st, AccountID: &acct, CA: &ca, Limit: 25, Offset: 10,
	})
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOrdersRepo_AdminListOrders_LimitClamped(t *testing.T) {
	r, mock := newOrdersRepo(t)
	mock.ExpectQuery(`SELECT .+ FROM cert\.orders ORDER BY created_at DESC LIMIT \$1 OFFSET \$2`).
		WithArgs(50, 0). // 500 → clamped to 50; -3 → clamped to 0
		WillReturnRows(pgxmock.NewRows(orderColumns()))

	_, err := r.AdminListOrders(context.Background(), AdminOrderFilter{Limit: 500, Offset: -3})
	require.NoError(t, err)
}

func TestOrdersRepo_AdminListOrders_DBError(t *testing.T) {
	r, mock := newOrdersRepo(t)
	mock.ExpectQuery(`SELECT .+ FROM cert\.orders`).
		WithArgs(50, 0).
		WillReturnError(errors.New("boom"))
	_, err := r.AdminListOrders(context.Background(), AdminOrderFilter{})
	require.Error(t, err)
}

func TestOrdersRepo_AdminListOrders_ScanError(t *testing.T) {
	r, mock := newOrdersRepo(t)
	// Return a row missing one column → Scan fails.
	cols := []string{"id"}
	mock.ExpectQuery(`SELECT .+ FROM cert\.orders`).
		WithArgs(50, 0).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(int64(1)))
	_, err := r.AdminListOrders(context.Background(), AdminOrderFilter{})
	require.Error(t, err)
}

func TestOrdersRepo_CountByCASince(t *testing.T) {
	r, mock := newOrdersRepo(t)
	since := time.Now().UTC()

	mock.ExpectQuery(`SELECT count\(\*\) FROM cert.orders\s+WHERE ca = \$1 AND created_at >= \$2`).
		WithArgs("lets-encrypt", since).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(123)))

	n, err := r.CountByCASince(context.Background(), "lets-encrypt", since)
	require.NoError(t, err)
	require.Equal(t, 123, n)
}

func TestOrdersRepo_CountByCASince_DBError(t *testing.T) {
	r, mock := newOrdersRepo(t)
	mock.ExpectQuery(`SELECT count`).
		WithArgs("lets-encrypt", pgxmock.AnyArg()).
		WillReturnError(errors.New("io"))
	_, err := r.CountByCASince(context.Background(), "lets-encrypt", time.Now())
	require.Error(t, err)
}
