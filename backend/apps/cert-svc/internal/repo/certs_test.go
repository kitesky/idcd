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

func newCertsRepo(t *testing.T) (*CertsRepo, pgxmock.PgxPoolIface) {
	t.Helper()
	pool := newMockPool(t)
	return &CertsRepo{pool: pool}, pool
}

func sampleCert() *Cert {
	now := time.Now().UTC()
	return &Cert{
		OrderID:           101,
		AccountID:         "42",
		SANs:              []string{"example.com"},
		Issuer:            "Let's Encrypt R3",
		SerialHex:         "deadbeef",
		FingerprintSHA256: "fp",
		LeafPEM:           "leaf",
		ChainPEM:          "chain",
		KeyKMSHandle:      "kms://k/v1",
		NotBefore:         now,
		NotAfter:          now.Add(90 * 24 * time.Hour),
		Status:            CertStatusIssued,
	}
}

func certColumns() []string {
	return []string{
		"id", "order_id", "account_id", "sans", "issuer", "serial_hex",
		"fingerprint_sha256", "leaf_pem", "chain_pem", "key_kms_handle",
		"not_before", "not_after", "status", "revoked_at", "revoke_reason", "created_at",
	}
}

func sampleCertRow(id int64) []any {
	now := time.Now().UTC()
	return []any{
		id, int64(101), "42", []string{"example.com"}, "Let's Encrypt R3", "deadbeef",
		"fp", "leaf", "chain", "kms://k/v1",
		now, now.Add(90 * 24 * time.Hour), "issued", (*time.Time)(nil), (*string)(nil), now,
	}
}

func TestCertsRepo_Insert_Success(t *testing.T) {
	r, mock := newCertsRepo(t)
	c := sampleCert()
	now := time.Now().UTC()

	mock.ExpectQuery(`INSERT INTO cert\.certs`).
		WithArgs(anyArgs(12)...).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow(int64(555), now))

	id, err := r.Insert(context.Background(), c)
	require.NoError(t, err)
	assert.Equal(t, int64(555), id)
	assert.Equal(t, int64(555), c.ID)
}

func TestCertsRepo_Insert_UniqueViolation(t *testing.T) {
	r, mock := newCertsRepo(t)
	mock.ExpectQuery(`INSERT INTO cert\.certs`).
		WithArgs(anyArgs(12)...).
		WillReturnError(&pgconn.PgError{Code: pgUniqueViolation})

	id, err := r.Insert(context.Background(), sampleCert())
	assert.ErrorIs(t, err, ErrConflict)
	assert.Equal(t, int64(0), id)
}

func TestCertsRepo_Insert_DBError(t *testing.T) {
	r, mock := newCertsRepo(t)
	mock.ExpectQuery(`INSERT INTO cert\.certs`).
		WithArgs(anyArgs(12)...).
		WillReturnError(errors.New("io"))
	_, err := r.Insert(context.Background(), sampleCert())
	require.Error(t, err)
}

func TestCertsRepo_GetByID(t *testing.T) {
	r, mock := newCertsRepo(t)

	mock.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(555)).
		WillReturnRows(pgxmock.NewRows(certColumns()).AddRow(sampleCertRow(555)...))

	c, err := r.GetByID(context.Background(), 555)
	require.NoError(t, err)
	assert.Equal(t, CertStatusIssued, c.Status)
}

func TestCertsRepo_GetByID_NotFound(t *testing.T) {
	r, mock := newCertsRepo(t)
	mock.ExpectQuery(`SELECT .+ FROM cert\.certs`).
		WithArgs(int64(0)).
		WillReturnError(pgx.ErrNoRows)
	_, err := r.GetByID(context.Background(), 0)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestCertsRepo_GetByID_DBError(t *testing.T) {
	r, mock := newCertsRepo(t)
	mock.ExpectQuery(`SELECT .+ FROM cert\.certs`).
		WithArgs(int64(0)).
		WillReturnError(errors.New("io"))
	_, err := r.GetByID(context.Background(), 0)
	require.Error(t, err)
}

func TestCertsRepo_ListByAccount(t *testing.T) {
	r, mock := newCertsRepo(t)

	mock.ExpectQuery(`WHERE account_id = \$1\s+ORDER BY`).
		WithArgs("42", 5, 0).
		WillReturnRows(pgxmock.NewRows(certColumns()).AddRow(sampleCertRow(1)...))

	out, err := r.ListByAccount(context.Background(), "42", nil, 5, 0)
	require.NoError(t, err)
	require.Len(t, out, 1)

	st := CertStatusIssued
	mock.ExpectQuery(`WHERE account_id = \$1 AND status = \$2`).
		WithArgs("42", "issued", 5, 0).
		WillReturnRows(pgxmock.NewRows(certColumns()).AddRow(sampleCertRow(2)...))
	out, err = r.ListByAccount(context.Background(), "42", &st, 5, 0)
	require.NoError(t, err)
	require.Len(t, out, 1)
}

func TestCertsRepo_ListByAccount_DBError(t *testing.T) {
	r, mock := newCertsRepo(t)
	mock.ExpectQuery(`SELECT .+ FROM cert\.certs`).
		WithArgs("1", 10, 0).
		WillReturnError(errors.New("io"))
	_, err := r.ListByAccount(context.Background(), "1", nil, 10, 0)
	require.Error(t, err)
}

func TestCertsRepo_ListExpiringBefore(t *testing.T) {
	r, mock := newCertsRepo(t)
	t0 := time.Now().UTC()

	mock.ExpectQuery(`WHERE status = 'issued' AND not_after < \$1`).
		WithArgs(t0, 100).
		WillReturnRows(pgxmock.NewRows(certColumns()).AddRow(sampleCertRow(9)...))

	out, err := r.ListExpiringBefore(context.Background(), t0, 100)
	require.NoError(t, err)
	require.Len(t, out, 1)
}

func TestCertsRepo_ListExpiringBefore_DBError(t *testing.T) {
	r, mock := newCertsRepo(t)
	mock.ExpectQuery(`WHERE status = 'issued'`).
		WithArgs(pgxmock.AnyArg(), 10).
		WillReturnError(errors.New("io"))
	_, err := r.ListExpiringBefore(context.Background(), time.Now(), 10)
	require.Error(t, err)
}

func TestCertsRepo_UpdateStatus_OK(t *testing.T) {
	r, mock := newCertsRepo(t)
	now := time.Now().UTC()
	reason := "key compromised"

	mock.ExpectExec(`UPDATE cert\.certs\s+SET status`).
		WithArgs("revoked", &now, &reason, int64(555), "issued").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	require.NoError(t, r.UpdateStatus(context.Background(), 555, CertStatusIssued, CertStatusRevoked, &now, &reason))
}

func TestCertsRepo_UpdateStatus_OptimisticLockMiss(t *testing.T) {
	r, mock := newCertsRepo(t)
	mock.ExpectExec(`UPDATE cert\.certs\s+SET status`).
		WithArgs("expired", (*time.Time)(nil), (*string)(nil), int64(1), "issued").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	err := r.UpdateStatus(context.Background(), 1, CertStatusIssued, CertStatusExpired, nil, nil)
	assert.ErrorIs(t, err, ErrInvalidStatus)
}

func TestCertsRepo_UpdateStatus_DBError(t *testing.T) {
	r, mock := newCertsRepo(t)
	mock.ExpectExec(`UPDATE cert\.certs`).
		WithArgs("revoked", (*time.Time)(nil), (*string)(nil), int64(1), "issued").
		WillReturnError(errors.New("io"))
	err := r.UpdateStatus(context.Background(), 1, CertStatusIssued, CertStatusRevoked, nil, nil)
	require.Error(t, err)
}

func TestCertsRepo_MaxCertsPerRegisteredDomainSince(t *testing.T) {
	r, mock := newCertsRepo(t)
	since := time.Now().UTC()

	mock.ExpectQuery(`WHERE issuer = \$1 AND created_at >= \$2`).
		WithArgs("lets-encrypt", since).
		WillReturnRows(pgxmock.NewRows([]string{"max"}).AddRow(37))

	n, err := r.MaxCertsPerRegisteredDomainSince(context.Background(), "lets-encrypt", since)
	require.NoError(t, err)
	require.Equal(t, 37, n)
}

func TestCertsRepo_MaxCertsPerRegisteredDomainSince_DBError(t *testing.T) {
	r, mock := newCertsRepo(t)
	mock.ExpectQuery(`MAX`).
		WithArgs("lets-encrypt", pgxmock.AnyArg()).
		WillReturnError(errors.New("io"))
	_, err := r.MaxCertsPerRegisteredDomainSince(context.Background(), "lets-encrypt", time.Now())
	require.Error(t, err)
}
