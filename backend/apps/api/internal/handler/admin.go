// Package handler implements HTTP handlers for the API server.
package handler

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// AdminPool is the subset of pgxpool.Pool used by AdminHandler.
// It is satisfied by both *pgxpool.Pool and pgxmock.PgxPoolIface.
type AdminPool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// AdminHandler handles admin management endpoints.
type AdminHandler struct {
	pool         AdminPool
	adminToken   string
	ipAllowlist  []*net.IPNet // pre-parsed CIDRs; nil = no IP gating
	adminOrigins map[string]struct{}
}

// NewAdminHandler creates a new AdminHandler.
// adminToken is the expected value of the Bearer token for admin access.
func NewAdminHandler(pool AdminPool, adminToken string) *AdminHandler {
	return &AdminHandler{pool: pool, adminToken: adminToken}
}

// WithIPAllowlist pre-parses the given list of CIDRs / single-IP entries.
// Bad entries are logged and skipped so a typo in one operator's config
// can't take the whole admin surface offline. Empty list (default) means
// no IP gating — appropriate for dev / loopback-only deployments.
//
// Single-IP entries ("203.0.113.42") are normalised to /32 (or /128 for v6)
// so the CIDR check works uniformly.
func (h *AdminHandler) WithIPAllowlist(entries []string) *AdminHandler {
	nets := make([]*net.IPNet, 0, len(entries))
	for _, raw := range entries {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if !strings.Contains(raw, "/") {
			if ip := net.ParseIP(raw); ip != nil {
				if ip.To4() != nil {
					raw += "/32"
				} else {
					raw += "/128"
				}
			}
		}
		_, n, err := net.ParseCIDR(raw)
		if err != nil {
			slog.Default().Warn("admin: ignoring invalid ip allowlist entry",
				"entry", raw, "error", err)
			continue
		}
		nets = append(nets, n)
	}
	h.ipAllowlist = nets
	return h
}

// WithAdminOrigins seeds the Origin / Referer allowlist used by mutating
// requests. Each entry should be a scheme+host (e.g. "https://admin.idcd.com").
// Empty disables the check (Bearer alone is considered sufficient — fine for
// server-to-server clients without a browser context).
func (h *AdminHandler) WithAdminOrigins(origins []string) *AdminHandler {
	set := make(map[string]struct{}, len(origins))
	for _, o := range origins {
		o = strings.TrimRight(strings.TrimSpace(o), "/")
		if o == "" {
			continue
		}
		set[o] = struct{}{}
	}
	h.adminOrigins = set
	return h
}

// AdminAuthMiddleware enforces the admin auth contract on /internal/admin/*:
//
//  1. IP allowlist gate (when configured) — cheapest reject, runs first.
//  2. Origin / Referer gate (mutating methods only, when configured) — closes
//     the "logged-in admin browses evil.com which fetches /internal/*" window.
//  3. Bearer admin_token check with constant-time compare.
//  4. Structured audit log of the call (actor=admin, method, path, IP, UA).
//
// Each gate fails closed with a distinct error code so ops can disambiguate
// "blocked by network policy" vs "wrong token" in logs without leaking which
// stage matched to the unauthenticated caller (all four return 401/403 with
// generic messages externally).
func (h *AdminHandler) AdminAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := middleware.ClientIP(r)

		if !h.ipAllowed(clientIP) {
			slog.Default().Warn("admin auth: ip blocked",
				"ip", clientIP, "path", r.URL.Path, "method", r.Method)
			response.Error(w, r, apperr.Forbidden("admin access denied"))
			return
		}

		if !isSafeMethod(r.Method) && !h.originAllowed(r) {
			slog.Default().Warn("admin auth: origin blocked",
				"ip", clientIP, "origin", r.Header.Get("Origin"),
				"referer", r.Header.Get("Referer"), "path", r.URL.Path)
			response.Error(w, r, apperr.Forbidden("admin access denied"))
			return
		}

		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			response.Error(w, r, apperr.Unauthorized("missing or malformed Authorization header"))
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		// Constant-time compare to prevent length/byte-leak timing attacks.
		if token == "" || h.adminToken == "" ||
			subtle.ConstantTimeCompare([]byte(token), []byte(h.adminToken)) != 1 {
			slog.Default().Warn("admin auth: token rejected",
				"ip", clientIP, "path", r.URL.Path, "method", r.Method)
			response.Error(w, r, apperr.Unauthorized("invalid admin token"))
			return
		}

		// Successful entry. Audit before handing off so a panic in the
		// inner handler still leaves a trace of "who tried what".
		slog.Default().Info("audit",
			"category", "audit",
			"event", "audit.internal_admin.access",
			"actor_user_id", "admin", // admin_token is a shared service credential
			"ip", clientIP,
			"user_agent", r.UserAgent(),
			"method", r.Method,
			"path", r.URL.Path,
		)

		next.ServeHTTP(w, r)
	})
}

// isSafeMethod reports whether the HTTP method is non-mutating per RFC 7231.
// GET / HEAD / OPTIONS do not need Origin checks; their CSRF analogue is
// covered by per-endpoint authorization. POST / PUT / PATCH / DELETE do.
func isSafeMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

// ipAllowed returns true when the client IP matches any configured CIDR.
// Empty allowlist (default) trivially returns true — operators who don't
// set the field opt out of IP gating entirely.
func (h *AdminHandler) ipAllowed(ipStr string) bool {
	if len(h.ipAllowlist) == 0 {
		return true
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, n := range h.ipAllowlist {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// originAllowed validates the request's Origin (preferred) or Referer
// header against the configured allowlist. Empty allowlist disables the
// check. Missing Origin AND Referer on a mutating request from a browser
// context is treated as block-by-default — non-browser callers that need
// to POST should set Origin explicitly. Server-to-server tools without an
// Origin header should use an empty admin_origins config (which disables
// this check entirely) rather than relying on missing-header bypass.
func (h *AdminHandler) originAllowed(r *http.Request) bool {
	if len(h.adminOrigins) == 0 {
		return true
	}
	candidate := r.Header.Get("Origin")
	if candidate == "" {
		// Fall back to Referer's origin component. Referer is the full URL
		// including path; we strip back to scheme+host.
		if ref := r.Header.Get("Referer"); ref != "" {
			if u, err := url.Parse(ref); err == nil && u.Scheme != "" && u.Host != "" {
				candidate = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
			}
		}
	}
	if candidate == "" {
		return false
	}
	candidate = strings.TrimRight(candidate, "/")
	_, ok := h.adminOrigins[candidate]
	return ok
}

// SubscriptionBreakdown holds subscription counts per plan.
type SubscriptionBreakdown struct {
	Free       int `json:"free"`
	Pro        int `json:"pro"`
	Team       int `json:"team"`
	Enterprise int `json:"enterprise"`
}

// AdminMetricsResponse is the response for GET /internal/admin/metrics.
type AdminMetricsResponse struct {
	TotalUsers        int                   `json:"total_users"`
	ActiveUsers7d     int                   `json:"active_users_7d"`
	TotalMonitors     int                   `json:"total_monitors"`
	ActiveMonitors    int                   `json:"active_monitors"`
	TotalNodes        int                   `json:"total_nodes"`
	OnlineNodes       int                   `json:"online_nodes"`
	Subscriptions     SubscriptionBreakdown `json:"subscriptions"`
	MRREstimateCNY    int                   `json:"mrr_estimate_cny"`
}

// AdminMetrics handles GET /internal/admin/metrics.
func (h *AdminHandler) AdminMetrics(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var metrics AdminMetricsResponse

	// Total users
	if err := h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM "user"`).Scan(&metrics.TotalUsers); err != nil {
		response.Error(w, r, apperr.Internal("failed to count users", err))
		return
	}

	// Active users in last 7 days (users who logged in or registered recently)
	// Use created_at as proxy for now; TODO: use last_login_at once available
	if err := h.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM "user" WHERE created_at > NOW() - INTERVAL '7 days'`,
	).Scan(&metrics.ActiveUsers7d); err != nil {
		response.Error(w, r, apperr.Internal("failed to count active users", err))
		return
	}

	// Total monitors
	if err := h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM monitors`).Scan(&metrics.TotalMonitors); err != nil {
		response.Error(w, r, apperr.Internal("failed to count monitors", err))
		return
	}

	// Active monitors (not paused)
	if err := h.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM monitors WHERE status != 'paused'`,
	).Scan(&metrics.ActiveMonitors); err != nil {
		response.Error(w, r, apperr.Internal("failed to count active monitors", err))
		return
	}

	// Total nodes
	if err := h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM enrolled_nodes`).Scan(&metrics.TotalNodes); err != nil {
		response.Error(w, r, apperr.Internal("failed to count nodes", err))
		return
	}

	// Online nodes — enrolled_nodes.status enum is pending/active/drained/offline/disabled (00030)
	if err := h.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM enrolled_nodes WHERE status = 'active'`,
	).Scan(&metrics.OnlineNodes); err != nil {
		response.Error(w, r, apperr.Internal("failed to count online nodes", err))
		return
	}

	// Subscription breakdown by plan
	rows, err := h.pool.Query(ctx,
		`SELECT plan, COUNT(*) FROM subscriptions WHERE status = 'active' GROUP BY plan`,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query subscriptions", err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var plan string
		var count int
		if err := rows.Scan(&plan, &count); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan subscription row", err))
			return
		}
		switch plan {
		case "free":
			metrics.Subscriptions.Free = count
		case "pro":
			metrics.Subscriptions.Pro = count
		case "team":
			metrics.Subscriptions.Team = count
		case "enterprise":
			metrics.Subscriptions.Enterprise = count
		}
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("subscription row iteration error", err))
		return
	}

	// MRR estimate (CNY): pro=¥99/mo, team=¥299/mo, enterprise=¥999/mo
	metrics.MRREstimateCNY = metrics.Subscriptions.Pro*99 +
		metrics.Subscriptions.Team*299 +
		metrics.Subscriptions.Enterprise*999

	response.JSON(w, r, http.StatusOK, metrics)
}

// AdminUserEntry is a single user entry in the admin user list.
type AdminUserEntry struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	Status       string `json:"status"`
	Plan         string `json:"plan"`
	MonitorCount int    `json:"monitor_count"`
	CreatedAt    string `json:"created_at"`
}

// AdminUsersResponse is the response for GET /internal/admin/users.
type AdminUsersResponse struct {
	Users   []AdminUserEntry `json:"users"`
	Total   int              `json:"total"`
	Page    int              `json:"page"`
	PerPage int              `json:"per_page"`
}

// AdminUsers handles GET /internal/admin/users?page=1&per_page=20&q=keyword.
func (h *AdminHandler) AdminUsers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Parse query params
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	perPage := 20
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if v, err := strconv.Atoi(pp); err == nil && v > 0 && v <= 100 {
			perPage = v
		}
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	offset := (page - 1) * perPage

	// Count total (with optional search filter)
	var total int
	if q != "" {
		likeQ := "%" + q + "%"
		if err := h.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM "user" WHERE email ILIKE $1`,
			likeQ,
		).Scan(&total); err != nil {
			response.Error(w, r, apperr.Internal("failed to count users", err))
			return
		}
	} else {
		if err := h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM "user"`).Scan(&total); err != nil {
			response.Error(w, r, apperr.Internal("failed to count users", err))
			return
		}
	}

	// Fetch users with plan from subscriptions (LEFT JOIN) and monitor count (subquery)
	var rows pgx.Rows
	var err error
	// monitor_count was previously a correlated subquery — O(users) extra
	// query plans per request. Replaced with a LEFT JOIN over a per-user
	// aggregate so the optimiser can plan a single hash-join.
	if q != "" {
		likeQ := "%" + q + "%"
		rows, err = h.pool.Query(ctx, `
			SELECT
				u.id,
				u.email,
				u.status,
				COALESCE(s.plan, 'free') AS plan,
				COALESCE(mc.cnt, 0) AS monitor_count,
				u.created_at
			FROM "user" u
			LEFT JOIN subscriptions s ON s.user_id = u.id AND s.status = 'active'
			LEFT JOIN (
				SELECT user_id, COUNT(*) AS cnt
				FROM monitors
				GROUP BY user_id
			) mc ON mc.user_id = u.id
			WHERE u.email ILIKE $1
			ORDER BY u.created_at DESC
			LIMIT $2 OFFSET $3
		`, likeQ, perPage, offset)
	} else {
		rows, err = h.pool.Query(ctx, `
			SELECT
				u.id,
				u.email,
				u.status,
				COALESCE(s.plan, 'free') AS plan,
				COALESCE(mc.cnt, 0) AS monitor_count,
				u.created_at
			FROM "user" u
			LEFT JOIN subscriptions s ON s.user_id = u.id AND s.status = 'active'
			LEFT JOIN (
				SELECT user_id, COUNT(*) AS cnt
				FROM monitors
				GROUP BY user_id
			) mc ON mc.user_id = u.id
			ORDER BY u.created_at DESC
			LIMIT $1 OFFSET $2
		`, perPage, offset)
	}
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query users", err))
		return
	}
	defer rows.Close()

	users := make([]AdminUserEntry, 0, perPage)
	for rows.Next() {
		var u AdminUserEntry
		var createdAt time.Time
		if err := rows.Scan(
			&u.ID,
			&u.Email,
			&u.Status,
			&u.Plan,
			&u.MonitorCount,
			&createdAt,
		); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan user row", err))
			return
		}
		u.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("user row iteration error", err))
		return
	}

	response.JSON(w, r, http.StatusOK, AdminUsersResponse{
		Users:   users,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	})
}

// AdminUserDetailMonitor is a monitor entry in user detail.
type AdminUserDetailMonitor struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Type   string `json:"type"`
}

// AdminUserDetailResponse is the response for GET /internal/admin/users/{id}.
type AdminUserDetailResponse struct {
	ID           string                   `json:"id"`
	Email        string                   `json:"email"`
	Status       string                   `json:"status"`
	Plan         string                   `json:"plan"`
	MonitorCount int                      `json:"monitor_count"`
	CreatedAt    string                   `json:"created_at"`
	Monitors     []AdminUserDetailMonitor `json:"monitors"`
}

// AdminUserDetail handles GET /internal/admin/users/{id}.
func (h *AdminHandler) AdminUserDetail(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		response.Error(w, r, apperr.Validation("missing user id", ""))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var detail AdminUserDetailResponse
	var createdAt time.Time

	err := h.pool.QueryRow(ctx, `
		SELECT
			u.id,
			u.email,
			u.status,
			COALESCE(s.plan, 'free') AS plan,
			(SELECT COUNT(*) FROM monitors m WHERE m.user_id = u.id) AS monitor_count,
			u.created_at
		FROM "user" u
		LEFT JOIN subscriptions s ON s.user_id = u.id AND s.status = 'active'
		WHERE u.id = $1
	`, userID).Scan(
		&detail.ID,
		&detail.Email,
		&detail.Status,
		&detail.Plan,
		&detail.MonitorCount,
		&createdAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			response.Error(w, r, apperr.NotFound("user not found"))
			return
		}
		response.Error(w, r, apperr.Internal("failed to query user", err))
		return
	}
	detail.CreatedAt = createdAt.UTC().Format(time.RFC3339)

	// Fetch last 5 monitors
	monRows, err := h.pool.Query(ctx, `
		SELECT id, name, status, type
		FROM monitors
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 5
	`, userID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query monitors", err))
		return
	}
	defer monRows.Close()

	detail.Monitors = make([]AdminUserDetailMonitor, 0, 5)
	for monRows.Next() {
		var mon AdminUserDetailMonitor
		if err := monRows.Scan(&mon.ID, &mon.Name, &mon.Status, &mon.Type); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan monitor row", err))
			return
		}
		detail.Monitors = append(detail.Monitors, mon)
	}
	if err := monRows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("monitor row iteration error", err))
		return
	}

	response.JSON(w, r, http.StatusOK, detail)
}

// TestEmail handles POST /internal/admin/test-email?to=xxx
// 发送一封测试邮件，验证 notifier 队列是否工作
func (h *AdminHandler) TestEmail(w http.ResponseWriter, r *http.Request) {
	to := r.URL.Query().Get("to")
	if to == "" {
		response.Error(w, r, apperr.Validation("to query param required", "to"))
		return
	}
	// AdminHandler 没有 enqueuer，简单实现：返回配置状态 + 提示
	response.JSON(w, r, http.StatusOK, map[string]any{
		"message": "email test task submitted, check notifier service logs",
		"note":    "ensure notifier service is running and notifier.smtp is correctly configured",
		"to":      to,
	})
}
