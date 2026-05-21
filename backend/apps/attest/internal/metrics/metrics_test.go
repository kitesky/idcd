package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

// TestMetricsRegistered is the sanity check that promauto registered
// every collector against the default registry without panicking.
func TestMetricsRegistered(t *testing.T) {
	if KMSSignAttempts == nil {
		t.Fatal("KMSSignAttempts is nil — promauto registration failed")
	}
	if KMSSignDuration == nil {
		t.Fatal("KMSSignDuration is nil — promauto registration failed")
	}
	if KMSSignRetries == nil {
		t.Fatal("KMSSignRetries is nil — promauto registration failed")
	}
	if RefundRetryQueueLength == nil {
		t.Fatal("RefundRetryQueueLength is nil — promauto registration failed")
	}
	if VerdictRecords == nil {
		t.Fatal("VerdictRecords is nil — promauto registration failed")
	}
}

func TestRecordKMSSign_Outcomes(t *testing.T) {
	KMSSignAttempts.Reset()

	RecordKMSSign("success", 0.123)
	RecordKMSSign("kms_error", 0.5)
	RecordKMSSign("timeout", 30.0)
	RecordKMSSign("", 0) // empty outcome collapses to "unknown", zero duration skips histogram

	assert.Equal(t, float64(1),
		testutil.ToFloat64(KMSSignAttempts.WithLabelValues("success")))
	assert.Equal(t, float64(1),
		testutil.ToFloat64(KMSSignAttempts.WithLabelValues("kms_error")))
	assert.Equal(t, float64(1),
		testutil.ToFloat64(KMSSignAttempts.WithLabelValues("timeout")))
	assert.Equal(t, float64(1),
		testutil.ToFloat64(KMSSignAttempts.WithLabelValues("unknown")))
}

func TestRecordKMSSignRetry(t *testing.T) {
	before := testutil.ToFloat64(KMSSignRetries)
	RecordKMSSignRetry()
	RecordKMSSignRetry()
	after := testutil.ToFloat64(KMSSignRetries)
	assert.InDelta(t, before+2, after, 1e-9)
}

func TestSetRefundRetryQueueLength_ClampsNegative(t *testing.T) {
	SetRefundRetryQueueLength(17)
	assert.Equal(t, float64(17), testutil.ToFloat64(RefundRetryQueueLength))

	SetRefundRetryQueueLength(0)
	assert.Equal(t, float64(0), testutil.ToFloat64(RefundRetryQueueLength))

	// Negative is clamped to 0 so a bug in the caller doesn't poison the gauge.
	SetRefundRetryQueueLength(-5)
	assert.Equal(t, float64(0), testutil.ToFloat64(RefundRetryQueueLength))
}

func TestRecordVerdict(t *testing.T) {
	VerdictRecords.Reset()

	RecordVerdict("committed")
	RecordVerdict("committed")
	RecordVerdict("rejected")
	RecordVerdict("something-else") // → "unknown"

	assert.Equal(t, float64(2),
		testutil.ToFloat64(VerdictRecords.WithLabelValues("committed")))
	assert.Equal(t, float64(1),
		testutil.ToFloat64(VerdictRecords.WithLabelValues("rejected")))
	assert.Equal(t, float64(1),
		testutil.ToFloat64(VerdictRecords.WithLabelValues("unknown")))
}

func TestNormalize(t *testing.T) {
	assert.Equal(t, "unknown", normalize(""))
	assert.Equal(t, "success", normalize("success"))
}
