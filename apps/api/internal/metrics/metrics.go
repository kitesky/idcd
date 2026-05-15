// Package metrics defines idcd-specific Prometheus business metrics.
// All metrics are registered via promauto and become available on the
// Prometheus default registry, which is exposed on :9091/metrics (internal)
// and /internal/metrics (public router, VPN-only).
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ActiveMonitors is the current number of monitors in active (non-paused) state.
	ActiveMonitors = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "idcd_monitors_active_total",
		Help: "Number of active monitors",
	})

	// TotalUsers is the total number of registered users in the system.
	TotalUsers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "idcd_users_total",
		Help: "Total registered users",
	})

	// AlertEventsFiring is the current number of firing (unresolved) alert events.
	AlertEventsFiring = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "idcd_alert_events_firing_total",
		Help: "Number of currently firing alert events",
	})

	// HTTPRequests counts all HTTP requests handled by the API,
	// labelled by HTTP method and status code.
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "idcd_http_requests_total",
		Help: "Total HTTP requests handled",
	}, []string{"method", "status"})
)
