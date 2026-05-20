package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
)

// anyArgs returns a slice of n pgxmock.AnyArg matchers.
func anyArgs(n int) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = pgxmock.AnyArg()
	}
	return out
}

// fakeEnqueuer captures EnqueueOrder calls without touching Redis.
type fakeEnqueuer struct {
	mu       sync.Mutex
	calls    []int64
	failWith error
}

func (f *fakeEnqueuer) EnqueueOrder(_ context.Context, orderID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failWith != nil {
		return f.failWith
	}
	f.calls = append(f.calls, orderID)
	return nil
}

func (f *fakeEnqueuer) Calls() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]int64, len(f.calls))
	copy(out, f.calls)
	return out
}

// quietLogger discards logs in unit tests.
func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newRenewalTest(t *testing.T, opts ...RenewalOption) (*RenewalScheduler, pgxmock.PgxPoolIface, *fakeEnqueuer) {
	t.Helper()
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	enq := &fakeEnqueuer{}
	defaults := []RenewalOption{
		WithRenewalInterval(20 * time.Millisecond),
		WithRenewalLead(30 * 24 * time.Hour),
		WithRenewalScanLimit(50),
		WithRenewalLogger(quietLogger()),
		withRenewalNow(func() time.Time { return time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC) }),
	}
	sched := NewRenewalScheduler(repo.NewWithPool(pool), enq, append(defaults, opts...)...)
	return sched, pool, enq
}

func certColumns() []string {
	return []string{
		"id", "order_id", "account_id", "sans", "issuer", "serial_hex",
		"fingerprint_sha256", "leaf_pem", "chain_pem", "key_kms_handle",
		"not_before", "not_after", "status", "revoked_at", "revoke_reason", "created_at",
	}
}

func sampleCertRow(id, orderID int64, notAfter time.Time) []any {
	return []any{
		id, orderID, "7", []string{"example.com"}, "letsencrypt", "01",
		"fp", "leaf", "chain", "kms://k",
		notAfter.Add(-90 * 24 * time.Hour), notAfter,
		"issued", (*time.Time)(nil), (*string)(nil), notAfter.Add(-90 * 24 * time.Hour),
	}
}

func renewalJobColumns() []string {
	return []string{
		"id", "cert_id", "scheduled_at", "attempt_count", "last_error",
		"status", "new_order_id", "created_at",
	}
}

func orderRowForRenew(id int64) []any {
	return []any{
		id, "7", []string{"example.com"}, []string(nil), (*string)(nil),
		"free-dv", "lets-encrypt",
		(*string)(nil), (*string)(nil), (*int64)(nil), 90,
		"dns-01", (*int64)(nil), "issued", (*string)(nil), (*int64)(nil),
		(*string)(nil), 0, (*string)(nil), (*string)(nil),
		time.Now().UTC(), (*time.Time)(nil),
	}
}

// --- tests ----------------------------------------------------------------

func TestNewRenewalScheduler_Defaults(t *testing.T) {
	sched := NewRenewalScheduler(&repo.Repos{}, &fakeEnqueuer{})
	assert.Equal(t, DefaultRenewalInterval, sched.interval)
	assert.Equal(t, DefaultRenewalLead, sched.leadTime)
	assert.Equal(t, defaultRenewalLimit, sched.scanLimit)
	assert.NotNil(t, sched.logger)
}

func TestRenewalOptions_Override(t *testing.T) {
	sched := NewRenewalScheduler(&repo.Repos{}, &fakeEnqueuer{},
		WithRenewalInterval(5*time.Second),
		WithRenewalLead(7*24*time.Hour),
		WithRenewalScanLimit(10),
		WithRenewalLogger(quietLogger()),
	)
	assert.Equal(t, 5*time.Second, sched.interval)
	assert.Equal(t, 7*24*time.Hour, sched.leadTime)
	assert.Equal(t, 10, sched.scanLimit)
}

func TestRenewalOptions_ZeroValuesIgnored(t *testing.T) {
	sched := NewRenewalScheduler(&repo.Repos{}, &fakeEnqueuer{},
		WithRenewalInterval(0),
		WithRenewalLead(0),
		WithRenewalScanLimit(0),
		WithRenewalLogger(nil),
	)
	assert.Equal(t, DefaultRenewalInterval, sched.interval)
	assert.Equal(t, DefaultRenewalLead, sched.leadTime)
	assert.Equal(t, defaultRenewalLimit, sched.scanLimit)
	assert.NotNil(t, sched.logger)
}

func TestRun_RejectsUnconfigured(t *testing.T) {
	sched := NewRenewalScheduler(nil, nil)
	err := sched.Run(context.Background())
	require.Error(t, err)
}

func TestScanAndEnqueue_NoExpiringCerts(t *testing.T) {
	sched, pool, enq := newRenewalTest(t)

	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE status = 'issued'`).
		WithArgs(pgxmock.AnyArg(), 50).
		WillReturnRows(pgxmock.NewRows(certColumns()))

	require.NoError(t, sched.scanAndEnqueue(context.Background()))
	assert.Empty(t, enq.Calls())
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestScanAndEnqueue_ListExpiringError(t *testing.T) {
	sched, pool, _ := newRenewalTest(t)

	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE status = 'issued'`).
		WithArgs(pgxmock.AnyArg(), 50).
		WillReturnError(errors.New("db down"))

	err := sched.scanAndEnqueue(context.Background())
	require.Error(t, err)
}

func TestScanAndEnqueue_HappyPath(t *testing.T) {
	sched, pool, enq := newRenewalTest(t)
	ctx := context.Background()
	notAfter := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	// 1) List expiring certs — one cert id=11 referencing order id=101.
	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE status = 'issued'`).
		WithArgs(pgxmock.AnyArg(), 50).
		WillReturnRows(pgxmock.NewRows(certColumns()).
			AddRow(sampleCertRow(11, 101, notAfter)...))

	// 2) List queued renewal jobs — empty so we proceed to enqueue.
	pool.ExpectQuery(`SELECT .+ FROM cert\.renewal_jobs\s+WHERE status = 'queued'`).
		WithArgs(50 * 4).
		WillReturnRows(pgxmock.NewRows(renewalJobColumns()))

	// 3) GetByID for source order 101.
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE id`).
		WithArgs(int64(101)).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "account_id", "sans", "sans_unicode", "common_name", "tier", "ca",
			"reseller_channel", "reseller_order_ref", "organization_id", "validity_days",
			"challenge_type", "dns_credential_id", "status", "csr_pem", "cert_id",
			"billing_invoice_id", "retry_count", "last_error", "idempotency_key",
			"created_at", "finalized_at",
		}).AddRow(orderRowForRenew(101)...))

	// 4) Insert new order — returns id=202.
	pool.ExpectQuery(`INSERT INTO cert\.orders`).
		WithArgs(anyArgs(16)...).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).
			AddRow(int64(202), time.Now().UTC()))

	// 5) Insert renewal_jobs row — returns id=303.
	pool.ExpectQuery(`INSERT INTO cert\.renewal_jobs`).
		WithArgs(int64(11), pgxmock.AnyArg(), "queued").
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).
			AddRow(int64(303), time.Now().UTC()))

	// 6) UpdateStatus to attach new_order_id.
	pool.ExpectExec(`UPDATE cert\.renewal_jobs\s+SET status`).
		WithArgs("queued", (*string)(nil), pgxmock.AnyArg(), int64(303)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	require.NoError(t, sched.scanAndEnqueue(ctx))
	assert.Equal(t, []int64{202}, enq.Calls())
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestScanAndEnqueue_SkipsCertWithActiveJob(t *testing.T) {
	sched, pool, enq := newRenewalTest(t)
	ctx := context.Background()
	notAfter := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE status = 'issued'`).
		WithArgs(pgxmock.AnyArg(), 50).
		WillReturnRows(pgxmock.NewRows(certColumns()).
			AddRow(sampleCertRow(11, 101, notAfter)...))

	// Queued list contains a job for cert id=11 — must skip.
	now := time.Now().UTC()
	pool.ExpectQuery(`SELECT .+ FROM cert\.renewal_jobs\s+WHERE status = 'queued'`).
		WithArgs(50 * 4).
		WillReturnRows(pgxmock.NewRows(renewalJobColumns()).AddRow(
			int64(50), int64(11), now, 0, (*string)(nil), "queued", (*int64)(nil), now,
		))

	require.NoError(t, sched.scanAndEnqueue(ctx))
	assert.Empty(t, enq.Calls())
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestScanAndEnqueue_ContinuesAfterPerCertError(t *testing.T) {
	sched, pool, enq := newRenewalTest(t)
	ctx := context.Background()
	notAfter := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	// Two expiring certs.
	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE status = 'issued'`).
		WithArgs(pgxmock.AnyArg(), 50).
		WillReturnRows(pgxmock.NewRows(certColumns()).
			AddRow(sampleCertRow(11, 101, notAfter)...).
			AddRow(sampleCertRow(12, 102, notAfter)...))

	pool.ExpectQuery(`SELECT .+ FROM cert\.renewal_jobs\s+WHERE status = 'queued'`).
		WithArgs(50 * 4).
		WillReturnRows(pgxmock.NewRows(renewalJobColumns()))

	// First cert: GetByID fails → enqueueOne returns error; loop continues.
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE id`).
		WithArgs(int64(101)).
		WillReturnError(errors.New("db hiccup"))

	// Second cert: full happy path.
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE id`).
		WithArgs(int64(102)).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "account_id", "sans", "sans_unicode", "common_name", "tier", "ca",
			"reseller_channel", "reseller_order_ref", "organization_id", "validity_days",
			"challenge_type", "dns_credential_id", "status", "csr_pem", "cert_id",
			"billing_invoice_id", "retry_count", "last_error", "idempotency_key",
			"created_at", "finalized_at",
		}).AddRow(orderRowForRenew(102)...))
	pool.ExpectQuery(`INSERT INTO cert\.orders`).
		WithArgs(anyArgs(16)...).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).
			AddRow(int64(202), time.Now().UTC()))
	pool.ExpectQuery(`INSERT INTO cert\.renewal_jobs`).
		WithArgs(int64(12), pgxmock.AnyArg(), "queued").
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).
			AddRow(int64(303), time.Now().UTC()))
	pool.ExpectExec(`UPDATE cert\.renewal_jobs\s+SET status`).
		WithArgs("queued", (*string)(nil), pgxmock.AnyArg(), int64(303)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	require.NoError(t, sched.scanAndEnqueue(ctx))
	assert.Equal(t, []int64{202}, enq.Calls())
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestScanAndEnqueue_EnqueueFailureDoesNotPanic(t *testing.T) {
	sched, pool, enq := newRenewalTest(t)
	enq.failWith = errors.New("redis down")
	ctx := context.Background()
	notAfter := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE status = 'issued'`).
		WithArgs(pgxmock.AnyArg(), 50).
		WillReturnRows(pgxmock.NewRows(certColumns()).
			AddRow(sampleCertRow(11, 101, notAfter)...))
	pool.ExpectQuery(`SELECT .+ FROM cert\.renewal_jobs\s+WHERE status = 'queued'`).
		WithArgs(50 * 4).
		WillReturnRows(pgxmock.NewRows(renewalJobColumns()))
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE id`).
		WithArgs(int64(101)).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "account_id", "sans", "sans_unicode", "common_name", "tier", "ca",
			"reseller_channel", "reseller_order_ref", "organization_id", "validity_days",
			"challenge_type", "dns_credential_id", "status", "csr_pem", "cert_id",
			"billing_invoice_id", "retry_count", "last_error", "idempotency_key",
			"created_at", "finalized_at",
		}).AddRow(orderRowForRenew(101)...))
	pool.ExpectQuery(`INSERT INTO cert\.orders`).
		WithArgs(anyArgs(16)...).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).
			AddRow(int64(202), time.Now().UTC()))
	pool.ExpectQuery(`INSERT INTO cert\.renewal_jobs`).
		WithArgs(int64(11), pgxmock.AnyArg(), "queued").
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).
			AddRow(int64(303), time.Now().UTC()))
	pool.ExpectExec(`UPDATE cert\.renewal_jobs\s+SET status`).
		WithArgs("queued", (*string)(nil), pgxmock.AnyArg(), int64(303)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// scanAndEnqueue swallows per-cert enqueue failures.
	require.NoError(t, sched.scanAndEnqueue(ctx))
	assert.Empty(t, enq.Calls())
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestScanAndEnqueue_ListQueuedError(t *testing.T) {
	sched, pool, _ := newRenewalTest(t)
	notAfter := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE status = 'issued'`).
		WithArgs(pgxmock.AnyArg(), 50).
		WillReturnRows(pgxmock.NewRows(certColumns()).
			AddRow(sampleCertRow(11, 101, notAfter)...))
	pool.ExpectQuery(`SELECT .+ FROM cert\.renewal_jobs\s+WHERE status = 'queued'`).
		WithArgs(50 * 4).
		WillReturnError(errors.New("io"))

	err := sched.scanAndEnqueue(context.Background())
	require.Error(t, err)
}

func TestRun_StopsOnContextCancel(t *testing.T) {
	sched, pool, _ := newRenewalTest(t,
		WithRenewalInterval(50*time.Millisecond),
	)

	// Expect at least one immediate scan; allow any number of subsequent
	// ticks before cancel.
	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE status = 'issued'`).
		WithArgs(pgxmock.AnyArg(), 50).
		WillReturnRows(pgxmock.NewRows(certColumns()))
	// Allow Tick #2 if it happens before cancel — match any subsequent
	// invocations by repeating the expectation.
	for i := 0; i < 10; i++ {
		pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE status = 'issued'`).
			WithArgs(pgxmock.AnyArg(), 50).
			WillReturnRows(pgxmock.NewRows(certColumns())).
			Maybe()
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- sched.Run(ctx) }()

	time.Sleep(80 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

func TestRun_SwallowsScanErrors(t *testing.T) {
	sched, pool, _ := newRenewalTest(t,
		WithRenewalInterval(30*time.Millisecond),
	)

	// First scan errors; subsequent scans return empty.
	pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE status = 'issued'`).
		WithArgs(pgxmock.AnyArg(), 50).
		WillReturnError(errors.New("scan bust"))
	for i := 0; i < 10; i++ {
		pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE status = 'issued'`).
			WithArgs(pgxmock.AnyArg(), 50).
			WillReturnRows(pgxmock.NewRows(certColumns())).
			Maybe()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	require.NoError(t, sched.Run(ctx))
}
