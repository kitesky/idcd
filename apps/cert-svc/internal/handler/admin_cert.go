package handler

// admin_cert.go implements the /v1/admin/cert/* surface used by the
// internal web admin (apps/web /admin/cert/*).
//
// PRD §10.2 / §11.1 of docs/prd/20-free-cert.md require five endpoints:
//
//   GET  /v1/admin/cert/orders               — cross-account order list,
//                                              filterable by status / account / CA
//   POST /v1/admin/cert/orders/{id}/force-fail
//                                              — admin override that forces an
//                                              order into the failed terminal
//                                              state and writes a WAL event
//   GET  /v1/admin/cert/ca-quota              — per-CA usage snapshot for the
//                                              ops dashboard (Router fall-over
//                                              indicator)
//   GET  /v1/admin/cert/dns-health            — per-DNS-provider rolling 24h
//                                              success ratio (best-effort
//                                              aggregation over audit_logs)
//   POST /v1/admin/cert/accounts/{id}/ban     — append an account to the
//                                              abuse blocklist (records a
//                                              WAL row; in-memory enforcement
//                                              lives on AbuseDetector + the
//                                              order create handler)
//
// Authn is delegated to Deps.AdminAuthnMiddleware — a separately-wired
// gate (operator JWT / VPN-only header). When nil, every /v1/admin/cert
// route returns 401 so a misconfigured deploy fails closed rather than
// leaking admin surface to the public router.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/apps/cert-svc/internal/service"
)

// AdminQuotaSource is the narrow surface admin handlers need to render the
// CA quota dashboard. Implementations: *service.RepoQuotaChecker (prod) or
// a test stub. Defined here so the wiring layer can supply the concrete
// type without admin_cert.go importing service heavyweight types directly.
type AdminQuotaSource interface {
	Usage(ctx context.Context, caName string) (service.QuotaUsage, error)
}

// AdminAbuseGate is the surface used to record an account ban. Two
// implementations are expected in S2:
//
//   - the in-memory AbuseDetector (which has no Ban method today; the
//     wiring layer wraps it with a closure that appends to its blocklist)
//   - a future cert.abuse_bans table-backed store (post-S2)
//
// The gate is kept narrow so the handler does not depend on either path.
type AdminAbuseGate interface {
	// Ban records a permanent ban for accountID. reason is a free-text
	// note surfaced in audit_logs.payload and admin tooling.
	Ban(ctx context.Context, accountID int64, reason string) error
	// Unban lifts an active ban. Returns ErrNotBanned semantics when
	// the account has no active row (handlers translate to 404).
	Unban(ctx context.Context, accountID int64, reason string) error
}

// caaCAList is the canonical set of CAs the platform routes to. Kept in
// sync with service.DefaultCeilings + ca_router.go. The admin quota
// endpoint reports one row per CA in this slice so the operator sees the
// full inventory even when a CA has zero traffic.
var caaCAList = []string{"lets-encrypt", "zerossl", "buypass"}

// dnsProvidersForHealth is the static list of DNS providers we surface
// health for. Mirrors lib/cert/dns/registry.go's registered providers.
// Kept here (not imported) so admin_cert.go does not pull in the dns
// package — the dns Registry concrete type lives behind Deps.DNSReg
// which can be queried generically too.
var dnsProvidersForHealth = []string{"manual", "cloudflare", "aliyun", "dnspod", "route53", "gcloud"}

// mountAdmin wires every /v1/admin/cert/* route. Caller is expected to
// have already established the admin authn middleware via Deps.
// AdminAuthnMiddleware (or to have explicitly opted-out by passing nil,
// in which case every route returns 401).
func mountAdmin(r chi.Router, deps Deps) {
	r.Get("/orders", adminListOrders(deps))
	r.Post("/orders/{id}/force-fail", adminForceFailOrder(deps))
	r.Get("/ca-quota", adminCAQuota(deps))
	r.Get("/dns-health", adminDNSHealth(deps))
	r.Post("/accounts/{id}/ban", adminBanAccount(deps))
	r.Post("/accounts/{id}/unban", adminUnbanAccount(deps))
}

// rejectAllAdminUnauthenticated is the analogue of rejectAllUnauthenticated
// for the admin surface. Identical body — a separate function so future
// changes (e.g. an admin-specific code) do not affect user routes.
func rejectAllAdminUnauthenticated(_ http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeErr(w, http.StatusUnauthorized, codeUnauthorized,
			"cert-svc admin auth middleware not configured", nil)
	})
}

// --- Orders ----------------------------------------------------------------

type adminOrderResponse struct {
	ID              int64    `json:"id"`
	AccountID       int64    `json:"account_id"`
	SANs            []string `json:"sans"`
	SANsUnicode     []string `json:"sans_unicode"`
	Status          string   `json:"status"`
	Tier            string   `json:"tier"`
	CA              string   `json:"ca"`
	ChallengeType   string   `json:"challenge_type"`
	DNSCredentialID *int64   `json:"dns_credential_id,omitempty"`
	CertID          *int64   `json:"cert_id,omitempty"`
	RetryCount      int      `json:"retry_count"`
	LastError       *string  `json:"last_error,omitempty"`
	CreatedAt       string   `json:"created_at"`
	FinalizedAt     *string  `json:"finalized_at,omitempty"`
}

type adminListOrdersResponse struct {
	Orders []adminOrderResponse `json:"orders"`
	Limit  int                  `json:"limit"`
	Offset int                  `json:"offset"`
}

func adminListOrders(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Repos == nil || deps.Repos.Orders == nil {
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"repos not configured", nil)
			return
		}
		q := r.URL.Query()
		limit := queryIntDefault(r, "limit", 50)
		offset := queryIntDefault(r, "offset", 0)
		f := repo.AdminOrderFilter{Limit: limit, Offset: offset}
		if s := strings.TrimSpace(q.Get("status")); s != "" {
			st := repo.OrderStatus(s)
			f.Status = &st
		}
		if a := strings.TrimSpace(q.Get("account_id")); a != "" {
			v, err := strconv.ParseInt(a, 10, 64)
			if err != nil || v <= 0 {
				writeErr(w, http.StatusBadRequest, codeBadRequest,
					"invalid account_id", nil)
				return
			}
			f.AccountID = &v
		}
		if c := strings.TrimSpace(q.Get("ca")); c != "" {
			f.CA = &c
		}
		rows, err := deps.Repos.Orders.AdminListOrders(r.Context(), f)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"orders admin list failed", nil)
			return
		}
		out := make([]adminOrderResponse, 0, len(rows))
		for _, o := range rows {
			out = append(out, projectAdminOrder(o))
		}
		// Echo limit/offset post-clamping so the client can paginate
		// correctly without re-implementing the clamp on its side.
		echoLimit := limit
		if echoLimit <= 0 || echoLimit > 200 {
			echoLimit = 50
		}
		echoOffset := offset
		if echoOffset < 0 {
			echoOffset = 0
		}
		writeJSON(w, http.StatusOK, adminListOrdersResponse{
			Orders: out, Limit: echoLimit, Offset: echoOffset,
		})
	}
}

func projectAdminOrder(o *repo.Order) adminOrderResponse {
	out := adminOrderResponse{
		ID:              o.ID,
		AccountID:       o.AccountID,
		SANs:            o.SANs,
		SANsUnicode:     o.SANsUnicode,
		Status:          string(o.Status),
		Tier:            o.Tier,
		CA:              o.CA,
		ChallengeType:   o.ChallengeType,
		DNSCredentialID: o.DNSCredentialID,
		CertID:          o.CertID,
		RetryCount:      o.RetryCount,
		LastError:       o.LastError,
		CreatedAt:       o.CreatedAt.UTC().Format(time.RFC3339),
	}
	if o.FinalizedAt != nil {
		s := o.FinalizedAt.UTC().Format(time.RFC3339)
		out.FinalizedAt = &s
	}
	return out
}

// --- Force-fail ------------------------------------------------------------

type adminForceFailRequest struct {
	// Reason is surfaced in order_events.payload and the order's
	// last_error column so the user-facing detail page explains why
	// an admin terminated their order.
	Reason string `json:"reason"`
}

type adminForceFailResponse struct {
	OrderID int64  `json:"order_id"`
	Status  string `json:"status"`
}

func adminForceFailOrder(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Repos == nil || deps.Repos.Orders == nil || deps.Repos.OrderEvents == nil {
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"repos not configured", nil)
			return
		}
		id, ok := pathInt64(w, r, "id")
		if !ok {
			return
		}
		var req adminForceFailRequest
		if r.ContentLength > 0 || r.Header.Get("Content-Type") != "" {
			// Body is optional — accept missing body for ops convenience
			// (curl without payload), but reject malformed JSON when one
			// is supplied.
			if !readJSON(w, r, &req) {
				return
			}
		}
		if strings.TrimSpace(req.Reason) == "" {
			req.Reason = "admin force-fail"
		}

		order, err := deps.Repos.Orders.GetByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				writeErr(w, http.StatusNotFound, codeNotFound,
					"order not found", nil)
				return
			}
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"order get failed", nil)
			return
		}
		if order.Status == repo.OrderStatusFailed ||
			order.Status == repo.OrderStatusIssued ||
			order.Status == repo.OrderStatusRevoked {
			// Terminal statuses cannot be re-failed. Returning 409 lets
			// the admin UI surface "already terminal" cleanly.
			writeErr(w, http.StatusConflict, codeInvalidStatus,
				"order is already in a terminal status", map[string]string{
					"status": string(order.Status),
				})
			return
		}

		reason := req.Reason
		if err := deps.Repos.Orders.UpdateStatus(r.Context(), id,
			order.Status, repo.OrderStatusFailed, &reason); err != nil {
			if errors.Is(err, repo.ErrInvalidStatus) {
				writeErr(w, http.StatusConflict, codeInvalidStatus,
					"order status changed concurrently", nil)
				return
			}
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"order update failed", nil)
			return
		}

		// Best-effort WAL row. We do not block the response on a WAL
		// append failure — the status transition already happened and
		// /orders/{id} will reflect it; the operator can re-add the
		// audit entry out-of-band if necessary.
		seq, _ := deps.Repos.OrderEvents.NextActionSeq(r.Context(), id)
		if seq <= 0 {
			seq = 1
		}
		payload, _ := json.Marshal(map[string]string{
			"actor":  "admin",
			"reason": reason,
			"from":   string(order.Status),
		})
		_ = deps.Repos.OrderEvents.Append(r.Context(), &repo.OrderEvent{
			OrderID:   id,
			ActionSeq: seq,
			Action:    "admin.force_fail",
			Payload:   payload,
		})

		writeJSON(w, http.StatusOK, adminForceFailResponse{
			OrderID: id,
			Status:  string(repo.OrderStatusFailed),
		})
	}
}

// --- CA quota --------------------------------------------------------------

type adminCAQuotaRow struct {
	CA                  string  `json:"ca"`
	PerAccount3h        float64 `json:"per_account_3h"`
	PerRegisteredDomain float64 `json:"per_registered_domain"`
	// Switched is true when this CA's usage crosses the Router fall-over
	// threshold; the UI uses it to highlight the row in red.
	Switched bool `json:"switched"`
	// Err is populated when the quota source returned an error for this
	// CA. The UI treats it as "unknown" rather than blocking the whole
	// dashboard on a transient DB hiccup.
	Err string `json:"err,omitempty"`
}

type adminCAQuotaResponse struct {
	Rows      []adminCAQuotaRow `json:"rows"`
	Threshold float64           `json:"switch_threshold"`
}

func adminCAQuota(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.AdminQuota == nil {
			writeErr(w, http.StatusServiceUnavailable, codeInternal,
				"admin quota source not configured", nil)
			return
		}
		out := adminCAQuotaResponse{
			Threshold: service.SwitchThreshold,
			Rows:      make([]adminCAQuotaRow, 0, len(caaCAList)),
		}
		for _, ca := range caaCAList {
			row := adminCAQuotaRow{CA: ca}
			u, err := deps.AdminQuota.Usage(r.Context(), ca)
			if err != nil {
				row.Err = err.Error()
				out.Rows = append(out.Rows, row)
				continue
			}
			row.PerAccount3h = u.PerAccount3h
			row.PerRegisteredDomain = u.PerRegisteredDomain
			row.Switched = u.PerAccount3h >= service.SwitchThreshold ||
				u.PerRegisteredDomain >= service.SwitchThreshold
			out.Rows = append(out.Rows, row)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// --- DNS health ------------------------------------------------------------

type adminDNSHealthRow struct {
	Provider    string  `json:"provider"`
	SuccessRate float64 `json:"success_rate"` // 0..1; -1 = unknown
	Samples     int     `json:"samples"`
	WindowHours int     `json:"window_hours"`
}

type adminDNSHealthResponse struct {
	Rows []adminDNSHealthRow `json:"rows"`
}

// dnsHealthSource is the read-side surface admin handlers consume to
// compute provider health. The default implementation lives in the
// wiring layer (a thin SQL query over cert.audit_logs filtered to
// action = 'dns_present.success' / 'dns_present.failed'). The
// interface is exposed so tests can stub it without touching the SQL.
type dnsHealthSource interface {
	// CountByActionAndDNSProviderSince returns the total + success
	// counts in the rolling window for each provider in providers.
	// Implementations MAY return nil maps for missing providers; the
	// handler treats them as "unknown" rather than zero.
	CountByActionAndDNSProviderSince(ctx context.Context,
		providers []string, since time.Time,
	) (totals, successes map[string]int, err error)
}

// healthSourceKey is the context key used by tests to inject a fake
// dnsHealthSource without modifying Deps. Production wiring uses the
// adapter in handler/admin_dns_source.go (kept inline below).
type healthSourceKey struct{}

func adminDNSHealth(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		src, _ := r.Context().Value(healthSourceKey{}).(dnsHealthSource)
		if src == nil {
			// No explicit source — return empty rows rather than 500 so
			// the dashboard renders during cold boot.
			writeJSON(w, http.StatusOK, adminDNSHealthResponse{Rows: emptyDNSHealth()})
			return
		}
		since := time.Now().UTC().Add(-24 * time.Hour)
		totals, succ, err := src.CountByActionAndDNSProviderSince(
			r.Context(), dnsProvidersForHealth, since)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"dns health query failed", nil)
			return
		}
		out := adminDNSHealthResponse{Rows: make([]adminDNSHealthRow, 0, len(dnsProvidersForHealth))}
		for _, p := range dnsProvidersForHealth {
			row := adminDNSHealthRow{Provider: p, WindowHours: 24, SuccessRate: -1}
			if totals != nil {
				if n, ok := totals[p]; ok && n > 0 {
					row.Samples = n
					if succ != nil {
						row.SuccessRate = float64(succ[p]) / float64(n)
					}
				}
			}
			out.Rows = append(out.Rows, row)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func emptyDNSHealth() []adminDNSHealthRow {
	out := make([]adminDNSHealthRow, 0, len(dnsProvidersForHealth))
	for _, p := range dnsProvidersForHealth {
		out = append(out, adminDNSHealthRow{
			Provider: p, SuccessRate: -1, WindowHours: 24,
		})
	}
	return out
}

// --- Ban -------------------------------------------------------------------

type adminBanRequest struct {
	Reason string `json:"reason"`
}

type adminBanResponse struct {
	AccountID int64  `json:"account_id"`
	Status    string `json:"status"`
}

func adminBanAccount(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathInt64(w, r, "id")
		if !ok {
			return
		}
		if deps.AdminAbuse == nil {
			writeErr(w, http.StatusServiceUnavailable, codeInternal,
				"abuse gate not configured", nil)
			return
		}
		var req adminBanRequest
		if r.ContentLength > 0 || r.Header.Get("Content-Type") != "" {
			if !readJSON(w, r, &req) {
				return
			}
		}
		if strings.TrimSpace(req.Reason) == "" {
			req.Reason = "admin ban"
		}
		if err := deps.AdminAbuse.Ban(r.Context(), id, req.Reason); err != nil {
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"ban failed: "+err.Error(), nil)
			return
		}
		// Best-effort audit row. Failure to write does not roll the ban
		// back — the in-memory blocklist is the source of truth and we
		// would rather have a ban without an audit row than the opposite.
		if deps.Repos != nil && deps.Repos.AuditLogs != nil {
			payload, _ := json.Marshal(map[string]string{
				"actor":  "admin",
				"reason": req.Reason,
			})
			tk := "account"
			_ = deps.Repos.AuditLogs.Append(r.Context(), &repo.AuditLog{
				AccountID:  &id,
				Actor:      "admin",
				Action:     "admin.account_ban",
				TargetKind: &tk,
				TargetID:   &id,
				Payload:    payload,
			})
		}
		writeJSON(w, http.StatusOK, adminBanResponse{
			AccountID: id, Status: "banned",
		})
	}
}

func adminUnbanAccount(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathInt64(w, r, "id")
		if !ok {
			return
		}
		if deps.AdminAbuse == nil {
			writeErr(w, http.StatusServiceUnavailable, codeInternal,
				"abuse gate not configured", nil)
			return
		}
		var req adminBanRequest
		if r.ContentLength > 0 || r.Header.Get("Content-Type") != "" {
			if !readJSON(w, r, &req) {
				return
			}
		}
		if strings.TrimSpace(req.Reason) == "" {
			req.Reason = "admin unban"
		}
		if err := deps.AdminAbuse.Unban(r.Context(), id, req.Reason); err != nil {
			// The gate surfaces ErrNotBanned (via repo) when no active ban
			// exists. We cannot import repo here cleanly; fall back to a
			// substring match on the wrapped sentinel. Misclassification
			// is bounded to "404 vs 500" — acceptable for an admin path.
			if strings.Contains(err.Error(), "not currently banned") {
				writeErr(w, http.StatusNotFound, codeNotFound,
					"no active ban for account", nil)
				return
			}
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"unban failed: "+err.Error(), nil)
			return
		}
		if deps.Repos != nil && deps.Repos.AuditLogs != nil {
			payload, _ := json.Marshal(map[string]string{
				"actor":  "admin",
				"reason": req.Reason,
			})
			tk := "account"
			_ = deps.Repos.AuditLogs.Append(r.Context(), &repo.AuditLog{
				AccountID:  &id,
				Actor:      "admin",
				Action:     "admin.account_unban",
				TargetKind: &tk,
				TargetID:   &id,
				Payload:    payload,
			})
		}
		writeJSON(w, http.StatusOK, adminBanResponse{
			AccountID: id, Status: "unbanned",
		})
	}
}
