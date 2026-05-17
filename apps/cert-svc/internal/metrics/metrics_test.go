package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordOrderResult_HappyPath(t *testing.T) {
	OrdersTotal.Reset()
	OrderDurationSeconds.Reset()

	RecordOrderResult("issued", "lets-encrypt", "free-dv", 42*time.Second)
	RecordOrderResult("issued", "lets-encrypt", "free-dv", 60*time.Second)
	RecordOrderResult("failed", "zerossl", "free-dv", 0)

	assert.Equal(t, float64(2),
		testutil.ToFloat64(OrdersTotal.WithLabelValues("issued", "lets-encrypt", "free-dv")))
	assert.Equal(t, float64(1),
		testutil.ToFloat64(OrdersTotal.WithLabelValues("failed", "zerossl", "free-dv")))

	// One label combo observed (zerossl path was duration=0 → skipped).
	require.Equal(t, 1, testutil.CollectAndCount(OrderDurationSeconds))
}

func TestRecordOrderResult_ZeroDurationSkipsHistogram(t *testing.T) {
	OrdersTotal.Reset()
	OrderDurationSeconds.Reset()

	RecordOrderResult("revoked", "lets-encrypt", "free-dv", 0)

	assert.Equal(t, float64(1),
		testutil.ToFloat64(OrdersTotal.WithLabelValues("revoked", "lets-encrypt", "free-dv")))
	require.Equal(t, 0, testutil.CollectAndCount(OrderDurationSeconds))
}

func TestRecordOrderResult_NormalisesEmptyLabels(t *testing.T) {
	OrdersTotal.Reset()
	OrderDurationSeconds.Reset()

	RecordOrderResult("", "", "", 10*time.Second)

	assert.Equal(t, float64(1),
		testutil.ToFloat64(OrdersTotal.WithLabelValues("unknown", "unknown", "unknown")))
}

func TestRecordACMEError_KnownAndUnknown(t *testing.T) {
	ACMEErrorsTotal.Reset()

	RecordACMEError("lets-encrypt", "rate_limited")
	RecordACMEError("lets-encrypt", "dns_propagation")
	RecordACMEError("lets-encrypt", "wat-is-this") // → "other"
	RecordACMEError("zerossl", "challenge_failed")

	assert.Equal(t, float64(1),
		testutil.ToFloat64(ACMEErrorsTotal.WithLabelValues("lets-encrypt", "rate_limited")))
	assert.Equal(t, float64(1),
		testutil.ToFloat64(ACMEErrorsTotal.WithLabelValues("lets-encrypt", "dns_propagation")))
	assert.Equal(t, float64(1),
		testutil.ToFloat64(ACMEErrorsTotal.WithLabelValues("lets-encrypt", "other")))
	assert.Equal(t, float64(1),
		testutil.ToFloat64(ACMEErrorsTotal.WithLabelValues("zerossl", "challenge_failed")))
}

func TestRecordRenewalJob(t *testing.T) {
	RenewalJobsTotal.Reset()

	RecordRenewalJob("succeeded")
	RecordRenewalJob("succeeded")
	RecordRenewalJob("failed")
	RecordRenewalJob("aborted")
	RecordRenewalJob("") // → "unknown"

	assert.Equal(t, float64(2),
		testutil.ToFloat64(RenewalJobsTotal.WithLabelValues("succeeded")))
	assert.Equal(t, float64(1),
		testutil.ToFloat64(RenewalJobsTotal.WithLabelValues("failed")))
	assert.Equal(t, float64(1),
		testutil.ToFloat64(RenewalJobsTotal.WithLabelValues("aborted")))
	assert.Equal(t, float64(1),
		testutil.ToFloat64(RenewalJobsTotal.WithLabelValues("unknown")))
}

func TestSetQueueDepth(t *testing.T) {
	QueueDepth.Reset()
	SetQueueDepth("cert:order_events", 17)
	assert.Equal(t, float64(17),
		testutil.ToFloat64(QueueDepth.WithLabelValues("cert:order_events")))
	SetQueueDepth("cert:order_events", 3) // gauge replaces, not adds
	assert.Equal(t, float64(3),
		testutil.ToFloat64(QueueDepth.WithLabelValues("cert:order_events")))
}

func TestSetCAQuotaUsed_ClampsRange(t *testing.T) {
	CAQuotaUsed.Reset()

	SetCAQuotaUsed("lets-encrypt", 0.75)
	assert.InDelta(t, 0.75, testutil.ToFloat64(CAQuotaUsed.WithLabelValues("lets-encrypt")), 1e-9)

	SetCAQuotaUsed("zerossl", -0.5) // clamp ↑ 0
	assert.Equal(t, float64(0), testutil.ToFloat64(CAQuotaUsed.WithLabelValues("zerossl")))

	SetCAQuotaUsed("buypass", 1.5) // clamp ↓ 1
	assert.Equal(t, float64(1), testutil.ToFloat64(CAQuotaUsed.WithLabelValues("buypass")))
}

func TestNormalizeLabel(t *testing.T) {
	assert.Equal(t, "unknown", normalizeLabel(""))
	assert.Equal(t, "issued", normalizeLabel("issued"))
}

func TestClassifyACMEError(t *testing.T) {
	for _, known := range []string{
		"rate_limited", "dns_propagation", "invalid_csr",
		"challenge_failed", "ca_unreachable", "account_key_invalid", "timeout",
	} {
		assert.Equal(t, known, classifyACMEError(known))
	}
	assert.Equal(t, "other", classifyACMEError("anything else"))
	assert.Equal(t, "other", classifyACMEError(""))
}
