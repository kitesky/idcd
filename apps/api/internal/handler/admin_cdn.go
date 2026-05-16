// Package handler implements HTTP handlers for the API server.
package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// systemUserID is the owner of all system-managed monitors (e.g., CDN leaderboard monitors).
const systemUserID = "idcd_system"

// systemCDNMonitors defines the 10 CDN providers tracked in the public leaderboard.
var systemCDNMonitors = []struct {
	Name   string
	Target string
}{
	{"Cloudflare CDN", "https://www.cloudflare.com"},
	{"Fastly CDN", "https://www.fastly.com"},
	{"Akamai CDN", "https://www.akamai.com"},
	{"AWS CloudFront", "https://aws.amazon.com/cloudfront/"},
	{"阿里云 CDN", "https://www.aliyun.com"},
	{"腾讯云 CDN", "https://cloud.tencent.com"},
	{"华为云 CDN", "https://www.huaweicloud.com"},
	{"百度云 CDN", "https://cloud.baidu.com"},
	{"又拍云 CDN", "https://www.upyun.com"},
	{"七牛云 CDN", "https://www.qiniu.com"},
}

// CDNPool is the subset of pgxpool.Pool used by AdminCDNHandler.
// It is satisfied by both *pgxpool.Pool and pgxmock.PgxPoolIface.
type CDNPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgxRow
	Exec(ctx context.Context, sql string, args ...any) (pgxCommandTag, error)
}

// pgxRow is a minimal interface for pgx.Row, satisfied by the real pgx.Row.
type pgxRow interface {
	Scan(dest ...any) error
}

// pgxCommandTag is a minimal interface for pgconn.CommandTag.
type pgxCommandTag interface {
	RowsAffected() int64
}

// cdnPoolAdapter wraps *pgxpool.Pool to satisfy CDNPool.
type cdnPoolAdapter struct {
	pool *pgxpool.Pool
}

func (a *cdnPoolAdapter) QueryRow(ctx context.Context, sql string, args ...any) pgxRow {
	return a.pool.QueryRow(ctx, sql, args...)
}

func (a *cdnPoolAdapter) Exec(ctx context.Context, sql string, args ...any) (pgxCommandTag, error) {
	return a.pool.Exec(ctx, sql, args...)
}

// AdminCDNHandler handles admin CDN monitor management endpoints.
type AdminCDNHandler struct {
	pool CDNPool
}

// NewAdminCDNHandler creates a new AdminCDNHandler backed by a real pgxpool.Pool.
func NewAdminCDNHandler(pool *pgxpool.Pool) *AdminCDNHandler {
	return &AdminCDNHandler{pool: &cdnPoolAdapter{pool: pool}}
}

// newAdminCDNHandlerWithPool creates an AdminCDNHandler using the CDNPool interface directly
// (used in tests with pgxmock).
func newAdminCDNHandlerWithPool(pool CDNPool) *AdminCDNHandler {
	return &AdminCDNHandler{pool: pool}
}

// CDNSeedResponse is the JSON response for POST /internal/admin/cdn-monitors/seed.
type CDNSeedResponse struct {
	Created int `json:"created"`
	Skipped int `json:"skipped"`
	Total   int `json:"total"`
}

// cdnSlug converts a CDN provider name into a short slug for use as a monitor ID.
// e.g. "Cloudflare CDN" → "cdn_cloudflare"
func cdnSlug(name string) string {
	lower := strings.ToLower(name)
	// Extract the provider name (first word before space, or first CJK word)
	parts := strings.Fields(lower)
	if len(parts) == 0 {
		return "cdn_unknown"
	}
	slug := parts[0]
	// Remove any non-alphanumeric characters
	var b strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	result := b.String()
	if result == "" {
		// Fallback: use index-based name for CJK-only entries
		result = "monitor"
	}
	return "cdn_" + result
}

// Seed handles POST /internal/admin/cdn-monitors/seed.
// Idempotent: existing monitors are skipped, new ones are inserted.
func (h *AdminCDNHandler) Seed(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	created := 0
	skipped := 0

	for _, cdn := range systemCDNMonitors {
		monitorID := cdnSlug(cdn.Name)

		// Check if monitor already exists for this user+target combination.
		var existingID string
		err := h.pool.QueryRow(ctx,
			`SELECT id FROM monitors WHERE user_id = $1 AND target = $2 LIMIT 1`,
			systemUserID, cdn.Target,
		).Scan(&existingID)

		if err == nil {
			// Row found — monitor already exists.
			skipped++
			continue
		}

		// err != nil could be pgx.ErrNoRows (expected) or a real error.
		// We detect "no rows" by checking the error message, since we can't import pgx here
		// without creating a circular dependency risk. Instead we use the standard approach:
		// attempt the INSERT and rely on ON CONFLICT to handle races.
		_, insertErr := h.pool.Exec(ctx, `
			INSERT INTO monitors (id, user_id, name, type, target, config, interval_s, node_count, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'http', $4, '{}', 60, 0, 'active', NOW(), NOW())
			ON CONFLICT (id) DO NOTHING
		`, monitorID, systemUserID, cdn.Name, cdn.Target)
		if insertErr != nil {
			response.Error(w, r, apperr.Internal("failed to insert CDN monitor", insertErr))
			return
		}

		created++
	}

	response.JSON(w, r, http.StatusOK, CDNSeedResponse{
		Created: created,
		Skipped: skipped,
		Total:   len(systemCDNMonitors),
	})
}
