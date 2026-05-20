package handler

import (
	"context"
	"encoding/json"
	"net"
	"net/http"

	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

const adminBlocklistKey = "idcd:ip:blocklist"

// BlocklistStore is the Redis interface used by AdminBlocklistHandler.
type BlocklistStore interface {
	SAdd(ctx context.Context, key string, members ...string) error
	SRem(ctx context.Context, key string, members ...string) error
}

// AdminBlocklistHandler handles the IP blocklist admin API.
type AdminBlocklistHandler struct {
	store BlocklistStore
}

// NewAdminBlocklistHandler creates a new AdminBlocklistHandler.
func NewAdminBlocklistHandler(store BlocklistStore) *AdminBlocklistHandler {
	return &AdminBlocklistHandler{store: store}
}

// blockIPRequest is the JSON body for BlockIP and UnblockIP.
type blockIPRequest struct {
	IP string `json:"ip"`
}

// BlockIP handles POST /internal/admin/block-ip.
// Body: {"ip":"1.2.3.4"}
// Adds the IP to the Redis blocklist set.
func (h *AdminBlocklistHandler) BlockIP(w http.ResponseWriter, r *http.Request) {
	var req blockIPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.IP == "" {
		response.Error(w, r, apperr.Validation("ip is required", ""))
		return
	}
	if net.ParseIP(req.IP) == nil {
		response.Error(w, r, apperr.Validation("invalid IP address format", req.IP))
		return
	}
	if err := h.store.SAdd(r.Context(), adminBlocklistKey, req.IP); err != nil {
		response.Error(w, r, apperr.Internal("failed to block IP", err))
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]string{"ip": req.IP, "status": "blocked"})
}

// UnblockIP handles DELETE /internal/admin/block-ip.
// Body: {"ip":"1.2.3.4"}
// Removes the IP from the Redis blocklist set.
func (h *AdminBlocklistHandler) UnblockIP(w http.ResponseWriter, r *http.Request) {
	var req blockIPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.IP == "" {
		response.Error(w, r, apperr.Validation("ip is required", ""))
		return
	}
	if net.ParseIP(req.IP) == nil {
		response.Error(w, r, apperr.Validation("invalid IP address format", req.IP))
		return
	}
	if err := h.store.SRem(r.Context(), adminBlocklistKey, req.IP); err != nil {
		response.Error(w, r, apperr.Internal("failed to unblock IP", err))
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]string{"ip": req.IP, "status": "unblocked"})
}
