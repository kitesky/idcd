package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/kite365/idcd/lib/shared/idgen"
)

// StubProvider implements Provider using in-memory state.
// It is intended for development and testing only — it never calls a real
// payment gateway.
//
// Behaviour:
//   - Subscribe: generates IDs immediately, returns a /billing/stub-confirm URL.
//   - Cancel:    marks the in-memory record as cancelled.
//   - Charge:    generates a chg_xxx + ext_xxx ID and returns a stub PayURL.
//   - ParseWebhook: expects a simple JSON body (StubWebhookPayload).
//   - RefundPayment: always succeeds instantly.
type StubProvider struct {
	mu      sync.RWMutex
	subs    map[string]*StubSubscription // keyed by SubscriptionID
	charges map[string]*StubCharge       // keyed by ChargeID
}

// StubCharge is the in-memory representation of a one-shot charge in the stub provider.
type StubCharge struct {
	ChargeID    string
	ExtTxnID    string
	UserID      string
	AmountCents int64
	Currency    string
	Channel     string
	ItemRef     string
	Description string
	Metadata    map[string]string
	Status      string // "pending" | "succeeded" | "failed"
	ExpiresAt   time.Time
}

// StubSubscription is the in-memory representation of a subscription in the stub provider.
type StubSubscription struct {
	SubscriptionID string
	ExtSubID       string
	UserID         string
	Plan           Plan
	Status         string    // "pending" | "active" | "cancelled"
	ExpiresAt      time.Time
}

// StubWebhookPayload is the JSON body expected by ParseWebhook on the stub provider.
// Real providers would have their own signed formats; this is intentionally simple.
type StubWebhookPayload struct {
	EventType      string            `json:"event_type"`
	ExtTxnID       string            `json:"ext_txn_id"`
	ExtSubID       string            `json:"ext_sub_id"`
	AmountCents    int64             `json:"amount_cents"`
	Currency       string            `json:"currency"`
	UserID         string            `json:"user_id"`
	SubscriptionID string            `json:"subscription_id"`
	Metadata       map[string]string `json:"metadata"`
}

// NewStubProvider creates a new StubProvider with empty in-memory state.
func NewStubProvider() *StubProvider {
	return &StubProvider{
		subs:    make(map[string]*StubSubscription),
		charges: make(map[string]*StubCharge),
	}
}

// Name implements Provider.
func (p *StubProvider) Name() string { return "stub" }

// Subscribe implements Provider.
// It generates an internal subscription ID and a stub confirm URL.
// The subscription starts in status "pending" until stub-confirm is called.
func (p *StubProvider) Subscribe(_ context.Context, req SubscribeRequest) (*SubscribeResult, error) {
	if req.UserID == "" {
		return nil, errors.New("billing/stub: Subscribe: user_id is required")
	}
	if req.Plan == "" {
		return nil, errors.New("billing/stub: Subscribe: plan is required")
	}

	subID := idgen.Subscription()
	extSubID := "stub_" + subID
	expiresAt := time.Now().UTC().Add(30 * 24 * time.Hour)

	record := &StubSubscription{
		SubscriptionID: subID,
		ExtSubID:       extSubID,
		UserID:         req.UserID,
		Plan:           req.Plan,
		Status:         "pending",
		ExpiresAt:      expiresAt,
	}

	p.mu.Lock()
	p.subs[subID] = record
	p.mu.Unlock()

	payURL := fmt.Sprintf("/billing/stub-confirm?sub_id=%s&plan=%s", subID, req.Plan)

	return &SubscribeResult{
		SubscriptionID: subID,
		ExtSubID:       extSubID,
		PayURL:         payURL,
		ExpiresAt:      expiresAt,
	}, nil
}

// Cancel implements Provider.
// Marks the in-memory subscription as cancelled. Returns an error if not found.
func (p *StubProvider) Cancel(_ context.Context, req CancelRequest) error {
	if req.SubscriptionID == "" {
		return errors.New("billing/stub: Cancel: subscription_id is required")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	sub, ok := p.subs[req.SubscriptionID]
	if !ok {
		return fmt.Errorf("billing/stub: Cancel: subscription %q not found", req.SubscriptionID)
	}
	sub.Status = "cancelled"
	return nil
}

// Charge implements Provider.
// It generates an internal charge ID and a stub confirm URL. The charge
// starts in status "pending" until a webhook event flips it. Metadata is
// stored verbatim so ParseWebhook can round-trip it back to callers.
func (p *StubProvider) Charge(_ context.Context, req ChargeRequest) (*ChargeResult, error) {
	if req.UserID == "" {
		return nil, errors.New("billing/stub: Charge: user_id is required")
	}
	if req.AmountCents <= 0 {
		return nil, fmt.Errorf("billing/stub: Charge: amount_cents must be positive, got %d", req.AmountCents)
	}
	if req.Channel != "" && !ValidUserChannel(req.Channel) {
		return nil, fmt.Errorf("billing/stub: Charge: invalid channel %q", req.Channel)
	}

	chargeID := idgen.New("chg_")
	extTxnID := "stub_ext_" + chargeID
	expiresAt := time.Now().UTC().Add(2 * time.Hour)

	// Copy metadata defensively so caller mutations do not bleed into our store.
	metaCopy := make(map[string]string, len(req.Metadata))
	for k, v := range req.Metadata {
		metaCopy[k] = v
	}

	record := &StubCharge{
		ChargeID:    chargeID,
		ExtTxnID:    extTxnID,
		UserID:      req.UserID,
		AmountCents: req.AmountCents,
		Currency:    req.Currency,
		Channel:     req.Channel,
		ItemRef:     req.ItemRef,
		Description: req.Description,
		Metadata:    metaCopy,
		Status:      "pending",
		ExpiresAt:   expiresAt,
	}

	p.mu.Lock()
	p.charges[chargeID] = record
	p.mu.Unlock()

	payURL := fmt.Sprintf("/billing/stub-charge-confirm?charge_id=%s", chargeID)
	if req.ItemRef != "" {
		payURL += "&item_ref=" + req.ItemRef
	}

	return &ChargeResult{
		ChargeID:  chargeID,
		ExtTxnID:  extTxnID,
		PayURL:    payURL,
		ExpiresAt: expiresAt,
	}, nil
}

// GetCharge returns an in-memory charge record (for testing / confirm flow).
func (p *StubProvider) GetCharge(chargeID string) (*StubCharge, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	c, ok := p.charges[chargeID]
	return c, ok
}

// ParseWebhook implements Provider.
// Accepts a JSON body matching StubWebhookPayload; no signature verification.
func (p *StubProvider) ParseWebhook(_ context.Context, body []byte, _ map[string]string) (*WebhookEvent, error) {
	if len(body) == 0 {
		return nil, errors.New("billing/stub: ParseWebhook: empty body")
	}

	var payload StubWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("billing/stub: ParseWebhook: invalid JSON: %w", err)
	}
	if payload.EventType == "" {
		return nil, errors.New("billing/stub: ParseWebhook: event_type is required")
	}

	return &WebhookEvent{
		EventType:      payload.EventType,
		ExtTxnID:       payload.ExtTxnID,
		ExtSubID:       payload.ExtSubID,
		AmountCents:    payload.AmountCents,
		Currency:       payload.Currency,
		UserID:         payload.UserID,
		SubscriptionID: payload.SubscriptionID,
		Metadata:       payload.Metadata,
	}, nil
}

// RefundPayment implements Provider.
// The stub always returns nil (success). A real provider would call an API here.
func (p *StubProvider) RefundPayment(_ context.Context, extTxnID string, amountCents int64, _ string) error {
	if extTxnID == "" {
		return errors.New("billing/stub: RefundPayment: ext_txn_id is required")
	}
	if amountCents <= 0 {
		return fmt.Errorf("billing/stub: RefundPayment: amount_cents must be positive, got %d", amountCents)
	}
	return nil
}

// ConfirmSubscription marks a pending subscription as active (called by stub-confirm handler).
// Returns an error if the subscription is not found or already cancelled.
func (p *StubProvider) ConfirmSubscription(subID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	sub, ok := p.subs[subID]
	if !ok {
		return fmt.Errorf("billing/stub: ConfirmSubscription: subscription %q not found", subID)
	}
	if sub.Status == "cancelled" {
		return fmt.Errorf("billing/stub: ConfirmSubscription: subscription %q is already cancelled", subID)
	}
	sub.Status = "active"
	return nil
}

// GetSubscription returns an in-memory subscription record (for testing/confirm flow).
func (p *StubProvider) GetSubscription(subID string) (*StubSubscription, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	sub, ok := p.subs[subID]
	return sub, ok
}
