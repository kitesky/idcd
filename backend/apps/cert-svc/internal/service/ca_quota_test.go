package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSwitchThresholdValue(t *testing.T) {
	// Pinned per PRD §3.2 — changes here must be paired with a
	// docs/PRD update + ops review.
	assert.Equal(t, 0.70, SwitchThreshold)
}

func TestNopQuotaChecker_AlwaysZero(t *testing.T) {
	var qc QuotaChecker = nopQuotaChecker{}

	u, err := qc.Usage(context.Background(), "lets-encrypt")
	require.NoError(t, err)
	assert.Equal(t, 0.0, u.PerRegisteredDomain)
	assert.Equal(t, 0.0, u.PerAccount3h)

	// Behavior is independent of the CA name.
	u, err = qc.Usage(context.Background(), "zerossl")
	require.NoError(t, err)
	assert.Equal(t, QuotaUsage{}, u)
}

// fakeQuota is a test double for QuotaChecker. usage is returned
// verbatim; if err is non-nil it short-circuits the call.
type fakeQuota struct {
	usage QuotaUsage
	err   error
	calls int
	last  string
}

func (f *fakeQuota) Usage(_ context.Context, name string) (QuotaUsage, error) {
	f.calls++
	f.last = name
	if f.err != nil {
		return QuotaUsage{}, f.err
	}
	return f.usage, nil
}

func TestFakeQuota_RecordsCallsAndReturnsConfigured(t *testing.T) {
	q := &fakeQuota{usage: QuotaUsage{PerRegisteredDomain: 0.5, PerAccount3h: 0.6}}

	u, err := q.Usage(context.Background(), "lets-encrypt")
	require.NoError(t, err)
	assert.Equal(t, 0.5, u.PerRegisteredDomain)
	assert.Equal(t, 0.6, u.PerAccount3h)
	assert.Equal(t, 1, q.calls)
	assert.Equal(t, "lets-encrypt", q.last)

	q.err = errors.New("boom")
	_, err = q.Usage(context.Background(), "zerossl")
	require.Error(t, err)
	assert.Equal(t, 2, q.calls)
	assert.Equal(t, "zerossl", q.last)
}
