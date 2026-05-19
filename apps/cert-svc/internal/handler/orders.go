package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/apps/cert-svc/internal/service"
	"github.com/kite365/idcd/lib/shared/pagination"
)

// daily order quota per account. S1 simplification — the real model
// (free vs paid tier, per-domain limits, hourly burst) is W4+.
const dailyOrderQuota = 20

// challengeDNS01 is the only ACME challenge cert-svc supports in S1.
const challengeDNS01 = "dns-01"

func mountOrders(r chi.Router, deps Deps) {
	r.Post("/orders", createOrder(deps))
	r.Get("/orders", listOrders(deps))
	r.Get("/orders/{id}", getOrder(deps))
	r.Post("/orders/{id}/retry", retryOrder(deps))
	r.Post("/orders/{id}/manual-ready", manualReadyOrder(deps))
}

// createOrderRequest is the wire shape of POST /v1/cert/orders. SANs are
// the human-friendly Unicode form; the handler normalises them to
// ASCII / Punycode before persisting.
type createOrderRequest struct {
	SANs              []string `json:"sans"`
	Challenge         string   `json:"challenge"`
	DNSCredentialID   *int64   `json:"dns_credential_id,omitempty"`
	CA                string   `json:"ca,omitempty"`
	IdempotencyKey    string   `json:"idempotency_key,omitempty"`
}

type createOrderResponse struct {
	OrderID int64  `json:"order_id"`
	Status  string `json:"status"`
}

func createOrder(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := requireUser(w, r)
		if !ok {
			return
		}
		if deps.Repos == nil || deps.Service == nil {
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"cert-svc dependencies not configured", nil)
			return
		}

		var req createOrderRequest
		if !readJSON(w, r, &req) {
			return
		}
		if len(req.SANs) == 0 {
			writeErr(w, http.StatusBadRequest, codeDomainInvalid,
				"sans is required", nil)
			return
		}
		if len(req.SANs) > 10 {
			writeErr(w, http.StatusBadRequest, codeDomainInvalid,
				"at most 10 SANs per order", nil)
			return
		}
		if req.Challenge == "" {
			req.Challenge = challengeDNS01
		}
		if req.Challenge != challengeDNS01 {
			writeErr(w, http.StatusBadRequest, codeBadRequest,
				"only dns-01 challenge supported in S1", nil)
			return
		}

		// Canonicalise each SAN. We keep both the Punycode (DB / ACME
		// wire) and the original Unicode (display).
		fields := map[string]string{}
		ascii := make([]string, 0, len(req.SANs))
		unicode := make([]string, 0, len(req.SANs))
		for i, raw := range req.SANs {
			norm, err := normaliseSAN(raw)
			if err != nil {
				fields["sans["+strconv.Itoa(i)+"]"] = err.Error()
				continue
			}
			ascii = append(ascii, norm)
			unicode = append(unicode, raw)
		}
		if len(fields) > 0 {
			writeErr(w, http.StatusBadRequest, codeDomainInvalid,
				"one or more SANs rejected", fields)
			return
		}
		ascii = dedupePreserveOrder(ascii)
		unicode = dedupePreserveOrder(unicode)
		if len(ascii) == 0 {
			writeErr(w, http.StatusBadRequest, codeDomainInvalid,
				"no valid SAN survived normalisation", nil)
			return
		}

		// Validate DNS credential ownership when supplied.
		if req.DNSCredentialID != nil {
			cred, err := deps.Repos.DNSCredentials.GetByID(r.Context(), *req.DNSCredentialID)
			if err != nil {
				if errors.Is(err, repo.ErrNotFound) {
					writeErr(w, http.StatusUnprocessableEntity, codeCredentialInvalid,
						"dns credential not found", nil)
					return
				}
				writeErr(w, http.StatusInternalServerError, codeInternal,
					"dns credential lookup failed", nil)
				return
			}
			if cred.AccountID != accountID {
				writeErr(w, http.StatusForbidden, codeForbidden,
					"dns credential does not belong to this account", nil)
				return
			}
			if cred.RevokedAt != nil {
				writeErr(w, http.StatusUnprocessableEntity, codeCredentialInvalid,
					"dns credential has been revoked", nil)
				return
			}
		}

		// Daily quota — simple count over orders created in the last 24h.
		// We page through the most recent orders rather than adding a
		// schema-level COUNT query so S1 keeps the repo surface small.
		if used, err := dailyOrderCount(r.Context(), deps.Repos.Orders, accountID); err == nil {
			if used >= dailyOrderQuota {
				writeErr(w, http.StatusTooManyRequests, codeQuotaExceeded,
					"daily order quota exhausted", nil)
				return
			}
		}

		ca := req.CA
		if ca == "" {
			ca = "lets-encrypt"
		}

		// Abuse → CAA precheck → insert → enqueue. Abuse must run first
		// because we'd rather not even consult DNS on flagged accounts.
		if abuse := deps.Service.Abuse; abuse != nil {
			if err := abuse.Check(r.Context(), accountID, ascii); err != nil {
				if errors.Is(err, service.ErrAccountBanned) {
					writeErr(w, http.StatusForbidden, codeAccountBanned,
						err.Error(), nil)
					return
				}
				if errors.Is(err, service.ErrAbuseBlocked) {
					writeErr(w, http.StatusForbidden, codeAbuseBlocked,
						err.Error(), nil)
					return
				}
				slog.Default().Warn("abuse check errored",
					"account_id", accountID, "error", err)
			}
		}
		if err := deps.Service.CheckCAA(r.Context(), ascii, ca); err != nil {
			if errors.Is(err, service.ErrCAAForbidden) {
				writeErr(w, http.StatusUnprocessableEntity, codeCAAForbid,
					"CAA records forbid the target CA: "+err.Error(),
					map[string]string{
						"ca":             ca,
						"required_value": caaExpectedFor(ca),
					})
				return
			}
			// Transient lookup failures fall through — the CA will
			// re-check CAA at validation time.
			slog.Default().Warn("CAA check errored, continuing",
				"account_id", accountID, "error", err)
		}

		commonName := ascii[0]
		var idemPtr *string
		if req.IdempotencyKey != "" {
			k := req.IdempotencyKey
			idemPtr = &k
		}
		order := &repo.Order{
			AccountID:       accountID,
			SANs:            ascii,
			SANsUnicode:     unicode,
			CommonName:      &commonName,
			Tier:            "free-dv",
			CA:              ca,
			ValidityDays:    90,
			ChallengeType:   challengeDNS01,
			DNSCredentialID: req.DNSCredentialID,
			Status:          repo.OrderStatusDraft,
			IdempotencyKey:  idemPtr,
		}
		id, err := deps.Repos.Orders.Insert(r.Context(), order)
		if err != nil {
			if errors.Is(err, repo.ErrConflict) && id > 0 {
				// Idempotent replay — return the existing order with 200.
				writeJSON(w, http.StatusOK, createOrderResponse{
					OrderID: id,
					Status:  string(repo.OrderStatusDraft),
				})
				return
			}
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"order insert failed", nil)
			return
		}

		// Best-effort enqueue. If Redis is down the worker poll loop
		// will still pick the order up from cert.orders on its next
		// sweep; the client still sees a 201.
		_ = deps.Service.EnqueueOrder(r.Context(), id)

		writeJSON(w, http.StatusCreated, createOrderResponse{
			OrderID: id,
			Status:  string(repo.OrderStatusDraft),
		})
	}
}

// dailyOrderCount returns the number of orders an account has created in
// the last 24h.
func dailyOrderCount(ctx context.Context, orders *repo.OrdersRepo, accountID string) (int, error) {
	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	return orders.CountByAccountSince(ctx, accountID, cutoff)
}

type orderResponse struct {
	ID              int64    `json:"id"`
	AccountID  string    `json:"account_id"`
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

type orderDetailResponse struct {
	orderResponse
	Events           []orderEventResponse `json:"events"`
	ManualChallenge  *manualChallenge     `json:"manual_challenge"`
}

type orderEventResponse struct {
	Action     string `json:"action"`
	ActionSeq  int    `json:"action_seq"`
	OccurredAt string `json:"occurred_at"`
}

type manualChallenge struct {
	FQDN    string `json:"fqdn,omitempty"`
	Value   string `json:"value,omitempty"`
	Message string `json:"message,omitempty"`
}

type listOrdersResponse struct {
	Orders []orderResponse `json:"orders"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

func listOrders(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := requireUser(w, r)
		if !ok {
			return
		}
		if deps.Repos == nil {
			writeErr(w, http.StatusInternalServerError, codeInternal, "repos not configured", nil)
			return
		}
		limit := pagination.Clamp(queryIntDefault(r, "limit", pagination.DefaultPageSize))
		offset := queryIntDefault(r, "offset", 0)
		if offset < 0 {
			offset = 0
		}
		var statusFilter *repo.OrderStatus
		if s := r.URL.Query().Get("status"); s != "" {
			os := repo.OrderStatus(s)
			statusFilter = &os
		}
		rows, err := deps.Repos.Orders.ListByAccount(r.Context(), accountID, statusFilter, limit, offset)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, codeInternal, "orders list failed", nil)
			return
		}
		out := make([]orderResponse, 0, len(rows))
		for _, o := range rows {
			out = append(out, projectOrder(o))
		}
		writeJSON(w, http.StatusOK, listOrdersResponse{
			Orders: out,
			Limit:  limit,
			Offset: offset,
		})
	}
}

func getOrder(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := requireUser(w, r)
		if !ok {
			return
		}
		id, ok := pathInt64(w, r, "id")
		if !ok {
			return
		}
		if deps.Repos == nil {
			writeErr(w, http.StatusInternalServerError, codeInternal, "repos not configured", nil)
			return
		}
		order, err := deps.Repos.Orders.GetByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				writeErr(w, http.StatusNotFound, codeNotFound, "order not found", nil)
				return
			}
			writeErr(w, http.StatusInternalServerError, codeInternal, "order get failed", nil)
			return
		}
		if order.AccountID != accountID {
			writeErr(w, http.StatusForbidden, codeForbidden, "order does not belong to this account", nil)
			return
		}
		events, err := deps.Repos.OrderEvents.ListByOrder(r.Context(), id)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, codeInternal, "events fetch failed", nil)
			return
		}
		eventResp := make([]orderEventResponse, 0, len(events))
		for _, ev := range events {
			eventResp = append(eventResp, orderEventResponse{
				Action:     ev.Action,
				ActionSeq:  ev.ActionSeq,
				OccurredAt: ev.OccurredAt.UTC().Format(time.RFC3339),
			})
		}
		mc := manualChallengeFromEvents(order, events)
		writeJSON(w, http.StatusOK, orderDetailResponse{
			orderResponse:   projectOrder(order),
			Events:          eventResp,
			ManualChallenge: mc,
		})
	}
}

func retryOrder(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := requireUser(w, r)
		if !ok {
			return
		}
		id, ok := pathInt64(w, r, "id")
		if !ok {
			return
		}
		if deps.Repos == nil || deps.Service == nil {
			writeErr(w, http.StatusInternalServerError, codeInternal, "deps not configured", nil)
			return
		}
		order, err := deps.Repos.Orders.GetByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				writeErr(w, http.StatusNotFound, codeNotFound, "order not found", nil)
				return
			}
			writeErr(w, http.StatusInternalServerError, codeInternal, "order get failed", nil)
			return
		}
		if order.AccountID != accountID {
			writeErr(w, http.StatusForbidden, codeForbidden, "order does not belong to this account", nil)
			return
		}
		if order.Status != repo.OrderStatusFailed {
			writeErr(w, http.StatusConflict, codeInvalidStatus,
				"only failed orders can be retried", nil)
			return
		}
		if err := deps.Service.RetryOrder(r.Context(), id); err != nil {
			writeErr(w, http.StatusInternalServerError, codeInternal, "retry failed: "+err.Error(), nil)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"order_id": id, "status": "validating"})
	}
}

type manualReadyRequest struct {
	FQDN  string `json:"fqdn"`
	Value string `json:"value"`
}

func manualReadyOrder(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := requireUser(w, r)
		if !ok {
			return
		}
		id, ok := pathInt64(w, r, "id")
		if !ok {
			return
		}
		if deps.Repos == nil || deps.Service == nil {
			writeErr(w, http.StatusInternalServerError, codeInternal, "deps not configured", nil)
			return
		}
		var req manualReadyRequest
		if !readJSON(w, r, &req) {
			return
		}
		if req.FQDN == "" || req.Value == "" {
			writeErr(w, http.StatusBadRequest, codeBadRequest,
				"fqdn and value are required", nil)
			return
		}
		order, err := deps.Repos.Orders.GetByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				writeErr(w, http.StatusNotFound, codeNotFound, "order not found", nil)
				return
			}
			writeErr(w, http.StatusInternalServerError, codeInternal, "order get failed", nil)
			return
		}
		if order.AccountID != accountID {
			writeErr(w, http.StatusForbidden, codeForbidden,
				"order does not belong to this account", nil)
			return
		}
		// PublishManualReady bridges server → worker via Redis pub/sub
		// (the in-process MarkManualChallengeReady is worker-only).
		if err := deps.Service.PublishManualReady(r.Context(), id, req.FQDN, req.Value); err != nil {
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"manual ready publish failed", nil)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"order_id": id, "status": "validating"})
	}
}

// manualChallengeFromEvents extracts the most recent manual_challenge_ready
// event (written by the worker's wrapped manual solver) and returns it as
// the wire-level manualChallenge struct. Returns nil when the order isn't
// in a manual-mode validating state — callers serialise nil as a null in
// the JSON response so the frontend can branch on its presence.
//
// Multi-SAN orders write one event per Present call; we surface only the
// last one. The frontend currently shows a single TXT record at a time;
// once it learns to walk an array we can swap this to return all of them.
func manualChallengeFromEvents(order *repo.Order, events []*repo.OrderEvent) *manualChallenge {
	// Manual mode = no stored DNS credential. Once the order is issued
	// or revoked the user can't act on the challenge anymore, so skip.
	if order.DNSCredentialID != nil {
		return nil
	}
	switch order.Status {
	case repo.OrderStatusIssued, repo.OrderStatusRevoked, repo.OrderStatusRevoking:
		return nil
	}
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Action != service.ActionManualChallengeReady {
			continue
		}
		var p struct {
			FQDN  string `json:"fqdn"`
			Value string `json:"value"`
		}
		if err := json.Unmarshal(events[i].Payload, &p); err != nil || p.FQDN == "" {
			continue
		}
		return &manualChallenge{FQDN: p.FQDN, Value: p.Value}
	}
	// Worker hasn't reached Present yet (still keying / building CSR).
	// Tell the user to retry shortly instead of leaking implementation
	// detail.
	return &manualChallenge{Message: "Challenge not ready yet, retry shortly"}
}

// projectOrder turns a repo.Order into the wire response.
func projectOrder(o *repo.Order) orderResponse {
	out := orderResponse{
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

// caaExpectedFor returns the canonical CAA issue value the user needs to
// add for the supplied CA — surfaced in the error body so the operator
// knows what to write into their DNS zone. Mirrors caaCAToTag from the
// service package; duplicated here to avoid leaking package-internal
// state.
func caaExpectedFor(caID string) string {
	switch caID {
	case "letsencrypt", "lets-encrypt":
		return "letsencrypt.org"
	case "zerossl":
		return "sectigo.com"
	case "buypass":
		return "buypass.com"
	case "gts", "google":
		return "pki.goog"
	}
	return ""
}

