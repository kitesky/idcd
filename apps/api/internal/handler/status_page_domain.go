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
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/netutil"
)

// statusPageCNAMETarget is the canonical CNAME value users must point their domain to.
const statusPageCNAMETarget = "status.idcd.com."

// asyncVerifySem caps concurrent background DNS-verify goroutines to prevent
// goroutine accumulation under burst traffic.
var asyncVerifySem = make(chan struct{}, 20)

// statusPageDomainQuerier is the subset of DB operations required by StatusPageDomainHandler.
type statusPageDomainQuerier interface {
	GetStatusPageByID(ctx context.Context, id string) (idcdmain.StatusPage, error)
	GetStatusPageByCustomDomain(ctx context.Context, customDomain string) (idcdmain.StatusPage, error)
	SetStatusPageCustomDomain(ctx context.Context, arg idcdmain.SetStatusPageCustomDomainParams) (idcdmain.StatusPage, error)
	MarkCustomDomainVerified(ctx context.Context, id string) error
}

// domainValidator is a function type for DNS CNAME lookups (injectable for testing).
// The context allows callers to impose a deadline on slow DNS queries.
type domainValidator func(ctx context.Context, domain string) (string, error)

// StatusPageDomainHandler handles custom domain binding and verification for status pages.
type StatusPageDomainHandler struct {
	q             statusPageDomainQuerier
	logger        *slog.Logger
	lookupCNAME   domainValidator
}

// NewStatusPageDomainHandler creates a StatusPageDomainHandler wired to the given querier.
func NewStatusPageDomainHandler(q statusPageDomainQuerier, logger *slog.Logger) *StatusPageDomainHandler {
	return &StatusPageDomainHandler{
		q:      q,
		logger: logger,
		lookupCNAME: func(ctx context.Context, domain string) (string, error) {
			return net.DefaultResolver.LookupCNAME(ctx, domain)
		},
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
	host = netutil.NormalizeDomain(host)
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
// Uses the request context so it respects the handler's deadline rather than blocking indefinitely.
func isPrivateDomain(ctx context.Context, domain string) bool {
	rctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupHost(rctx, domain)
	if err != nil {
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

	if req.CustomDomain == "" {
		if _, err := h.q.SetStatusPageCustomDomain(ctx, idcdmain.SetStatusPageCustomDomainParams{
			ID:           id,
			CustomDomain: nil,
			UserID:       userID,
		}); err != nil {
			response.Error(w, r, apperr.Internal("failed to clear custom domain", err))
			return
		}
		response.JSON(w, r, http.StatusOK, DomainResponse{
			CustomDomain: "",
			Verified:     false,
		})
		return
	}

	if reason := isValidCustomDomain(req.CustomDomain); reason != "" {
		response.Error(w, r, apperr.Validation(reason, "custom_domain"))
		return
	}

	if isPrivateDomain(ctx, req.CustomDomain) {
		response.Error(w, r, apperr.Validation("domain resolves to a private IP address", "custom_domain"))
		return
	}

	domain := netutil.NormalizeDomain(req.CustomDomain)
	updated, err := h.q.SetStatusPageCustomDomain(ctx, idcdmain.SetStatusPageCustomDomainParams{
		ID:           id,
		CustomDomain: &domain,
		UserID:       userID,
	})
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to set custom domain", err))
		return
	}

	go h.asyncVerifyDNS(updated.ID, domain)

	instructions := "Please add a CNAME record: " + domain + " → status.idcd.com"
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

	cname, err := h.lookupCNAME(ctx, domain)
	if err != nil || !strings.Contains(strings.ToLower(cname), statusPageCNAMETarget) {
		errMsg := "CNAME not correctly configured"
		if err != nil {
			errMsg = "DNS lookup failed: " + err.Error()
		}
		response.JSON(w, r, http.StatusOK, VerifyDomainResponse{
			Verified: false,
			Error:    errMsg,
		})
		return
	}

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
// Errors are silently ignored — the user can trigger manual verification via the verify endpoint.
// asyncVerifySem caps concurrency; the 15-second context prevents indefinite blocking.
func (h *StatusPageDomainHandler) asyncVerifyDNS(statusPageID, domain string) {
	select {
	case asyncVerifySem <- struct{}{}:
		defer func() { <-asyncVerifySem }()
	default:
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cname, err := h.lookupCNAME(ctx, domain)
	if err != nil {
		return
	}
	if !strings.Contains(strings.ToLower(cname), statusPageCNAMETarget) {
		return
	}
	if markErr := h.q.MarkCustomDomainVerified(ctx, statusPageID); markErr != nil {
		if h.logger != nil {
			h.logger.Error("asyncVerifyDNS: failed to mark verified", "id", statusPageID, "error", markErr)
		}
	}
}
