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

// TestBusinessMetricsRegistered (P1-11 Phase 1) verifies that the new auth /
// quota counters and gauges are non-nil and accept the documented label sets
// without panicking. We do not assert on specific values — promauto's
// AlreadyRegisteredError handling means each test invocation reuses the
// existing collector, so concrete counts would race across the suite.
func TestBusinessMetricsRegistered(t *testing.T) {
	if metrics.RegistrationAttempts == nil {
		t.Fatal("RegistrationAttempts is nil — promauto registration failed")
	}
	if metrics.LoginAttempts == nil {
		t.Fatal("LoginAttempts is nil — promauto registration failed")
	}
	if metrics.OTPVerifyAttempts == nil {
		t.Fatal("OTPVerifyAttempts is nil — promauto registration failed")
	}
	if metrics.TokensIssued == nil {
		t.Fatal("TokensIssued is nil — promauto registration failed")
	}
	if metrics.QuotaUsageRatio == nil {
		t.Fatal("QuotaUsageRatio is nil — promauto registration failed")
	}

	// Exercise the documented outcome / kind / type vocabularies. A typo here
	// would not error today (CounterVec accepts any string), but if we ever
	// switch to bounded labels via WithLabelValues + ConstLabels these calls
	// would catch a divergence with the docstrings above.
	for _, outcome := range []string{
		"success", "duplicate_email", "invalid_input", "weak_password", "internal",
	} {
		metrics.RegistrationAttempts.WithLabelValues(outcome).Inc()
	}
	for _, outcome := range []string{
		"success", "invalid_credentials", "account_disabled", "mfa_required",
		"invalid_input", "internal",
	} {
		metrics.LoginAttempts.WithLabelValues(outcome).Inc()
	}
	for _, outcome := range []string{
		"success", "invalid", "expired", "attempts_exceeded", "internal",
	} {
		metrics.OTPVerifyAttempts.WithLabelValues(outcome, "email_verify").Inc()
	}
	for _, kind := range []string{
		"session", "pat", "api_key",
		"mcp_personal", "mcp_workspace", "mcp_service", "two_factor_session",
	} {
		metrics.TokensIssued.WithLabelValues(kind).Inc()
	}
	metrics.QuotaUsageRatio.WithLabelValues("ws_test", "free").Set(0.5)
	metrics.QuotaUsageRatio.WithLabelValues("ws_test", "pro").Set(0.8)
}
