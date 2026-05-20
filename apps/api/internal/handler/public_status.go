package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// PublicStatusHandler serves the no-auth endpoints powering idcd.com/status.
// The endpoints read status_uptime_5min / status_uptime_daily / status_incidents
// (see lib/db/migrations/idcd_main/00050_status_uptime.sql) and shape the
// payloads to match the frontend status-page components.
//
// All responses set Cache-Control: public, max-age=60 — the data only refreshes
// every 5 minutes anyway, and a 60s CDN cache shields the DB from spiky traffic
// during an outage when this page becomes the most-visited URL on the site.
type PublicStatusHandler struct {
	pool *pgxpool.Pool
}

// NewPublicStatusHandler wires the handler to the main pgx pool.
func NewPublicStatusHandler(pool *pgxpool.Pool) *PublicStatusHandler {
	return &PublicStatusHandler{pool: pool}
}

// publicStatusCacheControl is exported as a const so server.go (or tests)
// can verify the header value if needed.
const publicStatusCacheControl = "public, max-age=60"

// Service status string codes mirror the frontend ServiceStatus enum.
const (
	statusStrOperational = "operational"
	statusStrDegraded    = "degraded"
	statusStrOutage      = "outage"
	statusStrMaintenance = "maintenance"
)

// statusFromInt converts the SMALLINT status code stored in DB
// (1..4) to the JSON string used by the frontend.
func statusFromInt(s int16) string {
	switch s {
	case 1:
		return statusStrOperational
	case 2:
		return statusStrDegraded
	case 3:
		return statusStrOutage
	case 4:
		return statusStrMaintenance
	default:
		// Unknown values fall back to outage rather than synthesizing a new
		// status — being loud about "we don't know" is safer than green-washing.
		return statusStrOutage
	}
}

// ── Response types ────────────────────────────────────────────────────────────

// PublicStatusOverviewResponse is the shape consumed by
// app/(public)/status/page.tsx (and the legacy customer-facing page after
// migration, but they don't share routes).
type PublicStatusOverviewResponse struct {
	OverallStatus string                          `json:"overall_status"`
	GeneratedAt   time.Time                       `json:"generated_at"`
	Services      []PublicStatusService           `json:"services"`
	Nodes         []PublicStatusNodeCountryGroup  `json:"nodes"`
}

// PublicStatusService is one tile in the "services" section. History
// is at most 90 entries (today + 89 prior days), oldest-first.
type PublicStatusService struct {
	Key           string                  `json:"key"`
	CurrentStatus string                  `json:"current_status"`
	UptimePercent float64                 `json:"uptime_percent"`
	History       []PublicStatusDailyBar  `json:"history"`
}

// PublicStatusDailyBar is one cell in the 90-day uptime bar.
type PublicStatusDailyBar struct {
	Day         string  `json:"day"` // YYYY-MM-DD UTC
	Status      string  `json:"status"`
	UptimePct   float64 `json:"uptime_pct"`
	IncidentIDs []int64 `json:"incident_ids,omitempty"`
}

// PublicStatusNodeCountryGroup groups nodes by ISO country code so the
// frontend can render "🇸🇬 SG · N online" cards without computing the
// rollup itself.
type PublicStatusNodeCountryGroup struct {
	CountryCode string             `json:"country_code"`
	OnlineCount int                `json:"online_count"`
	TotalCount  int                `json:"total_count"`
	Nodes       []PublicStatusNode `json:"nodes"`
}

// PublicStatusNode is one row inside a country group.
type PublicStatusNode struct {
	NodeID        string `json:"node_id"`
	City          string `json:"city,omitempty"`
	IP            string `json:"ip,omitempty"`
	Status        string `json:"status"`
	LastSeenAgeS  int64  `json:"last_seen_age_s"` // -1 if never seen
}

// PublicStatusIncident matches one row of status_incidents for the
// "Recent Events" list.
type PublicStatusIncident struct {
	ID         int64      `json:"id"`
	ServiceKey string     `json:"service_key"`
	StartedAt  time.Time  `json:"started_at"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
	Severity   string     `json:"severity"`
	Title      string     `json:"title"`
	Summary    string     `json:"summary,omitempty"`
	Related    []string   `json:"related,omitempty"`
}

// PublicStatusIncidentsResponse wraps the recent incidents list.
type PublicStatusIncidentsResponse struct {
	Incidents []PublicStatusIncident `json:"incidents"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// Overview handles GET /v1/public/status/overview. Returns the current status
// of all configured services plus the node fleet, grouped by country.
func (h *PublicStatusHandler) Overview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	services, err := h.loadServices(ctx)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to load services", err))
		return
	}
	nodes, err := h.loadNodes(ctx)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to load nodes", err))
		return
	}

	overall := computeOverall(services, nodes)
	resp := PublicStatusOverviewResponse{
		OverallStatus: overall,
		GeneratedAt:   time.Now().UTC(),
		Services:      services,
		Nodes:         nodes,
	}

	w.Header().Set("Cache-Control", publicStatusCacheControl)
	response.JSON(w, r, http.StatusOK, resp)
}

// Incidents handles GET /v1/public/status/incidents. Returns the last 30 days
// of incidents sorted by started_at DESC.
func (h *PublicStatusHandler) Incidents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	since := time.Now().Add(-30 * 24 * time.Hour)

	rows, err := h.pool.Query(ctx, `
		SELECT id, service_key, started_at, ended_at, severity, title,
		       COALESCE(summary, ''), COALESCE(related, ARRAY[]::TEXT[])
		  FROM status_incidents
		 WHERE started_at >= $1
		 ORDER BY started_at DESC
		 LIMIT 200
	`, since)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query incidents", err))
		return
	}
	defer rows.Close()

	out := []PublicStatusIncident{}
	for rows.Next() {
		var inc PublicStatusIncident
		if err := rows.Scan(
			&inc.ID, &inc.ServiceKey, &inc.StartedAt, &inc.EndedAt,
			&inc.Severity, &inc.Title, &inc.Summary, &inc.Related,
		); err != nil {
			response.Error(w, r, apperr.Internal("scan incident row", err))
			return
		}
		out = append(out, inc)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("iterate incident rows", err))
		return
	}

	w.Header().Set("Cache-Control", publicStatusCacheControl)
	response.JSON(w, r, http.StatusOK, PublicStatusIncidentsResponse{Incidents: out})
}

// ── Loaders (split for testability) ───────────────────────────────────────────

// loadServices returns one PublicStatusService per non-node service_key —
// any key that doesn't start with "node:" is treated as a service tile.
//
// CurrentStatus = latest 5min bucket (within the last 30min — anything older
// counts as "no recent data" → outage).
// UptimePercent = most recent status_uptime_daily row.
// History = last 90 days of status_uptime_daily, oldest-first.
func (h *PublicStatusHandler) loadServices(ctx context.Context) ([]PublicStatusService, error) {
	// Current status from the most recent 5min bucket per service_key.
	currentRows, err := h.pool.Query(ctx, `
		WITH latest AS (
		  SELECT DISTINCT ON (service_key)
		         service_key, status, bucket_at
		    FROM status_uptime_5min
		   WHERE service_key NOT LIKE 'node:%'
		     AND bucket_at >= NOW() - INTERVAL '30 minutes'
		   ORDER BY service_key, bucket_at DESC
		)
		SELECT service_key, status FROM latest
	`)
	if err != nil {
		return nil, err
	}
	defer currentRows.Close()

	currentByKey := map[string]int16{}
	for currentRows.Next() {
		var key string
		var status int16
		if err := currentRows.Scan(&key, &status); err != nil {
			return nil, err
		}
		currentByKey[key] = status
	}

	// Daily history per service_key, last 90 days oldest-first.
	historyRows, err := h.pool.Query(ctx, `
		SELECT service_key, day, uptime_pct, worst_status, incident_ids
		  FROM status_uptime_daily
		 WHERE service_key NOT LIKE 'node:%'
		   AND day >= CURRENT_DATE - INTERVAL '89 days'
		 ORDER BY service_key, day ASC
	`)
	if err != nil {
		return nil, err
	}
	defer historyRows.Close()

	type tmp struct {
		bars   []PublicStatusDailyBar
		latest float64
	}
	byKey := map[string]*tmp{}

	for historyRows.Next() {
		var (
			key       string
			day       time.Time
			uptimePct float64
			worst     int16
			incidents []int64
		)
		if err := historyRows.Scan(&key, &day, &uptimePct, &worst, &incidents); err != nil {
			return nil, err
		}
		entry := byKey[key]
		if entry == nil {
			entry = &tmp{}
			byKey[key] = entry
		}
		entry.bars = append(entry.bars, PublicStatusDailyBar{
			Day:         day.Format("2006-01-02"),
			Status:      statusFromInt(worst),
			UptimePct:   uptimePct,
			IncidentIDs: incidents,
		})
		entry.latest = uptimePct
	}

	// Ensure services with no daily-history rows yet still appear (e.g.
	// fresh deploy where collector hasn't done its first rollup). We
	// union the keys we saw in either query.
	keys := map[string]struct{}{}
	for k := range currentByKey {
		keys[k] = struct{}{}
	}
	for k := range byKey {
		keys[k] = struct{}{}
	}

	out := make([]PublicStatusService, 0, len(keys))
	for key := range keys {
		entry := byKey[key]
		var bars []PublicStatusDailyBar
		var latestUptime float64 = 100
		if entry != nil {
			bars = entry.bars
			latestUptime = entry.latest
		}
		curStatusInt, ok := currentByKey[key]
		if !ok {
			curStatusInt = 3 // outage when there's no recent ping
		}
		out = append(out, PublicStatusService{
			Key:           key,
			CurrentStatus: statusFromInt(curStatusInt),
			UptimePercent: latestUptime,
			History:       bars,
		})
	}
	// Stable order for the frontend — alphabetical by key keeps the layout
	// deterministic across requests.
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// loadNodes returns node fleet rows from the most recent 5min bucket
// (within the last 30min — older = treat as outage / unknown), grouped by
// country_code. Each row's detail JSONB contains country_code / city / ip /
// last_seen_age_s populated by the status collector.
func (h *PublicStatusHandler) loadNodes(ctx context.Context) ([]PublicStatusNodeCountryGroup, error) {
	rows, err := h.pool.Query(ctx, `
		WITH latest AS (
		  SELECT DISTINCT ON (service_key)
		         service_key, status, detail, bucket_at
		    FROM status_uptime_5min
		   WHERE service_key LIKE 'node:%'
		     AND bucket_at >= NOW() - INTERVAL '30 minutes'
		   ORDER BY service_key, bucket_at DESC
		)
		SELECT service_key, status, detail FROM latest
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type nodeDetail struct {
		CountryCode  string `json:"country_code"`
		City         string `json:"city"`
		IP           string `json:"ip"`
		LastSeenAgeS int64  `json:"last_seen_age_s"`
	}

	byCountry := map[string]*PublicStatusNodeCountryGroup{}
	for rows.Next() {
		var (
			serviceKey string
			status     int16
			detailRaw  []byte
		)
		if err := rows.Scan(&serviceKey, &status, &detailRaw); err != nil {
			return nil, err
		}
		var d nodeDetail
		_ = json.Unmarshal(detailRaw, &d) // tolerate missing/old shapes

		nodeID := strings.TrimPrefix(serviceKey, "node:")
		country := d.CountryCode
		if country == "" {
			country = "XX" // unknown bucket
		}

		grp := byCountry[country]
		if grp == nil {
			grp = &PublicStatusNodeCountryGroup{CountryCode: country}
			byCountry[country] = grp
		}
		grp.TotalCount++
		if status == 1 {
			grp.OnlineCount++
		}
		grp.Nodes = append(grp.Nodes, PublicStatusNode{
			NodeID:       nodeID,
			City:         d.City,
			IP:           d.IP,
			Status:       statusFromInt(status),
			LastSeenAgeS: d.LastSeenAgeS,
		})
	}

	out := make([]PublicStatusNodeCountryGroup, 0, len(byCountry))
	for _, g := range byCountry {
		// Sort nodes inside group by node_id for stable rendering.
		sort.Slice(g.Nodes, func(i, j int) bool { return g.Nodes[i].NodeID < g.Nodes[j].NodeID })
		out = append(out, *g)
	}
	// Alphabetical by country code keeps the page layout stable.
	sort.Slice(out, func(i, j int) bool { return out[i].CountryCode < out[j].CountryCode })
	return out, nil
}

// computeOverall picks the worst current status across all services + the
// node fleet, using the standard "any outage → outage; any degraded →
// degraded; else operational" rollup. Maintenance is treated as degraded
// for the overall banner (the dedicated status string is preserved at the
// per-service level).
func computeOverall(services []PublicStatusService, groups []PublicStatusNodeCountryGroup) string {
	hasDegraded := false
	for _, s := range services {
		switch s.CurrentStatus {
		case statusStrOutage:
			return statusStrOutage
		case statusStrDegraded, statusStrMaintenance:
			hasDegraded = true
		}
	}
	// Node fleet: a country with 0 online nodes when it had any registered
	// = outage; partial online = degraded.
	for _, g := range groups {
		if g.TotalCount == 0 {
			continue
		}
		if g.OnlineCount == 0 {
			return statusStrOutage
		}
		if g.OnlineCount < g.TotalCount {
			hasDegraded = true
		}
	}
	if hasDegraded {
		return statusStrDegraded
	}
	return statusStrOperational
}
