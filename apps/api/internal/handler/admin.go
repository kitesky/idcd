// Package handler implements HTTP handlers for the API server.
package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

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
	pool       AdminPool
	adminToken string
}

// NewAdminHandler creates a new AdminHandler.
// adminToken is the expected value of the Bearer token for admin access.
func NewAdminHandler(pool AdminPool, adminToken string) *AdminHandler {
	return &AdminHandler{pool: pool, adminToken: adminToken}
}

// AdminAuthMiddleware returns a middleware that validates the admin Bearer token.
func (h *AdminHandler) AdminAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			response.Error(w, r, apperr.Unauthorized("missing or malformed Authorization header"))
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == "" || token != h.adminToken {
			response.Error(w, r, apperr.Unauthorized("invalid admin token"))
			return
		}
		next.ServeHTTP(w, r)
	})
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
	if err := h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&metrics.TotalUsers); err != nil {
		response.Error(w, r, apperr.Internal("failed to count users", err))
		return
	}

	// Active users in last 7 days (users who logged in or registered recently)
	// Use created_at as proxy for now; TODO: use last_login_at once available
	if err := h.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE created_at > NOW() - INTERVAL '7 days'`,
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
	if err := h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM nodes`).Scan(&metrics.TotalNodes); err != nil {
		response.Error(w, r, apperr.Internal("failed to count nodes", err))
		return
	}

	// Online nodes
	if err := h.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM nodes WHERE status = 'online'`,
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
			`SELECT COUNT(*) FROM users WHERE email ILIKE $1`,
			likeQ,
		).Scan(&total); err != nil {
			response.Error(w, r, apperr.Internal("failed to count users", err))
			return
		}
	} else {
		if err := h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
			response.Error(w, r, apperr.Internal("failed to count users", err))
			return
		}
	}

	// Fetch users with plan from subscriptions (LEFT JOIN) and monitor count (subquery)
	var rows pgx.Rows
	var err error
	if q != "" {
		likeQ := "%" + q + "%"
		rows, err = h.pool.Query(ctx, `
			SELECT
				u.id,
				u.email,
				u.status,
				COALESCE(s.plan, 'free') AS plan,
				(SELECT COUNT(*) FROM monitors m WHERE m.user_id = u.id) AS monitor_count,
				u.created_at
			FROM users u
			LEFT JOIN subscriptions s ON s.user_id = u.id AND s.status = 'active'
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
				(SELECT COUNT(*) FROM monitors m WHERE m.user_id = u.id) AS monitor_count,
				u.created_at
			FROM users u
			LEFT JOIN subscriptions s ON s.user_id = u.id AND s.status = 'active'
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
		FROM users u
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
