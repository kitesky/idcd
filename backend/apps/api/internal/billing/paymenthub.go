package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"time"

	payment "github.com/wangzheng/payment-go-sdk"

	"github.com/kite365/idcd/lib/shared/idgen"
)

// webhookReplayWindowSecs is the ±tolerance for the X-Webhook-Timestamp header.
// Events older or newer than this window are rejected to prevent replay attacks.
const webhookReplayWindowSecs = int64(300) // ±5 minutes

// PaymentHubConfig holds credentials and routing config for the payment platform.
type PaymentHubConfig struct {
	BaseURL       string
	APIKey        string
	APISecret     string
	WebhookSecret string
	// Channel is the default channel when the user does not specify one:
	// "alipay", "wechat_pay", or "paymenthub".
	Channel string
	// Currency is ISO 4217, e.g. "CNY" or "USD".
	Currency string
}

// PaymentHubProvider implements Provider by calling the payment aggregation platform.
// It is safe for concurrent use.
type PaymentHubProvider struct {
	client payment.ClientInterface
	cfg    PaymentHubConfig
}

// NewPaymentHubProvider wires a PaymentHubProvider with the given config.
func NewPaymentHubProvider(cfg PaymentHubConfig) *PaymentHubProvider {
	client := payment.New(cfg.BaseURL,
		payment.WithAPIKey(cfg.APIKey),
		payment.WithAPISecret(cfg.APISecret),
		payment.WithRetry(2, 500*time.Millisecond),
	)
	return &PaymentHubProvider{client: client, cfg: cfg}
}

// newPaymentHubProviderWithClient is used in tests to inject a mock client.
func newPaymentHubProviderWithClient(cfg PaymentHubConfig, client payment.ClientInterface) *PaymentHubProvider {
	return &PaymentHubProvider{client: client, cfg: cfg}
}

// Name implements Provider.
func (p *PaymentHubProvider) Name() string { return "payment_hub" }

// Subscribe implements Provider.
// It creates an order on the payment platform and returns the checkout URL.
// The returned SubscriptionID is an idcd-generated sub_xxx ID embedded in the
// order metadata so webhook events can reference the same DB row.
func (p *PaymentHubProvider) Subscribe(ctx context.Context, req SubscribeRequest) (*SubscribeResult, error) {
	if req.UserID == "" {
		return nil, errors.New("billing/payment_hub: Subscribe: user_id is required")
	}
	if req.Plan == "" {
		return nil, errors.New("billing/payment_hub: Subscribe: plan is required")
	}
	if req.AmountCents <= 0 {
		return nil, fmt.Errorf("billing/payment_hub: Subscribe: amount_cents must be positive, got %d", req.AmountCents)
	}

	currency := req.Currency
	if currency == "" {
		currency = p.cfg.Currency
	}
	if currency == "" {
		currency = "CNY"
	}

	// Channel: user's choice takes priority; fall back to config default.
	channel := req.Channel
	if channel == "" {
		channel = p.cfg.Channel
	}
	// Method is derived from channel automatically; unknown channels get "checkout".
	method := channelWebMethod(channel)
	if method == "" {
		method = "checkout"
	}

	// Generate a stable idcd subscription ID upfront; embed it in metadata so
	// order.paid webhook events can look it up without an extra DB round-trip.
	subID := idgen.Subscription()
	appOrderID := idgen.Order()

	// Merge caller metadata (so internal keys take precedence). Lets handler
	// pass promotion_id / coupon_code so they round-trip via webhook.
	metadata := make(map[string]string, len(req.Metadata)+3)
	maps.Copy(metadata, req.Metadata)
	metadata["idcd_sub_id"] = subID
	metadata["user_id"] = req.UserID
	metadata["plan"] = string(req.Plan)

	resp, err := p.client.CreateOrder(ctx, &payment.CreateOrderReq{
		AppOrderID: appOrderID,
		Channel:    channel,
		Method:     method,
		Amount:     req.AmountCents,
		Currency:   currency,
		Subject:    "idcd " + string(req.Plan),
		ProductID:  string(req.Plan),
		ReturnURL:  req.ReturnURL,
		Metadata:   metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("billing/payment_hub: Subscribe: %w", err)
	}

	payURL := extractPayURL(resp.PayData)
	if payURL == "" {
		payURL = req.ReturnURL
	}

	expiresAt := time.Now().UTC().Add(30 * 24 * time.Hour)
	if resp.ExpiredAt != "" {
		if t, err := time.Parse(time.RFC3339, resp.ExpiredAt); err == nil {
			expiresAt = t
		}
	}

	return &SubscribeResult{
		SubscriptionID: subID,
		ExtSubID:       resp.OrderNo,
		PayURL:         payURL,
		ExpiresAt:      expiresAt,
	}, nil
}

// Cancel implements Provider.
// The payment platform does not expose a public subscription-cancel endpoint;
// PaymentHub subscriptions are cancelled via PaymentHub's customer portal. The DB state
// is updated when the subscription.cancelled webhook arrives.
func (p *PaymentHubProvider) Cancel(_ context.Context, _ CancelRequest) error {
	return nil
}

// Charge implements Provider.
// It creates a one-shot order on the payment platform and returns the checkout
// URL. The caller-supplied Metadata is merged with idcd-internal keys and
// passed through verbatim so order.paid webhooks round-trip every entry back
// to the BillingHandler (e.g. verdict_order_id).
func (p *PaymentHubProvider) Charge(ctx context.Context, req ChargeRequest) (*ChargeResult, error) {
	if req.UserID == "" {
		return nil, errors.New("billing/payment_hub: Charge: user_id is required")
	}
	if req.AmountCents <= 0 {
		return nil, fmt.Errorf("billing/payment_hub: Charge: amount_cents must be positive, got %d", req.AmountCents)
	}

	currency := req.Currency
	if currency == "" {
		currency = p.cfg.Currency
	}
	if currency == "" {
		currency = "CNY"
	}

	channel := req.Channel
	if channel == "" {
		channel = p.cfg.Channel
	}
	method := channelWebMethod(channel)
	if method == "" {
		method = "checkout"
	}

	chargeID := idgen.New("chg_")
	appOrderID := idgen.Order()

	// Merge caller metadata first (so internal keys take precedence).
	metadata := make(map[string]string, len(req.Metadata)+3)
	maps.Copy(metadata, req.Metadata)
	metadata["idcd_charge_id"] = chargeID
	metadata["user_id"] = req.UserID
	if req.ItemRef != "" {
		metadata["item_ref"] = req.ItemRef
	}

	subject := req.Description
	if subject == "" {
		subject = "idcd charge"
	}

	resp, err := p.client.CreateOrder(ctx, &payment.CreateOrderReq{
		AppOrderID: appOrderID,
		Channel:    channel,
		Method:     method,
		Amount:     req.AmountCents,
		Currency:   currency,
		Subject:    subject,
		ProductID:  req.ItemRef,
		ReturnURL:  req.ReturnURL,
		Metadata:   metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("billing/payment_hub: Charge: %w", err)
	}

	payURL := extractPayURL(resp.PayData)
	if payURL == "" {
		payURL = req.ReturnURL
	}

	expiresAt := time.Now().UTC().Add(2 * time.Hour)
	if resp.ExpiredAt != "" {
		if t, err := time.Parse(time.RFC3339, resp.ExpiredAt); err == nil {
			expiresAt = t
		}
	}

	return &ChargeResult{
		ChargeID:  chargeID,
		ExtTxnID:  resp.OrderNo,
		PayURL:    payURL,
		ExpiresAt: expiresAt,
	}, nil
}

// ParseWebhook implements Provider.
// It verifies the HMAC-SHA256 signature (X-Webhook-Timestamp / X-Webhook-Signature)
// and maps the payment platform event to a billing.WebhookEvent.
func (p *PaymentHubProvider) ParseWebhook(_ context.Context, body []byte, headers map[string]string) (*WebhookEvent, error) {
	if len(body) == 0 {
		return nil, errors.New("billing/payment_hub: ParseWebhook: empty body")
	}

	timestamp := headers["X-Webhook-Timestamp"]
	signature := headers["X-Webhook-Signature"]
	if timestamp == "" || signature == "" {
		return nil, errors.New("billing/payment_hub: ParseWebhook: missing signature headers")
	}

	// Timestamp replay check (±5 min).
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("billing/payment_hub: ParseWebhook: invalid timestamp: %w", err)
	}
	now := time.Now().Unix()
	if now < ts-webhookReplayWindowSecs || now > ts+webhookReplayWindowSecs {
		return nil, errors.New("billing/payment_hub: ParseWebhook: timestamp expired")
	}

	// HMAC-SHA256(secret, timestamp + "." + body)
	mac := hmac.New(sha256.New, []byte(p.cfg.WebhookSecret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return nil, errors.New("billing/payment_hub: ParseWebhook: invalid signature")
	}

	var raw payment.WebhookEvent
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("billing/payment_hub: ParseWebhook: %w", err)
	}

	return mapWebhookEvent(&raw)
}

// RefundPayment implements Provider.
// extTxnID is the payment platform's OrderNo for the payment to refund.
func (p *PaymentHubProvider) RefundPayment(ctx context.Context, extTxnID string, amountCents int64, reason string) error {
	if extTxnID == "" {
		return errors.New("billing/payment_hub: RefundPayment: ext_txn_id is required")
	}
	if amountCents <= 0 {
		return fmt.Errorf("billing/payment_hub: RefundPayment: amount_cents must be positive, got %d", amountCents)
	}

	_, err := p.client.CreateRefund(ctx, &payment.RefundReq{
		OrderNo:     extTxnID,
		AppRefundID: idgen.Refund(),
		Amount:      amountCents,
		Reason:      reason,
	})
	if err != nil {
		return fmt.Errorf("billing/payment_hub: RefundPayment: %w", err)
	}
	return nil
}

// mapWebhookEvent converts a payment SDK event to a billing.WebhookEvent.
func mapWebhookEvent(e *payment.WebhookEvent) (*WebhookEvent, error) {
	evt := &WebhookEvent{
		Metadata: make(map[string]string),
	}

	switch e.EventType {
	case payment.EventOrderPaid:
		data, err := e.OrderData()
		if err != nil {
			return nil, fmt.Errorf("billing/payment_hub: parse order data: %w", err)
		}
		evt.EventType = EventPaymentSucceeded
		evt.ExtTxnID = data.OrderNo
		evt.ExtSubID = data.OrderNo
		evt.AmountCents = data.PaidAmount
		evt.Currency = data.Currency
		maps.Copy(evt.Metadata, data.Metadata)
		// idcd_sub_id was embedded in metadata by Subscribe(); use it as the DB key.
		if subID, ok := data.Metadata["idcd_sub_id"]; ok {
			evt.SubscriptionID = subID
		}
		if userID, ok := data.Metadata["user_id"]; ok {
			evt.UserID = userID
		}

	case payment.EventOrderClosed, payment.EventOrderExpired:
		data, err := e.OrderData()
		if err != nil {
			return nil, fmt.Errorf("billing/payment_hub: parse order data: %w", err)
		}
		evt.EventType = EventPaymentFailed
		evt.ExtTxnID = data.OrderNo
		if subID, ok := data.Metadata["idcd_sub_id"]; ok {
			evt.SubscriptionID = subID
		}
		if userID, ok := data.Metadata["user_id"]; ok {
			evt.UserID = userID
		}

	case payment.EventRefundSucceeded:
		data, err := e.RefundData()
		if err != nil {
			return nil, fmt.Errorf("billing/payment_hub: parse refund data: %w", err)
		}
		evt.EventType = EventRefundSucceeded
		evt.ExtTxnID = data.OrderNo
		evt.AmountCents = data.RefundAmount
		evt.Currency = data.Currency

	case payment.EventRefundFailed:
		data, err := e.RefundData()
		if err != nil {
			return nil, fmt.Errorf("billing/payment_hub: parse refund data: %w", err)
		}
		evt.EventType = EventRefundFailed
		evt.ExtTxnID = data.OrderNo

	case payment.EventSubscriptionActivated, payment.EventSubscriptionRenewed:
		data, err := e.SubscriptionData()
		if err != nil {
			return nil, fmt.Errorf("billing/payment_hub: parse subscription data: %w", err)
		}
		evt.EventType = EventPaymentSucceeded
		evt.ExtSubID = data.SubscriptionID
		evt.UserID = data.AppUserID
		// SubscriptionID intentionally left empty: at renewal time, the
		// subscription platform ID differs from the initial order_no that we
		// use as the DB primary key. Subscription state is already updated via
		// the accompanying order.paid event; this event is therefore a no-op.

	case payment.EventSubscriptionCancelled, payment.EventSubscriptionExpired, payment.EventSubscriptionRevoked:
		data, err := e.SubscriptionData()
		if err != nil {
			return nil, fmt.Errorf("billing/payment_hub: parse subscription data: %w", err)
		}
		evt.EventType = EventSubscriptionCancelled
		evt.ExtSubID = data.SubscriptionID
		evt.UserID = data.AppUserID
		// SubscriptionID left empty (same reason as above); the billing handler
		// skips the DB update when SubscriptionID is "".

	default:
		// Unknown event — return it with a pass-through event type so callers
		// can log or ignore it without treating it as an error.
		evt.EventType = e.EventType
	}

	return evt, nil
}

// channelWebMethod returns the appropriate payment method for web QR-code flows.
// Both Alipay "page" and WeChat "native" render a QR code the user scans.
func channelWebMethod(channel string) string {
	switch channel {
	case ChannelAlipay:
		return "page" // PC 扫码/跳转页面
	case ChannelWeChatPay:
		return "native" // 生成二维码
	case ChannelPaymentHub:
		return "checkout"
	default:
		return ""
	}
}

// extractPayURL pulls the payment redirect URL from the pay_data map.
// The key varies by channel/method.
func extractPayURL(payData map[string]any) string {
	if payData == nil {
		return ""
	}
	candidates := []string{
		"checkout_url", // PaymentHub
		"pay_url",
		"code_url",     // WeChat Native (QR code)
		"h5_url",       // WeChat H5
		"wap_url",      // Alipay WAP
		"page_url",     // Alipay Page
		"qr_code_url",
	}
	for _, key := range candidates {
		if v, ok := payData[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}
