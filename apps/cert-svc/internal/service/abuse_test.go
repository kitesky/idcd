package service

import (
	"context"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
)

// abuseOrderRowColumns mirrors repo.ordersColumns (kept local so we
// don't touch the repo package's test helpers).
func abuseOrderRowColumns() []string {
	return []string{
		"id", "account_id", "sans", "sans_unicode", "common_name", "tier", "ca",
		"reseller_channel", "reseller_order_ref", "organization_id", "validity_days",
		"challenge_type", "dns_credential_id", "status", "csr_pem", "cert_id",
		"billing_invoice_id", "retry_count", "last_error", "idempotency_key",
		"created_at", "finalized_at",
	}
}

func abuseHistoryRow(id int64, sans []string, createdAt time.Time) []any {
	return []any{
		id, int64(42), sans, sans, (*string)(nil),
		"free-dv", "letsencrypt",
		(*string)(nil), (*string)(nil), (*int64)(nil), 90,
		"dns-01", (*int64)(nil), "issued", (*string)(nil), (*int64)(nil),
		(*string)(nil), 0, (*string)(nil), (*string)(nil),
		createdAt, (*time.Time)(nil),
	}
}

func newAbuseDetectorWithMock(t *testing.T, opts ...AbuseOption) (*AbuseDetector, pgxmock.PgxPoolIface) {
	t.Helper()
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	repos := repo.NewWithPool(pool)
	return NewAbuseDetector(repos, opts...), pool
}

func TestAbuse_Blocklist_RootMatch(t *testing.T) {
	a := NewAbuseDetector(nil)
	err := a.Check(context.Background(), 42, []string{"taobao.com"})
	require.ErrorIs(t, err, ErrAbuseBlocked)
}

func TestAbuse_Blocklist_SubdomainMatch(t *testing.T) {
	a := NewAbuseDetector(nil)
	err := a.Check(context.Background(), 42, []string{"shop.tmall.com"})
	require.ErrorIs(t, err, ErrAbuseBlocked)
}

func TestAbuse_Blocklist_GovCNMatchesGovCNRoot(t *testing.T) {
	a := NewAbuseDetector(nil)
	err := a.Check(context.Background(), 42, []string{"moe.gov.cn"})
	require.ErrorIs(t, err, ErrAbuseBlocked)
}

func TestAbuse_Blocklist_NoFalsePositive(t *testing.T) {
	// "notbaidu.com" must NOT match the "baidu.com" blocklist entry.
	a, pool := newAbuseDetectorWithMock(t)
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders`).
		WithArgs(int64(42), 500, 0).
		WillReturnRows(pgxmock.NewRows(abuseOrderRowColumns()))
	require.NoError(t, a.Check(context.Background(), 42, []string{"shop.notbaidu.com"}))
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestAbuse_Blocklist_Wildcard(t *testing.T) {
	a := NewAbuseDetector(nil)
	err := a.Check(context.Background(), 42, []string{"*.alipay.com"})
	require.ErrorIs(t, err, ErrAbuseBlocked)
}

func TestAbuse_Burst_FiveDistinctRootsIn1h(t *testing.T) {
	clock := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	a, pool := newAbuseDetectorWithMock(t,
		WithAbuseClock(func() time.Time { return clock }),
	)
	// Five prior distinct roots within last hour.
	rows := pgxmock.NewRows(abuseOrderRowColumns())
	for i, d := range []string{"a.com", "b.com", "c.com", "d.com", "e.com"} {
		rows.AddRow(abuseHistoryRow(int64(100+i), []string{d}, clock.Add(-30*time.Minute))...)
	}
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders`).
		WithArgs(int64(42), 500, 0).
		WillReturnRows(rows)

	// The new order adds a 6th distinct root.
	err := a.Check(context.Background(), 42, []string{"f.com"})
	require.ErrorIs(t, err, ErrAbuseBlocked)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestAbuse_Burst_HistoryOutsideWindowIsIgnored(t *testing.T) {
	clock := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	a, pool := newAbuseDetectorWithMock(t,
		WithAbuseClock(func() time.Time { return clock }),
	)
	rows := pgxmock.NewRows(abuseOrderRowColumns())
	// Five prior orders but all older than 1h.
	for i, d := range []string{"a.com", "b.com", "c.com", "d.com", "e.com"} {
		rows.AddRow(abuseHistoryRow(int64(100+i), []string{d}, clock.Add(-2*time.Hour))...)
	}
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders`).
		WithArgs(int64(42), 500, 0).
		WillReturnRows(rows)

	require.NoError(t, a.Check(context.Background(), 42, []string{"f.com"}))
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestAbuse_Sustained_TenOrdersInWeek(t *testing.T) {
	clock := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	a, pool := newAbuseDetectorWithMock(t,
		WithAbuseClock(func() time.Time { return clock }),
	)
	rows := pgxmock.NewRows(abuseOrderRowColumns())
	for i := 0; i < 10; i++ {
		rows.AddRow(abuseHistoryRow(int64(100+i), []string{"target.com"}, clock.Add(-time.Duration(i+1)*time.Hour))...)
	}
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders`).
		WithArgs(int64(42), 500, 0).
		WillReturnRows(rows)

	err := a.Check(context.Background(), 42, []string{"foo.target.com"})
	require.ErrorIs(t, err, ErrAbuseBlocked)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestAbuse_Allow_QuietAccount(t *testing.T) {
	clock := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	a, pool := newAbuseDetectorWithMock(t,
		WithAbuseClock(func() time.Time { return clock }),
	)
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders`).
		WithArgs(int64(42), 500, 0).
		WillReturnRows(pgxmock.NewRows(abuseOrderRowColumns()))

	require.NoError(t, a.Check(context.Background(), 42, []string{"new.example.com"}))
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestAbuse_Allow_HistoryFetchFailsOpens(t *testing.T) {
	clock := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	a, pool := newAbuseDetectorWithMock(t,
		WithAbuseClock(func() time.Time { return clock }),
	)
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders`).
		WithArgs(int64(42), 500, 0).
		WillReturnError(context.DeadlineExceeded)
	require.NoError(t, a.Check(context.Background(), 42, []string{"example.com"}))
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestAbuse_NilDetector_Safe(t *testing.T) {
	var a *AbuseDetector
	require.NoError(t, a.Check(context.Background(), 42, []string{"example.com"}))
}

func TestAbuse_EmptySANs(t *testing.T) {
	a := NewAbuseDetector(nil)
	require.NoError(t, a.Check(context.Background(), 42, nil))
}

func TestAbuse_CustomBlocklist(t *testing.T) {
	a := NewAbuseDetector(nil, WithAbuseBlocklist([]string{"forbidden.test"}))
	require.NoError(t, a.Check(context.Background(), 42, []string{"taobao.com"}),
		"default blocklist must be replaced, not augmented")
	require.ErrorIs(t, a.Check(context.Background(), 42, []string{"sub.forbidden.test"}), ErrAbuseBlocked)
}

func TestAbuse_CustomBurstThreshold(t *testing.T) {
	clock := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	a, pool := newAbuseDetectorWithMock(t,
		WithAbuseClock(func() time.Time { return clock }),
		WithAbuseBurst(10*time.Minute, 2),
	)
	rows := pgxmock.NewRows(abuseOrderRowColumns())
	rows.AddRow(abuseHistoryRow(1, []string{"a.com"}, clock.Add(-5*time.Minute))...)
	rows.AddRow(abuseHistoryRow(2, []string{"b.com"}, clock.Add(-5*time.Minute))...)
	pool.ExpectQuery(`SELECT .+ FROM cert\.orders`).
		WithArgs(int64(42), 500, 0).
		WillReturnRows(rows)
	err := a.Check(context.Background(), 42, []string{"c.com"})
	require.ErrorIs(t, err, ErrAbuseBlocked)
}

func TestDomainRoot_TwoLabelPSL(t *testing.T) {
	require.Equal(t, "moe.gov.cn", domainRoot("a.moe.gov.cn"))
	require.Equal(t, "example.co.uk", domainRoot("www.example.co.uk"))
	require.Equal(t, "example.com", domainRoot("a.b.example.com"))
	require.Equal(t, "example.com", domainRoot("*.example.com"))
	require.Equal(t, "", domainRoot(""))
	require.Equal(t, "single", domainRoot("single"))
}

func TestUniqueRoots(t *testing.T) {
	got := uniqueRoots([]string{"a.example.com", "b.example.com", "x.other.com"})
	require.Equal(t, []string{"example.com", "other.com"}, got)
}
