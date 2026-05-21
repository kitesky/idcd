package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestMetricsRegistered(t *testing.T) {
	if SSEConnections == nil {
		t.Fatal("SSEConnections is nil — promauto registration failed")
	}
	if ToolInvocations == nil {
		t.Fatal("ToolInvocations is nil — promauto registration failed")
	}
	if ToolDuration == nil {
		t.Fatal("ToolDuration is nil — promauto registration failed")
	}
}

func TestSSEConnectionsGauge(t *testing.T) {
	// Reset to zero so the test result is deterministic when the suite
	// runs after another test that bumped the gauge.
	SSEConnections.Set(0)
	SSEConnections.Inc()
	SSEConnections.Inc()
	assert.Equal(t, float64(2), testutil.ToFloat64(SSEConnections))
	SSEConnections.Dec()
	assert.Equal(t, float64(1), testutil.ToFloat64(SSEConnections))
	SSEConnections.Set(0)
}

func TestRecordToolInvocation(t *testing.T) {
	ToolInvocations.Reset()

	RecordToolInvocation("monitors.list", "ok", 0.123)
	RecordToolInvocation("monitors.list", "ok", 0.456)
	RecordToolInvocation("monitors.create", "tool_failure", 0.789)
	RecordToolInvocation("", "internal_error", 0)        // both fall back, duration=0 skips histogram
	RecordToolInvocation("monitors.list", "", 0.1)       // outcome falls back

	assert.Equal(t, float64(2),
		testutil.ToFloat64(ToolInvocations.WithLabelValues("monitors.list", "ok")))
	assert.Equal(t, float64(1),
		testutil.ToFloat64(ToolInvocations.WithLabelValues("monitors.create", "tool_failure")))
	assert.Equal(t, float64(1),
		testutil.ToFloat64(ToolInvocations.WithLabelValues("unknown", "internal_error")))
	assert.Equal(t, float64(1),
		testutil.ToFloat64(ToolInvocations.WithLabelValues("monitors.list", "unknown")))
}
