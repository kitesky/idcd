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

func newBansRepo(t *testing.T) (*AbuseBansRepo, pgxmock.PgxPoolIface) {
	t.Helper()
	pool := newMockPool(t)
	return &AbuseBansRepo{pool: pool}, pool
}

func TestAbuseBansRepo_Ban_Success(t *testing.T) {
	r, mock := newBansRepo(t)
	now := time.Now().UTC()
	mock.ExpectQuery(`INSERT INTO cert\.abuse_bans`).
		WithArgs("7", "spam", "admin").
		WillReturnRows(pgxmock.NewRows([]string{"id", "banned_at"}).
			AddRow(int64(101), now))

	ban, err := r.Ban(context.Background(), "7", "spam", "")
	require.NoError(t, err)
	assert.Equal(t, int64(101), ban.ID)
	assert.Equal(t, "7", ban.AccountID)
	assert.Equal(t, "spam", ban.Reason)
	assert.Equal(t, "admin", ban.BannedBy)
	assert.Equal(t, now, ban.BannedAt)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAbuseBansRepo_Ban_CustomActor(t *testing.T) {
	r, mock := newBansRepo(t)
	now := time.Now().UTC()
	mock.ExpectQuery(`INSERT INTO cert\.abuse_bans`).
		WithArgs("7", "phishing", "ops:alice").
		WillReturnRows(pgxmock.NewRows([]string{"id", "banned_at"}).
			AddRow(int64(102), now))

	ban, err := r.Ban(context.Background(), "7", "phishing", "ops:alice")
	require.NoError(t, err)
	assert.Equal(t, "ops:alice", ban.BannedBy)
}

func TestAbuseBansRepo_Ban_AlreadyBanned(t *testing.T) {
	r, mock := newBansRepo(t)
	mock.ExpectQuery(`INSERT INTO cert\.abuse_bans`).
		WithArgs("7", "dup", "admin").
		WillReturnError(&pgconn.PgError{Code: "23505"})

	_, err := r.Ban(context.Background(), "7", "dup", "admin")
	assert.ErrorIs(t, err, ErrAlreadyBanned)
}

func TestAbuseBansRepo_Ban_DBError(t *testing.T) {
	r, mock := newBansRepo(t)
	mock.ExpectQuery(`INSERT INTO cert\.abuse_bans`).
		WithArgs("7", "x", "admin").
		WillReturnError(errors.New("connection refused"))

	_, err := r.Ban(context.Background(), "7", "x", "admin")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrAlreadyBanned)
}

func TestAbuseBansRepo_IsBanned_Active(t *testing.T) {
	r, mock := newBansRepo(t)
	now := time.Now().UTC()
	mock.ExpectQuery(`SELECT id, account_id, reason, banned_by, banned_at`).
		WithArgs("42").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "account_id", "reason", "banned_by", "banned_at",
			"lifted_at", "lifted_by", "lifted_reason",
		}).AddRow(int64(9), "42", "spam", "admin", now, nil, nil, nil))

	banned, ban, err := r.IsBanned(context.Background(), "42")
	require.NoError(t, err)
	assert.True(t, banned)
	require.NotNil(t, ban)
	assert.Equal(t, "42", ban.AccountID)
	assert.Equal(t, "spam", ban.Reason)
	assert.Nil(t, ban.LiftedAt)
}

func TestAbuseBansRepo_IsBanned_NotFound(t *testing.T) {
	r, mock := newBansRepo(t)
	mock.ExpectQuery(`SELECT id, account_id, reason, banned_by, banned_at`).
		WithArgs("42").
		WillReturnError(pgx.ErrNoRows)

	banned, ban, err := r.IsBanned(context.Background(), "42")
	require.NoError(t, err)
	assert.False(t, banned)
	assert.Nil(t, ban)
}

func TestAbuseBansRepo_IsBanned_DBError(t *testing.T) {
	r, mock := newBansRepo(t)
	mock.ExpectQuery(`SELECT id, account_id, reason, banned_by, banned_at`).
		WithArgs("42").
		WillReturnError(errors.New("connection refused"))

	_, _, err := r.IsBanned(context.Background(), "42")
	require.Error(t, err)
}

func TestAbuseBansRepo_Lift_Success(t *testing.T) {
	r, mock := newBansRepo(t)
	mock.ExpectExec(`UPDATE cert\.abuse_bans`).
		WithArgs("42", "ops:alice", "false positive").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	require.NoError(t, r.Lift(context.Background(), "42", "ops:alice", "false positive"))
}

func TestAbuseBansRepo_Lift_DefaultActor(t *testing.T) {
	r, mock := newBansRepo(t)
	mock.ExpectExec(`UPDATE cert\.abuse_bans`).
		WithArgs("42", "admin", "manual review").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	require.NoError(t, r.Lift(context.Background(), "42", "", "manual review"))
}

func TestAbuseBansRepo_Lift_NotBanned(t *testing.T) {
	r, mock := newBansRepo(t)
	mock.ExpectExec(`UPDATE cert\.abuse_bans`).
		WithArgs("42", "admin", "noop").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err := r.Lift(context.Background(), "42", "admin", "noop")
	assert.ErrorIs(t, err, ErrNotBanned)
}

func TestAbuseBansRepo_Lift_DBError(t *testing.T) {
	r, mock := newBansRepo(t)
	mock.ExpectExec(`UPDATE cert\.abuse_bans`).
		WithArgs("42", "admin", "x").
		WillReturnError(errors.New("conn refused"))

	err := r.Lift(context.Background(), "42", "admin", "x")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotBanned)
}
