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

// newLeaderboardTestHandlerWithMock creates a LeaderboardHandler backed by a pgxmock pool.
func newLeaderboardTestHandlerWithMock(t *testing.T) (*LeaderboardHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	h := &LeaderboardHandler{pool: mockPool}
	return h, mockPool
}

// TestCdnRanking_NilPool verifies that pool=nil returns an empty list with HTTP 200.
func TestCdnRanking_NilPool(t *testing.T) {
	h := NewLeaderboardHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboard/cdn", nil)
	ctx := context.WithValue(req.Context(), "request_id", "test-leaderboard-nil")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.CdnRanking(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var envelope struct {
		Data struct {
			Data        []CdnEntry `json:"data"`
			Month       string     `json:"month"`
			SampleCount int        `json:"sample_count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data.Data, "data should not be nil")
	assert.Empty(t, envelope.Data.Data, "expected empty data when pool is nil")
	assert.Equal(t, 0, envelope.Data.SampleCount)
	assert.NotEmpty(t, envelope.Data.Month, "month should be set")
}

// TestCdnRanking_WithData verifies correct ranking when the pool returns CDN rows.
func TestCdnRanking_WithData(t *testing.T) {
	h, mockPool := newLeaderboardTestHandlerWithMock(t)
	defer mockPool.Close()

	cols := []string{"name", "p50"}
	mockPool.ExpectQuery(`SELECT`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows(cols).
			AddRow("Cloudflare CDN", float64(42.5)).
			AddRow("Fastly CDN", float64(68.1)).
			AddRow("Akamai CDN", float64(91.3)))

	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboard/cdn?month=2026-05", nil)
	ctx := context.WithValue(req.Context(), "request_id", "test-leaderboard-data")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.CdnRanking(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var envelope struct {
		Data struct {
			Data        []CdnEntry `json:"data"`
			Month       string     `json:"month"`
			SampleCount int        `json:"sample_count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))

	assert.Equal(t, "2026-05", envelope.Data.Month)
	assert.Equal(t, 3, envelope.Data.SampleCount)
	require.Len(t, envelope.Data.Data, 3)

	// Verify rank ordering and data integrity
	assert.Equal(t, 1, envelope.Data.Data[0].Rank)
	assert.Equal(t, "Cloudflare CDN", envelope.Data.Data[0].Name)
	assert.InDelta(t, 42.5, envelope.Data.Data[0].GlobalP50, 0.01)

	assert.Equal(t, 2, envelope.Data.Data[1].Rank)
	assert.Equal(t, "Fastly CDN", envelope.Data.Data[1].Name)

	assert.Equal(t, 3, envelope.Data.Data[2].Rank)
	assert.Equal(t, "Akamai CDN", envelope.Data.Data[2].Name)

	// Trend and Change should be empty/zero for first release
	assert.Empty(t, envelope.Data.Data[0].Trend)
	assert.Equal(t, 0.0, envelope.Data.Data[0].Change)

	assert.NoError(t, mockPool.ExpectationsWereMet())
}

// TestCdnRanking_InvalidMonth verifies that an invalid month returns 400.
func TestCdnRanking_InvalidMonth(t *testing.T) {
	h := NewLeaderboardHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboard/cdn?month=bad-month", nil)
	ctx := context.WithValue(req.Context(), "request_id", "test-leaderboard-badmonth")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.CdnRanking(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
