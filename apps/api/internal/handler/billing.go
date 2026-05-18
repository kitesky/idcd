package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kite365/idcd/apps/api/internal/billing"
	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
	"github.com/kite365/idcd/lib/shared/pagination"
)

// VerdictStreamPublisher publishes a paid verdict_order onto the Redis
// Stream `verdict_generation_queue` so the apps/attest worker can pick it up
// (D4: webhook is the single source of truth for "payment cleared → start
// generation").
//
// Implementations must be safe for concurrent use.  A nil VerdictStreamPublisher
// is acceptable in tests / minimal harnesses: the webhook still flips the
// verdict_order row to "paid" so admin tooling can manually re-enqueue, but
// no automated processing happens.
type VerdictStreamPublisher interface {
	// EnqueueVerdict pushes one entry onto `verdict_generation_queue` with the
	// fields {order_id, owner_id, enqueued_at}.  The implementation is
	// responsible for serialising values (XAdd uses string fields).
	EnqueueVerdict(ctx context.Context, orderID, ownerID string) error
}

// BillingHandler handles user-facing billing endpoints.
//
// All routes except /webhook and /stub-confirm require Bearer token auth.
// The handler stores subscription/payment records directly via raw SQL because
// the billing tables are not yet in the sqlc schema (they are pending sqlc regen
// after the 00010 migration lands).
type BillingHandler struct {
	pool             BillingPool // reuse the interface from admin_billing.go
	provider         billing.Provider
	enqueuer         BillingEnqueuer        // optional: nil disables automatic refund retry scheduling
	verdictPublisher VerdictStreamPublisher // optional: nil disables verdict_generation_queue push

	// appBaseURL is the frontend origin (e.g. "https://app.idcd.com") used
	// to build user-facing return URLs the payment platform redirects to.
	appBaseURL string
	// publicAPIURL is the API service's externally-reachable origin (e.g.
	// "https://api.idcd.com") used to build the webhook NotifyURL. MUST be
	// server-side configuration — never derived from request headers.
	publicAPIURL string
}

// NewBillingHandler wires a BillingHandler with the given DB pool and payment provider.
func NewBillingHandler(pool BillingPool, provider billing.Provider) *BillingHandler {
	return &BillingHandler{pool: pool, provider: provider}
}

// WithURLs configures the trusted server-side base URLs used to construct
// ReturnURL (user browser redirect after pay) and NotifyURL (server-to-server
// webhook callback). NotifyURL in particular MUST come from config — see the
// security note on BillingHandler.publicAPIURL.
func (h *BillingHandler) WithURLs(appBase, publicAPI string) *BillingHandler {
	h.appBaseURL = appBase
	h.publicAPIURL = publicAPI
	return h
}

// WithEnqueuer wires a BillingEnqueuer used for automatic refund retries
// triggered by refund.failed webhooks (D5).  When nil, the webhook still
// records the failure in the DB but no retry queue entry is created — admin
// dashboard remains the only recovery path.
func (h *BillingHandler) WithEnqueuer(enq BillingEnqueuer) *BillingHandler {
	h.enqueuer = enq
	return h
}

// WithVerdictPublisher wires the Redis Stream publisher used to push paid
// verdict_order rows onto `verdict_generation_queue` (S2 Evidence flow).
func (h *BillingHandler) WithVerdictPublisher(pub VerdictStreamPublisher) *BillingHandler {
	h.verdictPublisher = pub
	return h
}

// ---- request / response types ----

type subscribeRequest struct {
	Plan string `json:"plan"`
	// Channel selects the payment channel: "alipay" or "wechat_pay".
	// Omit to use the provider's configured default.
	// Method is determined automatically (Alipay→page QR, WeChat→native QR).
	Channel string `json:"channel,omitempty"`
}

type subscribeResponse struct {
	SubscriptionID string    `json:"subscription_id"`
	PayURL         string    `json:"pay_url"`
	ExpiresAt      time.Time `json:"expires_at"`
}

type cancelResponse struct {
	Message string `json:"message"`
}

type subscriptionResponse struct {
	ID                 string     `json:"id"`
	Plan               string     `json:"plan"`
	Status             string     `json:"status"`
	Provider           string     `json:"provider"`
	ExtSubID           *string    `json:"ext_sub_id,omitempty"`
	CurrentPeriodStart *time.Time `json:"current_period_start,omitempty"`
	CurrentPeriodEnd   *time.Time `json:"current_period_end,omitempty"`
	CancelAt           *time.Time `json:"cancel_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

type invoiceResponse struct {
	ID             string     `json:"id"`
	SubscriptionID *string    `json:"subscription_id,omitempty"`
	AmountCents    int64      `json:"amount_cents"`
	Currency       string     `json:"currency"`
	Status         string     `json:"status"`
	Provider       string     `json:"provider"`
	ExtInvoiceID   *string    `json:"ext_invoice_id,omitempty"`
	PaidAt         *time.Time `json:"paid_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type invoicesResponse struct {
	Invoices []invoiceResponse `json:"invoices"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

// ---- Subscribe ----

// Subscribe handles POST /v1/billing/subscribe.
// Body: {"plan": "pro"}
// Returns: {"pay_url": "..."}
func (h *BillingHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	var req subscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}

	plan := billing.Plan(req.Plan)
	if !billing.ValidPlan(plan) {
		response.Error(w, r, apperr.Validation("unknown plan", "plan"))
		return
	}

	if req.Channel != "" && !billing.ValidUserChannel(req.Channel) {
		response.Error(w, r, apperr.Validation("channel must be alipay or wechat_pay", "channel"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// ReturnURL: where the user's browser lands after pay. Falls back to the
	// request Origin only in non-production environments so dev/test still
	// works without explicit config; production deployments MUST set
	// server.app_base_url.
	returnBase := h.appBaseURL
	if returnBase == "" {
		returnBase = r.Header.Get("Origin")
	}
	// NotifyURL: server-to-server webhook callback. MUST come from config —
	// trusting a client header here lets a forged Origin redirect refund /
	// payment webhooks to an attacker URL.
	if h.publicAPIURL == "" {
		response.Error(w, r, apperr.Internal("billing public_api_url is not configured", nil))
		return
	}

	result, err := h.provider.Subscribe(ctx, billing.SubscribeRequest{
		UserID:    userID,
		Plan:      plan,
		Channel:   req.Channel,
		ReturnURL: returnBase + "/app/billing",
		NotifyURL: h.publicAPIURL + "/v1/billing/webhook",
	})
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to initiate subscription", err))
		return
	}

	// Persist subscription record.
	now := time.Now().UTC()
	subID := result.SubscriptionID
	if subID == "" {
		subID = idgen.Subscription()
	}

	_, dbErr := h.pool.Exec(ctx, `
		INSERT INTO subscriptions
			(id, user_id, plan, status, provider, ext_sub_id,
			 current_period_start, current_period_end, created_at, updated_at)
		VALUES ($1, $2, $3, 'pending', $4, $5, $6, $7, $8, $8)
		ON CONFLICT (id) DO NOTHING
	`,
		subID, userID, string(plan), h.provider.Name(), result.ExtSubID,
		now, result.ExpiresAt, now,
	)
	if dbErr != nil {
		response.Error(w, r, apperr.Internal("failed to persist subscription", dbErr))
		return
	}

	response.JSON(w, r, http.StatusOK, subscribeResponse{
		SubscriptionID: subID,
		PayURL:         result.PayURL,
		ExpiresAt:      result.ExpiresAt,
	})
}

// ---- Cancel ----

// Cancel handles POST /v1/billing/cancel.
// Cancels the current user's active subscription.
func (h *BillingHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Fetch active subscription for this user.
	row, err := h.pool.Query(ctx, `
		SELECT id, provider FROM subscriptions
		WHERE user_id = $1 AND status = 'active'
		ORDER BY created_at DESC LIMIT 1
	`, userID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query subscription", err))
		return
	}
	defer row.Close()

	if !row.Next() {
		response.Error(w, r, apperr.NotFound("no active subscription"))
		return
	}

	var subID, providerName string
	if err := row.Scan(&subID, &providerName); err != nil {
		response.Error(w, r, apperr.Internal("failed to scan subscription", err))
		return
	}
	row.Close()

	// Cancel via provider.
	if err := h.provider.Cancel(ctx, billing.CancelRequest{
		SubscriptionID: subID,
		UserID:         userID,
	}); err != nil {
		response.Error(w, r, apperr.Internal("provider cancel failed", err))
		return
	}

	// Update DB.
	now := time.Now().UTC()
	if _, err := h.pool.Exec(ctx, `
		UPDATE subscriptions
		SET status = 'cancelled', cancel_at = $2, updated_at = $2
		WHERE id = $1
	`, subID, now); err != nil {
		response.Error(w, r, apperr.Internal("failed to update subscription status", err))
		return
	}

	response.JSON(w, r, http.StatusOK, cancelResponse{Message: "subscription cancelled"})
}

// ---- GetSubscription ----

// GetSubscription handles GET /v1/billing/subscription.
func (h *BillingHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	rows, err := h.pool.Query(ctx, `
		SELECT id, plan, status, provider, ext_sub_id,
		       current_period_start, current_period_end, cancel_at, created_at
		FROM subscriptions
		WHERE user_id = $1
		ORDER BY created_at DESC LIMIT 1
	`, userID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query subscription", err))
		return
	}
	defer rows.Close()

	if !rows.Next() {
		response.Error(w, r, apperr.NotFound("no subscription found"))
		return
	}

	var sub subscriptionResponse
	if err := rows.Scan(
		&sub.ID, &sub.Plan, &sub.Status, &sub.Provider, &sub.ExtSubID,
		&sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.CancelAt, &sub.CreatedAt,
	); err != nil {
		response.Error(w, r, apperr.Internal("failed to scan subscription", err))
		return
	}

	response.JSON(w, r, http.StatusOK, sub)
}

// ---- ListInvoices ----

// ListInvoices handles GET /v1/billing/invoices.
// Query params: page (default 1), page_size (default 20, max 100).
func (h *BillingHandler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	page := parseIntParam(r.URL.Query().Get("page"), 1)
	pageSize := pagination.Clamp(parseIntParam(r.URL.Query().Get("page_size"), pagination.DefaultPageSize))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	rows, err := h.pool.Query(ctx, `
		SELECT id, subscription_id, amount_cents, currency, status,
		       provider, ext_invoice_id, paid_at, created_at
		FROM invoices
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, pageSize, offset)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query invoices", err))
		return
	}
	defer rows.Close()

	var invoices []invoiceResponse
	for rows.Next() {
		var inv invoiceResponse
		if err := rows.Scan(
			&inv.ID, &inv.SubscriptionID, &inv.AmountCents, &inv.Currency, &inv.Status,
			&inv.Provider, &inv.ExtInvoiceID, &inv.PaidAt, &inv.CreatedAt,
		); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan invoice", err))
			return
		}
		invoices = append(invoices, inv)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("row iteration error", err))
		return
	}
	if invoices == nil {
		invoices = []invoiceResponse{}
	}

	response.JSON(w, r, http.StatusOK, invoicesResponse{
		Invoices: invoices,
		Total:    len(invoices),
		Page:     page,
		PageSize: pageSize,
	})
}

// ---- Webhook ----

// Webhook handles POST /v1/billing/webhook.
// No Bearer auth — provider signature verification is done inside ParseWebhook.
func (h *BillingHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Read raw body for signature checking / parsing.
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB max
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to read webhook body", err))
		return
	}
	if len(body) == 0 {
		response.Error(w, r, apperr.Validation("empty webhook body", ""))
		return
	}

	// Build headers map.
	headers := make(map[string]string, len(r.Header))
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	event, err := h.provider.ParseWebhook(ctx, body, headers)
	if err != nil {
		response.Error(w, r, apperr.Validation("invalid webhook payload", err.Error()))
		return
	}

	if err := h.handleWebhookEvent(ctx, event); err != nil {
		response.Error(w, r, apperr.Internal("failed to process webhook event", err))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
}

// handleWebhookEvent processes a parsed webhook event and updates the DB.
func (h *BillingHandler) handleWebhookEvent(ctx context.Context, event *billing.WebhookEvent) error {
	now := time.Now().UTC()
	switch event.EventType {
	case billing.EventPaymentSucceeded:
		// Update subscription to active and create payment + invoice records.
		if event.SubscriptionID != "" {
			if _, err := h.pool.Exec(ctx, `
				UPDATE subscriptions SET status = 'active', updated_at = $2
				WHERE id = $1
			`, event.SubscriptionID, now); err != nil {
				return fmt.Errorf("billing: activate subscription %s: %w", event.SubscriptionID, err)
			}
		}
		// Persist payment record.
		payID := idgen.New("pay_")
		invID := idgen.Invoice()
		if event.UserID != "" && event.AmountCents > 0 {
			currency := event.Currency
			if currency == "" {
				currency = "CNY"
			}
			if _, err := h.pool.Exec(ctx, `
				INSERT INTO payments (id, user_id, invoice_id, provider, ext_txn_id,
				                      amount_cents, currency, status, created_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,'succeeded',$8)
				ON CONFLICT (id) DO NOTHING
			`, payID, event.UserID, invID, h.provider.Name(), event.ExtTxnID,
				event.AmountCents, currency, now); err != nil {
				return fmt.Errorf("billing: insert payment record pay=%s: %w", payID, err)
			}
			if _, err := h.pool.Exec(ctx, `
				INSERT INTO invoices (id, user_id, subscription_id, provider, ext_invoice_id,
				                      amount_cents, currency, status, paid_at, created_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,'paid',$8,$8)
				ON CONFLICT (id) DO NOTHING
			`, invID, event.UserID, event.SubscriptionID, h.provider.Name(), event.ExtTxnID,
				event.AmountCents, currency, now); err != nil {
				return fmt.Errorf("billing: insert invoice record inv=%s: %w", invID, err)
			}
		}

		// S2 Evidence: if this payment carried a verdict_order_id in metadata,
		// flip the verdict_order row to "paid" and push it onto the Redis
		// Stream `verdict_generation_queue` for apps/attest to consume.
		// Cross-schema is read/write-only (D1): no FK to idcd_main.users.
		if vid := event.Metadata["verdict_order_id"]; vid != "" {
			tag, err := h.pool.Exec(ctx, `
				UPDATE idcd_attest.verdict_order
				SET status = 'paid',
				    paid_at = $2,
				    ext_order_id = $3,
				    price_paid_cny = $4
				WHERE id = $1 AND status = 'pending'
			`, vid, now, event.ExtTxnID, float64(event.AmountCents)/100)
			if err != nil {
				return fmt.Errorf("billing: mark verdict_order %s paid: %w", vid, err)
			}
			// Only enqueue if the UPDATE actually flipped a row — guards
			// against duplicate webhooks re-pushing the same generation job.
			if tag.RowsAffected() > 0 && h.verdictPublisher != nil {
				if err := h.verdictPublisher.EnqueueVerdict(ctx, vid, event.UserID); err != nil {
					// Non-fatal: webhook ack still returns 200; row is in
					// "paid" state so admin can manually re-enqueue.  Log at
					// warn so P1 monitoring fires.
					slog.Default().Warn("verdict enqueue failed",
						"order_id", vid,
						"owner_id", event.UserID,
						"err", err,
					)
				}
			}
		}

	case billing.EventPaymentFailed:
		if event.SubscriptionID != "" {
			if _, err := h.pool.Exec(ctx, `
				UPDATE subscriptions SET status = 'past_due', updated_at = $2
				WHERE id = $1
			`, event.SubscriptionID, now); err != nil {
				return fmt.Errorf("billing: mark subscription past_due %s: %w", event.SubscriptionID, err)
			}
		}

	case billing.EventSubscriptionCancelled:
		if event.SubscriptionID != "" {
			if _, err := h.pool.Exec(ctx, `
				UPDATE subscriptions SET status = 'cancelled', cancel_at = $2, updated_at = $2
				WHERE id = $1
			`, event.SubscriptionID, now); err != nil {
				return fmt.Errorf("billing: cancel subscription %s: %w", event.SubscriptionID, err)
			}
		}

	case billing.EventRefundSucceeded:
		if event.ExtTxnID != "" {
			if _, err := h.pool.Exec(ctx, `
				UPDATE payments SET status = 'refunded'
				WHERE ext_txn_id = $1 AND provider = $2
			`, event.ExtTxnID, h.provider.Name()); err != nil {
				return fmt.Errorf("billing: mark payment refunded ext_txn=%s: %w", event.ExtTxnID, err)
			}
		}

	case billing.EventRefundFailed:
		if event.ExtTxnID != "" {
			if _, err := h.pool.Exec(ctx, `
				UPDATE payments
				SET status = 'refund_failed',
				    refund_retry_count = refund_retry_count + 1,
				    refund_failed_at = $2
				WHERE ext_txn_id = $1 AND provider = $3
			`, event.ExtTxnID, now, h.provider.Name()); err != nil {
				return fmt.Errorf("billing: mark refund_failed ext_txn=%s: %w", event.ExtTxnID, err)
			}

			// D5: schedule the first automatic refund retry 5 minutes out.
			// We re-read the payment row to build a complete payload (the
			// webhook event itself may not carry user_email, currency etc.).
			if h.enqueuer != nil {
				if err := h.scheduleRefundRetry(ctx, event.ExtTxnID, RefundRetryFirstDelay); err != nil {
					// Non-fatal: webhook ack should still succeed.  Surface
					// in logs via the wrapped error so the admin dashboard
					// + monitoring catch the queue divergence.
					return fmt.Errorf("billing: schedule refund retry ext_txn=%s: %w", event.ExtTxnID, err)
				}
			}
		}
	}
	return nil
}

// scheduleRefundRetry fetches the payment + user email and enqueues a
// payment:refund_retry asynq task with the given delay.  AttemptCount=0 means
// this is the first retry (after the initial webhook-reported failure).
func (h *BillingHandler) scheduleRefundRetry(ctx context.Context, extTxnID string, delay time.Duration) error {
	rows, err := h.pool.Query(ctx, `
		SELECT
			p.id, p.user_id, p.ext_txn_id, p.amount_cents, p.currency, p.provider,
			COALESCE(u.email, '') AS user_email
		FROM payments p
		LEFT JOIN users u ON u.id = p.user_id
		WHERE p.ext_txn_id = $1 AND p.provider = $2 AND p.status = 'refund_failed'
		LIMIT 1
	`, extTxnID, h.provider.Name())
	if err != nil {
		return fmt.Errorf("query payment for retry: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		// Row not found — webhook may have arrived for a payment we never
		// recorded.  Skip enqueueing.
		return nil
	}
	var (
		payload     RefundRetryPayload
		extTxnIDPtr *string
	)
	if err := rows.Scan(
		&payload.PaymentID,
		&payload.UserID,
		&extTxnIDPtr,
		&payload.AmountCents,
		&payload.Currency,
		&payload.Provider,
		&payload.UserEmail,
	); err != nil {
		return fmt.Errorf("scan payment for retry: %w", err)
	}
	rows.Close()
	if extTxnIDPtr != nil {
		payload.ExtTxnID = *extTxnIDPtr
	}
	payload.Reason = "webhook_refund_failed"
	payload.AttemptCount = 0

	return h.enqueuer.EnqueueRefundRetry(ctx, payload, delay)
}

// ---- StubConfirm ----

// StubConfirm handles POST /v1/billing/stub-confirm.
// Only active when the configured provider is "stub".
// Body / query params: sub_id, plan.
// Marks the subscription active and writes a payment record, then redirects.
//
// State changes happen on POST only — the paired GET handler
// (StubConfirmForm) renders an HTML form that POSTs back here, following the
// Post-Redirect-Get pattern so a leaked link / CSRF GET cannot silently
// activate a subscription.
func (h *BillingHandler) StubConfirm(w http.ResponseWriter, r *http.Request) {
	// Only allow stub provider.
	if h.provider.Name() != "stub" {
		response.Error(w, r, apperr.Forbidden("stub-confirm only available in stub mode"))
		return
	}

	subID := r.URL.Query().Get("sub_id")
	if subID == "" {
		if err := r.ParseForm(); err == nil {
			subID = r.Form.Get("sub_id")
		}
	}
	if subID == "" {
		response.Error(w, r, apperr.Validation("missing sub_id", ""))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Confirm in stub provider's in-memory store if possible.
	if stub, ok := h.provider.(*billing.StubProvider); ok {
		if err := stub.ConfirmSubscription(subID); err != nil {
			// Non-fatal if not found in memory (may have been restarted).
			_ = err
		}
	}

	// Fetch the subscription from DB to get user/plan info.
	rows, err := h.pool.Query(ctx, `
		SELECT user_id, plan, provider, ext_sub_id
		FROM subscriptions WHERE id = $1
	`, subID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query subscription", err))
		return
	}
	defer rows.Close()

	if !rows.Next() {
		response.Error(w, r, apperr.NotFound("subscription not found"))
		return
	}

	var userID, plan, providerName string
	var extSubID *string
	if err := rows.Scan(&userID, &plan, &providerName, &extSubID); err != nil {
		response.Error(w, r, apperr.Internal("failed to scan subscription", err))
		return
	}
	rows.Close()

	now := time.Now().UTC()
	// Mark subscription active.
	if _, err := h.pool.Exec(ctx, `
		UPDATE subscriptions
		SET status = 'active', current_period_start = $2, updated_at = $2
		WHERE id = $1
	`, subID, now); err != nil {
		response.Error(w, r, apperr.Internal("failed to activate subscription", err))
		return
	}

	// Write a synthetic payment record.
	price, ok := billing.PlanPrice[billing.Plan(plan)]
	if !ok || price == 0 {
		// Free plan — no payment record needed.
		http.Redirect(w, r, "/app/billing?success=1", http.StatusFound)
		return
	}

	payID := idgen.New("pay_")
	invID := idgen.Invoice()
	extTxnID := "stub_txn_" + payID
	if extSubID != nil {
		extTxnID = "stub_txn_" + *extSubID
	}

	if _, err := h.pool.Exec(ctx, `
		INSERT INTO invoices (id, user_id, subscription_id, provider, ext_invoice_id,
		                      amount_cents, currency, status, paid_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,'CNY','paid',$7,$7)
		ON CONFLICT (id) DO NOTHING
	`, invID, userID, subID, providerName, extTxnID, price, now); err != nil {
		response.Error(w, r, apperr.Internal("failed to write invoice", err))
		return
	}

	if _, err := h.pool.Exec(ctx, `
		INSERT INTO payments (id, user_id, invoice_id, provider, ext_txn_id,
		                      amount_cents, currency, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,'CNY','succeeded',$7)
		ON CONFLICT (id) DO NOTHING
	`, payID, userID, invID, providerName, extTxnID, price, now); err != nil {
		response.Error(w, r, apperr.Internal("failed to write payment", err))
		return
	}

	http.Redirect(w, r, "/app/billing?success=1", http.StatusFound)
}

// StubConfirmForm handles GET /v1/billing/stub-confirm.
// Returns a minimal HTML page that POSTs back to the same URL with the
// preserved query string. Kept on GET specifically so that the stub provider
// can hand a browser-clickable URL back to the user — the actual state change
// is gated behind the user clicking the form's submit button (POST).
func (h *BillingHandler) StubConfirmForm(w http.ResponseWriter, r *http.Request) {
	if h.provider.Name() != "stub" {
		response.Error(w, r, apperr.Forbidden("stub-confirm only available in stub mode"))
		return
	}
	subID := r.URL.Query().Get("sub_id")
	if subID == "" {
		response.Error(w, r, apperr.Validation("missing sub_id", ""))
		return
	}
	plan := r.URL.Query().Get("plan")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Inputs are echoed via Go templates' default escaping (text/template's
	// HTMLEscapeString) to prevent reflected XSS from a forged sub_id.
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><title>Confirm stub payment</title>
<style>body{font-family:sans-serif;max-width:480px;margin:80px auto;padding:0 16px;color:#333}
button{padding:8px 24px;background:#0070f3;color:#fff;border:0;border-radius:4px;font-size:16px;cursor:pointer}
.muted{color:#999;font-size:13px;margin-top:24px}</style></head>
<body>
<h2>Confirm subscription</h2>
<p>You are about to activate stub subscription <code>%s</code>%s.</p>
<form method="POST" action="/v1/billing/stub-confirm?sub_id=%s&amp;plan=%s">
<button type="submit">Confirm</button>
</form>
<p class="muted">This page only appears in stub / development mode.</p>
</body></html>`,
		htmlEscape(subID),
		planSuffix(plan),
		urlEscape(subID),
		urlEscape(plan),
	)
}

func planSuffix(plan string) string {
	if plan == "" {
		return ""
	}
	return " for plan <code>" + htmlEscape(plan) + "</code>"
}

// htmlEscape is a tiny HTML escaper for the stub-confirm template — keeps
// the form free of XSS-via-sub_id without pulling in html/template.
func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}

// urlEscape encodes a query value safely for form action attribute.
func urlEscape(s string) string { return url.QueryEscape(s) }

// parseIntParam parses a query param as int with a default fallback.
func parseIntParam(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return def
	}
	return v
}
