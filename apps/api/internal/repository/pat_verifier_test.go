package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/api/internal/middleware"
)

func newPATVerifierTest(t *testing.T) (*PATVerifier, pgxmock.PgxPoolIface) {
	t.Helper()
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	return NewPATVerifier(pool), pool
}

func TestPATVerifier_VerifyPAT_Success(t *testing.T) {
	v, mock := newPATVerifierTest(t)
	defer mock.Close()

	const raw = "idcd_pat_abcdef1234567890"
	hash := middleware.HashToken(raw)
	exp := time.Now().UTC().Add(24 * time.Hour)

	mock.ExpectQuery(`SELECT id, user_id, expires_at FROM personal_access_tokens`).
		WithArgs(hash).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "expires_at"}).
			AddRow("pat_123", "u_42", &exp))

	info, err := v.VerifyPAT(context.Background(), raw)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "pat_123", info.ID)
	assert.Equal(t, "u_42", info.UserID)
	require.NotNil(t, info.ExpiresAt)
	assert.WithinDuration(t, exp, *info.ExpiresAt, time.Second)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPATVerifier_VerifyPAT_NoExpiry(t *testing.T) {
	v, mock := newPATVerifierTest(t)
	defer mock.Close()

	const raw = "idcd_pat_noexpire"
	hash := middleware.HashToken(raw)

	mock.ExpectQuery(`SELECT id, user_id, expires_at FROM personal_access_tokens`).
		WithArgs(hash).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "expires_at"}).
			AddRow("pat_nx", "u_1", (*time.Time)(nil)))

	info, err := v.VerifyPAT(context.Background(), raw)
	require.NoError(t, err)
	assert.Nil(t, info.ExpiresAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPATVerifier_VerifyPAT_NotFound(t *testing.T) {
	v, mock := newPATVerifierTest(t)
	defer mock.Close()

	const raw = "idcd_pat_bogus"
	hash := middleware.HashToken(raw)

	mock.ExpectQuery(`SELECT id, user_id, expires_at FROM personal_access_tokens`).
		WithArgs(hash).
		WillReturnError(pgx.ErrNoRows)

	info, err := v.VerifyPAT(context.Background(), raw)
	require.Error(t, err)
	assert.Nil(t, info)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPATVerifier_VerifyPAT_DBError(t *testing.T) {
	v, mock := newPATVerifierTest(t)
	defer mock.Close()

	const raw = "idcd_pat_db_err"
	hash := middleware.HashToken(raw)
	sentinel := errors.New("connection reset")

	mock.ExpectQuery(`SELECT id, user_id, expires_at FROM personal_access_tokens`).
		WithArgs(hash).
		WillReturnError(sentinel)

	info, err := v.VerifyPAT(context.Background(), raw)
	require.Error(t, err)
	assert.Nil(t, info)
	assert.ErrorIs(t, err, sentinel)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPATVerifier_VerifyPAT_EmptyToken(t *testing.T) {
	v, _ := newPATVerifierTest(t)
	info, err := v.VerifyPAT(context.Background(), "")
	require.Error(t, err)
	assert.Nil(t, info)
}

func TestPATVerifier_TouchLastUsed_Success(t *testing.T) {
	v, mock := newPATVerifierTest(t)
	defer mock.Close()

	mock.ExpectExec(`UPDATE personal_access_tokens`).
		WithArgs(pgxmock.AnyArg(), "pat_123").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := v.TouchLastUsed(context.Background(), "pat_123")
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPATVerifier_TouchLastUsed_EmptyID(t *testing.T) {
	v, _ := newPATVerifierTest(t)
	err := v.TouchLastUsed(context.Background(), "")
	require.Error(t, err)
}

func TestPATVerifier_TouchLastUsed_DBError(t *testing.T) {
	v, mock := newPATVerifierTest(t)
	defer mock.Close()

	sentinel := errors.New("write conflict")
	mock.ExpectExec(`UPDATE personal_access_tokens`).
		WithArgs(pgxmock.AnyArg(), "pat_x").
		WillReturnError(sentinel)

	err := v.TouchLastUsed(context.Background(), "pat_x")
	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel)
	require.NoError(t, mock.ExpectationsWereMet())
}

// Compile-time interface check — guarantees the verifier still satisfies the
// middleware contract if the interface ever changes.
var _ middleware.PATVerifier = (*PATVerifier)(nil)
