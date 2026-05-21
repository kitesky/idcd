// Package metrics defines idcd-specific Prometheus business metrics.
// All metrics are registered via promauto and become available on the
// Prometheus default registry, which is exposed on :9091/metrics (internal)
// and /internal/metrics (public router, VPN-only).
//
// Naming convention (P1-11 Phase 1):
//
//   - existing gauges keep their legacy names (idcd_monitors_active_total,
//     idcd_users_total, idcd_alert_events_firing_total, idcd_http_requests_total)
//     so dashboards already pointing at them keep working.
//   - new business metrics follow idcd_api_<subsystem>_<noun>_<unit> to stay
//     consistent with the path-D gateway counter (idcd_gateway_stale_epoch_total).
//
// Label cardinality discipline:
//
//   - outcome / error_class / provider — bounded enum, safe.
//   - workspace_id — bounded to ~1k workspaces in S2; if we ever blow past
//     ~10k we will collapse to per-plan aggregation. Documented inline below.
//   - never label by raw user-supplied strings (email, user_id, target).
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

	// ---------------------------------------------------------------
	// P1-11 Phase 1: business metrics for auth / OTP / token / quota.
	// ---------------------------------------------------------------

	// RegistrationAttempts counts /v1/auth/register outcomes.
	//
	//	outcome — "success" | "duplicate_email" | "invalid_input" |
	//	          "weak_password" | "internal"
	RegistrationAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "idcd_api",
		Subsystem: "auth",
		Name:      "registration_attempts_total",
		Help:      "用户注册尝试次数 (按 outcome 分类)",
	}, []string{"outcome"})

	// LoginAttempts counts /v1/auth/login outcomes.
	//
	//	outcome — "success" | "invalid_credentials" | "account_disabled" |
	//	          "mfa_required" | "invalid_input" | "internal"
	LoginAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "idcd_api",
		Subsystem: "auth",
		Name:      "login_attempts_total",
		Help:      "登录尝试次数 (按 outcome 分类)",
	}, []string{"outcome"})

	// OTPVerifyAttempts counts OTP verification outcomes across all flows
	// (email verify, password reset, 2FA-step). Bounded to a handful of
	// outcomes — never label by otp_id or user_id.
	//
	//	outcome — "success" | "invalid" | "expired" |
	//	          "attempts_exceeded" | "internal"
	//	type    — "email_verify" | "password_reset" | "two_factor" | ...
	OTPVerifyAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "idcd_api",
		Subsystem: "auth",
		Name:      "otp_verify_attempts_total",
		Help:      "OTP 验证尝试次数 (按 outcome + type 分类)",
	}, []string{"outcome", "type"})

	// TokensIssued counts tokens issued by the API, by token kind.
	//
	//	kind — "session" | "pat" | "api_key" | "mcp_personal" |
	//	       "mcp_workspace" | "mcp_service" | "two_factor_session"
	TokensIssued = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "idcd_api",
		Subsystem: "auth",
		Name:      "tokens_issued_total",
		Help:      "已签发的认证 token 数 (按 kind 分类, D2 90d 上限)",
	}, []string{"kind"})

	// QuotaUsageRatio is the current API quota usage per (workspace_id, plan).
	//
	// Cardinality note: workspace_id is intentionally a label. At current
	// (S2) scale that is < 1k workspaces — manageable. If we cross ~10k we
	// will collapse to per-plan aggregation only (see top-of-file note).
	QuotaUsageRatio = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "idcd_api",
		Subsystem: "quota",
		Name:      "usage_ratio",
		Help:      "API quota 当前使用率 (0-1, per workspace, per plan)",
	}, []string{"workspace_id", "plan"})
)
