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

func newACMERepo(t *testing.T) (*ACMEAccountsRepo, pgxmock.PgxPoolIface) {
	t.Helper()
	pool := newMockPool(t)
	return &ACMEAccountsRepo{pool: pool}, pool
}

func TestACMEAccountsRepo_GetByCAEnv_Success(t *testing.T) {
	r, mock := newACMERepo(t)
	now := time.Now().UTC()
	kid := "kid"
	hmac := "kms://h"

	mock.ExpectQuery(`SELECT .+ FROM cert\.acme_accounts\s+WHERE ca`).
		WithArgs("lets-encrypt", "prod").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "ca", "env", "account_url", "key_kms_handle",
			"eab_kid", "eab_hmac_kms_handle", "created_at",
		}).AddRow(int64(1), "lets-encrypt", "prod", "https://acme/acct/1", "kms://k1", &kid, &hmac, now))

	a, err := r.GetByCAEnv(context.Background(), "lets-encrypt", "prod")
	require.NoError(t, err)
	assert.Equal(t, int64(1), a.ID)
	assert.Equal(t, "kid", *a.EABKID)
}

func TestACMEAccountsRepo_GetByCAEnv_NotFound(t *testing.T) {
	r, mock := newACMERepo(t)
	mock.ExpectQuery(`SELECT .+ FROM cert\.acme_accounts`).
		WithArgs("x", "y").
		WillReturnError(pgx.ErrNoRows)
	_, err := r.GetByCAEnv(context.Background(), "x", "y")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestACMEAccountsRepo_GetByCAEnv_DBError(t *testing.T) {
	r, mock := newACMERepo(t)
	mock.ExpectQuery(`SELECT .+ FROM cert\.acme_accounts`).
		WithArgs("x", "y").
		WillReturnError(errors.New("io"))
	_, err := r.GetByCAEnv(context.Background(), "x", "y")
	require.Error(t, err)
}

func TestACMEAccountsRepo_Insert_Success(t *testing.T) {
	r, mock := newACMERepo(t)
	a := &ACMEAccount{CA: "lets-encrypt", Env: "prod", AccountURL: "u", KeyKMSHandle: "k"}
	now := time.Now().UTC()

	mock.ExpectQuery(`INSERT INTO cert\.acme_accounts`).
		WithArgs(a.CA, a.Env, a.AccountURL, a.KeyKMSHandle, a.EABKID, a.EABHMACKMSHandle).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow(int64(2), now))

	id, err := r.Insert(context.Background(), a)
	require.NoError(t, err)
	assert.Equal(t, int64(2), id)
}

func TestACMEAccountsRepo_Insert_UniqueViolation(t *testing.T) {
	r, mock := newACMERepo(t)
	mock.ExpectQuery(`INSERT INTO cert\.acme_accounts`).
		WithArgs(anyArgs(6)...).
		WillReturnError(&pgconn.PgError{Code: pgUniqueViolation})
	_, err := r.Insert(context.Background(), &ACMEAccount{})
	assert.ErrorIs(t, err, ErrConflict)
}

func TestACMEAccountsRepo_Insert_DBError(t *testing.T) {
	r, mock := newACMERepo(t)
	mock.ExpectQuery(`INSERT INTO cert\.acme_accounts`).
		WithArgs(anyArgs(6)...).
		WillReturnError(errors.New("io"))
	_, err := r.Insert(context.Background(), &ACMEAccount{})
	require.Error(t, err)
}
