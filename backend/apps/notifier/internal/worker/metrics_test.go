package worker

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

// TestLegacyMetricsRegistered ensures the existing notifier_* collectors
// still register after the P1-11 additions.
func TestLegacyMetricsRegistered(t *testing.T) {
	if MetricsEmailsSent == nil {
		t.Fatal("MetricsEmailsSent is nil — promauto registration failed")
	}
	if MetricsWebhookCalls == nil {
		t.Fatal("MetricsWebhookCalls is nil — promauto registration failed")
	}
	if MetricsRefundRetries == nil {
		t.Fatal("MetricsRefundRetries is nil — promauto registration failed")
	}
	if MetricsSendDuration == nil {
		t.Fatal("MetricsSendDuration is nil — promauto registration failed")
	}
}

// TestNewBusinessMetricsRegistered (P1-11 Phase 1) exercises the new
// idcd_notifier_* counters with their documented label vocabulary.
func TestNewBusinessMetricsRegistered(t *testing.T) {
	if EmailSent == nil {
		t.Fatal("EmailSent is nil — promauto registration failed")
	}
	if EmailRetries == nil {
		t.Fatal("EmailRetries is nil — promauto registration failed")
	}

	for _, outcome := range []string{"ok", "fail"} {
		for _, provider := range []string{"ses", "smtp", "unknown"} {
			for _, tpl := range []string{
				"verify_email", "reset_password", "welcome",
				"refund_apology", "billing", "alert", "unknown",
			} {
				EmailSent.WithLabelValues(outcome, provider, tpl).Inc()
			}
		}
	}
	for _, reason := range []string{
		"transient_smtp", "ses_throttled", "rate_limited", "unknown",
	} {
		EmailRetries.WithLabelValues(reason).Inc()
	}
}

func TestTemplateFromSubject(t *testing.T) {
	cases := map[string]string{
		"Please verify your email":   "verify_email",
		"验证您的邮箱":                     "verify_email",
		"Reset your password":        "reset_password",
		"密码重置":                       "reset_password",
		"Welcome to idcd":            "welcome",
		"欢迎加入":                       "welcome",
		"Refund failed":              "refund_apology",
		"退款失败":                       "refund_apology",
		"Your monthly billing":       "billing",
		"alert: monitor down":        "alert",
		"账单":                         "billing",
		"告警: 服务异常":                   "alert",
		"":                           "unknown",
		"some other random subject":  "unknown",
	}
	for subject, want := range cases {
		got := templateFromSubject(subject)
		assert.Equalf(t, want, got, "subject=%q", subject)
	}
}

// TestEmailSentIncrement is a smoke test confirming that EmailSent.Inc()
// is observable via testutil.ToFloat64 — guards against a future
// refactor that accidentally drops the WithLabelValues call.
func TestEmailSentIncrement(t *testing.T) {
	before := testutil.ToFloat64(EmailSent.WithLabelValues("ok", "ses", "verify_email"))
	EmailSent.WithLabelValues("ok", "ses", "verify_email").Inc()
	after := testutil.ToFloat64(EmailSent.WithLabelValues("ok", "ses", "verify_email"))
	assert.InDelta(t, before+1, after, 1e-9)
}
