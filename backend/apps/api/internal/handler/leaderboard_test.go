package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newLeaderboardTestHandler(t *testing.T) (*LeaderboardHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	h := NewLeaderboardHandler(mockPool)
	return h, mockPool
}

func TestLeaderboardHandler_CDNLeaderboard_Empty(t *testing.T) {
	h, mockPool := newLeaderboardTestHandler(t)
	defer mockPool.Close()

	rows := pgxmock.NewRows([]string{
		"id", "name", "target", "check_count",
		"avg_latency_ms", "p50_latency_ms", "p95_latency_ms", "uptime_pct",
	})
	mockPool.ExpectQuery("SELECT").WithArgs(24, systemUserID).WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboard/cdn", nil)
	ctx := context.WithValue(req.Context(), "request_id", "test-req-lb-empty")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.CDNLeaderboard(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Data LeaderboardResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Data.Total)
	assert.Empty(t, resp.Data.Entries)
	assert.Equal(t, 24, resp.Data.WindowHours)
	assert.NotEmpty(t, resp.Data.GeneratedAt)

	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestLeaderboardHandler_CDNLeaderboard_WithData(t *testing.T) {
	h, mockPool := newLeaderboardTestHandler(t)
	defer mockPool.Close()

	rows := pgxmock.NewRows([]string{
		"id", "name", "target", "check_count",
		"avg_latency_ms", "p50_latency_ms", "p95_latency_ms", "uptime_pct",
	}).
		AddRow("cdn_cloudflare", "Cloudflare CDN", "https://www.cloudflare.com", int64(120), 45.2, 42.0, 80.0, 99.8).
		AddRow("cdn_fastly", "Fastly CDN", "https://www.fastly.com", int64(115), 52.7, 50.0, 95.0, 99.5)

	mockPool.ExpectQuery("SELECT").WithArgs(24, systemUserID).WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboard/cdn", nil)
	ctx := context.WithValue(req.Context(), "request_id", "test-req-lb-data")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.CDNLeaderboard(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Data LeaderboardResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Data.Total)
	assert.Len(t, resp.Data.Entries, 2)

	first := resp.Data.Entries[0]
	assert.Equal(t, 1, first.Rank)
	assert.Equal(t, "cdn_cloudflare", first.MonitorID)
	assert.Equal(t, "Cloudflare CDN", first.Name)
	assert.InDelta(t, 45.2, first.AvgLatency, 0.01)
	assert.InDelta(t, 99.8, first.Uptime, 0.01)

	second := resp.Data.Entries[1]
	assert.Equal(t, 2, second.Rank)
	assert.Equal(t, "cdn_fastly", second.MonitorID)

	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestNewLeaderboardHandler(t *testing.T) {
	h := NewLeaderboardHandler(nil)
	assert.NotNil(t, h)
}
