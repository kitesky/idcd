package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// VerdictReportHandler handles GET /v1/verdict/reports/{id}.
//
// The report row lives in idcd_attest.verdict_report; ownership is enforced by
// JOIN against idcd_attest.verdict_order.owner_id. D1 forbids cross-schema FK
// but cross-schema READ is fine — the join lives in app code in the sense
// that it never declares a REFERENCES constraint.
//
// Authorization model: when a report exists but the caller is not the owner
// of the underlying order, we return 404 (not 403) so the caller cannot
// enumerate report ids that belong to other tenants.
type VerdictReportHandler struct {
	pool BillingPool
}

// NewVerdictReportHandler wires a VerdictReportHandler.
func NewVerdictReportHandler(pool BillingPool) *VerdictReportHandler {
	return &VerdictReportHandler{pool: pool}
}

// GetVerdictReportResponse is the user-facing projection of one verdict_report
// row, plus the parent order_id (already a column on the report). Fields are
// the subset declared in docs/prd/18 §5 that callers / the report-detail UI
// actually need; bytes-only columns (signature, tsa_response_blob,
// blockchain_anchor) and bookkeeping columns (pdf_size_bytes, llm_*,
// confidence_label, self_verify_at, signature_key_version) are intentionally
// omitted — the public verify endpoint is the channel for crypto material.
type GetVerdictReportResponse struct {
	ID                 string    `json:"id"`
	OrderID            string    `json:"order_id"`
	PDFURL             string    `json:"pdf_url"`
	ContentHash        string    `json:"content_hash"`
	SignatureKeyID     string    `json:"signature_key_id"`
	TSAProvider        string    `json:"tsa_provider"`
	TSATime            time.Time `json:"tsa_time"`
	NodesUsed          []string  `json:"nodes_used"`
	NodeConsistencyPct *float64  `json:"node_consistency_pct,omitempty"`
	SelfVerifyStatus   *string   `json:"self_verify_status,omitempty"`
	ReportType         string    `json:"report_type"`
	ArchivedURL        *string   `json:"archived_url,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

// Get handles GET /v1/verdict/reports/{id}.
func (h *VerdictReportHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, r, apperr.Validation("missing id", "id"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	rows, err := h.pool.Query(ctx, `
		SELECT vr.id, vr.order_id, vr.pdf_url, vr.content_hash,
		       vr.signature_key_id, vr.tsa_provider, vr.tsa_time,
		       vr.nodes_used, vr.node_consistency_pct,
		       vr.self_verify_status, vr.report_type,
		       vr.archived_url, vr.created_at,
		       vo.owner_id
		FROM idcd_attest.verdict_report vr
		JOIN idcd_attest.verdict_order  vo ON vo.id = vr.order_id
		WHERE vr.id = $1
	`, id)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query verdict_report", err))
		return
	}
	defer rows.Close()

	if !rows.Next() {
		response.Error(w, r, apperr.NotFound("verdict_report not found"))
		return
	}

	var (
		resp         GetVerdictReportResponse
		nodesUsedRaw []byte
		ownerID      string
	)
	if err := rows.Scan(
		&resp.ID, &resp.OrderID, &resp.PDFURL, &resp.ContentHash,
		&resp.SignatureKeyID, &resp.TSAProvider, &resp.TSATime,
		&nodesUsedRaw, &resp.NodeConsistencyPct,
		&resp.SelfVerifyStatus, &resp.ReportType,
		&resp.ArchivedURL, &resp.CreatedAt,
		&ownerID,
	); err != nil {
		response.Error(w, r, apperr.Internal("failed to scan verdict_report", err))
		return
	}
	rows.Close()

	if ownerID != userID {
		response.Error(w, r, apperr.NotFound("verdict_report not found"))
		return
	}

	if len(nodesUsedRaw) > 0 {
		if err := json.Unmarshal(nodesUsedRaw, &resp.NodesUsed); err != nil {
			response.Error(w, r, apperr.Internal("failed to decode nodes_used", err))
			return
		}
	}
	if resp.NodesUsed == nil {
		resp.NodesUsed = []string{}
	}

	response.JSON(w, r, http.StatusOK, resp)
}
