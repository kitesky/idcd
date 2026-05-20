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

func newRenewalRepo(t *testing.T) (*RenewalJobsRepo, pgxmock.PgxPoolIface) {
	t.Helper()
	pool := newMockPool(t)
	return &RenewalJobsRepo{pool: pool}, pool
}

func TestRenewalJobsRepo_Insert_Success(t *testing.T) {
	r, mock := newRenewalRepo(t)
	j := &RenewalJob{CertID: 555, ScheduledAt: time.Now().UTC(), Status: "queued"}
	now := time.Now().UTC()

	mock.ExpectQuery(`INSERT INTO cert\.renewal_jobs`).
		WithArgs(j.CertID, j.ScheduledAt, j.Status).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow(int64(33), now))

	id, err := r.Insert(context.Background(), j)
	require.NoError(t, err)
	assert.Equal(t, int64(33), id)
}

func TestRenewalJobsRepo_Insert_UniqueViolation(t *testing.T) {
	r, mock := newRenewalRepo(t)
	mock.ExpectQuery(`INSERT INTO cert\.renewal_jobs`).
		WithArgs(anyArgs(3)...).
		WillReturnError(&pgconn.PgError{Code: pgUniqueViolation})
	_, err := r.Insert(context.Background(), &RenewalJob{})
	assert.ErrorIs(t, err, ErrConflict)
}

func TestRenewalJobsRepo_Insert_DBError(t *testing.T) {
	r, mock := newRenewalRepo(t)
	mock.ExpectQuery(`INSERT INTO cert\.renewal_jobs`).
		WithArgs(anyArgs(3)...).
		WillReturnError(errors.New("io"))
	_, err := r.Insert(context.Background(), &RenewalJob{})
	require.Error(t, err)
}

func TestRenewalJobsRepo_ListQueued(t *testing.T) {
	r, mock := newRenewalRepo(t)
	now := time.Now().UTC()

	mock.ExpectQuery(`SELECT .+ FROM cert\.renewal_jobs\s+WHERE status = 'queued'`).
		WithArgs(20).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "cert_id", "scheduled_at", "attempt_count", "last_error",
			"status", "new_order_id", "created_at",
		}).AddRow(int64(1), int64(555), now, 0, (*string)(nil), "queued", (*int64)(nil), now))

	out, err := r.ListQueued(context.Background(), 20)
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, "queued", out[0].Status)
}

func TestRenewalJobsRepo_ListQueued_DBError(t *testing.T) {
	r, mock := newRenewalRepo(t)
	mock.ExpectQuery(`SELECT .+ FROM cert\.renewal_jobs`).
		WithArgs(10).
		WillReturnError(errors.New("io"))
	_, err := r.ListQueued(context.Background(), 10)
	require.Error(t, err)
}

func TestRenewalJobsRepo_UpdateStatus(t *testing.T) {
	r, mock := newRenewalRepo(t)
	msg := "boom"
	newOrder := int64(202)

	mock.ExpectExec(`UPDATE cert\.renewal_jobs\s+SET status`).
		WithArgs("done", &msg, &newOrder, int64(33)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	require.NoError(t, r.UpdateStatus(context.Background(), 33, "done", &msg, &newOrder))

	mock.ExpectExec(`UPDATE cert\.renewal_jobs\s+SET status`).
		WithArgs("failed", (*string)(nil), (*int64)(nil), int64(0)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	assert.ErrorIs(t, r.UpdateStatus(context.Background(), 0, "failed", nil, nil), ErrNotFound)

	mock.ExpectExec(`UPDATE cert\.renewal_jobs\s+SET status`).
		WithArgs("x", (*string)(nil), (*int64)(nil), int64(1)).
		WillReturnError(errors.New("io"))
	require.Error(t, r.UpdateStatus(context.Background(), 1, "x", nil, nil))
}

func TestRenewalJobsRepo_IncrementAttempt(t *testing.T) {
	r, mock := newRenewalRepo(t)

	mock.ExpectExec(`UPDATE cert\.renewal_jobs\s+SET attempt_count`).
		WithArgs(int64(33)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	require.NoError(t, r.IncrementAttempt(context.Background(), 33))

	mock.ExpectExec(`UPDATE cert\.renewal_jobs\s+SET attempt_count`).
		WithArgs(int64(0)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	assert.ErrorIs(t, r.IncrementAttempt(context.Background(), 0), ErrNotFound)

	mock.ExpectExec(`UPDATE cert\.renewal_jobs\s+SET attempt_count`).
		WithArgs(int64(1)).
		WillReturnError(errors.New("io"))
	require.Error(t, r.IncrementAttempt(context.Background(), 1))
}
