// Package billing defines the payment provider interface and shared types.
// All payment providers (stub, wepay, alipay, stripe, etc.) must implement the
// Provider interface so business logic never depends on a concrete provider.
package billing

import (
	"context"
	"time"
)

// Plan 订阅档位
type Plan string

const (
	PlanFree      Plan = "free"
	PlanPro       Plan = "pro"
	PlanAgentPro  Plan = "agent_pro"
	PlanTeam      Plan = "team"
	PlanBusiness  Plan = "business"
)

// PlanPrice 各档位月费（分，人民币）
var PlanPrice = map[Plan]int64{
	PlanFree:     0,
	PlanPro:      9900,  // ¥99
	PlanAgentPro: 29900, // ¥299
	PlanTeam:     29900, // ¥299
	PlanBusiness: 99900, // ¥999
}

// ValidPlan returns true if p is a recognised plan identifier.
func ValidPlan(p Plan) bool {
	_, ok := PlanPrice[p]
	return ok
}

// User-selectable payment channels for web payments.
// Paddle is intentionally excluded — it is config-only (international cards, admin-initiated).
const (
	ChannelAlipay    = "alipay"
	ChannelWeChatPay = "wechat_pay"
	ChannelPaddle    = "paddle"
)

// UserChannels are the channels users can select on the web checkout page.
// Both use QR code scanning; method is determined by the provider, not the user.
var UserChannels = map[string]struct{}{
	ChannelAlipay:    {},
	ChannelWeChatPay: {},
}

// ValidUserChannel reports whether ch is a user-selectable channel.
func ValidUserChannel(ch string) bool {
	_, ok := UserChannels[ch]
	return ok
}

// SubscribeRequest 创建订阅请求
type SubscribeRequest struct {
	UserID  string
	Plan    Plan
	// Channel is the user-chosen payment channel: "alipay" or "wechat_pay".
	// Empty means use the provider's configured default.
	Channel string
	ReturnURL string // 支付完成后跳转
	NotifyURL string // 异步通知 URL
}

// SubscribeResult 创建订阅结果
type SubscribeResult struct {
	SubscriptionID string    // 我们系统的 sub_xxx ID
	ExtSubID       string    // 支付商的订阅 ID
	PayURL         string    // 拉起支付的 URL
	ExpiresAt      time.Time // 当前周期结束时间（UTC）
}

// CancelRequest 取消订阅请求
type CancelRequest struct {
	SubscriptionID string
	UserID         string
	Reason         string
}

// ChargeRequest 一次性收款（非订阅）。Verdict 报告 / 单次报告订单走它。
type ChargeRequest struct {
	UserID      string
	AmountCents int64
	Currency    string // "CNY" / "USD"
	Channel     string // alipay / wechat_pay
	ReturnURL   string
	NotifyURL   string

	// ItemRef 是业务订单 id（如 v_xxx）。透传到 Provider，部分通道
	// 会把它放回 metadata 让 webhook 找到上下文。
	ItemRef string

	// Description 在支付页显示给用户。
	Description string

	// Metadata 由调用方塞业务字段（如 verdict_order_id）。
	// Provider 必须把 Metadata 透传到 webhook event.Metadata。
	Metadata map[string]string
}

// ChargeResult 一次性收款结果。
type ChargeResult struct {
	ChargeID  string    // 我们系统的 chg_xxx
	ExtTxnID  string    // 支付商交易 id
	PayURL    string    // 拉起支付的 URL
	ExpiresAt time.Time
}

// WebhookEvent 支付回调事件（各支付商 webhook 解析后统一为此结构）
type WebhookEvent struct {
	// EventType is one of the EventType* constants below.
	EventType string
	// ExtTxnID is the provider-side transaction ID.
	ExtTxnID string
	// ExtSubID is the provider-side subscription ID.
	ExtSubID string
	// AmountCents is the amount in the smallest currency unit (fen for CNY).
	AmountCents int64
	// Currency is the ISO 4217 currency code, e.g. "CNY".
	Currency string
	// UserID is our system's user identifier.
	UserID string
	// SubscriptionID is our system's subscription identifier.
	SubscriptionID string
	// Metadata contains provider-specific key-value pairs.
	Metadata map[string]string
}

// Recognised EventType values.
const (
	EventPaymentSucceeded      = "payment.succeeded"
	EventPaymentFailed         = "payment.failed"
	EventSubscriptionCancelled = "subscription.cancelled"
	EventRefundSucceeded       = "refund.succeeded"
	EventRefundFailed          = "refund.failed"
)

// Provider is the payment provider abstraction.
// Implementations must be safe for concurrent use.
type Provider interface {
	// Name returns a short identifier for this provider, e.g. "stub".
	Name() string

	// Subscribe initiates a subscription and returns a payment URL.
	Subscribe(ctx context.Context, req SubscribeRequest) (*SubscribeResult, error)

	// Cancel cancels an active subscription.
	Cancel(ctx context.Context, req CancelRequest) error

	// Charge initiates a one-shot payment (Verdict reports, single
	// invoice purchases). Implementations MUST round-trip Metadata
	// into WebhookEvent.Metadata so the caller can correlate.
	Charge(ctx context.Context, req ChargeRequest) (*ChargeResult, error)

	// ParseWebhook parses a provider webhook HTTP body into a WebhookEvent.
	// headers contains the raw request headers (for signature verification).
	ParseWebhook(ctx context.Context, body []byte, headers map[string]string) (*WebhookEvent, error)

	// RefundPayment initiates a refund for the given provider transaction.
	// amountCents is the amount to refund (in the smallest currency unit).
	RefundPayment(ctx context.Context, extTxnID string, amountCents int64, reason string) error
}
