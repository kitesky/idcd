package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kite365/idcd/apps/api/internal/response"
)

// LeaderboardPool is the minimal DB interface required by LeaderboardHandler.
// *pgxpool.Pool satisfies this interface.
type LeaderboardPool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// cdnPatterns are ILIKE patterns used to identify CDN monitors by name.
var cdnPatterns = []string{
	"%cloudflare%", "%fastly%", "%akamai%", "%cloudfront%",
	"%阿里%", "%alicdn%", "%腾讯%", "%qcloud%", "%华为%",
	"%百度%", "%又拍%", "%七牛%",
}

// CdnEntry represents a single CDN provider's performance entry.
type CdnEntry struct {
	Rank        int       `json:"rank"`
	Name        string    `json:"name"`
	ShortName   string    `json:"short_name"`
	GlobalP50   float64   `json:"global_p50"`
	ChinaP50    float64   `json:"china_p50"`
	OverseasP50 float64   `json:"overseas_p50"`
	Trend       []float64 `json:"trend"`
	Change      float64   `json:"change"`
}

type cdnLeaderboardResponse struct {
	Data        []CdnEntry `json:"data"`
	Month       string     `json:"month"`
	SampleCount int        `json:"sample_count"`
}

// LeaderboardHandler serves the public CDN leaderboard endpoint.
type LeaderboardHandler struct {
	pool LeaderboardPool
}

// NewLeaderboardHandler creates a new LeaderboardHandler.
// pool may be nil, in which case an empty list is returned.
func NewLeaderboardHandler(pool *pgxpool.Pool) *LeaderboardHandler {
	if pool == nil {
		return &LeaderboardHandler{pool: nil}
	}
	return &LeaderboardHandler{pool: pool}
}

// CdnRanking handles GET /v1/leaderboard/cdn?month=YYYY-MM
func (h *LeaderboardHandler) CdnRanking(w http.ResponseWriter, r *http.Request) {
	// Parse optional month query param; default to current month.
	monthStr := r.URL.Query().Get("month")
	var monthStart, monthEnd time.Time
	if monthStr == "" {
		now := time.Now().UTC()
		monthStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	} else {
		t, err := time.Parse("2006-01", monthStr)
		if err != nil {
			response.JSON(w, r, http.StatusBadRequest, map[string]string{
				"error": "invalid month format, expected YYYY-MM",
			})
			return
		}
		monthStart = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	monthEnd = monthStart.AddDate(0, 1, 0)
	monthLabel := monthStart.Format("2006-01")

	emptyResp := cdnLeaderboardResponse{
		Data:        []CdnEntry{},
		Month:       monthLabel,
		SampleCount: 0,
	}

	if h.pool == nil {
		response.JSON(w, r, http.StatusOK, emptyResp)
		return
	}

	ctx := r.Context()

	rows, err := h.pool.Query(ctx, `
		SELECT
			m.name,
			PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY mc.response_ms) AS p50
		FROM monitor_checks mc
		JOIN monitors m ON m.id = mc.monitor_id
		WHERE mc.checked_at >= $1
		  AND mc.checked_at < $2
		  AND m.name ILIKE ANY($3)
		GROUP BY m.name
		ORDER BY p50 ASC
		LIMIT 50
	`, monthStart, monthEnd, cdnPatterns)
	if err != nil {
		// Fallback to empty list on error.
		response.JSON(w, r, http.StatusOK, emptyResp)
		return
	}
	defer rows.Close()

	var entries []CdnEntry
	rank := 1
	for rows.Next() {
		var name string
		var p50 float64
		if err := rows.Scan(&name, &p50); err != nil {
			continue
		}
		entries = append(entries, CdnEntry{
			Rank:        rank,
			Name:        name,
			ShortName:   name,
			GlobalP50:   p50,
			ChinaP50:    0,
			OverseasP50: 0,
			Trend:       []float64{},
			Change:      0.0,
		})
		rank++
	}
	if err := rows.Err(); err != nil {
		response.JSON(w, r, http.StatusOK, emptyResp)
		return
	}

	if entries == nil {
		entries = []CdnEntry{}
	}

	response.JSON(w, r, http.StatusOK, cdnLeaderboardResponse{
		Data:        entries,
		Month:       monthLabel,
		SampleCount: len(entries),
	})
}
