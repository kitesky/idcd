package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubOrdersCounter struct {
	count    int
	err      error
	calledCA string
	since    time.Time
}

func (s *stubOrdersCounter) CountByCASince(_ context.Context, ca string, since time.Time) (int, error) {
	s.calledCA = ca
	s.since = since
	return s.count, s.err
}

type stubCertsPeaker struct {
	peak     int
	err      error
	calledCA string
	since    time.Time
}

func (s *stubCertsPeaker) MaxCertsPerRegisteredDomainSince(_ context.Context, ca string, since time.Time) (int, error) {
	s.calledCA = ca
	s.since = since
	return s.peak, s.err
}

func fixedNow() time.Time { return time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC) }

func newQuotaChecker(orders OrdersCounter, certs CertsDomainPeaker, ceilings map[string]CAQuotaCeilings) *RepoQuotaChecker {
	q := NewRepoQuotaChecker(orders, certs, ceilings)
	q.now = fixedNow
	return q
}

func TestRepoQuotaChecker_DefaultCeilings_LE(t *testing.T) {
	o := &stubOrdersCounter{count: 60}    // 60/300 = 0.20
	c := &stubCertsPeaker{peak: 40}       // 40/50 = 0.80
	q := newQuotaChecker(o, c, nil)

	usage, err := q.Usage(context.Background(), "lets-encrypt")
	require.NoError(t, err)
	assert.InDelta(t, 0.20, usage.PerAccount3h, 0.001)
	assert.InDelta(t, 0.80, usage.PerRegisteredDomain, 0.001)

	assert.Equal(t, "lets-encrypt", o.calledCA)
	assert.Equal(t, fixedNow().Add(-3*time.Hour), o.since)
	assert.Equal(t, fixedNow().Add(-7*24*time.Hour), c.since)
}

func TestRepoQuotaChecker_UnknownCA_ReturnsZero(t *testing.T) {
	o := &stubOrdersCounter{count: 999}
	c := &stubCertsPeaker{peak: 999}
	q := newQuotaChecker(o, c, nil) // "zerossl" not in DefaultCeilings

	usage, err := q.Usage(context.Background(), "zerossl")
	require.NoError(t, err)
	assert.Equal(t, QuotaUsage{}, usage)
	assert.Empty(t, o.calledCA, "must not query repos for unknown CA")
}

func TestRepoQuotaChecker_CustomCeilings(t *testing.T) {
	o := &stubOrdersCounter{count: 50}
	c := &stubCertsPeaker{peak: 100}
	q := newQuotaChecker(o, c, map[string]CAQuotaCeilings{
		"acme-test": {NewOrdersPer3h: 100, CertsPerRegisteredDomainPerWeek: 200},
	})

	usage, err := q.Usage(context.Background(), "acme-test")
	require.NoError(t, err)
	assert.InDelta(t, 0.5, usage.PerAccount3h, 0.001)
	assert.InDelta(t, 0.5, usage.PerRegisteredDomain, 0.001)
}

func TestRepoQuotaChecker_ClampsRatioAboveOne(t *testing.T) {
	o := &stubOrdersCounter{count: 600} // > 300
	c := &stubCertsPeaker{peak: 75}     // > 50
	q := newQuotaChecker(o, c, nil)

	usage, err := q.Usage(context.Background(), "lets-encrypt")
	require.NoError(t, err)
	assert.Equal(t, 1.0, usage.PerAccount3h)
	assert.Equal(t, 1.0, usage.PerRegisteredDomain)
}

func TestRepoQuotaChecker_OrdersError_Surfaces(t *testing.T) {
	o := &stubOrdersCounter{err: errors.New("io")}
	c := &stubCertsPeaker{peak: 0}
	q := newQuotaChecker(o, c, nil)

	_, err := q.Usage(context.Background(), "lets-encrypt")
	require.Error(t, err)
}

func TestRepoQuotaChecker_CertsError_Surfaces(t *testing.T) {
	o := &stubOrdersCounter{count: 0}
	c := &stubCertsPeaker{err: errors.New("io")}
	q := newQuotaChecker(o, c, nil)

	_, err := q.Usage(context.Background(), "lets-encrypt")
	require.Error(t, err)
}

func TestRepoQuotaChecker_ZeroCeilingsSkipQuery(t *testing.T) {
	o := &stubOrdersCounter{count: 9999}
	c := &stubCertsPeaker{peak: 9999}
	q := newQuotaChecker(o, c, map[string]CAQuotaCeilings{
		"empty-ca": {NewOrdersPer3h: 0, CertsPerRegisteredDomainPerWeek: 0},
	})

	usage, err := q.Usage(context.Background(), "empty-ca")
	require.NoError(t, err)
	assert.Equal(t, QuotaUsage{}, usage)
	assert.Empty(t, o.calledCA)
}

func TestClampRatio(t *testing.T) {
	assert.Equal(t, 0.0, clampRatio(-0.5))
	assert.Equal(t, 0.0, clampRatio(0))
	assert.Equal(t, 0.5, clampRatio(0.5))
	assert.Equal(t, 1.0, clampRatio(1.0))
	assert.Equal(t, 1.0, clampRatio(99.0))
}
