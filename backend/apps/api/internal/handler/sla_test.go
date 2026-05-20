package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/api/internal/middleware"
)

func newSLATestHandler(t *testing.T) (*SLAHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	return NewSLAHandler(mockPool), mockPool
}

func injectSLAUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDContextKey(), userID)
	return r.WithContext(ctx)
}

func TestGetSLA_Unauthenticated(t *testing.T) {
	h, mockPool := newSLATestHandler(t)
	defer mockPool.Close()

	req := httptest.NewRequest(http.MethodGet, "/v1/reports/sla", nil)
	req = withReqID(req, "test-sla-unauth")
	rr := httptest.NewRecorder()

	h.GetSLA(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestGetSLA_EmptyData(t *testing.T) {
	h, mockPool := newSLATestHandler(t)
	defer mockPool.Close()

	uid := "u_test"

	cols := []string{"id", "name", "type", "month", "total_checks", "failed_checks"}
	mockPool.ExpectQuery(`SELECT`).
		WithArgs(uid, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows(cols))

	req := httptest.NewRequest(http.MethodGet, "/v1/reports/sla", nil)
	req = injectSLAUserID(req, uid)
	req = withReqID(req, "test-sla-empty")
	rr := httptest.NewRecorder()

	h.GetSLA(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Data SLAReportResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data.Monitors)
	assert.NotEmpty(t, resp.Data.Period.From)
	assert.NotEmpty(t, resp.Data.Period.To)

	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestGetSLA_WithData(t *testing.T) {
	h, mockPool := newSLATestHandler(t)
	defer mockPool.Close()

	uid := "u_withdata"
	march := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	april := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	cols := []string{"id", "name", "type", "month", "total_checks", "failed_checks"}
	mockPool.ExpectQuery(`SELECT`).
		WithArgs(uid, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows(cols).
			AddRow("mon-001", "API 网关健康检查", "http", march, int64(4320), int64(2)).
			AddRow("mon-001", "API 网关健康检查", "http", april, int64(4320), int64(0)))

	req := httptest.NewRequest(http.MethodGet, "/v1/reports/sla?months=3", nil)
	req = injectSLAUserID(req, uid)
	req = withReqID(req, "test-sla-data")
	rr := httptest.NewRecorder()

	h.GetSLA(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Data SLAReportResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Len(t, resp.Data.Monitors, 1)

	m := resp.Data.Monitors[0]
	assert.Equal(t, "mon-001", m.ID)
	assert.Equal(t, "API 网关健康检查", m.Name)
	assert.Equal(t, "http", m.Type)
	require.Len(t, m.Months, 2)

	assert.Equal(t, "2026-03", m.Months[0].Month)
	assert.Equal(t, int64(4320), m.Months[0].TotalChecks)
	assert.Equal(t, int64(2), m.Months[0].FailedChecks)
	assert.InDelta(t, 99.95, m.Months[0].UptimePct, 0.01)

	assert.Equal(t, "2026-04", m.Months[1].Month)
	assert.Equal(t, int64(0), m.Months[1].FailedChecks)
	assert.Equal(t, 100.0, m.Months[1].UptimePct)

	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestGetSLA_MonthsClamped(t *testing.T) {
	h, mockPool := newSLATestHandler(t)
	defer mockPool.Close()

	uid := "u_clamp"

	cols := []string{"id", "name", "type", "month", "total_checks", "failed_checks"}
	mockPool.ExpectQuery(`SELECT`).
		WithArgs(uid, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows(cols))

	req := httptest.NewRequest(http.MethodGet, "/v1/reports/sla?months=13", nil)
	req = injectSLAUserID(req, uid)
	req = withReqID(req, "test-sla-clamp")
	rr := httptest.NewRecorder()

	h.GetSLA(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestGetSLA_UptimePctCalculation(t *testing.T) {
	tests := []struct {
		name         string
		total        int64
		failed       int64
		expectedPct  float64
	}{
		{"perfect", 4320, 0, 100.0},
		{"two failures in 4320", 4320, 2, 99.95},
		{"six failures in 2160", 2160, 6, 99.72},
		{"all failures", 100, 100, 0.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, mockPool := newSLATestHandler(t)
			defer mockPool.Close()

			uid := "u_calc"
			month := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
			cols := []string{"id", "name", "type", "month", "total_checks", "failed_checks"}
			mockPool.ExpectQuery(`SELECT`).
				WithArgs(uid, pgxmock.AnyArg()).
				WillReturnRows(pgxmock.NewRows(cols).
					AddRow("mon-x", "Test", "http", month, tc.total, tc.failed))

			req := httptest.NewRequest(http.MethodGet, "/v1/reports/sla", nil)
			req = injectSLAUserID(req, uid)
			req = withReqID(req, "test-sla-calc-"+tc.name)
			rr := httptest.NewRecorder()

			h.GetSLA(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp struct {
				Data SLAReportResponse `json:"data"`
			}
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
			require.Len(t, resp.Data.Monitors, 1)
			require.Len(t, resp.Data.Monitors[0].Months, 1)
			assert.InDelta(t, tc.expectedPct, resp.Data.Monitors[0].Months[0].UptimePct, 0.01)

			assert.NoError(t, mockPool.ExpectationsWereMet())
		})
	}
}
