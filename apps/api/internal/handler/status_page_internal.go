// Package handler implements HTTP handlers for the API server.
package handler

import (
	"context"
	"net/http"

	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/netutil"
)

// statusPageInternalQuerier is the subset of DB operations required by StatusPageInternalHandler.
type statusPageInternalQuerier interface {
	GetStatusPageByCustomDomain(ctx context.Context, customDomain string) (idcdmain.StatusPage, error)
}

// StatusPageInternalHandler handles internal (non-public) status page lookup endpoints.
// These endpoints are consumed by the status Next.js app and must NOT be exposed to public traffic.
type StatusPageInternalHandler struct {
	q statusPageInternalQuerier
}

// NewStatusPageInternalHandler creates a StatusPageInternalHandler.
func NewStatusPageInternalHandler(q statusPageInternalQuerier) *StatusPageInternalHandler {
	return &StatusPageInternalHandler{q: q}
}

// byDomainResponse is the JSON response for the by-domain lookup.
type byDomainResponse struct {
	Slug string `json:"slug"`
}

// ByDomain handles GET /internal/status-pages/by-domain?domain={domain}.
// Returns the slug for a verified custom domain or 404.
func (h *StatusPageInternalHandler) ByDomain(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	domain := netutil.NormalizeDomain(r.URL.Query().Get("domain"))
	if domain == "" {
		response.Error(w, r, apperr.Validation("domain query parameter is required", "domain"))
		return
	}

	sp, err := h.q.GetStatusPageByCustomDomain(ctx, domain)
	if err != nil {
		response.Error(w, r, apperr.NotFound("status page not found for domain"))
		return
	}

	if !sp.CustomDomainVerifiedAt.Valid {
		response.Error(w, r, apperr.NotFound("custom domain not yet verified"))
		return
	}

	response.JSON(w, r, http.StatusOK, byDomainResponse{Slug: sp.Slug})
}
