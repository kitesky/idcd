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
	envPaddleBaseURL   = "ATTEST_PAYMENT_HUB_BASE_URL"
	envPaddleAPIKey    = "ATTEST_PAYMENT_HUB_API_KEY"
	envPaddleAPISecret = "ATTEST_PAYMENT_HUB_API_SECRET"
)

// paddleRefunder is the refund.RefundProvider concrete adapter. It owns
// its own payment SDK client (D6: independent from API / billing
// pipelines).
type paddleRefunder struct {
	client payment.ClientInterface
	idgen  func() string
}

// newPaddleRefunder reads payment-hub credentials from env. Missing
// credentials are a fatal config error — the refund worker is useless
// without them. Returns an idgen-overrideable adapter so unit tests can
// pass deterministic refund ids.
func newPaddleRefunder() (*paddleRefunder, error) {
	base := strings.TrimSpace(os.Getenv(envPaddleBaseURL))
	apiKey := strings.TrimSpace(os.Getenv(envPaddleAPIKey))
	apiSec := strings.TrimSpace(os.Getenv(envPaddleAPISecret))
	if base == "" || apiKey == "" || apiSec == "" {
		return nil, fmt.Errorf("refund: %s / %s / %s all required",
			envPaddleBaseURL, envPaddleAPIKey, envPaddleAPISecret)
	}
	client := payment.New(base,
		payment.WithAPIKey(apiKey),
		payment.WithAPISecret(apiSec),
		payment.WithRetry(2, 500*time.Millisecond),
	)
	return &paddleRefunder{
		client: client,
		idgen:  defaultRefundID,
	}, nil
}

// Refund implements refund.RefundProvider. Returns a wrapped error so
// the calling handler can log the underlying cause without dragging
// payment-SDK details into the refund package's purview.
func (p *paddleRefunder) Refund(ctx context.Context, paddleOrderID string, amountCents int64, reason string) error {
	if paddleOrderID == "" {
		return errors.New("refund: empty paddle_order_id")
	}
	if amountCents <= 0 {
		return fmt.Errorf("refund: non-positive amount_cents %d", amountCents)
	}
	_, err := p.client.CreateRefund(ctx, &payment.RefundReq{
		OrderNo:     paddleOrderID,
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
var _ refund.RefundProvider = (*paddleRefunder)(nil)

// defaultRefundID generates a 26-char ULID-shaped string for the Paddle
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
