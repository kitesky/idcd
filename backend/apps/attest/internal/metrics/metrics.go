// Package metrics defines idcd-attest business Prometheus metrics
// (P1-11 Phase 1).
//
// Naming convention: idcd_attest_<subsystem>_<noun>_<unit>. The attest
// service has no /metrics endpoint of its own in S2 (verify-only API +
// background workers); cmd/server adds one when this package is imported,
// alongside the existing /healthz route, so promauto-registered metrics
// here become scrape-able without any extra wiring on the worker side.
//
// Cardinality discipline:
//
//   - outcome / kind — bounded enum, safe.
//   - never label by report_id, order_id, user_id, email, idempotency_key.
//
// What this package surfaces:
//
//   - KMS sign attempts/duration/retries (D11 12h Shamir SOP — alert when
//     the retry counter starts ramping).
//   - Refund retry-queue length gauge (D5 — when this gauge stays > 0 for
//     more than the second-retry window, the queue is wedged).
//   - Verdict record outcomes (committed / rejected) — the D4 WAL story
//     becomes observable: committed is the success counter; rejected is
//     anything self-verify caught.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// KMSSignAttempts counts each call to cfg.Signer.Sign in the orchestrator,
// partitioned by outcome.
//
//	outcome — "success" | "kms_error" | "timeout"
var KMSSignAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "idcd_attest",
	Subsystem: "kms",
	Name:      "sign_attempts_total",
	Help:      "KMS sign 调用次数 (按 outcome 分类, D11 SOP 监控)",
}, []string{"outcome"})

// KMSSignDuration observes wall-clock latency of each successful KMS sign.
// Failures still observe — they help separate "KMS is fast but failing"
// from "KMS is slow and timing out".
var KMSSignDuration = promauto.NewHistogram(prometheus.HistogramOpts{
	Namespace: "idcd_attest",
	Subsystem: "kms",
	Name:      "sign_duration_seconds",
	Help:      "KMS sign 调用延迟分布 (含失败)",
	// 50ms .. 30s — KMS is normally <200ms; tail above 5s suggests rate-
	// limit or HSM stress; 30s aligns with cfg.SignTimeout default.
	Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30},
})

// KMSSignRetries counts the number of orchestrator retries triggered by
// a failed Sign. This is the D11 single-point indicator: when the rate
// of this counter rises, the Shamir-recovery clock is ticking.
var KMSSignRetries = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: "idcd_attest",
	Subsystem: "kms",
	Name:      "sign_retries_total",
	Help:      "KMS sign 重试次数 (D11 SOP 触发指示)",
})

// RefundRetryQueueLength is a gauge of the current delay-zone backlog
// (refund_delay_zone ZSET cardinality). The refund worker tick samples
// this after each scan so an empty queue resets the gauge promptly.
var RefundRetryQueueLength = promauto.NewGauge(prometheus.GaugeOpts{
	Namespace: "idcd_attest",
	Subsystem: "refund",
	Name:      "retry_queue_length",
	Help:      "退款重试队列当前深度 (D5 监控, refund_delay_zone ZSET size)",
})

// VerdictRecords counts terminal verdict outcomes.
//
//	outcome — "committed" — verdict pipeline reached SetDelivered + WORM
//	          "rejected"  — self-verify or pipeline failure (driveFailed)
var VerdictRecords = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "idcd_attest",
	Subsystem: "verdict",
	Name:      "records_total",
	Help:      "Verdict 终态计数 (按 outcome 分类)",
}, []string{"outcome"})

// ----------------------------------------------------------------------
// Helpers — single hook per call site so handler code stays one-liner.
// ----------------------------------------------------------------------

// RecordKMSSign is called by the orchestrator's runSignStep after every
// Signer.Sign return. duration may be zero (e.g. on a pre-Sign timeout
// where we never got to the call) — we skip the histogram in that case
// so the buckets don't fill up with misleading 0s samples.
func RecordKMSSign(outcome string, durationSeconds float64) {
	KMSSignAttempts.WithLabelValues(normalize(outcome)).Inc()
	if durationSeconds > 0 {
		KMSSignDuration.Observe(durationSeconds)
	}
}

// RecordKMSSignRetry is a thin alias for KMSSignRetries.Inc() so callers
// match the Record* hook pattern used elsewhere in the package.
func RecordKMSSignRetry() { KMSSignRetries.Inc() }

// SetRefundRetryQueueLength is the gauge setter the refund worker tick
// calls after each ZRangeByScoreWithScores pass — including when the
// scan returns zero members so the gauge resets.
func SetRefundRetryQueueLength(n int) {
	if n < 0 {
		n = 0
	}
	RefundRetryQueueLength.Set(float64(n))
}

// RecordVerdict bumps the verdict-records counter. outcome must be one
// of the documented enum values; unknown values collapse to "unknown"
// so dashboards stay legible.
func RecordVerdict(outcome string) {
	switch outcome {
	case "committed", "rejected":
		VerdictRecords.WithLabelValues(outcome).Inc()
	default:
		VerdictRecords.WithLabelValues("unknown").Inc()
	}
}

// normalize collapses empty values to "unknown" so Prometheus does not
// see an empty label, which is legal but confusing in dashboards.
func normalize(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}
