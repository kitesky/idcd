// Package metrics defines cert-svc Prometheus business metrics.
//
// All metrics are registered against the default Prometheus registry via
// promauto, so the standard /metrics endpoint exposes them alongside Go
// runtime metrics. cert-svc's main / worker / renewer / orchestrator call
// the helpers below to record state transitions.
//
// Naming follows the S2 W8 spec from docs/prd/20-free-cert.md §13:
//   - cert_orders_total{status, ca, tier}
//   - cert_order_duration_seconds{ca, tier}
//   - cert_queue_depth{queue}
//   - cert_ca_quota_used{ca}
//   - cert_acme_errors_total{ca, error_type}
//   - cert_renewal_jobs_total{status}
//
// Labels are kept low-cardinality on purpose; tier / ca are bounded enums.
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// OrdersTotal counts cert order outcomes.
//
//	status — "issued" | "failed" | "revoked"
//	ca     — "lets-encrypt" | "zerossl" | "buypass" | ...
//	tier   — "free-dv" | "paid-dv" | ...
var OrdersTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "cert_orders_total",
	Help: "Total cert orders by terminal status, partitioned by CA + tier.",
}, []string{"status", "ca", "tier"})

// OrderDurationSeconds observes end-to-end issuance latency.
// Buckets are wider than DefBuckets because ACME flows include DNS
// propagation waits — most successful issues land between 30s and 5min.
var OrderDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "cert_order_duration_seconds",
	Help:    "End-to-end wall-clock duration of cert order processing (draft → issued/failed).",
	Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1200, 1800, 3600},
}, []string{"ca", "tier"})

// QueueDepth is the current depth of the Redis stream queues cert-svc
// consumes / produces. The "queue" label carries the logical queue name
// (e.g. "cert:order_events", "cert:notifications").
var QueueDepth = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "cert_queue_depth",
	Help: "Current depth of cert-svc Redis streams.",
}, []string{"queue"})

// CAQuotaUsed is the most recent CA quota usage ratio (0..1) as reported
// by the RepoQuotaChecker. Updated by the periodic collector goroutine.
var CAQuotaUsed = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "cert_ca_quota_used",
	Help: "CA weekly / 3h quota usage ratio (0..1). Updated every 30s by the collector.",
}, []string{"ca"})

// ACMEErrorsTotal counts ACME-side failures grouped by upstream type.
//
//	ca         — "lets-encrypt" | "zerossl" | ...
//	error_type — "rate_limited" | "dns_propagation" | "invalid_csr" |
//	             "challenge_failed" | "ca_unreachable" | "other"
var ACMEErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "cert_acme_errors_total",
	Help: "ACME error count, partitioned by CA + classification.",
}, []string{"ca", "error_type"})

// CollectorScrapeFailures counts collector-side errors per scrape kind,
// so an operator can spot stale gauges via the rate of this counter even
// while QueueDepth / CAQuotaUsed retain their last good value.
//
//	kind — "queue_depth" | "ca_quota"
var CollectorScrapeFailures = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "cert_collector_scrape_failures_total",
	Help: "Collector scrape failures by source kind.",
}, []string{"kind"})

// RenewalJobsTotal counts renewal-job state transitions emitted by the
// renewal scheduler / worker.
//
//	status — "scheduled" | "running" | "succeeded" | "failed" | "aborted"
var RenewalJobsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "cert_renewal_jobs_total",
	Help: "Renewal job outcomes by terminal status.",
}, []string{"status"})

// ----------------------------------------------------------------------
// P1-11 Phase 1: idcd-namespaced order/issuance metrics.
//
// These augment the legacy cert_* metrics above (which are kept for the
// existing Grafana dashboards) with handler-level counters that surface
// orders that never even reach the orchestrator — bad CAA, abuse blocks,
// daily-quota exhaustion — so we can answer "how many issuance attempts
// did we reject before they entered the pipeline?" without joining over
// HTTP request logs.
// ----------------------------------------------------------------------

// IssueAttempts is the handler-level counter incremented on every
// POST /v1/orders, partitioned by outcome.
//
//	outcome — "accepted"           order row inserted, enqueued
//	          "idempotent_replay"  ON CONFLICT path returned existing
//	          "invalid_input"      SAN normalisation / size / challenge
//	          "quota_exceeded"     dailyOrderQuota cap reached
//	          "abuse_blocked"      service.ErrAccountBanned / ErrAbuseBlocked
//	          "caa_forbid"         CAA records reject the target CA
//	          "credential_invalid" dns credential lookup / ownership / revoked
//	          "internal"           db / repo failures
//	ca      — "lets-encrypt" | "zerossl" | "buypass" | "unknown"
var IssueAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "idcd_cert",
	Subsystem: "issue",
	Name:      "attempts_total",
	Help:      "证书签发尝试次数 (按 outcome + ca 分类, handler 层)",
}, []string{"outcome", "ca"})

// LERateLimitHits counts how often we observed Let's Encrypt rate-limit
// rejection. This is a narrow alias over ACMEErrorsTotal{ca="lets-encrypt",
// error_type="rate_limited"} but cheap to maintain and removes one
// PromQL multiplication step in alert rules.
var LERateLimitHits = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: "idcd_cert",
	Subsystem: "le",
	Name:      "rate_limit_hits_total",
	Help:      "Let's Encrypt rate-limit hits (alias for cert_acme_errors_total{ca=lets-encrypt,error_type=rate_limited}).",
})

// DNSChallengeFailures counts DNS-01 challenge failures by classified
// reason. Complements cert_acme_errors_total by exposing the granular
// reason set we already maintain in caa_check / dns code.
//
//	reason — "txt_propagation"   the _acme-challenge TXT was missing or stale
//	         "authorization_invalid" ACME returned authorization invalid
//	         "credential_error"  upstream provider API rejected our creds
//	         "unknown"           caller passed an unmapped error string
var DNSChallengeFailures = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "idcd_cert",
	Subsystem: "dns",
	Name:      "challenge_failures_total",
	Help:      "DNS-01 challenge failures (按 reason 分类)",
}, []string{"reason"})

// RecordIssueAttempt is the single hook each createOrder branch calls.
// We keep label normalisation in one place so unknown CAs collapse to
// "unknown" rather than blowing up the cardinality.
func RecordIssueAttempt(outcome, ca string) {
	IssueAttempts.WithLabelValues(normalizeLabel(outcome), normalizeLabel(ca)).Inc()
}

// RecordDNSChallengeFailure normalises the reason set so callers can pass
// arbitrary strings without polluting Prometheus with unbounded labels.
func RecordDNSChallengeFailure(reason string) {
	DNSChallengeFailures.WithLabelValues(classifyDNSChallengeFailure(reason)).Inc()
}

// classifyDNSChallengeFailure pins the reason vocabulary. Add a new
// branch here when you introduce a new failure class — the linter
// won't catch silent misspellings.
func classifyDNSChallengeFailure(s string) string {
	switch s {
	case "txt_propagation", "authorization_invalid", "credential_error":
		return s
	default:
		return "unknown"
	}
}

// RecordOrderResult is the single hook called by the orchestrator when an
// order reaches a terminal state. It bumps OrdersTotal and observes the
// duration histogram in one place so we cannot accidentally drift the
// labels between the two metrics.
//
// duration may be zero — pass time.Duration(0) when the start time is
// unknown (e.g. WAL replay where we only saw the final transition); we
// then skip the histogram observation so we do not pollute the buckets
// with a misleading 0s sample.
func RecordOrderResult(status, ca, tier string, duration time.Duration) {
	status = normalizeLabel(status)
	ca = normalizeLabel(ca)
	tier = normalizeLabel(tier)
	OrdersTotal.WithLabelValues(status, ca, tier).Inc()
	if duration > 0 {
		OrderDurationSeconds.WithLabelValues(ca, tier).Observe(duration.Seconds())
	}
}

// RecordACMEError increments ACMEErrorsTotal. errType is normalised so
// any unexpected value collapses to "other" — we never want unbounded
// label cardinality from raw upstream error strings.
func RecordACMEError(ca, errType string) {
	ACMEErrorsTotal.WithLabelValues(normalizeLabel(ca), classifyACMEError(errType)).Inc()
}

// RecordRenewalJob bumps RenewalJobsTotal for the given terminal status.
func RecordRenewalJob(status string) {
	RenewalJobsTotal.WithLabelValues(normalizeLabel(status)).Inc()
}

// SetQueueDepth updates the queue-depth gauge for one logical queue.
// Called by the collector goroutine after a successful XLEN.
func SetQueueDepth(queue string, depth int64) {
	QueueDepth.WithLabelValues(normalizeLabel(queue)).Set(float64(depth))
}

// SetCAQuotaUsed clamps the ratio to [0, 1] and updates the per-CA gauge.
func SetCAQuotaUsed(ca string, ratio float64) {
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	CAQuotaUsed.WithLabelValues(normalizeLabel(ca)).Set(ratio)
}

// normalizeLabel collapses empty values into "unknown" so Prometheus does
// not get an empty label, which is legal but confusing in dashboards.
func normalizeLabel(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

// classifyACMEError maps any non-canonical error string to "other" so
// upstream churn cannot explode label cardinality. The canonical set is
// pinned here — callers must extend this list when adding a new bucket.
func classifyACMEError(s string) string {
	switch s {
	case "rate_limited", "dns_propagation", "invalid_csr",
		"challenge_failed", "ca_unreachable", "account_key_invalid",
		"timeout":
		return s
	default:
		return "other"
	}
}
