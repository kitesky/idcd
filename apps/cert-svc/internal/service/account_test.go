package service

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/lib/cert/vault"
	"github.com/kite365/idcd/lib/cert/vault/envmaster"
)

// newAccountTestVault builds a deterministic envmaster vault — the
// AccountManager round-trips through Encrypt/Decrypt and we want each
// test to start from a clean key.
func newAccountTestVault(t *testing.T) vault.Vault {
	t.Helper()
	key := bytes.Repeat([]byte{0x42}, 32)
	v, err := envmaster.NewWithKey(key)
	require.NoError(t, err)
	return v
}

func newAcctMgr(t *testing.T) (*AccountManager, pgxmock.PgxPoolIface, vault.Vault) {
	t.Helper()
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	v := newAccountTestVault(t)
	return NewAccountManager(repo.NewWithPool(pool), v), pool, v
}

func TestAccountManager_GetOrCreate_FirstCallInserts(t *testing.T) {
	mgr, pool, _ := newAcctMgr(t)
	ctx := context.Background()

	pool.ExpectQuery(`SELECT .+ FROM cert\.acme_accounts\s+WHERE ca`).
		WithArgs("lets-encrypt", "staging").
		WillReturnError(pgx.ErrNoRows)

	pool.ExpectQuery(`INSERT INTO cert\.acme_accounts`).
		WithArgs("lets-encrypt", "staging", "", pgxmock.AnyArg(), (*string)(nil), (*string)(nil)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).
			AddRow(int64(1), time.Now().UTC()))

	signer, err := mgr.GetOrCreate(ctx, "lets-encrypt", "staging")
	require.NoError(t, err)
	assert.NotNil(t, signer)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestAccountManager_GetOrCreate_SecondCallReads(t *testing.T) {
	mgr, pool, vlt := newAcctMgr(t)
	ctx := context.Background()

	// Generate a key + encode handle the same way the prod path does.
	_, ek, err := vlt.GenerateKey(ctx, vault.KeyAlgECDSAP256)
	require.NoError(t, err)
	handle, err := encodeKeyHandle(ek)
	require.NoError(t, err)

	pool.ExpectQuery(`SELECT .+ FROM cert\.acme_accounts\s+WHERE ca`).
		WithArgs("lets-encrypt", "prod").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "ca", "env", "account_url", "key_kms_handle",
			"eab_kid", "eab_hmac_kms_handle", "created_at",
		}).AddRow(int64(7), "lets-encrypt", "prod", "https://acme/acct/7",
			handle, (*string)(nil), (*string)(nil), time.Now().UTC()))

	signer, err := mgr.GetOrCreate(ctx, "lets-encrypt", "prod")
	require.NoError(t, err)
	assert.NotNil(t, signer)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestAccountManager_GetOrCreate_DecodeFailure(t *testing.T) {
	mgr, pool, _ := newAcctMgr(t)
	ctx := context.Background()

	pool.ExpectQuery(`SELECT .+ FROM cert\.acme_accounts`).
		WithArgs("ca", "env").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "ca", "env", "account_url", "key_kms_handle",
			"eab_kid", "eab_hmac_kms_handle", "created_at",
		}).AddRow(int64(1), "ca", "env", "u", "not-base64-$$$",
			(*string)(nil), (*string)(nil), time.Now().UTC()))

	_, err := mgr.GetOrCreate(ctx, "ca", "env")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode handle")
}

func TestAccountManager_GetOrCreate_LookupError(t *testing.T) {
	mgr, pool, _ := newAcctMgr(t)
	ctx := context.Background()

	pool.ExpectQuery(`SELECT .+ FROM cert\.acme_accounts`).
		WithArgs("ca", "env").
		WillReturnError(errors.New("db down"))

	_, err := mgr.GetOrCreate(ctx, "ca", "env")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lookup")
}

func TestAccountManager_GetOrCreate_InsertConflictRetries(t *testing.T) {
	mgr, pool, vlt := newAcctMgr(t)
	ctx := context.Background()

	// First attempt: not found, then insert hits a UNIQUE race, then the
	// retry SELECT returns a freshly-inserted row.
	pool.ExpectQuery(`SELECT .+ FROM cert\.acme_accounts`).
		WithArgs("ca", "env").
		WillReturnError(pgx.ErrNoRows)
	pool.ExpectQuery(`INSERT INTO cert\.acme_accounts`).
		WithArgs("ca", "env", "", pgxmock.AnyArg(), (*string)(nil), (*string)(nil)).
		WillReturnError(&pgconn.PgError{Code: "23505"})

	_, ek, err := vlt.GenerateKey(ctx, vault.KeyAlgECDSAP256)
	require.NoError(t, err)
	handle, err := encodeKeyHandle(ek)
	require.NoError(t, err)

	pool.ExpectQuery(`SELECT .+ FROM cert\.acme_accounts`).
		WithArgs("ca", "env").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "ca", "env", "account_url", "key_kms_handle",
			"eab_kid", "eab_hmac_kms_handle", "created_at",
		}).AddRow(int64(2), "ca", "env", "u", handle,
			(*string)(nil), (*string)(nil), time.Now().UTC()))

	signer, err := mgr.GetOrCreate(ctx, "ca", "env")
	require.NoError(t, err)
	assert.NotNil(t, signer)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestAccountManager_GetOrCreate_RejectsBadArgs(t *testing.T) {
	mgr, _, _ := newAcctMgr(t)
	_, err := mgr.GetOrCreate(context.Background(), "", "prod")
	require.Error(t, err)
	_, err = mgr.GetOrCreate(context.Background(), "ca", "")
	require.Error(t, err)
}

func TestAccountManager_GetOrCreate_NilManager(t *testing.T) {
	var mgr *AccountManager
	_, err := mgr.GetOrCreate(context.Background(), "ca", "env")
	require.Error(t, err)
}

func TestAccountManager_GetOrCreate_NotConfigured(t *testing.T) {
	mgr := &AccountManager{}
	_, err := mgr.GetOrCreate(context.Background(), "ca", "env")
	require.Error(t, err)
}
