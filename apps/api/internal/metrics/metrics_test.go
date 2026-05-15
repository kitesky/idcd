package metrics_test

import (
	"testing"

	"github.com/kite365/idcd/apps/api/internal/metrics"
)

// TestMetricsRegistered verifies that all custom business metrics are non-nil
// (i.e. promauto registered them successfully without panicking).
func TestMetricsRegistered(t *testing.T) {
	if metrics.ActiveMonitors == nil {
		t.Fatal("ActiveMonitors is nil — promauto registration failed")
	}
	if metrics.TotalUsers == nil {
		t.Fatal("TotalUsers is nil — promauto registration failed")
	}
	if metrics.AlertEventsFiring == nil {
		t.Fatal("AlertEventsFiring is nil — promauto registration failed")
	}
	if metrics.HTTPRequests == nil {
		t.Fatal("HTTPRequests is nil — promauto registration failed")
	}
}

// TestMetricsOperations verifies that Gauge and CounterVec operations do not panic.
func TestMetricsOperations(t *testing.T) {
	// Gauge operations
	metrics.ActiveMonitors.Set(42)
	metrics.ActiveMonitors.Inc()
	metrics.ActiveMonitors.Dec()

	metrics.TotalUsers.Set(1000)
	metrics.AlertEventsFiring.Set(3)

	// CounterVec — label cardinality must match definition (method, status).
	metrics.HTTPRequests.WithLabelValues("GET", "200").Inc()
	metrics.HTTPRequests.WithLabelValues("POST", "201").Inc()
	metrics.HTTPRequests.WithLabelValues("DELETE", "404").Inc()
}
