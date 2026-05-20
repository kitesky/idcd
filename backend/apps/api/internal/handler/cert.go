package handler

import (
	"net/http"

	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// CertHandler exposes a minimal contract that the frontend cert module
// (apps/web/src/app/app/cert/*) requires to render without throwing. The
// signing pipeline (ACME issuance, real DNS-01 challenges, etc.) is not
// wired here — list endpoints return empty arrays, and mutations return
// 501 so we don't pretend to succeed.
type CertHandler struct{}

// NewCertHandler returns a handler with no dependencies.
func NewCertHandler() *CertHandler {
	return &CertHandler{}
}

// notImplemented uses chi's plain http.Error so we don't have to add a new
// apperr code just for this stub layer. The frontend translates HTTP_501 →
// CERT_NOT_IMPL and surfaces it as a localized message.
func notImplemented(w http.ResponseWriter, _ *http.Request, what string) {
	http.Error(w, "{\"error\":{\"code\":\"NOT_IMPLEMENTED\",\"message\":\""+what+" not yet wired\"}}", http.StatusNotImplemented)
	w.Header().Set("Content-Type", "application/json")
}

// ListOrders returns the user's pending/issued cert orders.
func (h *CertHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, r, http.StatusOK, map[string]any{"orders": []any{}})
}

// GetOrder returns a single order detail.
func (h *CertHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	response.Error(w, r, apperr.NotFound("cert order not found"))
}

// CreateOrder enqueues a new cert issuance.
func (h *CertHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, r, "cert issuance")
}

// RetryOrder retries a failed order.
func (h *CertHandler) RetryOrder(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, r, "cert retry")
}

// ManualReady signals that the user finished the manual DNS-01 step.
func (h *CertHandler) ManualReady(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, r, "manual ready")
}

// ListCerts returns issued certificates.
func (h *CertHandler) ListCerts(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, r, http.StatusOK, map[string]any{"certs": []any{}})
}

// GetCert returns a single cert detail.
func (h *CertHandler) GetCert(w http.ResponseWriter, r *http.Request) {
	response.Error(w, r, apperr.NotFound("cert not found"))
}

// DownloadCert returns the cert bundle.
func (h *CertHandler) DownloadCert(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, r, "cert download")
}

// RevokeCert revokes the cert.
func (h *CertHandler) RevokeCert(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, r, "cert revoke")
}

// ListDnsCredentials returns DNS provider credentials.
func (h *CertHandler) ListDnsCredentials(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, r, http.StatusOK, map[string]any{"dns_credentials": []any{}})
}

// CreateDnsCredential creates a new DNS provider credential.
func (h *CertHandler) CreateDnsCredential(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, r, "dns credential create")
}

// DeleteDnsCredential removes a DNS provider credential.
func (h *CertHandler) DeleteDnsCredential(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, r, "dns credential delete")
}

// HealthCheckDnsCredential validates a credential against its provider.
func (h *CertHandler) HealthCheckDnsCredential(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, r, "dns credential health check")
}

// Quota returns the user's cert quota usage.
func (h *CertHandler) Quota(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, r, http.StatusOK, map[string]any{
		"used":                 0,
		"limit":                0,
		"expiring_soon":        0,
		"monthly_success_rate": 0,
	})
}
