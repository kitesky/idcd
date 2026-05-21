package scheduler

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

// TestNewBusinessMetricsRegistered (P1-11 Phase 1) verifies that the new
// idcd_scheduler_* collectors are non-nil and accept the documented label
// vocabulary without panicking.
func TestNewBusinessMetricsRegistered(t *testing.T) {
	if MetricsDispatchLag == nil {
		t.Fatal("MetricsDispatchLag is nil — promauto registration failed")
	}
	if MetricsDispatchedTasks == nil {
		t.Fatal("MetricsDispatchedTasks is nil — promauto registration failed")
	}
	if MetricsEpochCurrent == nil {
		t.Fatal("MetricsEpochCurrent is nil — promauto registration failed")
	}

	// Walk every documented (probe_type, status) pair to catch typos.
	for _, pt := range []string{"http", "ping", "tcp", "dns"} {
		for _, st := range []string{"ok", "select_node_fail", "stream_push_fail"} {
			MetricsDispatchedTasks.WithLabelValues(pt, st).Inc()
		}
	}
	MetricsDispatchLag.Observe(0.5)
	MetricsDispatchLag.Observe(5.0)

	MetricsEpochCurrent.Reset()
	MetricsEpochCurrent.WithLabelValues("sched-1").Set(42)
	assert.Equal(t, float64(42),
		testutil.ToFloat64(MetricsEpochCurrent.WithLabelValues("sched-1")))

	// Unknown-node fallback path used by New(cfg) when NodeID is empty.
	MetricsEpochCurrent.WithLabelValues("unknown").Set(0)
	assert.Equal(t, float64(0),
		testutil.ToFloat64(MetricsEpochCurrent.WithLabelValues("unknown")))
}
