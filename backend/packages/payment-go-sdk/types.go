package payment

import "encoding/json"

// Channel constants
const (
	ChannelWeChatPay  = "wechat_pay"
	ChannelAlipay     = "alipay"
	ChannelPaymentHub     = "paymenthub"
	ChannelAppleIAP   = "apple_iap"
	ChannelGooglePlay = "google_play"
)

// WeChat Pay methods
const (
	MethodWeChatNative      = "native"
	MethodWeChatJSAPI       = "jsapi"
	MethodWeChatH5          = "h5"
	MethodWeChatApp         = "app"
	MethodWeChatMiniProgram = "mini_program"
)

// Alipay methods
const (
	MethodAlipayPage = "page"
	MethodAlipayWap  = "wap"
	MethodAlipayApp  = "app"
	MethodAlipayFace = "face"
)

// Product types
const (
	ProductTypeConsumable    = "consumable"
	ProductTypeNonConsumable = "non_consumable"
	ProductTypeSubscription  = "subscription"
	ProductTypeOneTime       = "one_time"
)

// Order status
const (
	OrderStatusPending           = "pending"
	OrderStatusPaying            = "paying"
	OrderStatusVerifying         = "verifying"
	OrderStatusPaid              = "paid"
	OrderStatusClosed            = "closed"
	OrderStatusExpired           = "expired"
	OrderStatusFailed            = "failed"
	OrderStatusVerifyFailed      = "verify_failed"
	OrderStatusRefunding         = "refunding"
	OrderStatusRefunded          = "refunded"
	OrderStatusPartiallyRefunded = "partially_refunded"
)

// Refund status
const (
	RefundStatusPending   = "pending"
	RefundStatusSucceeded = "succeeded"
	RefundStatusFailed    = "failed"
)

// Subscription status
const (
	SubscriptionStatusActive      = "active"
	SubscriptionStatusCancelled   = "cancelled"
	SubscriptionStatusExpired     = "expired"
	SubscriptionStatusGracePeriod = "grace_period"
)

// Webhook event types
const (
	EventOrderPaid               = "order.paid"
	EventOrderClosed             = "order.closed"
	EventOrderExpired            = "order.expired"
	EventRefundSucceeded         = "refund.succeeded"
	EventRefundFailed            = "refund.failed"
	EventSubscriptionActivated   = "subscription.activated"
	EventSubscriptionRenewed     = "subscription.renewed"
	EventSubscriptionCancelled   = "subscription.cancelled"
	EventSubscriptionExpired     = "subscription.expired"
	EventSubscriptionGracePeriod = "subscription.grace_period"
	EventSubscriptionRevoked     = "subscription.revoked"
)

// --- Request types ---

type CreateOrderReq struct {
	AppOrderID    string            `json:"app_order_id"`
	Channel       string            `json:"channel"`
	Method        string            `json:"method"`
	Amount        int64             `json:"amount"`
	Currency      string            `json:"currency"`
	Subject       string            `json:"subject"`
	Body          string            `json:"body,omitempty"`
	ProductID     string            `json:"product_id,omitempty"`
	ClientIP      string            `json:"client_ip,omitempty"`
	ReturnURL     string            `json:"return_url,omitempty"`
	OpenID        string            `json:"openid,omitempty"`
	ExpireSeconds int               `json:"expire_seconds,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type CreateOrderResp struct {
	OrderNo        string                 `json:"order_no"`
	ChannelOrderID string                 `json:"channel_order_id,omitempty"`
	Status         string                 `json:"status"`
	PayData        map[string]interface{} `json:"pay_data"`
	ExpiredAt      string                 `json:"expired_at,omitempty"`
}

type VerifyReceiptReq struct {
	AppOrderID    string            `json:"app_order_id"`
	Channel       string            `json:"channel"`
	ProductID     string            `json:"product_id"`
	ProductType   string            `json:"product_type"`
	ReceiptData   string            `json:"receipt_data"`
	TransactionID string            `json:"transaction_id,omitempty"`
	PackageName   string            `json:"package_name,omitempty"`
	Amount        int64             `json:"amount"`
	Currency      string            `json:"currency"`
	Sandbox       bool              `json:"sandbox,omitempty"`
	AppUserID     string            `json:"app_user_id,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type VerifyReceiptResp struct {
	OrderNo               string `json:"order_no"`
	Status                string `json:"status"`
	Verified              bool   `json:"verified"`
	OriginalTransactionID string `json:"original_transaction_id,omitempty"`
	ProductID             string `json:"product_id,omitempty"`
	ProductType           string `json:"product_type,omitempty"`
	ExpiresAt             string `json:"expires_at,omitempty"`
	IsTrialPeriod         bool   `json:"is_trial_period,omitempty"`
	Environment           string `json:"environment,omitempty"`
	RejectReason          string `json:"reject_reason,omitempty"`
}

type QueryOrderReq struct {
	OrderNo    string `json:"order_no,omitempty"`
	AppOrderID string `json:"app_order_id,omitempty"`
}

type QueryOrderResp struct {
	OrderNo        string            `json:"order_no"`
	AppOrderID     string            `json:"app_order_id"`
	Status         string            `json:"status"`
	Amount         int64             `json:"amount"`
	PaidAmount     int64             `json:"paid_amount,omitempty"`
	Currency       string            `json:"currency"`
	Channel        string            `json:"channel"`
	ChannelOrderID string            `json:"channel_order_id,omitempty"`
	PaidAt         string            `json:"paid_at,omitempty"`
	ExpiredAt      string            `json:"expired_at,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	CreatedAt      string            `json:"created_at,omitempty"`
}

type CloseOrderReq struct {
	OrderNo    string `json:"order_no,omitempty"`
	AppOrderID string `json:"app_order_id,omitempty"`
}

type RefundReq struct {
	OrderNo     string `json:"order_no,omitempty"`
	AppOrderID  string `json:"app_order_id,omitempty"`
	AppRefundID string `json:"app_refund_id,omitempty"`
	Amount      int64  `json:"amount"`
	Reason      string `json:"reason"`
}

type RefundResp struct {
	RefundNo        string  `json:"refund_no"`
	OrderNo         string  `json:"order_no"`
	AppRefundID     string  `json:"app_refund_id,omitempty"`
	Amount          int64   `json:"amount"`
	Currency        string  `json:"currency,omitempty"`
	Status          string  `json:"status"`
	ChannelRefundID string  `json:"channel_refund_id,omitempty"`
	Reason          string  `json:"reason,omitempty"`
	FailedReason    string  `json:"failed_reason,omitempty"`
	RefundedAt      string  `json:"refunded_at,omitempty"`
	CreatedAt       string  `json:"created_at,omitempty"`
}

type QueryRefundReq struct {
	RefundNo    string `json:"refund_no,omitempty"`
	AppRefundID string `json:"app_refund_id,omitempty"`
}

type QuerySubscriptionReq struct {
	AppUserID string `json:"app_user_id,omitempty"`
	ProductID string `json:"product_id,omitempty"`
	Status    string `json:"status,omitempty"`
}

type SubscriptionResp struct {
	SubscriptionID        string `json:"subscription_id,omitempty"`
	OriginalTransactionID string `json:"original_transaction_id"`
	ChannelProductID      string `json:"channel_product_id,omitempty"`
	Status                string `json:"status"`
	AutoRenew             bool   `json:"auto_renew"`
	Currency              string `json:"currency,omitempty"`
	PriceAmount           int64  `json:"price_amount,omitempty"`
	CurrentPeriodStart    string `json:"current_period_start,omitempty"`
	CurrentPeriodEnd      string `json:"current_period_end,omitempty"`
	ExpiresAt             string `json:"expires_at,omitempty"`
	PurchasedAt           string `json:"purchased_at,omitempty"`
	CancelledAt           string `json:"cancelled_at,omitempty"`
	ProductID             string `json:"product_id,omitempty"`
}

// --- Webhook types ---

type WebhookEvent struct {
	EventID   string          `json:"event_id"`
	EventType string          `json:"event_type"`
	CreatedAt string          `json:"created_at"`
	Data      json.RawMessage `json:"data"`
	RawBody   []byte          `json:"-"`
}

type OrderEventData struct {
	OrderNo        string            `json:"order_no"`
	AppOrderID     string            `json:"app_order_id"`
	Channel        string            `json:"channel"`
	Method         string            `json:"method"`
	Amount         int64             `json:"amount"`
	Currency       string            `json:"currency"`
	PaidAmount     int64             `json:"paid_amount"`
	Status         string            `json:"status"`
	PaidAt         string            `json:"paid_at"`
	ChannelOrderID string            `json:"channel_order_id"`
	ProductID      string            `json:"product_id"`
	Metadata       map[string]string `json:"metadata"`
}

type RefundEventData struct {
	RefundNo     string `json:"refund_no"`
	OrderNo      string `json:"order_no"`
	AppOrderID   string `json:"app_order_id"`
	AppRefundID  string `json:"app_refund_id"`
	Channel      string `json:"channel"`
	Amount       int64  `json:"amount"`
	RefundAmount int64  `json:"refund_amount"`
	Currency     string `json:"currency"`
	Status       string `json:"status"`
	RefundedAt   string `json:"refunded_at"`
	Reason       string `json:"reason"`
}

type SubscriptionEventData struct {
	SubscriptionID        string `json:"subscription_id"`
	OriginalTransactionID string `json:"original_transaction_id"`
	Channel               string `json:"channel"`
	ProductID             string `json:"product_id"`
	Status                string `json:"status"`
	AutoRenew             bool   `json:"auto_renew"`
	CurrentPeriodStart    string `json:"current_period_start"`
	CurrentPeriodEnd      string `json:"current_period_end"`
	RenewalCount          int    `json:"renewal_count"`
	AppUserID             string `json:"app_user_id"`
}

// OrderData parses the webhook event data as an order event.
func (e *WebhookEvent) OrderData() (*OrderEventData, error) {
	var d OrderEventData
	if err := json.Unmarshal(e.Data, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// RefundData parses the webhook event data as a refund event.
func (e *WebhookEvent) RefundData() (*RefundEventData, error) {
	var d RefundEventData
	if err := json.Unmarshal(e.Data, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// SubscriptionData parses the webhook event data as a subscription event.
func (e *WebhookEvent) SubscriptionData() (*SubscriptionEventData, error) {
	var d SubscriptionEventData
	if err := json.Unmarshal(e.Data, &d); err != nil {
		return nil, err
	}
	return &d, nil
}
