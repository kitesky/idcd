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
//   - ParseWebhook: expects a simple JSON body (StubWebhookPayload).
//   - RefundPayment: always succeeds instantly.
type StubProvider struct {
	mu   sync.RWMutex
	subs map[string]*StubSubscription // keyed by SubscriptionID
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
		subs: make(map[string]*StubSubscription),
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
	if !ValidPlan(req.Plan) {
		return nil, fmt.Errorf("billing/stub: Subscribe: unknown plan %q", req.Plan)
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
