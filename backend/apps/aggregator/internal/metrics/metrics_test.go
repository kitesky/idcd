package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

// TestLegacyMetricsRegistered is the sanity check that the existing
// aggregator_* collectors stay non-nil after the P1-11 additions.
func TestLegacyMetricsRegistered(t *testing.T) {
	if MessagesProcessed == nil {
		t.Fatal("MessagesProcessed is nil — promauto registration failed")
	}
	if MessagesFailed == nil {
		t.Fatal("MessagesFailed is nil — promauto registration failed")
	}
	if DLQMessages == nil {
		t.Fatal("DLQMessages is nil — promauto registration failed")
	}
	if ReclaimedMessages == nil {
		t.Fatal("ReclaimedMessages is nil — promauto registration failed")
	}
	if PELSize == nil {
		t.Fatal("PELSize is nil — promauto registration failed")
	}
}

// TestNewBusinessMetricsRegistered (P1-11 Phase 1) covers the new
// idcd_aggregator_* collectors with their documented label vocabulary.
func TestNewBusinessMetricsRegistered(t *testing.T) {
	if StreamConsumerLag == nil {
		t.Fatal("StreamConsumerLag is nil — promauto registration failed")
	}
	if ProcessedResults == nil {
		t.Fatal("ProcessedResults is nil — promauto registration failed")
	}
	if ProcessingDuration == nil {
		t.Fatal("ProcessingDuration is nil — promauto registration failed")
	}

	for _, outcome := range []string{"ok", "validation_failed", "downstream_error"} {
		for _, pt := range []string{"http", "ping", "tcp", "dns", "unknown"} {
			ProcessedResults.WithLabelValues(outcome, pt).Inc()
		}
	}
	for _, pt := range []string{"http", "ping", "tcp", "dns", "unknown"} {
		ProcessingDuration.WithLabelValues(pt).Observe(0.123)
	}

	StreamConsumerLag.Reset()
	StreamConsumerLag.WithLabelValues("probe.results").Set(0.5)
	assert.Equal(t, float64(0.5),
		testutil.ToFloat64(StreamConsumerLag.WithLabelValues("probe.results")))
}
