package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/gateway/internal/hub"
	"github.com/kite365/idcd/lib/shared/logger"
)

func newTestHub() *hub.Hub {
	return hub.New(30*time.Second, logger.Discard())
}

func TestSelectByPercent_Empty(t *testing.T) {
	result := selectByPercent(nil, 50)
	assert.Empty(t, result)
}

func TestSelectByPercent_100Pct(t *testing.T) {
	ids := []string{"n1", "n2", "n3", "n4", "n5"}
	result := selectByPercent(ids, 100)
	assert.Len(t, result, 5)
}

func TestSelectByPercent_1Pct(t *testing.T) {
	ids := make([]string, 200)
	for i := range ids {
		ids[i] = "node"
	}
	result := selectByPercent(ids, 1)
	assert.Equal(t, 2, len(result)) // ceil(200 * 1 / 100) = 2
}

func TestSelectByPercent_10Pct(t *testing.T) {
	ids := make([]string, 100)
	for i := range ids {
		ids[i] = "node"
	}
	result := selectByPercent(ids, 10)
	assert.Equal(t, 10, len(result))
}

func TestBroadcastUpgrade_MissingFields(t *testing.T) {
	h := NewUpgradeHandler(newTestHub())
	body := []byte(`{"version":"v1.0.0"}`)
	req := httptest.NewRequest(http.MethodPost, "/internal/broadcast-upgrade", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.BroadcastUpgrade(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestBroadcastUpgrade_NoConnections(t *testing.T) {
	h := NewUpgradeHandler(newTestHub())
	payload := broadcastUpgradeRequest{
		Version:     "v1.2.0",
		DownloadURL: "https://releases.idcd.com/agent",
		Checksum:    "sha256:abc",
		RolloutPct:  10,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/internal/broadcast-upgrade", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.BroadcastUpgrade(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp broadcastUpgradeResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, 0, resp.Dispatched)
	assert.Equal(t, 0, resp.Total)
}
