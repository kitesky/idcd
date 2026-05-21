// Package worker — Prometheus metrics for the notifier service (P1#19).
//
// All metrics are registered against the default Prometheus registry via
// promauto so the standard /metrics endpoint exposes them alongside Go
// runtime metrics. The handler call sites in handlers.go increment these
// counters / observe these histograms after each delivery attempt.
//
// Naming follows the docs/REVIEW-FINDINGS-2026-05-16.md spec:
//   - notifier_emails_sent_total{provider, status}
//   - notifier_webhook_calls_total{channel, status}
//   - notifier_refund_retries_total{outcome}
//   - notifier_send_duration_seconds{channel}
package worker

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MetricsEmailsSent counts email send attempts.
//
//	provider — "ses" | "smtp"
//	status   — "ok"  | "fail"
var MetricsEmailsSent = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "notifier_emails_sent_total",
	Help: "Total number of email send attempts, partitioned by provider and outcome.",
}, []string{"provider", "status"})

// MetricsWebhookCalls counts non-email channel deliveries (webhook / wecom /
// dingtalk / feishu). "channel" carries the channel type, "status" is the
// terminal outcome of the Send call.
var MetricsWebhookCalls = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "notifier_webhook_calls_total",
	Help: "Total number of webhook-style channel deliveries, partitioned by channel type and outcome.",
}, []string{"channel", "status"})

// MetricsRefundRetries counts D5 refund retry handler outcomes.
//
//	outcome — "ok"      PaymentHub accepted refund
//	          "retry"   scheduled another attempt
//	          "failed"  max attempts reached, apology email sent
var MetricsRefundRetries = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "notifier_refund_retries_total",
	Help: "Total D5 refund retry attempts, partitioned by terminal outcome.",
}, []string{"outcome"})

// MetricsSendDuration observes wall-clock time for each delivery attempt.
// The "channel" label is "email" for SMTP/SES sends and the channel type
// (webhook / wecom / dingtalk / feishu) for non-email channels.
var MetricsSendDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "notifier_send_duration_seconds",
	Help:    "Wall-clock duration of notifier delivery attempts.",
	Buckets: prometheus.DefBuckets,
}, []string{"channel"})

// ----------------------------------------------------------------------
// P1-11 Phase 1: idcd-namespaced notifier counters. The legacy
// notifier_* counters above keep working; this counter exposes
// per-(provider, template) outcome so the Grafana dashboard can pivot
// on either dimension without doing a multiplied PromQL join over
// notifier_emails_sent_total and a separate template label that didn't
// exist before.
// ----------------------------------------------------------------------

// EmailSent counts each Sender.Send call partitioned by outcome /
// provider / template. This is a denormalised duplicate of the legacy
// MetricsEmailsSent — the difference is the "template" label, which
// upgrades the signal from "emails are flowing" to "verify-email
// emails are flowing".
//
//	outcome  — "ok" | "fail"
//	provider — "ses" | "smtp" | "unknown"
//	template — "verify_email" | "reset_password" | "refund_apology" | ...
var EmailSent = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "idcd_notifier",
	Subsystem: "email",
	Name:      "sent_total",
	Help:      "邮件发送计数 (按 outcome + provider + template)",
}, []string{"outcome", "provider", "template"})

// EmailRetries counts retry decisions made by the notifier worker.
//
//	reason — "transient_smtp" | "ses_throttled" | "rate_limited" |
//	         "unknown"
var EmailRetries = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "idcd_notifier",
	Subsystem: "email",
	Name:      "retries_total",
	Help:      "邮件发送重试计数 (按 reason 分类)",
}, []string{"reason"})
