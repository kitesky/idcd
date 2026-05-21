// Package metrics defines aggregator-specific Prometheus metrics.
//
// All metrics are registered via promauto on the default registry, which is
// exposed by an internal HTTP listener (see cmd/aggregator/main.go).
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// MessagesProcessed counts every stream message processed successfully by the
	// aggregator, labelled by stream and consumer name.
	MessagesProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "aggregator_messages_processed_total",
		Help: "Total stream messages processed successfully",
	}, []string{"stream", "consumer"})

	// MessagesFailed counts processor failures (message stays in PEL for retry).
	MessagesFailed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "aggregator_messages_failed_total",
		Help: "Total stream messages whose processor returned an error",
	}, []string{"stream", "consumer"})

	// DLQMessages counts messages moved to the dead-letter stream after exceeding
	// the redelivery threshold.
	DLQMessages = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "aggregator_dlq_messages_total",
		Help: "Total stream messages moved to the dead-letter stream",
	}, []string{"stream"})

	// ReclaimedMessages counts messages reclaimed via XAUTOCLAIM (idle in PEL).
	ReclaimedMessages = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "aggregator_reclaimed_messages_total",
		Help: "Total stream messages reclaimed via XAUTOCLAIM",
	}, []string{"stream", "consumer"})

	// PELSize is the sampled count of pending (unACKed) messages per consumer.
	PELSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "aggregator_pel_size",
		Help: "Pending Entries List size sampled per consumer",
	}, []string{"stream", "consumer"})

	// ----------------------------------------------------------------------
	// P1-11 Phase 1: idcd-namespaced aggregator metrics. The legacy
	// aggregator_* counters above keep working; the new metrics surface
	// per-message wall-clock latency and stream-level lag — both required
	// inputs for the D12 SLA / P1-7 "data pipeline healthy?" question.
	// ----------------------------------------------------------------------

	// StreamConsumerLag is the time gap between "scheduler XADD" and
	// "aggregator processed the message", sampled per stream. Updated
	// inside the processor success path. Gauge (not histogram) because
	// downstream alerting wants the latest sample, not a distribution.
	StreamConsumerLag = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "idcd_aggregator",
		Subsystem: "stream",
		Name:      "consumer_lag_seconds",
		Help:      "最近一条消息的端到端延迟 (秒, scheduler 投递 → aggregator 处理完成)",
	}, []string{"stream"})

	// ProcessedResults counts probe results processed by terminal outcome,
	// partitioned by probe_type. Complements the legacy
	// aggregator_messages_processed_total which knows about streams but
	// not about the business-level outcome of each message.
	//
	//	outcome    — "ok" | "validation_failed" | "downstream_error"
	//	probe_type — "http" | "ping" | "tcp" | "dns" | "unknown"
	ProcessedResults = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "idcd_aggregator",
		Subsystem: "results",
		Name:      "processed_total",
		Help:      "Probe 结果处理终态计数 (按 outcome + probe_type)",
	}, []string{"outcome", "probe_type"})

	// ProcessingDuration observes the wall-clock time spent inside the
	// processor for each message, by probe_type so a slow HTTP probe path
	// doesn't get hidden by a healthy ping path.
	ProcessingDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "idcd_aggregator",
		Subsystem: "results",
		Name:      "processing_duration_seconds",
		Help:      "Aggregator 单条消息处理时长分布",
		Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	}, []string{"probe_type"})
)
