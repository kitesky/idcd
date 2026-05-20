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

func newAPIKeyVerifierTest(t *testing.T) (*APIKeyVerifier, pgxmock.PgxPoolIface) {
	t.Helper()
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	return NewAPIKeyVerifier(pool), pool
}

func TestAPIKeyVerifier_VerifyAPIKey_Success(t *testing.T) {
	v, mock := newAPIKeyVerifierTest(t)
	defer mock.Close()

	const raw = "sk_live_abcdef1234567890"
	hash := middleware.HashToken(raw)
	exp := time.Now().UTC().Add(30 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT id, owner_type, owner_id, status, expires_at FROM api_key`).
		WithArgs(hash).
		WillReturnRows(pgxmock.NewRows([]string{"id", "owner_type", "owner_id", "status", "expires_at"}).
			AddRow("key_42", "user", "u_42", "active", &exp))

	info, err := v.VerifyAPIKey(context.Background(), raw)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "key_42", info.ID)
	assert.Equal(t, "user", info.OwnerType)
	assert.Equal(t, "u_42", info.OwnerID)
	assert.Equal(t, "active", info.Status)
	require.NotNil(t, info.ExpiresAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyVerifier_VerifyAPIKey_RevokedRowReturned(t *testing.T) {
	// The middleware enforces status=='active'; the verifier returns whatever
	// row matches so the middleware sees the real status string.
	v, mock := newAPIKeyVerifierTest(t)
	defer mock.Close()

	const raw = "sk_live_revoked"
	hash := middleware.HashToken(raw)

	mock.ExpectQuery(`SELECT id, owner_type, owner_id, status, expires_at FROM api_key`).
		WithArgs(hash).
		WillReturnRows(pgxmock.NewRows([]string{"id", "owner_type", "owner_id", "status", "expires_at"}).
			AddRow("key_r", "user", "u_1", "revoked", (*time.Time)(nil)))

	info, err := v.VerifyAPIKey(context.Background(), raw)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "revoked", info.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyVerifier_VerifyAPIKey_NotFound(t *testing.T) {
	v, mock := newAPIKeyVerifierTest(t)
	defer mock.Close()

	const raw = "sk_test_unknown"
	hash := middleware.HashToken(raw)

	mock.ExpectQuery(`SELECT id, owner_type, owner_id, status, expires_at FROM api_key`).
		WithArgs(hash).
		WillReturnError(pgx.ErrNoRows)

	info, err := v.VerifyAPIKey(context.Background(), raw)
	require.Error(t, err)
	assert.Nil(t, info)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyVerifier_VerifyAPIKey_DBError(t *testing.T) {
	v, mock := newAPIKeyVerifierTest(t)
	defer mock.Close()

	const raw = "sk_live_dberr"
	hash := middleware.HashToken(raw)
	sentinel := errors.New("conn refused")

	mock.ExpectQuery(`SELECT id, owner_type, owner_id, status, expires_at FROM api_key`).
		WithArgs(hash).
		WillReturnError(sentinel)

	info, err := v.VerifyAPIKey(context.Background(), raw)
	require.Error(t, err)
	assert.Nil(t, info)
	assert.ErrorIs(t, err, sentinel)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyVerifier_VerifyAPIKey_EmptyKey(t *testing.T) {
	v, _ := newAPIKeyVerifierTest(t)
	info, err := v.VerifyAPIKey(context.Background(), "")
	require.Error(t, err)
	assert.Nil(t, info)
}

func TestAPIKeyVerifier_TouchLastUsed_Success(t *testing.T) {
	v, mock := newAPIKeyVerifierTest(t)
	defer mock.Close()

	mock.ExpectExec(`UPDATE api_key`).
		WithArgs(pgxmock.AnyArg(), "key_42").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := v.TouchLastUsed(context.Background(), "key_42")
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyVerifier_TouchLastUsed_EmptyID(t *testing.T) {
	v, _ := newAPIKeyVerifierTest(t)
	err := v.TouchLastUsed(context.Background(), "")
	require.Error(t, err)
}

func TestAPIKeyVerifier_TouchLastUsed_DBError(t *testing.T) {
	v, mock := newAPIKeyVerifierTest(t)
	defer mock.Close()

	sentinel := errors.New("deadlock detected")
	mock.ExpectExec(`UPDATE api_key`).
		WithArgs(pgxmock.AnyArg(), "key_x").
		WillReturnError(sentinel)

	err := v.TouchLastUsed(context.Background(), "key_x")
	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel)
	require.NoError(t, mock.ExpectationsWereMet())
}

// Compile-time interface check.
var _ middleware.APIKeyVerifier = (*APIKeyVerifier)(nil)
