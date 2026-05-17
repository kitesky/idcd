package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	payment "github.com/wangzheng/payment-go-sdk"
)

// --- mock client ---

type mockPaymentClient struct {
	createOrderFn      func(ctx context.Context, req *payment.CreateOrderReq) (*payment.CreateOrderResp, error)
	verifyReceiptFn    func(ctx context.Context, req *payment.VerifyReceiptReq) (*payment.VerifyReceiptResp, error)
	queryOrderFn       func(ctx context.Context, req *payment.QueryOrderReq) (*payment.QueryOrderResp, error)
	closeOrderFn       func(ctx context.Context, req *payment.CloseOrderReq) error
	createRefundFn     func(ctx context.Context, req *payment.RefundReq) (*payment.RefundResp, error)
	queryRefundFn      func(ctx context.Context, req *payment.QueryRefundReq) (*payment.RefundResp, error)
	querySubscriptionFn func(ctx context.Context, req *payment.QuerySubscriptionReq) (*payment.SubscriptionResp, error)
}

func (m *mockPaymentClient) CreateOrder(ctx context.Context, req *payment.CreateOrderReq) (*payment.CreateOrderResp, error) {
	return m.createOrderFn(ctx, req)
}
func (m *mockPaymentClient) VerifyReceipt(ctx context.Context, req *payment.VerifyReceiptReq) (*payment.VerifyReceiptResp, error) {
	return m.verifyReceiptFn(ctx, req)
}
func (m *mockPaymentClient) QueryOrder(ctx context.Context, req *payment.QueryOrderReq) (*payment.QueryOrderResp, error) {
	return m.queryOrderFn(ctx, req)
}
func (m *mockPaymentClient) CloseOrder(ctx context.Context, req *payment.CloseOrderReq) error {
	return m.closeOrderFn(ctx, req)
}
func (m *mockPaymentClient) CreateRefund(ctx context.Context, req *payment.RefundReq) (*payment.RefundResp, error) {
	return m.createRefundFn(ctx, req)
}
func (m *mockPaymentClient) QueryRefund(ctx context.Context, req *payment.QueryRefundReq) (*payment.RefundResp, error) {
	return m.queryRefundFn(ctx, req)
}
func (m *mockPaymentClient) QuerySubscription(ctx context.Context, req *payment.QuerySubscriptionReq) (*payment.SubscriptionResp, error) {
	return m.querySubscriptionFn(ctx, req)
}

// defaultCfg returns a minimal valid PaymentHubConfig.
func defaultCfg() PaymentHubConfig {
	return PaymentHubConfig{
		BaseURL:       "https://pay.example.com",
		APIKey:        "pk_test",
		APISecret:     "sk_test",
		WebhookSecret: "whsec_test",
		Channel:       "paymenthub",
		Currency:      "CNY",
	}
}

// webhookHeaders builds the HMAC-signed headers for a webhook body.
func webhookHeaders(secret string, body []byte) map[string]string {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))
	return map[string]string{
		"X-Webhook-Timestamp": ts,
		"X-Webhook-Signature": sig,
	}
}

// --- Subscribe tests ---

func TestPaymentHubProvider_Name(t *testing.T) {
	p := newPaymentHubProviderWithClient(defaultCfg(), nil)
	assert.Equal(t, "payment_hub", p.Name())
}

func TestPaymentHubProvider_Subscribe_Success(t *testing.T) {
	orderNo := "ORD20260115001"
	checkoutURL := "https://checkout.paymenthub.com/xxx"

	mock := &mockPaymentClient{
		createOrderFn: func(_ context.Context, req *payment.CreateOrderReq) (*payment.CreateOrderResp, error) {
			assert.Equal(t, "paymenthub", req.Channel)
			assert.Equal(t, "checkout", req.Method, "PaymentHub channel should auto-select checkout method")
			assert.Equal(t, PlanPrice[PlanPro], req.Amount)
			assert.Equal(t, "CNY", req.Currency)
			assert.Equal(t, "u_123", req.Metadata["user_id"])
			assert.Equal(t, "pro", req.Metadata["plan"])
			assert.NotEmpty(t, req.Metadata["idcd_sub_id"])
			return &payment.CreateOrderResp{
				OrderNo: orderNo,
				Status:  "pending",
				PayData: map[string]any{"checkout_url": checkoutURL},
			}, nil
		},
	}

	p := newPaymentHubProviderWithClient(defaultCfg(), mock)
	result, err := p.Subscribe(context.Background(), SubscribeRequest{
		UserID:    "u_123",
		Plan:      PlanPro,
		ReturnURL: "https://app.idcd.com/billing",
		NotifyURL: "https://api.idcd.com/v1/billing/webhook",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, result.SubscriptionID)
	assert.Equal(t, orderNo, result.ExtSubID)
	assert.Equal(t, checkoutURL, result.PayURL)
	assert.False(t, result.ExpiresAt.IsZero())
}

func TestPaymentHubProvider_Subscribe_MissingUserID(t *testing.T) {
	p := newPaymentHubProviderWithClient(defaultCfg(), nil)
	_, err := p.Subscribe(context.Background(), SubscribeRequest{Plan: PlanPro})
	assert.ErrorContains(t, err, "user_id is required")
}

func TestPaymentHubProvider_Subscribe_UnknownPlan(t *testing.T) {
	p := newPaymentHubProviderWithClient(defaultCfg(), nil)
	_, err := p.Subscribe(context.Background(), SubscribeRequest{UserID: "u_1", Plan: "enterprise"})
	assert.ErrorContains(t, err, "unknown plan")
}

func TestPaymentHubProvider_Subscribe_APIError(t *testing.T) {
	mock := &mockPaymentClient{
		createOrderFn: func(_ context.Context, _ *payment.CreateOrderReq) (*payment.CreateOrderResp, error) {
			return nil, &payment.APIError{Code: 10001, Message: "bad request"}
		},
	}
	p := newPaymentHubProviderWithClient(defaultCfg(), mock)
	_, err := p.Subscribe(context.Background(), SubscribeRequest{UserID: "u_1", Plan: PlanPro})
	assert.ErrorContains(t, err, "billing/payment_hub: Subscribe")
}

func TestPaymentHubProvider_Subscribe_FallbackPayURL(t *testing.T) {
	mock := &mockPaymentClient{
		createOrderFn: func(_ context.Context, _ *payment.CreateOrderReq) (*payment.CreateOrderResp, error) {
			return &payment.CreateOrderResp{OrderNo: "ORD001", Status: "pending", PayData: nil}, nil
		},
	}
	p := newPaymentHubProviderWithClient(defaultCfg(), mock)
	result, err := p.Subscribe(context.Background(), SubscribeRequest{
		UserID:    "u_1",
		Plan:      PlanPro,
		ReturnURL: "https://app.idcd.com/billing",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://app.idcd.com/billing", result.PayURL)
}

// --- Cancel tests ---

func TestPaymentHubProvider_Cancel_NoOp(t *testing.T) {
	p := newPaymentHubProviderWithClient(defaultCfg(), nil)
	err := p.Cancel(context.Background(), CancelRequest{SubscriptionID: "sub_abc", UserID: "u_1"})
	assert.NoError(t, err)
}

// --- RefundPayment tests ---

func TestPaymentHubProvider_RefundPayment_Success(t *testing.T) {
	mock := &mockPaymentClient{
		createRefundFn: func(_ context.Context, req *payment.RefundReq) (*payment.RefundResp, error) {
			assert.Equal(t, "ORD001", req.OrderNo)
			assert.Equal(t, int64(9900), req.Amount)
			assert.Equal(t, "user request", req.Reason)
			return &payment.RefundResp{RefundNo: "RFD001", Status: "pending"}, nil
		},
	}
	p := newPaymentHubProviderWithClient(defaultCfg(), mock)
	err := p.RefundPayment(context.Background(), "ORD001", 9900, "user request")
	assert.NoError(t, err)
}

func TestPaymentHubProvider_RefundPayment_EmptyTxnID(t *testing.T) {
	p := newPaymentHubProviderWithClient(defaultCfg(), nil)
	err := p.RefundPayment(context.Background(), "", 9900, "")
	assert.ErrorContains(t, err, "ext_txn_id is required")
}

func TestPaymentHubProvider_RefundPayment_ZeroAmount(t *testing.T) {
	p := newPaymentHubProviderWithClient(defaultCfg(), nil)
	err := p.RefundPayment(context.Background(), "ORD001", 0, "")
	assert.ErrorContains(t, err, "amount_cents must be positive")
}

func TestPaymentHubProvider_RefundPayment_APIError(t *testing.T) {
	mock := &mockPaymentClient{
		createRefundFn: func(_ context.Context, _ *payment.RefundReq) (*payment.RefundResp, error) {
			return nil, errors.New("network error")
		},
	}
	p := newPaymentHubProviderWithClient(defaultCfg(), mock)
	err := p.RefundPayment(context.Background(), "ORD001", 1000, "")
	assert.ErrorContains(t, err, "billing/payment_hub: RefundPayment")
}

// --- ParseWebhook tests ---

func buildWebhookBody(t *testing.T, eventType string, data any) []byte {
	t.Helper()
	raw, err := json.Marshal(data)
	require.NoError(t, err)
	body, err := json.Marshal(map[string]any{
		"event_id":   "evt_001",
		"event_type": eventType,
		"created_at": time.Now().Format(time.RFC3339),
		"data":       json.RawMessage(raw),
	})
	require.NoError(t, err)
	return body
}

func TestPaymentHubProvider_ParseWebhook_OrderPaid(t *testing.T) {
	subID := "sub_testABC"
	data := payment.OrderEventData{
		OrderNo:    "ORD001",
		PaidAmount: 9900,
		Currency:   "CNY",
		Metadata:   map[string]string{"idcd_sub_id": subID, "user_id": "u_1", "plan": "pro"},
	}
	body := buildWebhookBody(t, payment.EventOrderPaid, data)
	headers := webhookHeaders("whsec_test", body)

	p := newPaymentHubProviderWithClient(defaultCfg(), nil)
	evt, err := p.ParseWebhook(context.Background(), body, headers)

	require.NoError(t, err)
	assert.Equal(t, EventPaymentSucceeded, evt.EventType)
	assert.Equal(t, "ORD001", evt.ExtTxnID)
	assert.Equal(t, subID, evt.SubscriptionID)
	assert.Equal(t, "u_1", evt.UserID)
	assert.Equal(t, int64(9900), evt.AmountCents)
	assert.Equal(t, "CNY", evt.Currency)
}

func TestPaymentHubProvider_ParseWebhook_RefundSucceeded(t *testing.T) {
	data := payment.RefundEventData{
		OrderNo:      "ORD001",
		RefundAmount: 5000,
		Currency:     "CNY",
	}
	body := buildWebhookBody(t, payment.EventRefundSucceeded, data)
	headers := webhookHeaders("whsec_test", body)

	p := newPaymentHubProviderWithClient(defaultCfg(), nil)
	evt, err := p.ParseWebhook(context.Background(), body, headers)

	require.NoError(t, err)
	assert.Equal(t, EventRefundSucceeded, evt.EventType)
	assert.Equal(t, "ORD001", evt.ExtTxnID)
	assert.Equal(t, int64(5000), evt.AmountCents)
}

func TestPaymentHubProvider_ParseWebhook_RefundFailed(t *testing.T) {
	data := payment.RefundEventData{OrderNo: "ORD001"}
	body := buildWebhookBody(t, payment.EventRefundFailed, data)
	headers := webhookHeaders("whsec_test", body)

	p := newPaymentHubProviderWithClient(defaultCfg(), nil)
	evt, err := p.ParseWebhook(context.Background(), body, headers)

	require.NoError(t, err)
	assert.Equal(t, EventRefundFailed, evt.EventType)
	assert.Equal(t, "ORD001", evt.ExtTxnID)
}

func TestPaymentHubProvider_ParseWebhook_SubscriptionCancelled(t *testing.T) {
	data := payment.SubscriptionEventData{
		SubscriptionID: "SUB_ext_123",
		AppUserID:      "u_1",
	}
	body := buildWebhookBody(t, payment.EventSubscriptionCancelled, data)
	headers := webhookHeaders("whsec_test", body)

	p := newPaymentHubProviderWithClient(defaultCfg(), nil)
	evt, err := p.ParseWebhook(context.Background(), body, headers)

	require.NoError(t, err)
	assert.Equal(t, EventSubscriptionCancelled, evt.EventType)
	assert.Equal(t, "SUB_ext_123", evt.ExtSubID)
	assert.Equal(t, "u_1", evt.UserID)
	// SubscriptionID is empty because renewal-type events don't carry idcd DB key.
	assert.Empty(t, evt.SubscriptionID)
}

func TestPaymentHubProvider_ParseWebhook_InvalidSignature(t *testing.T) {
	body := buildWebhookBody(t, payment.EventOrderPaid, payment.OrderEventData{OrderNo: "ORD001"})
	headers := webhookHeaders("wrong_secret", body)

	p := newPaymentHubProviderWithClient(defaultCfg(), nil)
	_, err := p.ParseWebhook(context.Background(), body, headers)
	assert.ErrorContains(t, err, "invalid signature")
}

func TestPaymentHubProvider_ParseWebhook_MissingHeaders(t *testing.T) {
	body := []byte(`{"event_type":"order.paid"}`)
	p := newPaymentHubProviderWithClient(defaultCfg(), nil)
	_, err := p.ParseWebhook(context.Background(), body, map[string]string{})
	assert.ErrorContains(t, err, "missing signature headers")
}

func TestPaymentHubProvider_ParseWebhook_EmptyBody(t *testing.T) {
	p := newPaymentHubProviderWithClient(defaultCfg(), nil)
	_, err := p.ParseWebhook(context.Background(), nil, map[string]string{
		"X-Webhook-Timestamp": "1234567890",
		"X-Webhook-Signature": "abc",
	})
	assert.ErrorContains(t, err, "empty body")
}

func TestPaymentHubProvider_ParseWebhook_ExpiredTimestamp(t *testing.T) {
	body := buildWebhookBody(t, payment.EventOrderPaid, payment.OrderEventData{})
	oldTs := strconv.FormatInt(time.Now().Unix()-400, 10)
	mac := hmac.New(sha256.New, []byte("whsec_test"))
	mac.Write([]byte(oldTs))
	mac.Write([]byte("."))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	p := newPaymentHubProviderWithClient(defaultCfg(), nil)
	_, err := p.ParseWebhook(context.Background(), body, map[string]string{
		"X-Webhook-Timestamp": oldTs,
		"X-Webhook-Signature": sig,
	})
	assert.ErrorContains(t, err, "timestamp expired")
}

// --- extractPayURL tests ---

func TestExtractPayURL(t *testing.T) {
	cases := []struct {
		name     string
		data     map[string]any
		expected string
	}{
		{"nil", nil, ""},
		{"checkout_url", map[string]any{"checkout_url": "https://checkout.paymenthub.com/x"}, "https://checkout.paymenthub.com/x"},
		{"code_url", map[string]any{"code_url": "weixin://wxpay/..."}, "weixin://wxpay/..."},
		{"empty value", map[string]any{"checkout_url": ""}, ""},
		{"unknown key", map[string]any{"unknown": "https://x.com"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, extractPayURL(tc.data))
		})
	}
}

// --- mapWebhookEvent tests ---

func TestMapWebhookEvent_OrderExpired(t *testing.T) {
	data := payment.OrderEventData{
		OrderNo:  "ORD002",
		Metadata: map[string]string{"idcd_sub_id": "sub_xyz", "user_id": "u_2"},
	}
	raw, _ := json.Marshal(data)
	e := &payment.WebhookEvent{
		EventType: payment.EventOrderExpired,
		Data:      raw,
	}
	evt, err := mapWebhookEvent(e)
	require.NoError(t, err)
	assert.Equal(t, EventPaymentFailed, evt.EventType)
	assert.Equal(t, "sub_xyz", evt.SubscriptionID)
	assert.Equal(t, "u_2", evt.UserID)
}

func TestMapWebhookEvent_UnknownEventType(t *testing.T) {
	e := &payment.WebhookEvent{
		EventType: "some.unknown.event",
		Data:      json.RawMessage(`{}`),
	}
	evt, err := mapWebhookEvent(e)
	require.NoError(t, err)
	assert.Equal(t, "some.unknown.event", evt.EventType)
}

// Ensure PaymentHubProvider satisfies the Provider interface at compile time.
var _ Provider = (*PaymentHubProvider)(nil)

// --- channel selection and method derivation tests ---

func TestPaymentHubProvider_Subscribe_AlipayUsesPageMethod(t *testing.T) {
	var gotChannel, gotMethod string
	mock := &mockPaymentClient{
		createOrderFn: func(_ context.Context, req *payment.CreateOrderReq) (*payment.CreateOrderResp, error) {
			gotChannel = req.Channel
			gotMethod = req.Method
			return &payment.CreateOrderResp{OrderNo: "ORD001", Status: "pending"}, nil
		},
	}
	p := newPaymentHubProviderWithClient(defaultCfg(), mock)

	_, err := p.Subscribe(context.Background(), SubscribeRequest{
		UserID: "u_1", Plan: PlanPro, Channel: "alipay",
	})
	require.NoError(t, err)
	assert.Equal(t, "alipay", gotChannel)
	assert.Equal(t, "page", gotMethod, "Alipay web payment should use page (QR code)")
}

func TestPaymentHubProvider_Subscribe_WeChatUsesNativeMethod(t *testing.T) {
	var gotChannel, gotMethod string
	mock := &mockPaymentClient{
		createOrderFn: func(_ context.Context, req *payment.CreateOrderReq) (*payment.CreateOrderResp, error) {
			gotChannel = req.Channel
			gotMethod = req.Method
			return &payment.CreateOrderResp{OrderNo: "ORD001", Status: "pending"}, nil
		},
	}
	p := newPaymentHubProviderWithClient(defaultCfg(), mock)

	_, err := p.Subscribe(context.Background(), SubscribeRequest{
		UserID: "u_1", Plan: PlanPro, Channel: "wechat_pay",
	})
	require.NoError(t, err)
	assert.Equal(t, "wechat_pay", gotChannel)
	assert.Equal(t, "native", gotMethod, "WeChat web payment should use native (QR code)")
}

func TestPaymentHubProvider_Subscribe_FallsBackToConfigChannel(t *testing.T) {
	var gotChannel, gotMethod string
	mock := &mockPaymentClient{
		createOrderFn: func(_ context.Context, req *payment.CreateOrderReq) (*payment.CreateOrderResp, error) {
			gotChannel = req.Channel
			gotMethod = req.Method
			return &payment.CreateOrderResp{OrderNo: "ORD001", Status: "pending"}, nil
		},
	}
	// defaultCfg has Channel = "paymenthub" → method should be "checkout".
	p := newPaymentHubProviderWithClient(defaultCfg(), mock)

	_, err := p.Subscribe(context.Background(), SubscribeRequest{UserID: "u_1", Plan: PlanPro})
	require.NoError(t, err)
	assert.Equal(t, "paymenthub", gotChannel)
	assert.Equal(t, "checkout", gotMethod)
}

// --- currency default test ---

func TestPaymentHubProvider_Subscribe_DefaultCurrency(t *testing.T) {
	cfg := defaultCfg()
	cfg.Currency = "" // empty → should default to "CNY"

	var gotCurrency string
	mock := &mockPaymentClient{
		createOrderFn: func(_ context.Context, req *payment.CreateOrderReq) (*payment.CreateOrderResp, error) {
			gotCurrency = req.Currency
			return &payment.CreateOrderResp{OrderNo: "ORD001", Status: "pending"}, nil
		},
	}
	p := newPaymentHubProviderWithClient(cfg, mock)
	_, err := p.Subscribe(context.Background(), SubscribeRequest{UserID: "u_1", Plan: PlanPro})
	require.NoError(t, err)
	assert.Equal(t, "CNY", gotCurrency)
}

// --- ExpiresAt from response test ---

func TestPaymentHubProvider_Subscribe_ExpiresAtFromResponse(t *testing.T) {
	expiry := time.Now().UTC().Add(365 * 24 * time.Hour).Truncate(time.Second)
	mock := &mockPaymentClient{
		createOrderFn: func(_ context.Context, _ *payment.CreateOrderReq) (*payment.CreateOrderResp, error) {
			return &payment.CreateOrderResp{
				OrderNo:   "ORD001",
				Status:    "pending",
				ExpiredAt: expiry.Format(time.RFC3339),
			}, nil
		},
	}
	p := newPaymentHubProviderWithClient(defaultCfg(), mock)
	result, err := p.Subscribe(context.Background(), SubscribeRequest{UserID: "u_1", Plan: PlanPro})
	require.NoError(t, err)
	assert.Equal(t, expiry, result.ExpiresAt.UTC().Truncate(time.Second),
		"should use expiry time from payment platform response")
}

// --- Error format test ---

func TestPaymentHubProvider_Subscribe_ErrorWrapping(t *testing.T) {
	sentinel := fmt.Errorf("upstream error")
	mock := &mockPaymentClient{
		createOrderFn: func(_ context.Context, _ *payment.CreateOrderReq) (*payment.CreateOrderResp, error) {
			return nil, sentinel
		},
	}
	p := newPaymentHubProviderWithClient(defaultCfg(), mock)
	_, err := p.Subscribe(context.Background(), SubscribeRequest{UserID: "u_1", Plan: PlanPro})
	assert.ErrorIs(t, err, sentinel)
}
