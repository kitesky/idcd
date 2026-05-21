package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
	"github.com/kite365/idcd/lib/shared/apperr"
)

type statusPagePublicPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// statusPageCache is the minimal cache surface used by the public status
// page handler. Defined as an interface so tests pass a tiny in-memory
// mock and don't need a real Redis; *NewRedisStatusPageCache returns the
// production implementation that wraps go-redis.
type statusPageCache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
}

type StatusPagePublicHandler struct {
	pool     statusPagePublicPool
	logger   *slog.Logger
	cache    statusPageCache // nil disables caching (test / minimal harness)
	cacheTTL time.Duration
}

// defaultStatusPageCacheTTL is the short TTL used when caching the rendered
// public status page. 30s tunes for "an outage shows up within half a minute"
// without losing the bulk of the DB load saving — at 60+ req/min/page the
// hit rate is already > 95%. Longer TTLs would mute legitimate outage signals
// to the page's audience, defeating the page's purpose.
const defaultStatusPageCacheTTL = 30 * time.Second

// NewStatusPagePublicHandler constructs the handler. logger may be nil — a
// no-op logger is used in that case so tests don't need to wire one up.
// Caching is OFF by default; call WithCache to enable.
func NewStatusPagePublicHandler(pool statusPagePublicPool, logger *slog.Logger) *StatusPagePublicHandler {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(discardWriter{}, nil))
	}
	return &StatusPagePublicHandler{pool: pool, logger: logger, cacheTTL: defaultStatusPageCacheTTL}
}

// WithCache returns a copy of the handler with a cache backend attached.
// Pass nil to explicitly disable caching (handy in tests that want to bypass
// the cache layer without changing the wiring shape).
func (h *StatusPagePublicHandler) WithCache(c statusPageCache) *StatusPagePublicHandler {
	cp := *h
	cp.cache = c
	return &cp
}

// discardWriter implements io.Writer by discarding everything — used so a
// nil logger argument still gives a valid *slog.Logger.
type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// statusPagePublicCacheKey is the Redis key under which the rendered JSON
// response for a given slug is stored. Versioned with a `v1:` prefix so a
// future schema change can roll forward without serving mixed shapes during
// the rollout window.
func statusPagePublicCacheKey(slug string) string {
	return "cache:status-pub:v1:" + slug
}

// computeETag returns a short hex digest used as the ETag header. Truncated
// to 16 hex chars (64 bits) — collision risk is irrelevant for a cache key
// and the smaller header is friendlier on log lines and proxies.
func computeETag(body []byte) string {
	sum := sha256.Sum256(body)
	return `"` + hex.EncodeToString(sum[:8]) + `"`
}

type publicMonitorHistory struct {
	Date   string  `json:"date"`
	Status string  `json:"status"`
	Uptime float64 `json:"uptime"`
}

type publicMonitor struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Status        string                 `json:"status"`
	UptimePercent float64                `json:"uptime_percent"`
	History       []publicMonitorHistory `json:"history"`
}

type publicGroup struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Monitors []publicMonitor `json:"monitors"`
}

type statusPagePublicResponse struct {
	Slug          string        `json:"slug"`
	Title         string        `json:"title"`
	Description   string        `json:"description"`
	Branding      bool          `json:"branding"`
	OverallStatus string        `json:"overall_status"`
	Groups        []publicGroup `json:"groups"`
	Events        []struct{}    `json:"events"`
}

func (h *StatusPagePublicHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slug := chi.URLParam(r, "slug")

	// Try the cache before hitting the DB. The cached payload is the
	// marshalled inner `data` only (no request_id) so each hit can be wrapped
	// in the standard envelope with the current request's id.
	if h.cache != nil {
		if cached, err := h.cache.Get(ctx, statusPagePublicCacheKey(slug)); err == nil && cached != "" {
			h.writeCachedResponse(w, r, []byte(cached))
			return
		} else if err != nil {
			// Cache miss / Redis blip — proceed to DB. We don't distinguish
			// "not-found" from "Redis unreachable" here; both are
			// indistinguishable from the request's perspective and the next
			// DB-backed write will repopulate the cache when Redis comes back.
			h.logger.Debug("status_page_public: cache get miss",
				"slug", slug, "err", err)
		}
	}

	sp, err := h.getStatusPageBySlug(ctx, slug)
	if err != nil {
		response.Error(w, r, apperr.NotFound("status page not found"))
		return
	}

	monitors, err := h.listMonitorsByUser(ctx, sp.UserID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query monitors", err))
		return
	}

	// Pre-fetch current statuses for all monitors in a single query to avoid N+1.
	monitorIDs := make([]string, len(monitors))
	for i, m := range monitors {
		monitorIDs[i] = m.ID
	}
	currentStatuses, err := h.batchCurrentStatuses(ctx, monitorIDs)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query monitor statuses", err))
		return
	}

	// Pre-fetch 30-day daily aggregates for ALL monitors in one query, then
	// fan them out per monitor below. Previously buildPublicMonitor ran one
	// aggregation query per monitor — 50 monitors meant 50 round-trips.
	since := time.Now().UTC().Add(-30 * 24 * time.Hour)
	dailyByMonitor, err := h.batchDailyChecks(ctx, monitorIDs, since)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query monitor history", err))
		return
	}

	pubMonitors := make([]publicMonitor, 0, len(monitors))
	for _, m := range monitors {
		pm := buildPublicMonitorFromRows(m, currentStatuses[m.ID], dailyByMonitor[m.ID], since)
		pubMonitors = append(pubMonitors, pm)
	}

	overall := overallStatus(pubMonitors)

	desc := ""
	if sp.Description != nil {
		desc = *sp.Description
	}

	resp := statusPagePublicResponse{
		Slug:          sp.Slug,
		Title:         sp.Name,
		Description:   desc,
		Branding:      sp.Branding,
		OverallStatus: overall,
		Groups: []publicGroup{
			{ID: "default", Name: "Monitors", Monitors: pubMonitors},
		},
		Events: []struct{}{},
	}

	// Marshal the inner data only (no envelope) and stash it in the cache.
	// We rewrap with a fresh envelope on every read so request_id stays
	// per-request and never gets cross-leaked from the request that warmed
	// the entry.
	dataBytes, marshalErr := json.Marshal(resp)
	if marshalErr != nil {
		// JSON encoding failed on our own struct — extremely unexpected; fall
		// back to response.JSON which has its own encode-error path.
		response.JSON(w, r, http.StatusOK, resp)
		return
	}
	if h.cache != nil {
		// Best-effort: cache failure must not break the request. The next
		// caller will re-warm the entry from the DB.
		if setErr := h.cache.Set(ctx, statusPagePublicCacheKey(slug), string(dataBytes), h.cacheTTL); setErr != nil {
			h.logger.Warn("status_page_public: cache set failed",
				"slug", slug, "err", setErr)
		}
	}

	h.writeCachedResponse(w, r, dataBytes)
}

// writeCachedResponse wraps cached inner-data bytes in the standard envelope
// and emits ETag + Cache-Control headers. The 304 short-circuit keeps
// revalidating clients off the wire entirely when the ETag matches.
//
// We compute the ETag from the inner data (not the envelope) so a request
// that only differs in request_id still hits 304 on a stale browser cache.
func (h *StatusPagePublicHandler) writeCachedResponse(w http.ResponseWriter, r *http.Request, data []byte) {
	etag := computeETag(data)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=30")
	if match := r.Header.Get("If-None-Match"); match != "" && match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Wrap in the standard {data, request_id} envelope. We assemble the
	// bytes manually to avoid re-marshalling the cached data — it's already
	// valid JSON. Per response.SuccessResponse: {"data":<inner>,"request_id":"..."}.
	requestID := requestIDFromHeader(r)
	_, _ = w.Write([]byte(`{"data":`))
	_, _ = w.Write(data)
	_, _ = w.Write([]byte(`,"request_id":"` + requestID + `"}`))
}

// requestIDFromHeader extracts the request id installed by the RequestID
// middleware. Mirrors response.getRequestID's fallback chain (context →
// request header → "unknown") so cache-hit responses and fresh responses
// emit the same request_id field.
func requestIDFromHeader(r *http.Request) string {
	if val := r.Context().Value("request_id"); val != nil { //nolint:staticcheck // matches middleware convention
		if id, ok := val.(string); ok && id != "" {
			return id
		}
	}
	if id := r.Header.Get("X-Request-ID"); id != "" {
		return id
	}
	return "unknown"
}

func (h *StatusPagePublicHandler) getStatusPageBySlug(ctx context.Context, slug string) (idcdmain.StatusPage, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, user_id, slug, name, description, custom_domain,
		        custom_domain_verified_at, custom_domain_cert_expires_at, branding, created_at, updated_at
		 FROM status_pages WHERE slug = $1`,
		slug,
	)
	var sp idcdmain.StatusPage
	err := row.Scan(
		&sp.ID, &sp.UserID, &sp.Slug, &sp.Name, &sp.Description, &sp.CustomDomain,
		&sp.CustomDomainVerifiedAt, &sp.CustomDomainCertExpiresAt, &sp.Branding,
		&sp.CreatedAt, &sp.UpdatedAt,
	)
	if err != nil {
		return idcdmain.StatusPage{}, errors.New("not found")
	}
	return sp, nil
}

func (h *StatusPagePublicHandler) listMonitorsByUser(ctx context.Context, userID string) ([]idcdmain.Monitor, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, user_id, name, type, target, config, interval_s, node_count, status, last_check_at, next_check_at, created_at, updated_at
		 FROM monitors WHERE user_id = $1 AND status != 'archived' ORDER BY created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var monitors []idcdmain.Monitor
	for rows.Next() {
		var m idcdmain.Monitor
		if err := rows.Scan(
			&m.ID, &m.UserID, &m.Name, &m.Type, &m.Target, &m.Config,
			&m.IntervalS, &m.NodeCount, &m.Status, &m.LastCheckAt, &m.NextCheckAt,
			&m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		monitors = append(monitors, m)
	}
	return monitors, rows.Err()
}

type dayCheckRow struct {
	date    string
	total   int64
	success int64
}

// batchDailyChecks fetches 30-day per-day uptime aggregates for ALL monitors
// in a single query, grouped by (monitor_id, day). Returns
// map[monitor_id][]dayCheckRow. Failures degrade open — caller treats missing
// keys as "no data" (operational).
func (h *StatusPagePublicHandler) batchDailyChecks(ctx context.Context, monitorIDs []string, since time.Time) (map[string][]dayCheckRow, error) {
	result := make(map[string][]dayCheckRow, len(monitorIDs))
	if len(monitorIDs) == 0 {
		return result, nil
	}
	rows, err := h.pool.Query(ctx, `
		SELECT
			monitor_id,
			DATE(check_at AT TIME ZONE 'UTC') AS day,
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE status = 'up') AS success
		FROM monitor_checks
		WHERE monitor_id = ANY($1) AND check_at >= $2
		GROUP BY monitor_id, day
		ORDER BY monitor_id, day ASC`,
		monitorIDs, since,
	)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var monID string
		var dr dayCheckRow
		if err := rows.Scan(&monID, &dr.date, &dr.total, &dr.success); err != nil {
			return result, err
		}
		result[monID] = append(result[monID], dr)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	return result, nil
}

// buildPublicMonitorFromRows assembles the per-monitor response from the
// pre-fetched batch aggregates produced by batchDailyChecks. dayRows may be
// nil — that just means no check data in the window (handled as 100% / op).
func buildPublicMonitorFromRows(m idcdmain.Monitor, currentStatus string, dayRows []dayCheckRow, since time.Time) publicMonitor {
	history := buildHistory(dayRows, since)
	uptimePct := computeUptimePercent(dayRows)
	return publicMonitor{
		ID:            m.ID,
		Name:          m.Name,
		Status:        currentStatus,
		UptimePercent: uptimePct,
		History:       history,
	}
}

// buildHistory produces a 30-day window oldest-first (since+0 … since+29).
// Days with no check data are treated as 100% uptime / operational.
func buildHistory(dayRows []dayCheckRow, since time.Time) []publicMonitorHistory {
	type totals struct{ total, success int64 }
	byDate := make(map[string]totals, len(dayRows))
	for _, dr := range dayRows {
		byDate[dr.date] = totals{dr.total, dr.success}
	}

	// Build oldest-first directly (i=0 is since, i=29 is ~today).
	history := make([]publicMonitorHistory, 0, 30)
	for i := 0; i < 30; i++ {
		d := since.AddDate(0, 0, i)
		dateStr := d.Format("2006-01-02")
		if row, ok := byDate[dateStr]; ok {
			uptime := 0.0
			if row.total > 0 {
				uptime = math.Round(float64(row.success)/float64(row.total)*10000) / 100
			}
			status := dayStatus(row.total, row.success)
			history = append(history, publicMonitorHistory{Date: dateStr, Status: status, Uptime: uptime})
		} else {
			history = append(history, publicMonitorHistory{Date: dateStr, Status: "operational", Uptime: 100.0})
		}
	}
	return history
}

func dayStatus(total, success int64) string {
	if total == 0 || success == total {
		return "operational"
	}
	if float64(success)/float64(total) >= 0.5 {
		return "degraded"
	}
	return "outage"
}

func computeUptimePercent(dayRows []dayCheckRow) float64 {
	var totalChecks, successChecks int64
	for _, dr := range dayRows {
		totalChecks += dr.total
		successChecks += dr.success
	}
	if totalChecks == 0 {
		return 100.0
	}
	return math.Round(float64(successChecks)/float64(totalChecks)*10000) / 100
}

// batchCurrentStatuses fetches the 3 most-recent check statuses for every
// monitor in a single LATERAL JOIN, eliminating the N+1 round-trip.
//
// Fails closed: a query error surfaces to the caller so the public status page
// returns 5xx rather than silently claiming everything is "operational" when
// the DB is degraded — a fail-open here would mask real outages to the
// audience the page is for.
func (h *StatusPagePublicHandler) batchCurrentStatuses(ctx context.Context, monitorIDs []string) (map[string]string, error) {
	result := make(map[string]string, len(monitorIDs))
	for _, id := range monitorIDs {
		result[id] = "operational"
	}
	if len(monitorIDs) == 0 {
		return result, nil
	}

	rows, err := h.pool.Query(ctx, `
		SELECT mc.monitor_id, mc.status
		FROM UNNEST($1::text[]) AS ids(monitor_id)
		CROSS JOIN LATERAL (
			SELECT monitor_id, status
			FROM monitor_checks
			WHERE monitor_id = ids.monitor_id
			ORDER BY check_at DESC
			LIMIT 3
		) mc
	`, monitorIDs)
	if err != nil {
		h.logger.Warn("status_page_public: batchCurrentStatuses query failed",
			"err", err, "monitor_count", len(monitorIDs))
		return nil, err
	}
	defer rows.Close()

	statusMap := make(map[string][]string, len(monitorIDs))
	for rows.Next() {
		var monID, st string
		if err := rows.Scan(&monID, &st); err != nil {
			h.logger.Warn("status_page_public: batchCurrentStatuses scan failed", "err", err)
			return nil, err
		}
		statusMap[monID] = append(statusMap[monID], st)
	}
	if err := rows.Err(); err != nil {
		h.logger.Warn("status_page_public: batchCurrentStatuses iter failed", "err", err)
		return nil, err
	}

	for id, statuses := range statusMap {
		failures := 0
		for _, s := range statuses {
			if s != "up" {
				failures++
			}
		}
		switch {
		case failures == 0:
			result[id] = "operational"
		case failures == len(statuses):
			result[id] = "outage"
		default:
			result[id] = "degraded"
		}
	}
	return result, nil
}

func overallStatus(monitors []publicMonitor) string {
	worst := "operational"
	for _, m := range monitors {
		if m.Status == "outage" {
			return "outage"
		}
		if m.Status == "degraded" {
			worst = "degraded"
		}
	}
	return worst
}

// redisStatusPageCache adapts *redis.Client to the statusPageCache interface.
// `redis.Nil` (key missing) is converted to (empty string, nil err) — the
// handler treats those identically as "cache miss, proceed to DB".
type redisStatusPageCache struct {
	rdb redis.UniversalClient
}

// NewRedisStatusPageCache returns a cache-adapter for *redis.Client. Pass nil
// to opt out of caching (the constructor returns nil so StatusPagePublicHandler.WithCache
// is a no-op).
func NewRedisStatusPageCache(rdb redis.UniversalClient) statusPageCache {
	if rdb == nil {
		return nil
	}
	return &redisStatusPageCache{rdb: rdb}
}

func (c *redisStatusPageCache) Get(ctx context.Context, key string) (string, error) {
	v, err := c.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	return v, err
}

func (c *redisStatusPageCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}
