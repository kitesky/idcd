package billing_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/api/internal/billing"
)

// ---- helper ----

func newStub() *billing.StubProvider {
	return billing.NewStubProvider()
}

var ctx = context.Background()

// ---- Name ----

func TestStubProvider_Name(t *testing.T) {
	p := newStub()
	assert.Equal(t, "stub", p.Name())
}

// ---- Subscribe ----

func TestStubProvider_Subscribe_Success(t *testing.T) {
	p := newStub()
	res, err := p.Subscribe(ctx, billing.SubscribeRequest{
		UserID: "u_abc",
		Plan:   billing.PlanPro,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, res.SubscriptionID)
	assert.Contains(t, res.ExtSubID, "stub_")
	assert.Contains(t, res.PayURL, "/billing/stub-confirm?sub_id=")
	assert.Contains(t, res.PayURL, "plan=pro")
	assert.False(t, res.ExpiresAt.IsZero())
}

func TestStubProvider_Subscribe_AllPlans(t *testing.T) {
	plans := []billing.Plan{billing.PlanFree, billing.PlanPro, billing.PlanTeam, billing.PlanBusiness}
	p := newStub()
	for _, plan := range plans {
		res, err := p.Subscribe(ctx, billing.SubscribeRequest{UserID: "u_x", Plan: plan})
		require.NoError(t, err, "plan=%s", plan)
		assert.NotEmpty(t, res.SubscriptionID)
	}
}

func TestStubProvider_Subscribe_MissingUserID(t *testing.T) {
	p := newStub()
	_, err := p.Subscribe(ctx, billing.SubscribeRequest{Plan: billing.PlanPro})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user_id is required")
}

// Unknown-plan rejection moved to billing.DBPricing.ValidItem covered by
// pricing_test.go. Stub provider intentionally accepts any non-empty plan
// string so unit tests don't need to keep its allow-list in sync.
func TestStubProvider_Subscribe_MissingPlan(t *testing.T) {
	p := newStub()
	_, err := p.Subscribe(ctx, billing.SubscribeRequest{UserID: "u_abc"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plan is required")
}

func TestStubProvider_Subscribe_StartsAsPending(t *testing.T) {
	p := newStub()
	res, err := p.Subscribe(ctx, billing.SubscribeRequest{UserID: "u_abc", Plan: billing.PlanTeam})
	require.NoError(t, err)

	sub, ok := p.GetSubscription(res.SubscriptionID)
	require.True(t, ok)
	assert.Equal(t, "pending", sub.Status)
}

// ---- Cancel ----

func TestStubProvider_Cancel_Success(t *testing.T) {
	p := newStub()
	res, err := p.Subscribe(ctx, billing.SubscribeRequest{UserID: "u_u1", Plan: billing.PlanPro})
	require.NoError(t, err)

	err = p.Cancel(ctx, billing.CancelRequest{
		SubscriptionID: res.SubscriptionID,
		UserID:         "u_u1",
		Reason:         "testing",
	})
	require.NoError(t, err)

	sub, ok := p.GetSubscription(res.SubscriptionID)
	require.True(t, ok)
	assert.Equal(t, "cancelled", sub.Status)
}

func TestStubProvider_Cancel_MissingSubID(t *testing.T) {
	p := newStub()
	err := p.Cancel(ctx, billing.CancelRequest{UserID: "u_u1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "subscription_id is required")
}

func TestStubProvider_Cancel_NotFound(t *testing.T) {
	p := newStub()
	err := p.Cancel(ctx, billing.CancelRequest{SubscriptionID: "sub_nonexistent"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ---- Charge ----

func TestStubProvider_Charge_Success(t *testing.T) {
	p := newStub()
	res, err := p.Charge(ctx, billing.ChargeRequest{
		UserID:      "u_abc",
		AmountCents: 19900,
		Currency:    "CNY",
		Channel:     billing.ChannelAlipay,
		ItemRef:     "v_order_001",
		Description: "Verdict sla 报告",
		Metadata:    map[string]string{"verdict_order_id": "v_order_001"},
	})
	require.NoError(t, err)
	assert.Contains(t, res.ChargeID, "chg_")
	assert.Contains(t, res.ExtTxnID, "stub_ext_")
	assert.Contains(t, res.PayURL, "/billing/stub-charge-confirm?charge_id=")
	assert.Contains(t, res.PayURL, "item_ref=v_order_001")
	assert.False(t, res.ExpiresAt.IsZero())

	c, ok := p.GetCharge(res.ChargeID)
	require.True(t, ok)
	assert.Equal(t, "pending", c.Status)
	assert.Equal(t, int64(19900), c.AmountCents)
	assert.Equal(t, "v_order_001", c.Metadata["verdict_order_id"])
}

func TestStubProvider_Charge_MissingUserID(t *testing.T) {
	p := newStub()
	_, err := p.Charge(ctx, billing.ChargeRequest{AmountCents: 19900})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user_id is required")
}

func TestStubProvider_Charge_ZeroAmount(t *testing.T) {
	p := newStub()
	_, err := p.Charge(ctx, billing.ChargeRequest{UserID: "u_abc", AmountCents: 0})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "amount_cents must be positive")
}

func TestStubProvider_Charge_NegativeAmount(t *testing.T) {
	p := newStub()
	_, err := p.Charge(ctx, billing.ChargeRequest{UserID: "u_abc", AmountCents: -100})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "amount_cents must be positive")
}

func TestStubProvider_Charge_InvalidChannel(t *testing.T) {
	p := newStub()
	_, err := p.Charge(ctx, billing.ChargeRequest{
		UserID:      "u_abc",
		AmountCents: 19900,
		Channel:     "bitcoin",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid channel")
}

// TestStubProvider_Charge_WebhookRoundTrip ensures Metadata travels
// Charge → (out-of-band webhook) → ParseWebhook end-to-end so the billing
// handler can correlate the payment back to the verdict_order.
func TestStubProvider_Charge_WebhookRoundTrip(t *testing.T) {
	p := newStub()
	res, err := p.Charge(ctx, billing.ChargeRequest{
		UserID:      "u_abc",
		AmountCents: 19900,
		Currency:    "CNY",
		Channel:     billing.ChannelAlipay,
		ItemRef:     "v_order_777",
		Metadata:    map[string]string{"verdict_order_id": "v_order_777"},
	})
	require.NoError(t, err)

	// Mock webhook delivery: the gateway sends the same metadata back.
	payload := billing.StubWebhookPayload{
		EventType:   billing.EventPaymentSucceeded,
		ExtTxnID:    res.ExtTxnID,
		AmountCents: 19900,
		Currency:    "CNY",
		UserID:      "u_abc",
		Metadata:    map[string]string{"verdict_order_id": "v_order_777"},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	evt, err := p.ParseWebhook(ctx, body, nil)
	require.NoError(t, err)
	assert.Equal(t, billing.EventPaymentSucceeded, evt.EventType)
	assert.Equal(t, "v_order_777", evt.Metadata["verdict_order_id"])
	assert.Equal(t, "u_abc", evt.UserID)
	assert.Equal(t, res.ExtTxnID, evt.ExtTxnID)
}

// ---- ParseWebhook ----

func TestStubProvider_ParseWebhook_PaymentSucceeded(t *testing.T) {
	p := newStub()
	payload := billing.StubWebhookPayload{
		EventType:      billing.EventPaymentSucceeded,
		ExtTxnID:       "stub_txn_001",
		ExtSubID:       "stub_sub_001",
		AmountCents:    9900,
		Currency:       "CNY",
		UserID:         "u_abc",
		SubscriptionID: "sub_abc",
		Metadata:       map[string]string{"order_id": "ord_001"},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	event, err := p.ParseWebhook(ctx, body, nil)
	require.NoError(t, err)
	assert.Equal(t, billing.EventPaymentSucceeded, event.EventType)
	assert.Equal(t, "stub_txn_001", event.ExtTxnID)
	assert.Equal(t, int64(9900), event.AmountCents)
	assert.Equal(t, "CNY", event.Currency)
	assert.Equal(t, "ord_001", event.Metadata["order_id"])
}

func TestStubProvider_ParseWebhook_EmptyBody(t *testing.T) {
	p := newStub()
	_, err := p.ParseWebhook(ctx, []byte{}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty body")
}

func TestStubProvider_ParseWebhook_InvalidJSON(t *testing.T) {
	p := newStub()
	_, err := p.ParseWebhook(ctx, []byte("not json"), nil)
	assert.Error(t, err)
}

func TestStubProvider_ParseWebhook_MissingEventType(t *testing.T) {
	p := newStub()
	body, _ := json.Marshal(map[string]string{"foo": "bar"})
	_, err := p.ParseWebhook(ctx, body, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "event_type is required")
}

// ---- RefundPayment ----

func TestStubProvider_RefundPayment_Success(t *testing.T) {
	p := newStub()
	err := p.RefundPayment(ctx, "stub_txn_001", 9900, "customer_request")
	require.NoError(t, err)
}

func TestStubProvider_RefundPayment_MissingTxnID(t *testing.T) {
	p := newStub()
	err := p.RefundPayment(ctx, "", 9900, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ext_txn_id is required")
}

func TestStubProvider_RefundPayment_ZeroAmount(t *testing.T) {
	p := newStub()
	err := p.RefundPayment(ctx, "stub_txn_001", 0, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "amount_cents must be positive")
}

// ---- ConfirmSubscription ----

func TestStubProvider_ConfirmSubscription_Success(t *testing.T) {
	p := newStub()
	res, err := p.Subscribe(ctx, billing.SubscribeRequest{UserID: "u_abc", Plan: billing.PlanPro})
	require.NoError(t, err)

	err = p.ConfirmSubscription(res.SubscriptionID)
	require.NoError(t, err)

	sub, ok := p.GetSubscription(res.SubscriptionID)
	require.True(t, ok)
	assert.Equal(t, "active", sub.Status)
}

func TestStubProvider_ConfirmSubscription_NotFound(t *testing.T) {
	p := newStub()
	err := p.ConfirmSubscription("sub_nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStubProvider_ConfirmSubscription_AlreadyCancelled(t *testing.T) {
	p := newStub()
	res, err := p.Subscribe(ctx, billing.SubscribeRequest{UserID: "u_abc", Plan: billing.PlanPro})
	require.NoError(t, err)

	require.NoError(t, p.Cancel(ctx, billing.CancelRequest{SubscriptionID: res.SubscriptionID}))
	err = p.ConfirmSubscription(res.SubscriptionID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already cancelled")
}

// PlanPrice map + ValidPlan removed (pricing now lives in pricing_items table,
// see billing.DBPricing). Equivalent coverage in pricing_test.go:
//   - TestDBPricing_BasePrice_Plan / _VerdictTemplate
//   - TestDBPricing_ValidItem
//   - seed values asserted in seedItemsRows
