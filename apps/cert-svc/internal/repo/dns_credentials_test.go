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

func newDNSCredentialsRepo(t *testing.T) (*DNSCredentialsRepo, pgxmock.PgxPoolIface) {
	t.Helper()
	pool := newMockPool(t)
	return &DNSCredentialsRepo{pool: pool}, pool
}

func sampleDNSCred() *DNSCredential {
	return &DNSCredential{
		AccountID:     "42",
		Provider:      "cloudflare",
		DisplayName:   "primary",
		EncryptedBlob: []byte("enc"),
		DEKWrapped:    []byte("dek"),
		KEKKeyID:      "kek-1",
	}
}

func TestDNSCredentialsRepo_Insert_Success(t *testing.T) {
	r, mock := newDNSCredentialsRepo(t)
	c := sampleDNSCred()
	now := time.Now().UTC()

	mock.ExpectQuery(`INSERT INTO cert\.dns_credentials`).
		WithArgs("42", "cloudflare", "primary", []byte("enc"), []byte("dek"), "kek-1", "unknown").
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow(int64(13), now))

	id, err := r.Insert(context.Background(), c)
	require.NoError(t, err)
	assert.Equal(t, int64(13), id)
	assert.Equal(t, "unknown", c.HealthStatus)
}

func TestDNSCredentialsRepo_Insert_PreservesHealthStatus(t *testing.T) {
	r, mock := newDNSCredentialsRepo(t)
	c := sampleDNSCred()
	c.HealthStatus = "ok"

	mock.ExpectQuery(`INSERT INTO cert\.dns_credentials`).
		WithArgs("42", "cloudflare", "primary", []byte("enc"), []byte("dek"), "kek-1", "ok").
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow(int64(1), time.Now()))
	_, err := r.Insert(context.Background(), c)
	require.NoError(t, err)
}

func TestDNSCredentialsRepo_Insert_UniqueViolation(t *testing.T) {
	r, mock := newDNSCredentialsRepo(t)
	mock.ExpectQuery(`INSERT INTO cert\.dns_credentials`).
		WithArgs(anyArgs(7)...).
		WillReturnError(&pgconn.PgError{Code: pgUniqueViolation})
	_, err := r.Insert(context.Background(), sampleDNSCred())
	assert.ErrorIs(t, err, ErrConflict)
}

func TestDNSCredentialsRepo_Insert_DBError(t *testing.T) {
	r, mock := newDNSCredentialsRepo(t)
	mock.ExpectQuery(`INSERT INTO cert\.dns_credentials`).
		WithArgs(anyArgs(7)...).
		WillReturnError(errors.New("io"))
	_, err := r.Insert(context.Background(), sampleDNSCred())
	require.Error(t, err)
}

func TestDNSCredentialsRepo_GetByID_Success(t *testing.T) {
	r, mock := newDNSCredentialsRepo(t)
	now := time.Now().UTC()

	mock.ExpectQuery(`SELECT id, account_id, provider, display_name,\s+encrypted_blob`).
		WithArgs(int64(13)).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "account_id", "provider", "display_name",
			"encrypted_blob", "dek_wrapped", "kek_key_id",
			"health_status", "health_checked_at", "created_at", "revoked_at",
		}).AddRow(int64(13), "42", "cloudflare", "primary",
			[]byte("enc"), []byte("dek"), "kek-1",
			"ok", &now, now, (*time.Time)(nil)))

	c, err := r.GetByID(context.Background(), 13)
	require.NoError(t, err)
	assert.Equal(t, []byte("enc"), c.EncryptedBlob)
}

func TestDNSCredentialsRepo_GetByID_NotFound(t *testing.T) {
	r, mock := newDNSCredentialsRepo(t)
	mock.ExpectQuery(`SELECT .+ FROM cert\.dns_credentials`).
		WithArgs(int64(0)).
		WillReturnError(pgx.ErrNoRows)
	_, err := r.GetByID(context.Background(), 0)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestDNSCredentialsRepo_GetByID_DBError(t *testing.T) {
	r, mock := newDNSCredentialsRepo(t)
	mock.ExpectQuery(`SELECT .+ FROM cert\.dns_credentials`).
		WithArgs(int64(1)).
		WillReturnError(errors.New("io"))
	_, err := r.GetByID(context.Background(), 1)
	require.Error(t, err)
}

func TestDNSCredentialsRepo_ListByAccount_OmitsBlobs(t *testing.T) {
	r, mock := newDNSCredentialsRepo(t)
	now := time.Now().UTC()

	mock.ExpectQuery(`SELECT id, account_id, provider, display_name, kek_key_id`).
		WithArgs("42").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "account_id", "provider", "display_name", "kek_key_id",
			"health_status", "health_checked_at", "created_at", "revoked_at",
		}).AddRow(int64(1), "42", "cloudflare", "primary", "kek-1",
			"ok", &now, now, (*time.Time)(nil)))

	out, err := r.ListByAccount(context.Background(), "42")
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Nil(t, out[0].EncryptedBlob)
	assert.Nil(t, out[0].DEKWrapped)
}

func TestDNSCredentialsRepo_ListByAccount_DBError(t *testing.T) {
	r, mock := newDNSCredentialsRepo(t)
	mock.ExpectQuery(`SELECT .+ FROM cert\.dns_credentials`).
		WithArgs("1").
		WillReturnError(errors.New("io"))
	_, err := r.ListByAccount(context.Background(), "1")
	require.Error(t, err)
}

func TestDNSCredentialsRepo_Revoke(t *testing.T) {
	r, mock := newDNSCredentialsRepo(t)

	mock.ExpectExec(`UPDATE cert\.dns_credentials\s+SET revoked_at`).
		WithArgs(pgxmock.AnyArg(), int64(13)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	require.NoError(t, r.Revoke(context.Background(), 13))

	mock.ExpectExec(`UPDATE cert\.dns_credentials\s+SET revoked_at`).
		WithArgs(pgxmock.AnyArg(), int64(0)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	assert.ErrorIs(t, r.Revoke(context.Background(), 0), ErrNotFound)

	mock.ExpectExec(`UPDATE cert\.dns_credentials\s+SET revoked_at`).
		WithArgs(pgxmock.AnyArg(), int64(1)).
		WillReturnError(errors.New("io"))
	require.Error(t, r.Revoke(context.Background(), 1))
}

func TestDNSCredentialsRepo_UpdateHealthStatus(t *testing.T) {
	r, mock := newDNSCredentialsRepo(t)

	mock.ExpectExec(`UPDATE cert\.dns_credentials\s+SET health_status`).
		WithArgs("ok", pgxmock.AnyArg(), int64(13)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	require.NoError(t, r.UpdateHealthStatus(context.Background(), 13, "ok"))

	mock.ExpectExec(`UPDATE cert\.dns_credentials\s+SET health_status`).
		WithArgs("ok", pgxmock.AnyArg(), int64(0)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	assert.ErrorIs(t, r.UpdateHealthStatus(context.Background(), 0, "ok"), ErrNotFound)

	mock.ExpectExec(`UPDATE cert\.dns_credentials\s+SET health_status`).
		WithArgs("ok", pgxmock.AnyArg(), int64(1)).
		WillReturnError(errors.New("io"))
	require.Error(t, r.UpdateHealthStatus(context.Background(), 1, "ok"))
}
