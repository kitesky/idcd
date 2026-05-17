package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	payment "github.com/wangzheng/payment-go-sdk"

	"github.com/kite365/idcd/apps/attest/internal/refund"
)

// Payment-hub env vars. We deliberately do NOT add these to
// apps/attest/internal/config because they belong to a single binary
// (the refund worker) — keeping the binary's concrete deps local
// matches the verifier / generator pattern in this tree.
const (
	envPaymentHubBaseURL   = "ATTEST_PAYMENT_HUB_BASE_URL"
	envPaymentHubAPIKey    = "ATTEST_PAYMENT_HUB_API_KEY"
	envPaymentHubAPISecret = "ATTEST_PAYMENT_HUB_API_SECRET"
)

// paymentHubRefunder is the refund.RefundProvider concrete adapter. It owns
// its own payment SDK client (D6: independent from API / billing
// pipelines).
type paymentHubRefunder struct {
	client payment.ClientInterface
	idgen  func() string
}

// newPaymentHubRefunder reads payment-hub credentials from env. Missing
// credentials are a fatal config error — the refund worker is useless
// without them. Returns an idgen-overrideable adapter so unit tests can
// pass deterministic refund ids.
func newPaymentHubRefunder() (*paymentHubRefunder, error) {
	base := strings.TrimSpace(os.Getenv(envPaymentHubBaseURL))
	apiKey := strings.TrimSpace(os.Getenv(envPaymentHubAPIKey))
	apiSec := strings.TrimSpace(os.Getenv(envPaymentHubAPISecret))
	if base == "" || apiKey == "" || apiSec == "" {
		return nil, fmt.Errorf("refund: %s / %s / %s all required",
			envPaymentHubBaseURL, envPaymentHubAPIKey, envPaymentHubAPISecret)
	}
	client := payment.New(base,
		payment.WithAPIKey(apiKey),
		payment.WithAPISecret(apiSec),
		payment.WithRetry(2, 500*time.Millisecond),
	)
	return &paymentHubRefunder{
		client: client,
		idgen:  defaultRefundID,
	}, nil
}

// Refund implements refund.RefundProvider. Returns a wrapped error so
// the calling handler can log the underlying cause without dragging
// payment-SDK details into the refund package's purview.
func (p *paymentHubRefunder) Refund(ctx context.Context, extOrderID string, amountCents int64, reason string) error {
	if extOrderID == "" {
		return errors.New("refund: empty ext_order_id")
	}
	if amountCents <= 0 {
		return fmt.Errorf("refund: non-positive amount_cents %d", amountCents)
	}
	_, err := p.client.CreateRefund(ctx, &payment.RefundReq{
		OrderNo:     extOrderID,
		AppRefundID: p.idgen(),
		Amount:      amountCents,
		Reason:      reason,
	})
	if err != nil {
		return fmt.Errorf("payment-hub refund: %w", err)
	}
	return nil
}

// Compile-time interface check.
var _ refund.RefundProvider = (*paymentHubRefunder)(nil)

// defaultRefundID generates a 26-char ULID-shaped string for the PaymentHub
// app_refund_id. Importing lib/shared/idgen pulls in a separate go.work
// module; for this binary the inline implementation keeps the
// dependency graph trivial.
func defaultRefundID() string {
	return fmt.Sprintf("rfd_%d", time.Now().UnixNano())
}

// defaultJSONMarshal is the seam for apologyPayload encoding. Wrapped
// so tests can override if we ever need to. Defined here (rather than
// inline in main.go) to keep main.go focused on wiring.
func defaultJSONMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}
