package hub

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

// TestNewBusinessMetricsRegistered (P1-11 Phase 1) verifies that the new
// idcd_gateway_* collectors are non-nil and accept the documented labels.
func TestNewBusinessMetricsRegistered(t *testing.T) {
	if MetricsAgentReconnects == nil {
		t.Fatal("MetricsAgentReconnects is nil — promauto registration failed")
	}
	if MetricsAgentConnections == nil {
		t.Fatal("MetricsAgentConnections is nil — promauto registration failed")
	}
	if MetricsWSMessagesReceived == nil {
		t.Fatal("MetricsWSMessagesReceived is nil — promauto registration failed")
	}

	for _, outcome := range []string{"accepted", "rejected_enroll", "rejected_auth"} {
		MetricsAgentConnections.WithLabelValues(outcome).Inc()
	}
	for _, t := range []string{"task_ack", "result", "heartbeat", "error", "unknown"} {
		MetricsWSMessagesReceived.WithLabelValues(t).Inc()
	}

	before := testutil.ToFloat64(MetricsAgentReconnects)
	MetricsAgentReconnects.Inc()
	MetricsAgentReconnects.Inc()
	after := testutil.ToFloat64(MetricsAgentReconnects)
	assert.InDelta(t, before+2, after, 1e-9)
}
