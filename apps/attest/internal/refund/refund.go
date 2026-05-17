// Package refund implements the D5 refund worker handler — the consumer
// side that drains the two Redis Streams populated in S2:
//
//   - refund_initiate_queue (apps/attest/cmd/verifier/refund_enqueue.go):
//     Self-Verify catches a bad PDF and asks us to refund the user.
//   - refund_retry_queue (apps/attest/internal/handler/paddle/paddle.go):
//     a Paddle webhook arrived but our outbound refund call failed; the
//     first retry hint lands here.
//
// Both paths converge on the same delay-zone-driven retry ladder
// (DECISIONS.md §M D5): 5min first retry, 30min second retry, then
// either "refunded" (any retry succeeded) or "refund_failed" + apology
// email. Two failed retries are the absolute cap; we never loop
// forever because D11 (12h Shamir SOP) makes prolonged automatic
// retries pointless — operations takes over via the admin dashboard.
//
// Design notes:
//
//   - This package has zero direct Redis / Postgres dependencies. The
//     concrete adapters live in cmd/refund-worker. Tests against the
//     two public Handle* methods plus TickDelayZone run entirely on
//     in-memory fakes.
//   - The two stream handlers are intentionally tiny: they normalise the
//     input fields, persist any "first attempt failed" state, and ZADD
//     into the delay zone. The actual retry work happens in
//     processOrder, called only from TickDelayZone — that keeps retry
//     timing centralized in one place (the tick) instead of scattered
//     across the two producers.
//   - D6: refund worker never imports lib/attest/sign. It does not
//     create attestation records. It only mutates verdict_order rows
//     and calls Paddle. Nothing it does is part of the chain of
//     custody for the original verdict, so there is no WAL to write.
package refund

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// MaxAttempts is the cap on automatic Paddle refund retries. D5: after
// two failed retries we flip the order to refund_failed and send the
// apology email — operators handle the rest from the admin dashboard.
const MaxAttempts = 2

// FirstRetryDelay is the wait between the initial failure and attempt 1.
const FirstRetryDelay = 5 * time.Minute

// SecondRetryDelay is the wait between attempt 1 and attempt 2 (≈30min
// after the original failure when stacked on FirstRetryDelay).
const SecondRetryDelay = 25 * time.Minute

// DefaultDelayZoneKey is the Redis ZSET key holding scheduled retries.
// Members are encoded "<order_id>|<attempt>"; score is the unix-nanos at
// which the retry becomes eligible.
const DefaultDelayZoneKey = "refund_delay_zone"

// DefaultTickInterval is how often TickDelayZone scans for due members.
// 30 s gives a worst-case tail of ≈30 s on top of the 5 / 30 min cadence
// — plenty given the SLA is 1h for P0 and 24h for ordinary refunds.
const DefaultTickInterval = 30 * time.Second

// DefaultTickBatchSize caps how many members one TickDelayZone iteration
// processes. 100 is more than enough for the projected S2 volume
// (<50 refunds/day) and keeps each iteration's wall-clock bounded.
const DefaultTickBatchSize = 100

// memberSep is the delimiter between order_id and attempt in delay-zone
// members. order_id has prefix "v_" + ULID-style suffix and never
// contains "|" so the split is unambiguous.
const memberSep = "|"

// Reason strings stamped into refund_last_error so the admin dashboard
// (D5: refund_failed → P0) and any future SLA report can group failures
// by surface.
const (
	failureReasonPaddleAPI = "paddle refund api"
)

// Order is the projection refund worker needs. Kept narrow so the
// in-process fake the tests use does not have to reconstruct the full
// repo.Order struct.
type Order struct {
	ID              string
	Status          string
	PaddleOrderID   string
	PriceCNYYuan    float64
	RefundAttempts  int
	RefundedAt      *time.Time
	ApologySentAt   *time.Time
}

// PriceCents converts the order's PriceCNYYuan into integer cents for
// the Paddle API. We round half-away-from-zero so a 0.005-yuan rounding
// glitch in price_paid_cny upstream cannot silently drop a fen.
func (o *Order) PriceCents() int64 {
	return int64(math.Round(o.PriceCNYYuan * 100))
}

// IsTerminal returns true when the order has already reached a terminal
// refund state — refunded or refund_failed. The retry tick uses it to
// skip duplicate processing without holding any cross-tick lock.
func (o *Order) IsTerminal() bool {
	return o.Status == StatusRefunded || o.Status == StatusRefundFailed
}

// Verdict-order status values relevant to the refund worker. Duplicated
// here (rather than imported from repo) so this package stays free of
// the pgx dependency tree and miniredis-based tests do not drag pgx
// into their build.
const (
	StatusRefunded     = "refunded"
	StatusRefundFailed = "refund_failed"
)

// OrderStore is the narrow persistence surface the handler needs. The
// concrete implementation lives in cmd/refund-worker as a thin adapter
// around *repo.VerdictOrdersRepo + *repo.VerdictReportsRepo (the
// "report_id → order" hop is an application-level join per D1).
type OrderStore interface {
	// GetByReportID resolves the verdict_order behind a verdict_report.
	// Used by the initiate path (Self-Verify only knows the report id).
	GetByReportID(ctx context.Context, reportID string) (*Order, error)

	// GetByID fetches an order directly. Used by the retry tick to
	// re-read status before each Paddle call (idempotency / race guard).
	GetByID(ctx context.Context, orderID string) (*Order, error)

	// MarkRefunded transitions the order to status='refunded' and
	// stamps refunded_at = now. The current status is supplied so the
	// optimistic-lock UPDATE in the repo can reject stale writes.
	MarkRefunded(ctx context.Context, orderID, fromStatus string, at time.Time) error

	// MarkRefundFailed transitions to status='refund_failed' and stores
	// the apology-ready failure reason. D5: this state escalates to the
	// admin dashboard + P0 alert.
	MarkRefundFailed(ctx context.Context, orderID, fromStatus, errReason string) error

	// BumpRefundAttempt records that one more Paddle call failed and
	// stores the latest error. Caller-supplied attempt count is the new
	// (post-bump) value, which gives tests a deterministic post-state
	// independent of the repo's read-modify-write.
	BumpRefundAttempt(ctx context.Context, orderID, errReason string, newAttempt int) error

	// MarkApologySent stamps refund_apology_sent_at. Called once per
	// terminal refund_failed, immediately after a successful enqueue
	// onto the notifier queue.
	MarkApologySent(ctx context.Context, orderID string, at time.Time) error
}

// ErrOrderNotFound is the sentinel OrderStore implementations return
// when a lookup matches zero rows. The handler swallows it (log + ACK)
// because there is no recovery — the producer either sent us a stale
// id or the row was hard-deleted out from under us.
var ErrOrderNotFound = errors.New("refund: order not found")

// RefundProvider is the narrow Paddle-side dependency. The production
// adapter wraps billing.PaymentHubProvider.RefundPayment; tests pass an
// in-memory mock.
type RefundProvider interface {
	Refund(ctx context.Context, paddleOrderID string, amountCents int64, reason string) error
}

// ApologyMailer enqueues a single "we're sorry, the automated refund
// failed" email task. The concrete adapter publishes to the notifier
// asynq queue (handler.QueueBilling, task type "payment:refund_apology"
// — see cmd/refund-worker/main.go).
//
// We deliberately separate the apology enqueue from MarkApologySent: a
// failure to enqueue the email must leave refund_apology_sent_at NULL
// so the next operator sweep can re-send. The Mailer is therefore
// expected to be idempotent on its own side (task de-duplication keyed
// on order_id).
type ApologyMailer interface {
	SendApology(ctx context.Context, orderID, reason string) error
}

// ZSetClient is the minimum of *redis.Client the handler needs. Keeping
// the surface narrow lets the test suite plug miniredis without any
// awkward type assertions or shimmed clients.
type ZSetClient interface {
	ZAdd(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd
	ZRangeByScoreWithScores(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.ZSliceCmd
	ZRem(ctx context.Context, key string, members ...any) *redis.IntCmd
}

// Config bundles everything Handler needs. Mailer is optional: when nil
// the apology step degrades to "log only", which is the correct
// fail-open posture for early S2 deployments where the notifier
// integration has not landed yet. Logger is optional and falls back to
// slog.Default.
type Config struct {
	Orders   OrderStore
	Refunder RefundProvider
	Mailer   ApologyMailer
	Redis    ZSetClient

	DelayZoneKey string
	Logger       *slog.Logger
	Now          func() time.Time
}

// Handler executes the D5 refund worker logic. It is safe to invoke
// HandleInitiate, HandleRetryEnqueue, and TickDelayZone concurrently
// from independent goroutines; each operation re-reads order state
// before mutating, so two ticks picking the same delay-zone member at
// nearly the same time still produce a single Paddle call (the second
// finds status='refunded' and skips).
type Handler struct {
	cfg Config
}

// New validates cfg and returns a Handler. Required fields: Orders,
// Refunder, Redis. Missing required fields panic — refund worker is
// the only thing this binary does, so any wiring bug should fail at
// process start.
func New(cfg Config) *Handler {
	if cfg.Orders == nil {
		panic("refund: Orders is required")
	}
	if cfg.Refunder == nil {
		panic("refund: Refunder is required")
	}
	if cfg.Redis == nil {
		panic("refund: Redis is required")
	}
	if strings.TrimSpace(cfg.DelayZoneKey) == "" {
		cfg.DelayZoneKey = DefaultDelayZoneKey
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Handler{cfg: cfg}
}

// ----- helpers ------------------------------------------------------------

// encodeMember formats an order_id + attempt pair as a delay-zone member.
// Returned strings are stable and safe to embed in Redis members (no
// separator collision with order_id which is "v_" + base32).
func encodeMember(orderID string, attempt int) string {
	return orderID + memberSep + strconv.Itoa(attempt)
}

// parseMember decodes a delay-zone member produced by encodeMember.
// Returns an error for malformed inputs — the handler ACKs and skips
// rather than poison-pilling itself on garbage that some operator may
// have ZADDed by hand.
func parseMember(m string) (orderID string, attempt int, err error) {
	idx := strings.LastIndex(m, memberSep)
	if idx < 0 || idx == 0 || idx == len(m)-1 {
		return "", 0, fmt.Errorf("refund: malformed member %q", m)
	}
	id := m[:idx]
	a, parseErr := strconv.Atoi(m[idx+1:])
	if parseErr != nil {
		return "", 0, fmt.Errorf("refund: malformed attempt in %q: %w", m, parseErr)
	}
	if a < 1 {
		return "", 0, fmt.Errorf("refund: non-positive attempt in %q", m)
	}
	return id, a, nil
}

// retryDelay returns the wait before the given attempt should run.
// attempt is 1-indexed: delay(1) is the gap between the initial failure
// and the first retry; delay(2) is the gap between the first and second
// retry. Higher attempts never schedule (handled by the caller as
// terminal refund_failed) but the helper still returns SecondRetryDelay
// as a safe fallback so unit tests cannot index out of bounds.
func retryDelay(attempt int) time.Duration {
	switch attempt {
	case 1:
		return FirstRetryDelay
	case 2:
		return SecondRetryDelay
	default:
		return SecondRetryDelay
	}
}

// stringField returns the value of a stream-entry field as a string.
// XREADGROUP delivers values as interface{} (either string or []byte
// depending on the client). We accept both transparently.
func stringField(fields map[string]any, key string) string {
	v, ok := fields[key]
	if !ok {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return fmt.Sprintf("%v", s)
	}
}

// scheduleScore turns a UTC time into a delay-zone score. We use unix
// nanos so order-of-arrival between members scheduled within the same
// second is preserved (a microbatch of failures cannot collapse into a
// random visit order).
func scheduleScore(t time.Time) float64 {
	return float64(t.UTC().UnixNano())
}

// parseScheduledAt parses an RFC3339Nano timestamp from a stream entry.
// Producers (the Paddle webhook handler) emit RFC3339Nano; we tolerate
// RFC3339 too for forward compatibility.
func parseScheduledAt(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, errors.New("refund: empty scheduled_at")
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

// ----- public methods -----------------------------------------------------

// HandleInitiate consumes a refund_initiate_queue entry. Self-Verify
// caught a bad PDF; we attempt the first Paddle refund inline and, on
// failure, ZADD the order onto the delay zone for retry attempt 1.
//
// Returning a non-nil error leaves the message un-ACKed; the consumer
// loop logs it and Redis redelivers on the next iteration. Per the
// D4-aligned idempotency posture we make the handler ACK-safe: every
// state mutation is conditional on the order's current status, so a
// redelivered message resolves to a no-op when the previous delivery
// already advanced the state.
func (h *Handler) HandleInitiate(ctx context.Context, fields map[string]any) error {
	reportID := stringField(fields, "report_id")
	reason := stringField(fields, "reason")
	if reportID == "" {
		h.cfg.Logger.Warn("refund: initiate entry missing report_id")
		return nil
	}

	order, err := h.cfg.Orders.GetByReportID(ctx, reportID)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			h.cfg.Logger.Info("refund: no order for report", "report_id", reportID)
			return nil
		}
		return fmt.Errorf("refund initiate lookup %s: %w", reportID, err)
	}
	if order.IsTerminal() {
		h.cfg.Logger.Info("refund: initiate skipped — order already terminal",
			"order_id", order.ID, "status", order.Status)
		return nil
	}

	now := h.cfg.Now()
	refundErr := h.cfg.Refunder.Refund(ctx, order.PaddleOrderID, order.PriceCents(), reason)
	if refundErr == nil {
		if err := h.cfg.Orders.MarkRefunded(ctx, order.ID, order.Status, now); err != nil {
			return fmt.Errorf("refund initiate mark refunded %s: %w", order.ID, err)
		}
		h.cfg.Logger.Info("refund: initiate succeeded",
			"order_id", order.ID, "report_id", reportID)
		return nil
	}

	// First attempt failed — bump the counter, persist the error, and
	// schedule attempt 1 in FirstRetryDelay.
	if err := h.cfg.Orders.BumpRefundAttempt(ctx, order.ID, refundErr.Error(), 1); err != nil {
		return fmt.Errorf("refund initiate bump attempt %s: %w", order.ID, err)
	}
	if err := h.scheduleRetry(ctx, order.ID, 1, now.Add(FirstRetryDelay)); err != nil {
		return fmt.Errorf("refund initiate schedule retry %s: %w", order.ID, err)
	}
	h.cfg.Logger.Warn("refund: initiate failed; scheduled retry",
		"order_id", order.ID, "report_id", reportID, "err", refundErr)
	return nil
}

// HandleRetryEnqueue consumes a refund_retry_queue entry. The Paddle
// webhook handler produces here when the inline UpdateStatus / lookup
// path failed; we translate that producer hint into a delay-zone
// entry and ACK. All retry work then funnels through the tick goroutine.
func (h *Handler) HandleRetryEnqueue(ctx context.Context, fields map[string]any) error {
	orderID := stringField(fields, "order_id")
	if orderID == "" {
		h.cfg.Logger.Warn("refund: retry entry missing order_id")
		return nil
	}
	attemptStr := stringField(fields, "attempt")
	attempt, err := strconv.Atoi(attemptStr)
	if err != nil || attempt < 1 {
		// Producer guarantees attempt="1" on first hop; anything else is
		// a malformed entry that we cannot safely retry. ACK + log.
		h.cfg.Logger.Warn("refund: retry entry has bad attempt",
			"order_id", orderID, "attempt_raw", attemptStr)
		return nil
	}
	scheduledRaw := stringField(fields, "scheduled_at")
	scheduledAt, err := parseScheduledAt(scheduledRaw)
	if err != nil {
		h.cfg.Logger.Warn("refund: retry entry has bad scheduled_at",
			"order_id", orderID, "scheduled_at_raw", scheduledRaw, "err", err)
		return nil
	}
	if err := h.scheduleRetry(ctx, orderID, attempt, scheduledAt); err != nil {
		return fmt.Errorf("refund retry enqueue %s: %w", orderID, err)
	}
	h.cfg.Logger.Info("refund: retry enqueued",
		"order_id", orderID, "attempt", attempt, "due_at", scheduledAt.UTC().Format(time.RFC3339Nano))
	return nil
}

// TickDelayZone scans the delay zone once and processes every member
// whose scheduled time is in the past. Intended to be called on a
// 30 s timer from the worker process; safe to call manually from tests.
//
// Returns the count of members processed (regardless of refund outcome)
// so the caller can emit metrics or back off when the queue is empty.
func (h *Handler) TickDelayZone(ctx context.Context) (int, error) {
	now := h.cfg.Now()
	members, err := h.cfg.Redis.ZRangeByScoreWithScores(ctx, h.cfg.DelayZoneKey, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    strconv.FormatFloat(scheduleScore(now), 'f', -1, 64),
		Offset: 0,
		Count:  int64(DefaultTickBatchSize),
	}).Result()
	if err != nil {
		return 0, fmt.Errorf("refund tick zrange: %w", err)
	}
	if len(members) == 0 {
		return 0, nil
	}

	processed := 0
	for _, z := range members {
		memberStr, ok := z.Member.(string)
		if !ok {
			h.cfg.Logger.Warn("refund: tick saw non-string member", "type", fmt.Sprintf("%T", z.Member))
			_ = h.cfg.Redis.ZRem(ctx, h.cfg.DelayZoneKey, z.Member).Err()
			continue
		}

		orderID, attempt, parseErr := parseMember(memberStr)
		if parseErr != nil {
			h.cfg.Logger.Warn("refund: tick saw malformed member",
				"member", memberStr, "err", parseErr)
			_ = h.cfg.Redis.ZRem(ctx, h.cfg.DelayZoneKey, memberStr).Err()
			continue
		}

		// Atomically claim the member: ZREM returns 1 if we removed it,
		// 0 if another tick beat us to it. Two ticks picking the same
		// member is possible (no Lua script here), but only one of them
		// gets the ZREM success and proceeds.
		removed, remErr := h.cfg.Redis.ZRem(ctx, h.cfg.DelayZoneKey, memberStr).Result()
		if remErr != nil {
			h.cfg.Logger.Warn("refund: tick zrem failed",
				"member", memberStr, "err", remErr)
			continue
		}
		if removed == 0 {
			continue
		}

		if err := h.processRetry(ctx, orderID, attempt); err != nil {
			h.cfg.Logger.Error("refund: tick retry failed",
				"order_id", orderID, "attempt", attempt, "err", err)
			// Reschedule with the same delay so a transient DB hiccup
			// doesn't drop the work. Use the original attempt — we did
			// not actually consume a retry slot.
			next := h.cfg.Now().Add(retryDelay(attempt))
			if rerr := h.scheduleRetry(ctx, orderID, attempt, next); rerr != nil {
				h.cfg.Logger.Error("refund: tick reschedule failed",
					"order_id", orderID, "attempt", attempt, "err", rerr)
			}
		}
		processed++
	}
	return processed, nil
}

// scheduleRetry ZADDs one delay-zone member. Idempotent — duplicate
// ZADD on the same member just updates the score (which is fine: the
// later schedule wins).
func (h *Handler) scheduleRetry(ctx context.Context, orderID string, attempt int, dueAt time.Time) error {
	cmd := h.cfg.Redis.ZAdd(ctx, h.cfg.DelayZoneKey, redis.Z{
		Score:  scheduleScore(dueAt),
		Member: encodeMember(orderID, attempt),
	})
	if err := cmd.Err(); err != nil {
		return fmt.Errorf("refund schedule retry zadd: %w", err)
	}
	return nil
}

// processRetry runs one retry attempt drawn from the delay zone. The
// attempt number is "which attempt is this", 1-indexed; 1 is the first
// retry after the initial failure, 2 is the second (and last).
func (h *Handler) processRetry(ctx context.Context, orderID string, attempt int) error {
	order, err := h.cfg.Orders.GetByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			h.cfg.Logger.Info("refund: tick saw missing order; dropping",
				"order_id", orderID)
			return nil
		}
		return fmt.Errorf("get order: %w", err)
	}
	if order.IsTerminal() {
		h.cfg.Logger.Info("refund: tick skipped — order already terminal",
			"order_id", orderID, "status", order.Status)
		return nil
	}

	now := h.cfg.Now()
	refundErr := h.cfg.Refunder.Refund(ctx, order.PaddleOrderID, order.PriceCents(), failureReasonPaddleAPI)
	if refundErr == nil {
		if err := h.cfg.Orders.MarkRefunded(ctx, order.ID, order.Status, now); err != nil {
			return fmt.Errorf("mark refunded: %w", err)
		}
		h.cfg.Logger.Info("refund: retry succeeded",
			"order_id", orderID, "attempt", attempt)
		return nil
	}

	// Refund call failed again. Decide between scheduling the next
	// attempt and flipping to refund_failed.
	if attempt < MaxAttempts {
		nextAttempt := attempt + 1
		if err := h.cfg.Orders.BumpRefundAttempt(ctx, orderID, refundErr.Error(), nextAttempt); err != nil {
			return fmt.Errorf("bump attempt: %w", err)
		}
		nextDue := now.Add(retryDelay(nextAttempt))
		if err := h.scheduleRetry(ctx, orderID, nextAttempt, nextDue); err != nil {
			return fmt.Errorf("schedule next: %w", err)
		}
		h.cfg.Logger.Warn("refund: retry failed; scheduled next",
			"order_id", orderID, "attempt", attempt, "next_attempt", nextAttempt)
		return nil
	}

	// Terminal failure — flip status, send apology, log P0.
	if err := h.cfg.Orders.MarkRefundFailed(ctx, orderID, order.Status, refundErr.Error()); err != nil {
		return fmt.Errorf("mark refund_failed: %w", err)
	}
	h.cfg.Logger.Error("refund: terminal failure; P0",
		"order_id", orderID, "attempt", attempt, "err", refundErr,
		"alert", "refund_failed")
	if h.cfg.Mailer != nil {
		if err := h.cfg.Mailer.SendApology(ctx, orderID, refundErr.Error()); err != nil {
			// Apology enqueue failed — log loud but do NOT flip
			// refund_apology_sent_at. The next operator sweep can
			// re-send; the order is already in refund_failed which
			// drives the admin dashboard alert.
			h.cfg.Logger.Error("refund: apology enqueue failed",
				"order_id", orderID, "err", err)
			return nil
		}
		if err := h.cfg.Orders.MarkApologySent(ctx, orderID, h.cfg.Now()); err != nil {
			h.cfg.Logger.Error("refund: stamp apology_sent_at failed",
				"order_id", orderID, "err", err)
		}
	} else {
		h.cfg.Logger.Warn("refund: no apology mailer wired; user not notified",
			"order_id", orderID)
	}
	return nil
}
