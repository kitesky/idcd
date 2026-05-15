package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDiagnosticsRouter(h *NodeDiagnosticsHandler) chi.Router {
	r := chi.NewRouter()
	r.Get("/v1/nodes/{id}/diagnostics", h.Diagnostics)
	return r
}

func TestNodeDiagnostics_NotFound(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	mockPool.ExpectQuery(`SELECT id`).
		WithArgs("nod_unknown").
		WillReturnError(errors.New("no rows in result set"))

	h := &NodeDiagnosticsHandler{pool: mockPool}
	router := newDiagnosticsRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/nodes/nod_unknown/diagnostics", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestNodeDiagnostics_StubWhenNilPool(t *testing.T) {
	h := NewNodeDiagnosticsHandler(nil)
	router := newDiagnosticsRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/nodes/jp-tok-ntt-01/diagnostics", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var wrapper struct {
		Data NodeDiagnosticsResponse `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&wrapper))

	diag := wrapper.Data
	assert.Equal(t, "jp-tok-ntt-01", diag.NodeID)
	assert.NotNil(t, diag.HealthTrend)
	assert.Len(t, diag.HealthTrend, 24)
	assert.NotNil(t, diag.LastSeen)
}

func TestNodeDiagnostics_ResponseFields(t *testing.T) {
	h := NewNodeDiagnosticsHandler(nil)
	router := newDiagnosticsRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/nodes/cn-bj-ct-01/diagnostics", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var wrapper struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&wrapper))

	var fields map[string]any
	require.NoError(t, json.Unmarshal(wrapper.Data, &fields))

	assert.Contains(t, fields, "node_id")
	assert.Contains(t, fields, "latency_distribution")
	assert.Contains(t, fields, "health_trend")
	assert.Contains(t, fields, "location")
	assert.Contains(t, fields, "status")
}

func TestNodeDiagnostics_FoundInDB(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	nodeCols := []string{"id", "country", "city", "asn_org", "asn_org2", "status"}
	mockPool.ExpectQuery(`SELECT id`).
		WithArgs("jp-tok-ntt-01").
		WillReturnRows(pgxmock.NewRows(nodeCols).AddRow(
			"jp-tok-ntt-01", "JP", "Tokyo", "AS2914", "NTT", "active",
		))

	statsCols := []string{"total", "up", "p50", "p90", "p95", "p99", "min", "max"}
	total, up := 120, 118
	p50, p90, p95, p99 := 28.5, 42.1, 55.3, 110.6
	minLat, maxLat := 15.0, 290.0
	mockPool.ExpectQuery(`SELECT`).
		WithArgs("jp-tok-ntt-01").
		WillReturnRows(pgxmock.NewRows(statsCols).AddRow(
			&total, &up, &p50, &p90, &p95, &p99, &minLat, &maxLat,
		))

	mockPool.ExpectQuery(`SELECT`).
		WithArgs("jp-tok-ntt-01").
		WillReturnError(errors.New("no data"))

	h := &NodeDiagnosticsHandler{pool: mockPool}
	router := newDiagnosticsRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/nodes/jp-tok-ntt-01/diagnostics", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var wrapper struct {
		Data NodeDiagnosticsResponse `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&wrapper))

	diag := wrapper.Data
	assert.Equal(t, "jp-tok-ntt-01", diag.NodeID)
	assert.Equal(t, "JP", diag.Location.Country)
	assert.Equal(t, "active", diag.Status)
	assert.InDelta(t, 98.33, diag.Uptime24h, 0.01)
	assert.Equal(t, 120, diag.Checks24h)
	assert.Equal(t, 28.5, diag.LatencyDistribution.P50)
	assert.Equal(t, 110.6, diag.LatencyDistribution.P99)
	assert.NotNil(t, diag.HealthTrend)
}

func TestNodeDiagnosticsHandler_New(t *testing.T) {
	h := NewNodeDiagnosticsHandler(nil)
	assert.NotNil(t, h)
}

func TestStubHealthTrend(t *testing.T) {
	trend := stubHealthTrend()
	assert.Len(t, trend, 24)
	for _, pt := range trend {
		assert.Equal(t, 100.0, pt.SuccessRate)
		assert.Equal(t, 35.0, pt.AvgLatency)
	}
}

func TestLatencyDistribution_JSON(t *testing.T) {
	dist := LatencyDistribution{
		P50: 32.5,
		P90: 45.2,
		P95: 58.1,
		P99: 124.7,
		Min: 18.2,
		Max: 312.5,
	}

	data, err := json.Marshal(dist)
	require.NoError(t, err)

	var decoded LatencyDistribution
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, dist.P50, decoded.P50)
	assert.Equal(t, dist.P99, decoded.P99)
}
