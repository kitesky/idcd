// Package paymenthub implements the attest-svc PaymentHub webhook endpoint that
// drives D5 refund processing: when PaymentHub reports a refund event the
// attest service transitions the matching verdict_order to "refunded"
// and, on transient DB failure, enqueues the order onto the
// `refund_retry_queue` Redis Stream so the separate retry scheduler can
// drain it (5min / 30min cadence per DECISIONS.md §M D5).
//
// Signature verification is HMAC-SHA256 of "<timestamp>.<body>" using
// the webhook secret, identical to apps/api/internal/handler/billing.go;
// the helper is duplicated here rather than imported because
// apps/attest must not depend on apps/api.
package paymenthub

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/kite365/idcd/lib/shared/contracts"
	sharedstream "github.com/kite365/idcd/lib/shared/stream"
)

// Recognised PaymentHub event types. Anything else is acked with 200 and
// logged at debug; PaymentHub does not retry 2xx so unknown events stop
// flowing.
const (
	EventTransactionPaymentRefunded  = "transaction.payment_refunded"
	EventTransactionRefunded         = "transaction.refunded"
	EventSubscriptionPaymentRefunded = "subscription.payment_refunded"
)

// RefundRetryStream is the Redis Stream key the retry scheduler reads.
// Fields per entry: order_id, ext_event_id, attempt, scheduled_at.
//
// Consumer: apps/attest/cmd/refund-worker. The worker translates each
// entry into a delay-zone ZSET member keyed by (order_id, attempt) and
// runs the D5 5min / 30min retry ladder + apology email + refund_failed
// flip from there.
//
// 真值集中在 lib/shared/stream.RefundRetryQueue — 此处只是本地别名,
// 兼容 tests + 外部包对 paymenthub.RefundRetryStream 的引用。
const RefundRetryStream = sharedstream.RefundRetryQueue

// retryFirstDelay is the 5-minute delay applied to the first retry,
// matching D5.
const retryFirstDelay = 5 * time.Minute

// webhookReplayWindow caps how stale a webhook timestamp may be before
// we treat it as a replay. Matches apps/api/internal/billing
// webhookReplayWindowSecs (5 min) on both sides.
const webhookReplayWindow = 5 * time.Minute

// maxBodyBytes caps PaymentHub webhook payloads; events are sub-KB so 1 MiB
// is generous and prevents trivial memory DoS on the public endpoint.
const maxBodyBytes = 1 << 20

// OrderLookup returns the verdict_order id matching ext_order_id,
// together with the current status. Implementations live in main; the
// interface keeps this package independent of the repo layer so tests
// can fake it.
type OrderLookup interface {
	LookupByExtOrderID(ctx context.Context, extOrderID string) (orderID, status string, err error)
}

// ErrOrderNotFound is the sentinel OrderLookup implementations return
// when no verdict_order row matches the ext_order_id.
var ErrOrderNotFound = errors.New("paymenthub: verdict_order not found")

// OrderStatusUpdater is the subset of *repo.VerdictOrdersRepo this
// handler needs to flip status. *repo.VerdictOrdersRepo satisfies it.
type OrderStatusUpdater interface {
	UpdateStatus(ctx context.Context, id, fromStatus, toStatus string, errReason *string) error
}

// RetryEnqueuer pushes one entry onto the refund_retry_queue Redis Stream.
// *sharedstream.Client satisfies it via AddRefundRetryTyped; the interface
// keeps tests miniredis-friendly without forcing the package to know the
// concrete client type at every call site.
//
// P0-4 W3: switched from raw redis.XAdd → typed contract. Field-name
// typos in the producer now fail at compile time instead of silently
// dropping retry tickets (钱 / 合规关键路径, D5).
type RetryEnqueuer interface {
	AddRefundRetryTyped(ctx context.Context, e contracts.RefundRetryEvent) (string, error)
}

// Handler implements POST /webhooks/paymenthub.
//
// All fields except Logger and Now are required; constructing with any
// of Secret / Orders / Lookup / Redis nil will panic on first request.
// Logger defaults to slog.Default() and Now to time.Now.
type Handler struct {
	Secret []byte
	Lookup OrderLookup
	Orders OrderStatusUpdater
	Redis  RetryEnqueuer
	Logger *slog.Logger
	Now    func() time.Time
}

// ServeHTTP routes POST /webhooks/paymenthub. Anything else returns 405.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
	if err != nil {
		h.logger().Warn("paymenthub webhook: read body failed", "err", err)
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	if len(body) == 0 || int64(len(body)) > maxBodyBytes {
		http.Error(w, "invalid body length", http.StatusBadRequest)
		return
	}

	timestamp := r.Header.Get("X-Webhook-Timestamp")
	signature := r.Header.Get("X-Webhook-Signature")
	if !verifySignature(h.Secret, timestamp, body, signature, h.now()) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var evt event
	if err := json.Unmarshal(body, &evt); err != nil {
		h.logger().Warn("paymenthub webhook: malformed JSON", "err", err)
		// Still 200 — replaying malformed events is pointless.
		writeOK(w)
		return
	}

	ctx := r.Context()
	switch evt.EventType {
	case EventTransactionPaymentRefunded,
		EventTransactionRefunded,
		EventSubscriptionPaymentRefunded:
		h.processRefund(ctx, &evt)
	default:
		h.logger().Debug("paymenthub webhook: ignoring event", "event_type", evt.EventType)
	}

	writeOK(w)
}

// processRefund looks up the verdict_order, transitions to refunded,
// and on failure enqueues a refund_retry_queue entry. It NEVER returns
// an error — the webhook ack is always 200 for valid signatures so
// PaymentHub does not retry on transient DB issues; the retry queue is the
// authoritative recovery path.
func (h *Handler) processRefund(ctx context.Context, evt *event) {
	extOrderID := evt.Data.ExtOrderID
	if extOrderID == "" {
		h.logger().Warn("paymenthub webhook: refund event missing paymenthub order id",
			"event_id", evt.EventID, "event_type", evt.EventType)
		return
	}

	orderID, status, err := h.Lookup.LookupByExtOrderID(ctx, extOrderID)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			h.logger().Info("paymenthub webhook: no verdict_order for paymenthub id",
				"ext_order_id", extOrderID, "event_id", evt.EventID)
			return
		}
		h.enqueueRetry(ctx, extOrderID, evt.EventID)
		h.logger().Warn("paymenthub webhook: lookup failed",
			"ext_order_id", extOrderID, "err", err)
		return
	}

	updateErr := h.Orders.UpdateStatus(ctx, orderID, status, "refunded", nil)
	if updateErr != nil {
		h.logger().Warn("paymenthub webhook: UpdateStatus failed; enqueueing retry",
			"order_id", orderID, "from", status, "err", updateErr)
		h.enqueueRetry(ctx, orderID, evt.EventID)
		return
	}
	h.logger().Info("paymenthub webhook: verdict_order refunded",
		"order_id", orderID, "ext_order_id", extOrderID)
}

// enqueueRetry pushes one entry onto refund_retry_queue via the typed
// contracts.RefundRetryEvent path. The refund-worker (separate process)
// consumes from this stream and drives 5min/30min retries plus the
// T+15min apology email per D5.
//
// P0-4 W3: prior to the typed contract, this called rdb.XAdd directly with
// a map[string]any literal. A field-name typo (e.g. "ext_evnt_id") would
// silently slip past tests until a real retry produced a malformed entry
// that the consumer dropped. Now the field set lives in contracts.RefundRetryEvent
// and any drift fails at compile time.
func (h *Handler) enqueueRetry(ctx context.Context, orderID, extEventID string) {
	now := h.now()
	evt := contracts.RefundRetryEvent{
		OrderID:     orderID,
		ExtEventID:  extEventID,
		Attempt:     1,
		ScheduledAt: now.Add(retryFirstDelay).UTC(),
	}
	if _, err := h.Redis.AddRefundRetryTyped(ctx, evt); err != nil {
		h.logger().Error("paymenthub webhook: enqueue refund_retry_queue failed",
			"order_id", orderID, "err", err)
	}
}

func (h *Handler) logger() *slog.Logger {
	if h.Logger != nil {
		return h.Logger
	}
	return slog.Default()
}

func (h *Handler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

// event is the minimal PaymentHub webhook payload we consume. PaymentHub sends
// significantly more data per event; we only decode the fields used to
// drive the refund pipeline.
type event struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"`
	Data      eventData `json:"data"`
}

type eventData struct {
	ExtOrderID string `json:"ext_order_id"`
}

// verifySignature is HMAC-SHA256("<timestamp>.<body>", secret) hex,
// mirroring apps/api/internal/billing/paymenthub.go ParseWebhook so the
// same secret deploys both endpoints. Replay window is ±5 minutes
// against the caller-supplied "now".
func verifySignature(secret []byte, timestamp string, body []byte, signature string, now time.Time) bool {
	if len(secret) == 0 || timestamp == "" || signature == "" {
		return false
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	t := time.Unix(ts, 0)
	if now.Sub(t) > webhookReplayWindow || t.Sub(now) > webhookReplayWindow {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func writeOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
