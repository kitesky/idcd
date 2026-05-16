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
)
