package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/quota"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// QuotaHandlerPool is the minimal pgx interface needed by QuotaHandler.
type QuotaHandlerPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// QuotaRateLimiter is the interface for querying current API usage.
type QuotaRateLimiter interface {
	CurrentUsage(ctx context.Context, userID string) (used int, err error)
	DailyTrend(ctx context.Context, userID string) ([]quota.DayCount, error)
}

// QuotaHandler handles quota-related API endpoints.
type QuotaHandler struct {
	pool    QuotaHandlerPool
	rateLim QuotaRateLimiter
}

// NewQuotaHandler creates a QuotaHandler.
func NewQuotaHandler(pool QuotaHandlerPool, rateLim QuotaRateLimiter) *QuotaHandler {
	return &QuotaHandler{pool: pool, rateLim: rateLim}
}

// ─────────────────────────────────────────────────────────────────────────────
// Response types
// ─────────────────────────────────────────────────────────────────────────────

// QuotaUsageItem represents a resource usage + limit pair.
type QuotaUsageItem struct {
	Used  int `json:"used"`
	Limit int `json:"limit"`
}

// QuotaAPIUsageItem extends QuotaUsageItem with a reset_at Unix timestamp.
type QuotaAPIUsageItem struct {
	Used    int   `json:"used"`
	Limit   int   `json:"limit"`
	ResetAt int64 `json:"reset_at"`
}

// QuotaStatusResponse is the response body for GET /v1/account/quota.
type QuotaStatusResponse struct {
	Plan            string            `json:"plan"`
	Monitors        QuotaUsageItem    `json:"monitors"`
	Channels        QuotaUsageItem    `json:"channels"`
	StatusPages     QuotaUsageItem    `json:"status_pages"`
	APICalls        QuotaAPIUsageItem `json:"api_calls"`
	APICallsTrend   []quota.DayCount  `json:"api_calls_trend"`
	MinIntervalS    int               `json:"min_interval_s"`
	MaxNodes        int               `json:"max_nodes"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers
// ─────────────────────────────────────────────────────────────────────────────

// GetQuota handles GET /v1/account/quota.
// Returns current quota usage and limits for the authenticated user.
func (h *QuotaHandler) GetQuota(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	// Fetch plan
	plan := h.userPlan(ctx, userID)
	limits := quota.Limits(plan)

	// Fetch resource counts
	monCount := h.countMonitors(ctx, userID)
	chanCount := h.countChannels(ctx, userID)
	spCount := h.countStatusPages(ctx, userID)

	// Fetch daily API usage and 7-day trend.
	apiUsed := 0
	var trend []quota.DayCount
	if h.rateLim != nil {
		used, err := h.rateLim.CurrentUsage(ctx, userID)
		if err == nil {
			apiUsed = used
		}
		if t, err := h.rateLim.DailyTrend(ctx, userID); err == nil {
			trend = t
		}
	}

	now := time.Now().UTC()
	resetAt := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC).Unix()

	resp := QuotaStatusResponse{
		Plan: plan,
		Monitors: QuotaUsageItem{
			Used:  monCount,
			Limit: limits.MaxMonitors,
		},
		Channels: QuotaUsageItem{
			Used:  chanCount,
			Limit: limits.MaxChannels,
		},
		StatusPages: QuotaUsageItem{
			Used:  spCount,
			Limit: limits.MaxStatusPages,
		},
		APICalls: QuotaAPIUsageItem{
			Used:    apiUsed,
			Limit:   limits.MaxAPIDailyReqs,
			ResetAt: resetAt,
		},
		APICallsTrend: trend,
		MinIntervalS:  limits.MinIntervalS,
		MaxNodes:      limits.MaxNodes,
	}

	response.JSON(w, r, http.StatusOK, resp)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func (h *QuotaHandler) userPlan(ctx context.Context, userID string) string {
	if h.pool == nil {
		return "free"
	}
	var plan string
	err := h.pool.QueryRow(ctx,
		`SELECT plan FROM subscriptions WHERE user_id = $1 AND status = 'active' LIMIT 1`,
		userID,
	).Scan(&plan)
	if err != nil {
		return "free"
	}
	return plan
}

func (h *QuotaHandler) countMonitors(ctx context.Context, userID string) int {
	if h.pool == nil {
		return 0
	}
	var count int
	err := h.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM monitors WHERE user_id = $1 AND status != 'archived'`,
		userID,
	).Scan(&count)
	if err != nil {
		return 0
	}
	return count
}

func (h *QuotaHandler) countChannels(ctx context.Context, userID string) int {
	if h.pool == nil {
		return 0
	}
	var count int
	err := h.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM alert_channels WHERE user_id = $1`,
		userID,
	).Scan(&count)
	if err != nil {
		return 0
	}
	return count
}

func (h *QuotaHandler) countStatusPages(ctx context.Context, userID string) int {
	if h.pool == nil {
		return 0
	}
	var count int
	err := h.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM status_pages WHERE user_id = $1`,
		userID,
	).Scan(&count)
	if err != nil {
		return 0
	}
	return count
}
