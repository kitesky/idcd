// Package handler implements HTTP handlers for the API server.
package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/netutil"
)

// statusPageDomainQuerier is the subset of DB operations required by StatusPageDomainHandler.
type statusPageDomainQuerier interface {
	GetStatusPageByID(ctx context.Context, id string) (idcdmain.StatusPage, error)
	GetStatusPageByCustomDomain(ctx context.Context, customDomain string) (idcdmain.StatusPage, error)
	SetStatusPageCustomDomain(ctx context.Context, arg idcdmain.SetStatusPageCustomDomainParams) (idcdmain.StatusPage, error)
	MarkCustomDomainVerified(ctx context.Context, id string) error
}

// domainValidator is a function type for DNS CNAME lookups (injectable for testing).
type domainValidator func(domain string) (string, error)

// StatusPageDomainHandler handles custom domain binding and verification for status pages.
type StatusPageDomainHandler struct {
	q             statusPageDomainQuerier
	logger        *slog.Logger
	lookupCNAME   domainValidator
}

// NewStatusPageDomainHandler creates a StatusPageDomainHandler wired to the given querier.
func NewStatusPageDomainHandler(q statusPageDomainQuerier, logger *slog.Logger) *StatusPageDomainHandler {
	return &StatusPageDomainHandler{
		q:           q,
		logger:      logger,
		lookupCNAME: net.LookupCNAME,
	}
}

// withLookupCNAME returns a copy of the handler with a custom CNAME lookup function.
// Used for testing to inject a mock DNS resolver.
func (h *StatusPageDomainHandler) withLookupCNAME(fn domainValidator) *StatusPageDomainHandler {
	return &StatusPageDomainHandler{
		q:           h.q,
		logger:      h.logger,
		lookupCNAME: fn,
	}
}

// validDomainRE matches a valid public hostname.
// Allows labels of letters, digits, hyphens; at least two labels (i.e. has a dot).
var validDomainRE = regexp.MustCompile(
	`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`,
)

// idcdDomainSuffixes lists domains that users must not claim (prevents subdomain takeover).
var idcdDomainSuffixes = []string{
	".idcd.com",
	".status.idcd.com",
}

// isValidCustomDomain reports whether host is an acceptable custom domain.
// Returns a non-empty reason string when the domain is rejected.
func isValidCustomDomain(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return "custom_domain cannot be empty"
	}
	if !validDomainRE.MatchString(host) {
		return "invalid domain format"
	}
	for _, suffix := range idcdDomainSuffixes {
		if strings.HasSuffix(host, suffix) || host == strings.TrimPrefix(suffix, ".") {
			return "cannot use idcd.com subdomain as custom domain"
		}
	}
	return ""
}

// isPrivateDomain resolves the domain and returns true if any resolved IP is private.
// This prevents SSRF via custom domain binding.
func isPrivateDomain(domain string) bool {
	addrs, err := net.LookupHost(domain)
	if err != nil {
		// If DNS fails we treat it as safe (validation will catch the CNAME mismatch later).
		return false
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip != nil && netutil.IsPrivateIP(ip) {
			return true
		}
	}
	return false
}

// --- Request/Response types ---

// SetDomainRequest is the body for PATCH /v1/status-pages/{id}/domain.
type SetDomainRequest struct {
	CustomDomain string `json:"custom_domain"`
}

// DomainResponse is the JSON response for domain endpoints.
type DomainResponse struct {
	CustomDomain string `json:"custom_domain"`
	Verified     bool   `json:"verified"`
	Instructions string `json:"instructions,omitempty"`
}

// VerifyDomainResponse is the JSON response for the verify endpoint.
type VerifyDomainResponse struct {
	Verified bool   `json:"verified"`
	Error    string `json:"error,omitempty"`
}

// --- Handlers ---

// SetStatusPageDomain handles PATCH /v1/status-pages/{id}/domain.
func (h *StatusPageDomainHandler) SetStatusPageDomain(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	id := chi.URLParam(r, "id")

	var req SetDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid JSON request body", ""))
		return
	}

	// Fetch the status page and verify ownership.
	sp, err := h.q.GetStatusPageByID(ctx, id)
	if err != nil {
		response.Error(w, r, apperr.NotFound("status page not found"))
		return
	}
	if sp.UserID != userID {
		response.Error(w, r, apperr.NotFound("status page not found"))
		return
	}

	// Unbind: empty string clears the domain.
	if req.CustomDomain == "" {
		updated, err := h.q.SetStatusPageCustomDomain(ctx, idcdmain.SetStatusPageCustomDomainParams{
			ID:           id,
			CustomDomain: nil,
			UserID:       userID,
		})
		if err != nil {
			response.Error(w, r, apperr.Internal("failed to clear custom domain", err))
			return
		}
		_ = updated
		response.JSON(w, r, http.StatusOK, DomainResponse{
			CustomDomain: "",
			Verified:     false,
		})
		return
	}

	// Validate domain format.
	if reason := isValidCustomDomain(req.CustomDomain); reason != "" {
		response.Error(w, r, apperr.Validation(reason, "custom_domain"))
		return
	}

	// SSRF: reject domains that resolve to private IPs.
	if isPrivateDomain(req.CustomDomain) {
		response.Error(w, r, apperr.Validation("domain resolves to a private IP address", "custom_domain"))
		return
	}

	domain := strings.ToLower(strings.TrimSpace(req.CustomDomain))
	updated, err := h.q.SetStatusPageCustomDomain(ctx, idcdmain.SetStatusPageCustomDomainParams{
		ID:           id,
		CustomDomain: &domain,
		UserID:       userID,
	})
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to set custom domain", err))
		return
	}

	// Kick off an async background DNS verification (best-effort).
	go h.asyncVerifyDNS(updated.ID, domain)

	instructions := "请添加 CNAME 记录: " + domain + " → status.idcd.com"
	response.JSON(w, r, http.StatusOK, DomainResponse{
		CustomDomain: domain,
		Verified:     updated.CustomDomainVerifiedAt.Valid,
		Instructions: instructions,
	})
}

// VerifyStatusPageDomain handles GET /v1/status-pages/{id}/domain/verify.
func (h *StatusPageDomainHandler) VerifyStatusPageDomain(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	id := chi.URLParam(r, "id")

	sp, err := h.q.GetStatusPageByID(ctx, id)
	if err != nil {
		response.Error(w, r, apperr.NotFound("status page not found"))
		return
	}
	if sp.UserID != userID {
		response.Error(w, r, apperr.NotFound("status page not found"))
		return
	}
	if sp.CustomDomain == nil || *sp.CustomDomain == "" {
		response.Error(w, r, apperr.Validation("no custom domain configured", "custom_domain"))
		return
	}

	domain := *sp.CustomDomain

	// DNS CNAME lookup.
	cname, err := h.lookupCNAME(domain)
	if err != nil || !strings.Contains(strings.ToLower(cname), "status.idcd.com.") {
		errMsg := "CNAME 未正确配置"
		if err != nil {
			errMsg = "DNS 查询失败: " + err.Error()
		}
		response.JSON(w, r, http.StatusOK, VerifyDomainResponse{
			Verified: false,
			Error:    errMsg,
		})
		return
	}

	// Mark as verified.
	if markErr := h.q.MarkCustomDomainVerified(ctx, id); markErr != nil {
		if h.logger != nil {
			h.logger.Error("failed to mark custom domain verified", "id", id, "error", markErr)
		}
		response.Error(w, r, apperr.Internal("failed to mark domain verified", markErr))
		return
	}

	response.JSON(w, r, http.StatusOK, VerifyDomainResponse{Verified: true})
}

// asyncVerifyDNS attempts a background DNS check and marks the domain verified if it passes.
// Errors are logged and silently ignored — the user can trigger manual verification.
func (h *StatusPageDomainHandler) asyncVerifyDNS(statusPageID, domain string) {
	cname, err := h.lookupCNAME(domain)
	if err != nil {
		return
	}
	if !strings.Contains(strings.ToLower(cname), "status.idcd.com.") {
		return
	}
	if markErr := h.q.MarkCustomDomainVerified(context.Background(), statusPageID); markErr != nil {
		if h.logger != nil {
			h.logger.Error("asyncVerifyDNS: failed to mark verified", "id", statusPageID, "error", markErr)
		}
	}
}
