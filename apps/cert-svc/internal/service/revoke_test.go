package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/lib/cert/ca"
)

// newRevokeService wires a Service with the minimum dependencies the
// revoke path needs: a mocked pool, a fakeCA behind the router, and a
// usable ACME account key. The vault is omitted on purpose — revoke does
// not decrypt a key, it signs the JWS with the account key directly.
func newRevokeService(t *testing.T, fake *fakeCA) (*Service, pgxmock.PgxPoolIface) {
	t.Helper()
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	accountKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	svc := New(Config{
		Repos:      repo.NewWithPool(pool),
		Router:     NewRouter(fake),
		AccountKey: accountKey,
	})
	return svc, pool
}

// revokeTestCertColumns mirrors repo.certsColumns — kept in sync with
// orchestrator tests so the rowscanner finds every field.
func revokeTestCertColumns() []string {
	return []string{
		"id", "order_id", "account_id", "sans", "issuer", "serial_hex",
		"fingerprint_sha256", "leaf_pem", "chain_pem", "key_kms_handle",
		"not_before", "not_after", "status", "revoked_at", "revoke_reason", "created_at",
	}
}

func revokeTestCertRow(id, accountID int64, status string) []any {
	now := time.Now().UTC()
	return []any{
		id, int64(1), accountID, []string{"example.com"}, "lets-encrypt", "abc",
		"fp", "-----LEAF-----\n", "-----CHAIN-----\n", "handle",
		now, now.Add(90 * 24 * time.Hour), status,
		(*time.Time)(nil), (*string)(nil), now,
	}
}

// expectAuditAppend matches the INSERT INTO cert.audit_logs SQL emitted
// from AuditLogsRepo.Append. The payload is matched with AnyArg because
// we do not pin its JSON spelling here — the field-level test below
// covers the contents.
func expectAuditAppend(mock pgxmock.PgxPoolIface) {
	mock.ExpectQuery(`INSERT INTO cert\.audit_logs`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "occurred_at"}).
			AddRow(int64(99), time.Now()))
}

// TestRevokeCert_HappyPath drives issued → revoking → revoked, asserts
// the audit trail is written and any active renewal job is abandoned.
func TestRevokeCert_HappyPath(t *testing.T) {
	fake := &fakeCA{name: "lets-encrypt"}
	svc, mock := newRevokeService(t, fake)

	// 1. GetByID(certID=7)
	mock.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(7)).
		WillReturnRows(pgxmock.NewRows(revokeTestCertColumns()).
			AddRow(revokeTestCertRow(7, 42, "issued")...))

	// 2. UpdateStatus issued → revoking
	mock.ExpectExec(`UPDATE cert\.certs\s+SET status`).
		WithArgs("revoking", pgxmock.AnyArg(), pgxmock.AnyArg(),
			int64(7), "issued").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// 3. audit_logs append (started)
	expectAuditAppend(mock)

	// 4. (CA revoke is in-memory via fakeCA)

	// 5. UpdateStatus revoking → revoked
	mock.ExpectExec(`UPDATE cert\.certs\s+SET status`).
		WithArgs("revoked", pgxmock.AnyArg(), pgxmock.AnyArg(),
			int64(7), "revoking").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// 6. ListQueued (no jobs)
	mock.ExpectQuery(`SELECT .+ FROM cert\.renewal_jobs\s+WHERE status = 'queued'`).
		WithArgs(200).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "cert_id", "scheduled_at", "attempt_count", "last_error",
			"status", "new_order_id", "created_at",
		}))

	// 7. audit_logs append (completed)
	expectAuditAppend(mock)

	err := svc.RevokeCert(context.Background(), 42, 7, ca.RevokeUnspecified)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, 1, fake.revokeCalls)
}

// TestRevokeCert_CancelsActiveRenewalJobs verifies that a queued renewal
// job for the revoked cert is flipped to "abandoned"; jobs for other
// certs are left alone.
func TestRevokeCert_CancelsActiveRenewalJobs(t *testing.T) {
	fake := &fakeCA{name: "lets-encrypt"}
	svc, mock := newRevokeService(t, fake)

	mock.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(7)).
		WillReturnRows(pgxmock.NewRows(revokeTestCertColumns()).
			AddRow(revokeTestCertRow(7, 42, "issued")...))
	mock.ExpectExec(`UPDATE cert\.certs\s+SET status`).
		WithArgs("revoking", pgxmock.AnyArg(), pgxmock.AnyArg(),
			int64(7), "issued").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	expectAuditAppend(mock)
	mock.ExpectExec(`UPDATE cert\.certs\s+SET status`).
		WithArgs("revoked", pgxmock.AnyArg(), pgxmock.AnyArg(),
			int64(7), "revoking").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// One job for cert 7 (matching), one for cert 999 (skipped).
	now := time.Now().UTC()
	mock.ExpectQuery(`SELECT .+ FROM cert\.renewal_jobs\s+WHERE status = 'queued'`).
		WithArgs(200).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "cert_id", "scheduled_at", "attempt_count", "last_error",
			"status", "new_order_id", "created_at",
		}).
			AddRow(int64(1), int64(7), now, 0, (*string)(nil), "queued", (*int64)(nil), now).
			AddRow(int64(2), int64(999), now, 0, (*string)(nil), "queued", (*int64)(nil), now))

	// UpdateStatus abandoned for job 1 only.
	mock.ExpectExec(`UPDATE cert\.renewal_jobs\s+SET status`).
		WithArgs("abandoned", pgxmock.AnyArg(), pgxmock.AnyArg(), int64(1)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	expectAuditAppend(mock)

	err := svc.RevokeCert(context.Background(), 42, 7, ca.RevokeKeyCompromise)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestRevokeCert_NotOwned surfaces ErrForbidden when the cert belongs to
// a different account; nothing is written to the DB.
func TestRevokeCert_NotOwned(t *testing.T) {
	fake := &fakeCA{name: "lets-encrypt"}
	svc, mock := newRevokeService(t, fake)

	mock.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(7)).
		WillReturnRows(pgxmock.NewRows(revokeTestCertColumns()).
			AddRow(revokeTestCertRow(7, 999, "issued")...))

	err := svc.RevokeCert(context.Background(), 42, 7, ca.RevokeUnspecified)
	assert.ErrorIs(t, err, ErrForbidden)
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, 0, fake.revokeCalls)
}

// TestRevokeCert_NotIssued rejects a cert that is already revoked /
// expired — the user wants 409, not a re-revoke.
func TestRevokeCert_NotIssued(t *testing.T) {
	fake := &fakeCA{name: "lets-encrypt"}
	svc, mock := newRevokeService(t, fake)

	mock.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(7)).
		WillReturnRows(pgxmock.NewRows(revokeTestCertColumns()).
			AddRow(revokeTestCertRow(7, 42, "revoked")...))

	err := svc.RevokeCert(context.Background(), 42, 7, ca.RevokeUnspecified)
	assert.ErrorIs(t, err, ErrInvalidStatus)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestRevokeCert_CAFailureRollsBack: when the CA call errors, status is
// rolled back to issued and a failure audit row is written. The error
// from the CA layer is preserved so the handler can branch on it.
func TestRevokeCert_CAFailureRollsBack(t *testing.T) {
	fake := &fakeCA{name: "lets-encrypt", revokeErr: ca.ErrNetwork}
	svc, mock := newRevokeService(t, fake)

	mock.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(7)).
		WillReturnRows(pgxmock.NewRows(revokeTestCertColumns()).
			AddRow(revokeTestCertRow(7, 42, "issued")...))
	mock.ExpectExec(`UPDATE cert\.certs\s+SET status`).
		WithArgs("revoking", pgxmock.AnyArg(), pgxmock.AnyArg(),
			int64(7), "issued").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	expectAuditAppend(mock) // started

	// CA returns ErrNetwork → rollback to issued + failure audit.
	mock.ExpectExec(`UPDATE cert\.certs\s+SET status`).
		WithArgs("issued", pgxmock.AnyArg(), pgxmock.AnyArg(),
			int64(7), "revoking").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	expectAuditAppend(mock) // failed

	err := svc.RevokeCert(context.Background(), 42, 7, ca.RevokeUnspecified)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ca.ErrNetwork),
		"caller must be able to branch on ca.ErrNetwork (got %v)", err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestRevokeCert_NotFound surfaces ErrNotFound when the cert id does not
// exist.
func TestRevokeCert_NotFound(t *testing.T) {
	fake := &fakeCA{name: "lets-encrypt"}
	svc, mock := newRevokeService(t, fake)

	mock.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(99)).
		WillReturnRows(pgxmock.NewRows(revokeTestCertColumns()))

	err := svc.RevokeCert(context.Background(), 42, 99, ca.RevokeUnspecified)
	assert.ErrorIs(t, err, ErrNotFound)
}

// TestRevokeCert_NotConfigured surfaces ErrNotConfigured when the
// service was constructed without an account key.
func TestRevokeCert_NotConfigured(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	svc := New(Config{
		Repos:  repo.NewWithPool(pool),
		Router: NewRouter(&fakeCA{name: "lets-encrypt"}),
		// AccountKey omitted on purpose.
	})

	err = svc.RevokeCert(context.Background(), 42, 7, ca.RevokeUnspecified)
	assert.ErrorIs(t, err, ErrNotConfigured)
}

// TestRevokeCert_NoReposReturnsNotConfigured: defensive check that the
// service refuses to operate without repos wired.
func TestRevokeCert_NoReposReturnsNotConfigured(t *testing.T) {
	svc := New(Config{})
	err := svc.RevokeCert(context.Background(), 42, 7, ca.RevokeUnspecified)
	assert.ErrorIs(t, err, ErrNotConfigured)
}

// TestReasonString covers every supported reason and the fallback for
// unknown values.
func TestReasonString(t *testing.T) {
	cases := []struct {
		in   ca.RevokeReason
		want string
	}{
		{ca.RevokeUnspecified, "unspecified"},
		{ca.RevokeKeyCompromise, "keyCompromise"},
		{ca.RevokeCessationOfOperation, "cessationOfOperation"},
		{ca.RevokeCertificateHold, "certificateHold"},
		{ca.RevokeReason(99), "unspecified"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, reasonString(c.in), "reason=%v", c.in)
	}
}
