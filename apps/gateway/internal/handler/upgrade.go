package handler

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"time"

	"github.com/kite365/idcd/apps/gateway/internal/hub"
)

// UpgradeHandler handles the internal broadcast-upgrade endpoint.
type UpgradeHandler struct {
	hub *hub.Hub
}

// NewUpgradeHandler creates a new UpgradeHandler.
func NewUpgradeHandler(h *hub.Hub) *UpgradeHandler {
	return &UpgradeHandler{hub: h}
}

type broadcastUpgradeRequest struct {
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	Checksum    string `json:"checksum"`
	RolloutPct  int    `json:"rollout_pct"`
}

type broadcastUpgradeResponse struct {
	Dispatched int `json:"dispatched"`
	Total      int `json:"total"`
}

// BroadcastUpgrade handles POST /internal/broadcast-upgrade.
// It selects rollout_pct% of connected nodes at random and sends them an upgrade message.
func (h *UpgradeHandler) BroadcastUpgrade(w http.ResponseWriter, r *http.Request) {
	var req broadcastUpgradeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Version == "" || req.DownloadURL == "" || req.Checksum == "" {
		http.Error(w, "version, download_url and checksum are required", http.StatusBadRequest)
		return
	}
	pct := req.RolloutPct
	if pct <= 0 {
		pct = 1
	}
	if pct > 100 {
		pct = 100
	}

	allIDs := h.hub.GetAllNodeIDs()
	total := len(allIDs)

	selected := selectByPercent(allIDs, pct)

	payload, _ := json.Marshal(map[string]string{
		"version":      req.Version,
		"download_url": req.DownloadURL,
		"checksum":     req.Checksum,
	})
	msg := Message{Type: MsgTypeUpgrade, Payload: json.RawMessage(payload)}
	msgBytes, _ := json.Marshal(msg)

	dispatched := 0
	for _, nodeID := range selected {
		if h.hub.Broadcast(nodeID, msgBytes) {
			dispatched++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(broadcastUpgradeResponse{Dispatched: dispatched, Total: total}) //nolint:errcheck
}

// selectByPercent selects pct% of ids using a seeded random shuffle.
func selectByPercent(ids []string, pct int) []string {
	if len(ids) == 0 || pct <= 0 {
		return nil
	}
	if pct >= 100 {
		return ids
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec — non-cryptographic selection is intentional
	shuffled := make([]string, len(ids))
	copy(shuffled, ids)
	rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

	n := min((len(shuffled)*pct+99)/100, len(shuffled))
	return shuffled[:n]
}
