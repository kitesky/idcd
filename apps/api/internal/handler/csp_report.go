package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/packages/shared/apperr"
)

// CSPReportHandler handles Content Security Policy violation reports.
type CSPReportHandler struct {
	logger *slog.Logger
}

// NewCSPReportHandler creates a new CSP report handler.
func NewCSPReportHandler(logger *slog.Logger) *CSPReportHandler {
	return &CSPReportHandler{
		logger: logger,
	}
}

// CSPReport represents a CSP violation report from the browser.
// See: https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP#violation_report_syntax
type CSPReport struct {
	CSPReport struct {
		DocumentURI        string `json:"document-uri"`
		Referrer           string `json:"referrer"`
		BlockedURI         string `json:"blocked-uri"`
		ViolatedDirective  string `json:"violated-directive"`
		EffectiveDirective string `json:"effective-directive"`
		OriginalPolicy     string `json:"original-policy"`
		Disposition        string `json:"disposition"`
		StatusCode         int    `json:"status-code"`
		SourceFile         string `json:"source-file"`
		LineNumber         int    `json:"line-number"`
		ColumnNumber       int    `json:"column-number"`
	} `json:"csp-report"`
}

// Report handles POST /v1/csp-report - receives CSP violation reports.
// Logs violations for security monitoring and policy adjustment.
func (h *CSPReportHandler) Report(w http.ResponseWriter, r *http.Request) {
	// Limit body size to prevent abuse
	r.Body = http.MaxBytesReader(w, r.Body, 10*1024) // 10KB max
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("failed to read CSP report body", "error", err)
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}

	var report CSPReport
	if err := json.Unmarshal(body, &report); err != nil {
		h.logger.Error("failed to parse CSP report", "error", err, "body", string(body))
		response.Error(w, r, apperr.Validation("invalid JSON", err.Error()))
		return
	}

	// Log CSP violation for monitoring
	h.logger.Warn("CSP violation reported",
		"document_uri", report.CSPReport.DocumentURI,
		"blocked_uri", report.CSPReport.BlockedURI,
		"violated_directive", report.CSPReport.ViolatedDirective,
		"effective_directive", report.CSPReport.EffectiveDirective,
		"disposition", report.CSPReport.Disposition,
		"source_file", report.CSPReport.SourceFile,
		"line_number", report.CSPReport.LineNumber,
		"column_number", report.CSPReport.ColumnNumber,
		"user_agent", r.Header.Get("User-Agent"),
	)

	// Return 204 No Content - browser expects no response body
	w.WriteHeader(http.StatusNoContent)
}
