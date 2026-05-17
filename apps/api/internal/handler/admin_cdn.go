// Package handler implements HTTP handlers for the API server.
package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// systemUserID is the owner of all system-managed monitors (e.g., CDN leaderboard monitors).
const systemUserID = "idcd_system"

// systemCDNMonitors defines the 10 CDN providers tracked in the public leaderboard.
var systemCDNMonitors = []struct {
	ID     string
	Name   string
	Target string
}{
	{"cdn_cloudflare", "Cloudflare CDN", "https://www.cloudflare.com"},
	{"cdn_fastly", "Fastly CDN", "https://www.fastly.com"},
	{"cdn_akamai", "Akamai CDN", "https://www.akamai.com"},
	{"cdn_aws", "AWS CloudFront", "https://aws.amazon.com/cloudfront/"},
	{"cdn_alicloud", "Alibaba Cloud CDN", "https://www.aliyun.com"},
	{"cdn_tencentcloud", "Tencent Cloud CDN", "https://cloud.tencent.com"},
	{"cdn_huaweicloud", "Huawei Cloud CDN", "https://www.huaweicloud.com"},
	{"cdn_baiducloud", "Baidu Cloud CDN", "https://cloud.baidu.com"},
	{"cdn_upyun", "Upyun CDN", "https://www.upyun.com"},
	{"cdn_qiniu", "Qiniu CDN", "https://www.qiniu.com"},
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

// CDNSeedResponse is the JSON response for POST /internal/admin/cdn-monitors/seed.
type CDNSeedResponse struct {
	Created int `json:"created"`
	Skipped int `json:"skipped"`
	Total   int `json:"total"`
}

// Seed handles POST /internal/admin/cdn-monitors/seed.
// Idempotent: existing monitors (matched by ID) are skipped, new ones are inserted.
func (h *AdminCDNHandler) Seed(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	created := 0
	skipped := 0

	for _, cdn := range systemCDNMonitors {
		tag, insertErr := h.pool.Exec(ctx, `
			INSERT INTO monitors (id, user_id, name, type, target, config, interval_s, node_count, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'http', $4, '{}', 60, 0, 'active', NOW(), NOW())
			ON CONFLICT (id) DO NOTHING
		`, cdn.ID, systemUserID, cdn.Name, cdn.Target)
		if insertErr != nil {
			response.Error(w, r, apperr.Internal("failed to insert CDN monitor", insertErr))
			return
		}
		if tag.RowsAffected() == 0 {
			skipped++
		} else {
			created++
		}
	}

	response.JSON(w, r, http.StatusOK, CDNSeedResponse{
		Created: created,
		Skipped: skipped,
		Total:   len(systemCDNMonitors),
	})
}
