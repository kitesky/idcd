package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"net/url"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kite365/idcd/apps/notifier/internal/channel"
	"github.com/kite365/idcd/apps/notifier/internal/email"
	"github.com/kite365/idcd/apps/notifier/internal/template"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/asynqtask"
	"github.com/kite365/idcd/lib/shared/i18n"
)

// 通知器自有的 email task 类型（与外部 producer 无关，本服务内部 producer/consumer）。
const (
	TaskSendVerifyEmail   = "task:send_verify_email"
	TaskSendWelcome       = "task:send_welcome"
	TaskSendResetPassword = "task:send_reset_password"
)

// 跨服务共享的 asynq wire 名称 + D5 refund retry 策略集中在
// lib/shared/asynqtask；这里仅 re-export 给 worker 内的旧引用。新增 task
// 类型时先评估是否跨服务，如是则统一登记在 asynqtask 包。
const (
	TypeAlertNotification  = asynqtask.TaskAlertNotification
	TaskRefundRetry        = asynqtask.TaskRefundRetry
	TaskRefundApology      = asynqtask.TaskRefundApology
	RefundRetryFirstDelay  = asynqtask.RefundRetryFirstDelay
	RefundRetrySecondDelay = asynqtask.RefundRetrySecondDelay
	RefundRetryMaxAttempts = asynqtask.RefundRetryMaxAttempts
)

// catalogProvider is the test-overridable source of the shared i18n catalog
// used for subjects / from-name / footer copy. Production callers should
// never touch this — it exists so unit tests can swap in a hermetic catalog
// without rewriting the world.
var catalogProvider = func() *i18n.Catalog { return i18n.MustDefaultCatalog() }

// registryProvider mirrors catalogProvider for the locale registry. Kept
// separate so tests can swap one without the other.
var registryProvider = func() *i18n.Registry { return i18n.MustDefault() }

// SetI18nForTesting overrides both the catalog and registry providers and
// returns a restore function. Exported so handler tests can inject a hermetic
// catalog (without depending on lib/shared/i18n/messages on disk).
func SetI18nForTesting(cat *i18n.Catalog, reg *i18n.Registry) func() {
	prevCat, prevReg := catalogProvider, registryProvider
	catalogProvider = func() *i18n.Catalog { return cat }
	registryProvider = func() *i18n.Registry { return reg }
	return func() {
		catalogProvider = prevCat
		registryProvider = prevReg
	}
}

// resolveLocale falls back to the registry default when the payload omits a
// locale. Keeps payload schemas backward-compatible — old tasks enqueued
// before Phase 2b will still pick up the default locale.
func resolveLocale(loc string) string {
	if loc != "" {
		return loc
	}
	return registryProvider().DefaultCode()
}

// RefundRetryPayload mirrors the API-side payload structure.  Kept in sync
// manually because importing apps/api/... from notifier would create a
// circular dependency.
type RefundRetryPayload struct {
	PaymentID    string `json:"payment_id"`
	ExtTxnID     string `json:"ext_txn_id"`
	UserID       string `json:"user_id"`
	UserEmail    string `json:"user_email,omitempty"`
	AmountCents  int64  `json:"amount_cents"`
	Currency     string `json:"currency"`
	Provider     string `json:"provider"`
	Reason       string `json:"reason,omitempty"`
	AttemptCount int    `json:"attempt_count"`
	Locale       string `json:"locale,omitempty"`
}

// PaymentRefunder is the abstraction over the billing provider's Refund API.
// In production, wire an adapter around billing.Provider.RefundPayment.
type PaymentRefunder interface {
	RefundPayment(ctx context.Context, extTxnID string, amountCents int64, reason string) error
}

// PaymentStore is the persistence interface used by the refund retry handler
// to update the payment row after a PaymentHub call.
type PaymentStore interface {
	// MarkRefunded transitions the payment to status='refunded'.
	MarkRefunded(ctx context.Context, paymentID string) error
	// MarkRefundFailed bumps refund_retry_count and stamps refund_failed_at.
	// Called when an automated retry hits the cap and we surface the row in
	// the admin dashboard.
	MarkRefundFailed(ctx context.Context, paymentID string, retryCount int) error
}

// RefundRetryEnqueuer schedules the next refund retry attempt.  Implementations
// must use asynq.ProcessIn (or equivalent) to respect explicit delay semantics.
type RefundRetryEnqueuer interface {
	EnqueueRefundRetry(ctx context.Context, payload RefundRetryPayload, delay time.Duration) error
}

// RefundApologyPayload mirrors the payload emitted by the attest-side
// refund-worker's asynqApologyMailer (apps/attest/cmd/refund-worker/main.go).
// The producer pre-resolves every field needed to render and ship the
// apology email; the notifier does NOT perform any DB lookup of its own
// (D1: the cross-schema owner_id → user.email join lives in the refund
// worker, where the verdict_order row is already loaded).
//
// IMPORTANT: do not rename fields or add new ones without coordinating with
// the refund-worker — the task type is a wire contract.
type RefundApologyPayload struct {
	OrderID           string `json:"order_id"`              // verdict_order id (v_*)
	UserEmail         string `json:"user_email"`            // recipient (may be empty → ACK + P0 warn)
	ExtOrderID     string `json:"ext_order_id"`       // upstream gateway reference for support
	RefundAmountCents int64  `json:"refund_amount_cents"`   // paid amount that failed to refund
	Currency          string `json:"currency"`              // ISO 4217-ish; producer hard-codes "CNY" today
	FailureReason     string `json:"failure_reason"`        // final failure reason from last retry
	EnqueuedAt        string `json:"enqueued_at,omitempty"` // RFC3339Nano UTC, informational only
	Locale            string `json:"locale,omitempty"`      // optional override; resolveLocale falls back
}

// SendVerifyEmailPayload holds the payload for sending verification email.
type SendVerifyEmailPayload struct {
	To        string `json:"to"`              // recipient email address
	Code      string `json:"code"`            // 6-digit OTP code
	ExpiresIn string `json:"expires_in"`      // human-readable duration
	Locale    string `json:"locale,omitempty"` // selected by API at enqueue time; empty → registry default
}

// SendWelcomePayload holds the payload for sending welcome email.
type SendWelcomePayload struct {
	To       string `json:"to"`              // recipient email address
	Username string `json:"username"`        // user's display name
	Locale   string `json:"locale,omitempty"` // see SendVerifyEmailPayload.Locale
}

// SendResetPasswordPayload holds the payload for sending password reset email.
type SendResetPasswordPayload struct {
	To        string `json:"to"`              // recipient email address
	ResetURL  string `json:"reset_url"`       // password reset URL (already locale-prefixed)
	ExpiresIn string `json:"expires_in"`      // human-readable duration
	Locale    string `json:"locale,omitempty"` // see SendVerifyEmailPayload.Locale
}

// Handlers manages all email task handlers.
type Handlers struct {
	sender    email.Sender
	templates *template.Templates
	logger    *slog.Logger

	// Optional dependencies for D5 refund retry pipeline.  All three must be
	// wired together to enable HandleRefundRetry; if any is nil the handler
	// returns a clear error (and the task is retried by asynq).
	refunder         PaymentRefunder
	paymentStore     PaymentStore
	refundEnqueuer   RefundRetryEnqueuer
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(sender email.Sender, templates *template.Templates, logger *slog.Logger) *Handlers {
	return &Handlers{
		sender:    sender,
		templates: templates,
		logger:    logger,
	}
}

// WithRefundDeps wires the dependencies required for HandleRefundRetry.
// Production wiring goes through cmd/notifier/main.go; tests inject mocks
// directly.
func (h *Handlers) WithRefundDeps(
	refunder PaymentRefunder,
	store PaymentStore,
	enq RefundRetryEnqueuer,
) *Handlers {
	h.refunder = refunder
	h.paymentStore = store
	h.refundEnqueuer = enq
	return h
}

// HandleSendVerifyEmail processes verify email tasks.
func (h *Handlers) HandleSendVerifyEmail(ctx context.Context, task *asynq.Task) error {
	var payload SendVerifyEmailPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return apperr.Internal("解析验证邮件任务载荷失败", err)
	}

	if payload.To == "" {
		return apperr.Validation("收件人地址不能为空", "")
	}
	if payload.Code == "" {
		return apperr.Validation("验证码不能为空", "")
	}

	locale := resolveLocale(payload.Locale)

	html, err := h.templates.RenderVerifyEmail(locale, template.VerifyEmailData{
		Code:      payload.Code,
		ExpiresIn: payload.ExpiresIn,
	})
	if err != nil {
		return err
	}

	msg := email.Message{
		To:      payload.To,
		Subject: catalogProvider().T(locale, "email.verify_email.subject", nil),
		HTML:    html,
	}

	if err := h.sendEmail(ctx, msg); err != nil {
		h.logger.Error("发送验证邮件失败", "to", payload.To, "locale", locale, "error", err)
		return err
	}

	h.logger.Info("验证邮件发送成功", "to", payload.To, "locale", locale)
	return nil
}

// HandleSendWelcome processes welcome email tasks.
func (h *Handlers) HandleSendWelcome(ctx context.Context, task *asynq.Task) error {
	var payload SendWelcomePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return apperr.Internal("解析欢迎邮件任务载荷失败", err)
	}

	if payload.To == "" {
		return apperr.Validation("收件人地址不能为空", "")
	}
	if payload.Username == "" {
		payload.Username = payload.To // fallback to email as username
	}

	locale := resolveLocale(payload.Locale)

	html, err := h.templates.RenderWelcome(locale, template.WelcomeData{
		Username: payload.Username,
	})
	if err != nil {
		return err
	}

	msg := email.Message{
		To:      payload.To,
		Subject: catalogProvider().T(locale, "email.welcome.subject", nil),
		HTML:    html,
	}

	if err := h.sendEmail(ctx, msg); err != nil {
		h.logger.Error("发送欢迎邮件失败", "to", payload.To, "locale", locale, "error", err)
		return err
	}

	h.logger.Info("欢迎邮件发送成功", "to", payload.To, "locale", locale)
	return nil
}

// HandleSendResetPassword processes password reset email tasks.
func (h *Handlers) HandleSendResetPassword(ctx context.Context, task *asynq.Task) error {
	var payload SendResetPasswordPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return apperr.Internal("解析密码重置邮件任务载荷失败", err)
	}

	if payload.To == "" {
		return apperr.Validation("收件人地址不能为空", "")
	}
	if payload.ResetURL == "" {
		return apperr.Validation("重置链接不能为空", "")
	}

	locale := resolveLocale(payload.Locale)

	html, err := h.templates.RenderResetPassword(locale, template.ResetPasswordData{
		ResetURL:  payload.ResetURL,
		ExpiresIn: payload.ExpiresIn,
	})
	if err != nil {
		return err
	}

	msg := email.Message{
		To:      payload.To,
		Subject: catalogProvider().T(locale, "email.reset_password.subject", nil),
		HTML:    html,
	}

	if err := h.sendEmail(ctx, msg); err != nil {
		h.logger.Error("发送密码重置邮件失败", "to", payload.To, "locale", locale, "error", err)
		return err
	}

	h.logger.Info("密码重置邮件发送成功", "to", payload.To, "locale", locale)
	return nil
}

// AlertNotificationPayload holds the payload for sending an alert notification
// via a specific channel.
//
// Locale governs the channel adapters' subject / framing copy where
// applicable. For webhook-style adapters the locale is forwarded to the
// downstream payload so receivers can render their own translation.
type AlertNotificationPayload struct {
	ChannelType   string `json:"channel_type"`   // "webhook" | "wecom" | "dingtalk" | "feishu"
	ChannelConfig []byte `json:"channel_config"` // JSON-encoded channel config
	Title         string `json:"title"`
	Body          string `json:"body"`
	URL           string `json:"url"`
	Level         string `json:"level"`            // "critical" | "warning" | "info"
	Locale        string `json:"locale,omitempty"` // monitor owner's locale at enqueue time
}

// HandleAlertNotification processes alert notification tasks by routing to the
// appropriate channel adapter based on the channel_type field.
func (h *Handlers) HandleAlertNotification(ctx context.Context, task *asynq.Task) error {
	var payload AlertNotificationPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return apperr.Internal("解析告警通知任务载荷失败", err)
	}

	if payload.ChannelType == "" {
		return apperr.Validation("channel_type 不能为空", "")
	}

	locale := resolveLocale(payload.Locale)

	ch, err := h.buildChannel(payload.ChannelType, payload.ChannelConfig)
	if err != nil {
		return apperr.Validation(fmt.Sprintf("构建通道失败: %v", err), "")
	}

	p := channel.Payload{
		Title: payload.Title,
		Body:  payload.Body,
		URL:   payload.URL,
		Level: payload.Level,
	}

	start := time.Now()
	err = ch.Send(ctx, p)
	MetricsSendDuration.WithLabelValues(payload.ChannelType).Observe(time.Since(start).Seconds())
	if err != nil {
		MetricsWebhookCalls.WithLabelValues(payload.ChannelType, "fail").Inc()
		h.logger.Error("告警通知发送失败",
			"channel_type", payload.ChannelType,
			"locale", locale,
			"error", err,
		)
		return err
	}
	MetricsWebhookCalls.WithLabelValues(payload.ChannelType, "ok").Inc()

	h.logger.Info("告警通知发送成功", "channel_type", payload.ChannelType, "locale", locale)
	return nil
}

// buildChannel constructs the appropriate Channel adapter from type and raw JSON config.
func (h *Handlers) buildChannel(channelType string, cfgJSON []byte) (channel.Channel, error) {
	switch channelType {
	case "webhook":
		var cfg channel.WebhookConfig
		if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
			return nil, fmt.Errorf("unmarshal webhook config: %w", err)
		}
		ch, err := channel.NewWebhook(cfg)
		if err != nil {
			return nil, fmt.Errorf("build webhook channel: %w", err)
		}
		return ch, nil
	case "wecom":
		var cfg channel.WecomConfig
		if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
			return nil, fmt.Errorf("unmarshal wecom config: %w", err)
		}
		ch, err := channel.NewWecom(cfg)
		if err != nil {
			return nil, fmt.Errorf("build wecom channel: %w", err)
		}
		return ch, nil
	case "dingtalk":
		var cfg channel.DingtalkConfig
		if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
			return nil, fmt.Errorf("unmarshal dingtalk config: %w", err)
		}
		ch, err := channel.NewDingtalk(cfg)
		if err != nil {
			return nil, fmt.Errorf("build dingtalk channel: %w", err)
		}
		return ch, nil
	case "feishu":
		var cfg channel.FeishuConfig
		if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
			return nil, fmt.Errorf("unmarshal feishu config: %w", err)
		}
		ch, err := channel.NewFeishu(cfg)
		if err != nil {
			return nil, fmt.Errorf("build feishu channel: %w", err)
		}
		return ch, nil
	case "email":
		var cfg channel.EmailConfig
		if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
			return nil, fmt.Errorf("unmarshal email config: %w", err)
		}
		ch, err := channel.NewEmail(cfg, h.sender)
		if err != nil {
			return nil, fmt.Errorf("build email channel: %w", err)
		}
		return ch, nil
	default:
		return nil, fmt.Errorf("unsupported channel type: %s", channelType)
	}
}

// HandleRefundRetry processes a payment:refund_retry asynq task (D5).
//
// Flow:
//  1. Call PaymentRefunder.RefundPayment (PaymentHub Refund API).
//  2. On success: PaymentStore.MarkRefunded(paymentID) → ack task.
//  3. On failure:
//     - If attempt_count < RefundRetryMaxAttempts: bump attempt_count and
//       re-enqueue with the next explicit delay (5min → 30min schedule per D5).
//       refund_retry_count is left to be incremented when the next attempt
//       persists its outcome.
//     - If attempt_count >= RefundRetryMaxAttempts: send apology email to
//       the user, leave payment row in 'refund_failed' (bump retry_count one
//       last time) so the admin dashboard surfaces it.  Returning nil here
//       prevents asynq from re-running the task indefinitely.
//
// IMPORTANT: This handler MUST be scheduled with asynq.ProcessIn from the
// enqueuer side.  It deliberately does NOT participate in the generic
// exponential backoff configured in worker.go — refund retries follow the
// fixed D5 schedule (5min/30min) regardless of asynq's internal retry count.
func (h *Handlers) HandleRefundRetry(ctx context.Context, task *asynq.Task) error {
	var payload RefundRetryPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return apperr.Internal("解析 refund retry 任务载荷失败", err)
	}

	if payload.PaymentID == "" {
		return apperr.Validation("payment_id 不能为空", "")
	}
	if payload.ExtTxnID == "" {
		return apperr.Validation("ext_txn_id 不能为空", "")
	}
	if payload.AmountCents <= 0 {
		return apperr.Validation("amount_cents 必须为正数", "")
	}

	if h.refunder == nil || h.paymentStore == nil || h.refundEnqueuer == nil {
		// Fail loud — production must wire all three.  Returning a non-
		// validation error lets asynq retry on its own backoff so a wiring
		// bug is recoverable once notifier is restarted with proper deps.
		return apperr.Internal("refund retry deps not wired (refunder / store / enqueuer)", nil)
	}

	// Attempt the PaymentHub refund.
	refundErr := h.refunder.RefundPayment(ctx, payload.ExtTxnID, payload.AmountCents, payload.Reason)
	if refundErr == nil {
		// PaymentHub accepted the refund — persist the new state.
		if err := h.paymentStore.MarkRefunded(ctx, payload.PaymentID); err != nil {
			h.logger.Error("标记 payment 为 refunded 失败",
				"payment_id", payload.PaymentID,
				"error", err,
			)
			// Return error so asynq retries — DB hiccups are transient.
			return fmt.Errorf("mark refunded: %w", err)
		}
		MetricsRefundRetries.WithLabelValues("ok").Inc()
		h.logger.Info("退款重试成功",
			"payment_id", payload.PaymentID,
			"ext_txn_id", payload.ExtTxnID,
			"attempt_count", payload.AttemptCount,
		)
		return nil
	}

	// Refund failed again.  Decide between rescheduling and giving up.
	h.logger.Warn("退款重试失败",
		"payment_id", payload.PaymentID,
		"ext_txn_id", payload.ExtTxnID,
		"attempt_count", payload.AttemptCount,
		"error", refundErr,
	)

	nextAttempt := payload.AttemptCount + 1

	if nextAttempt < RefundRetryMaxAttempts {
		// Reschedule with the explicit second-stage delay.
		nextPayload := payload
		nextPayload.AttemptCount = nextAttempt
		if err := h.refundEnqueuer.EnqueueRefundRetry(ctx, nextPayload, RefundRetrySecondDelay); err != nil {
			return fmt.Errorf("reschedule refund retry: %w", err)
		}
		// Bump retry_count to reflect that one more attempt has occurred.
		if err := h.paymentStore.MarkRefundFailed(ctx, payload.PaymentID, nextAttempt); err != nil {
			h.logger.Warn("标记 payment 重试计数失败（非致命）",
				"payment_id", payload.PaymentID,
				"error", err,
			)
		}
		MetricsRefundRetries.WithLabelValues("retry").Inc()
		return nil
	}

	// Max attempts reached → send apology email + leave row in refund_failed
	// for admin dashboard escalation (D5).
	MetricsRefundRetries.WithLabelValues("failed").Inc()
	if err := h.paymentStore.MarkRefundFailed(ctx, payload.PaymentID, nextAttempt); err != nil {
		h.logger.Warn("最终标记 refund_failed 失败（非致命）",
			"payment_id", payload.PaymentID,
			"error", err,
		)
	}
	if payload.UserEmail != "" {
		if err := h.sendRefundApologyEmail(ctx, payload); err != nil {
			h.logger.Error("发送退款道歉邮件失败",
				"payment_id", payload.PaymentID,
				"user_email", payload.UserEmail,
				"error", err,
			)
			// Return error so asynq retries the apology email at least once.
			return fmt.Errorf("send apology email: %w", err)
		}
	} else {
		h.logger.Warn("无用户邮箱，跳过道歉邮件",
			"payment_id", payload.PaymentID,
			"user_id", payload.UserID,
		)
	}
	return nil
}

// sendRefundApologyEmail sends the D5 apology email after automated refund
// retries have been exhausted, rendering the localized refund_failed
// template (replaces the previous inline HTML).
func (h *Handlers) sendRefundApologyEmail(ctx context.Context, p RefundRetryPayload) error {
	locale := resolveLocale(p.Locale)
	amountDisplay := formatAmountDisplay(p.AmountCents, p.Currency)

	html, err := h.templates.RenderRefundFailed(locale, template.RefundFailedData{
		PaymentID:     p.PaymentID,
		ExtTxnID:      p.ExtTxnID,
		AmountDisplay: amountDisplay,
	})
	if err != nil {
		return err
	}

	msg := email.Message{
		To:      p.UserEmail,
		Subject: catalogProvider().T(locale, "email.refund_failed.subject", nil),
		HTML:    html,
	}
	return h.sendEmail(ctx, msg)
}

// HandleRefundApology processes a payment:refund_apology asynq task emitted
// by the attest-side refund-worker once the D5 retry ladder is exhausted
// and the verdict_order has flipped to status='refund_failed'.
//
// The payload is self-contained — the producer has already resolved
// user_email / ext_order_id / amount / currency via an
// application-level join (D1) — so this handler is pure render-and-send:
//
//  1. Decode the payload; bad JSON or missing order_id → asynq.SkipRetry so
//     the task drains instead of clogging the billing queue.
//  2. Resolve the locale via the i18n registry (default zh-CN equivalent).
//  3. If user_email is empty (rare race — user deleted after refund
//     failure?) ACK with a P0 alert hook; do not retry forever (D5
//     fail-open).
//  4. Render the localized refund_failed template and ship via the regular
//     email channel. Transient send errors bubble up → asynq retries.
//
// IMPORTANT: this handler remains decoupled from the apps/attest
// internal packages (D1: no cross-schema imports). Every datum required
// to build the email is on the asynq payload itself.
func (h *Handlers) HandleRefundApology(ctx context.Context, task *asynq.Task) error {
	var payload RefundApologyPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		// Malformed payloads are unrecoverable — skip retry so the task
		// drains instead of clogging the billing queue forever.
		MetricsRefundRetries.WithLabelValues("apology_bad_payload").Inc()
		return fmt.Errorf("%w: decode refund apology payload: %v", asynq.SkipRetry, err)
	}

	if strings.TrimSpace(payload.OrderID) == "" {
		MetricsRefundRetries.WithLabelValues("apology_bad_payload").Inc()
		return fmt.Errorf("%w: refund apology task missing order_id", asynq.SkipRetry)
	}

	locale := resolveLocale(payload.Locale)

	if strings.TrimSpace(payload.UserEmail) == "" {
		// D5 fail-open: ACK with a P0 alert hook so ops can find the row
		// on the admin dashboard and send the email manually. Looping
		// forever helps nobody.
		MetricsRefundRetries.WithLabelValues("apology_no_email").Inc()
		h.logger.Warn("退款道歉任务缺少用户邮箱, 已 ACK (P0)",
			"order_id", payload.OrderID,
			"failure_reason", payload.FailureReason,
			"alert", "refund_apology_no_email",
		)
		return nil
	}

	amountDisplay := formatAmountDisplay(payload.RefundAmountCents, payload.Currency)
	html, err := h.templates.RenderRefundFailed(locale, template.RefundFailedData{
		PaymentID:     payload.OrderID,
		ExtTxnID:      payload.ExtOrderID,
		AmountDisplay: amountDisplay,
	})
	if err != nil {
		return fmt.Errorf("refund apology: render template: %w", err)
	}

	msg := email.Message{
		To:      payload.UserEmail,
		Subject: catalogProvider().T(locale, "email.refund_failed.subject", nil),
		HTML:    html,
	}

	if err := h.sendEmail(ctx, msg); err != nil {
		h.logger.Error("发送退款道歉邮件失败",
			"order_id", payload.OrderID,
			"user_email", payload.UserEmail,
			"error", err,
		)
		return fmt.Errorf("refund apology: send email: %w", err)
	}

	MetricsRefundRetries.WithLabelValues("apology_sent").Inc()
	h.logger.Info("退款道歉邮件发送成功",
		"order_id", payload.OrderID,
		"user_email", payload.UserEmail,
		"locale", locale,
		"failure_reason", payload.FailureReason,
	)
	return nil
}

// sendEmail is the metric-instrumented wrapper around the raw Sender.Send call.
// It infers the provider label from the concrete sender type and records both
// the outcome counter and the send-duration histogram. Centralising the
// instrumentation here keeps the existing logging / error handling at each
// call site untouched.
//
// P1-11: callers may pre-extract the template name via templateFromSubject
// or pass it in directly through sendEmailTemplate; the un-templated
// sendEmail still works and records the new idcd_notifier_email_sent_total
// counter with template="unknown".
func (h *Handlers) sendEmail(ctx context.Context, msg email.Message) error {
	return h.sendEmailTemplate(ctx, msg, templateFromSubject(msg.Subject))
}

// sendEmailTemplate is the version called by handlers that know the
// template name up front (cleaner than re-deriving it from the subject).
func (h *Handlers) sendEmailTemplate(ctx context.Context, msg email.Message, templateName string) error {
	provider := emailProviderLabel(h.sender)
	start := time.Now()
	err := h.sender.Send(ctx, msg)
	MetricsSendDuration.WithLabelValues("email").Observe(time.Since(start).Seconds())
	tpl := templateName
	if tpl == "" {
		tpl = "unknown"
	}
	if err != nil {
		MetricsEmailsSent.WithLabelValues(provider, "fail").Inc()
		EmailSent.WithLabelValues("fail", provider, tpl).Inc()
		return err
	}
	MetricsEmailsSent.WithLabelValues(provider, "ok").Inc()
	EmailSent.WithLabelValues("ok", provider, tpl).Inc()
	return nil
}

// templateFromSubject is a best-effort classifier: every notifier-side
// handler hard-codes one of a small number of subject strings via the
// i18n catalog, so a substring lookup is sufficient. Unknown subjects
// collapse to "unknown" — better than blowing up label cardinality with
// raw user-visible subject lines.
//
// The English keywords are matched case-insensitively (so "Welcome to
// idcd" still classifies as "welcome"); the CJK keywords are matched
// literally because Chinese has no concept of case.
func templateFromSubject(subject string) string {
	s := strings.ToLower(subject)
	switch {
	case strings.Contains(s, "verify") || strings.Contains(subject, "验证"):
		return "verify_email"
	case strings.Contains(s, "reset") || strings.Contains(subject, "密码"):
		return "reset_password"
	case strings.Contains(s, "welcome") || strings.Contains(subject, "欢迎"):
		return "welcome"
	case strings.Contains(s, "refund") || strings.Contains(subject, "退款"):
		return "refund_apology"
	case strings.Contains(s, "billing") || strings.Contains(s, "invoice") ||
		strings.Contains(subject, "账单") || strings.Contains(subject, "发票"):
		return "billing"
	case strings.Contains(s, "alert") || strings.Contains(subject, "告警"):
		return "alert"
	default:
		return "unknown"
	}
}

// emailProviderLabel maps a concrete Sender to its provider label. Anything
// not recognised collapses to "unknown" so metrics stay non-cardinality-bombing
// when tests inject mocks.
func emailProviderLabel(s email.Sender) string {
	switch s.(type) {
	case *email.SESSender:
		return "ses"
	case *email.SMTPSender:
		return "smtp"
	default:
		return "unknown"
	}
}

// formatAmountDisplay renders cents+currency in a locale-neutral form.
// Phase 2b deliberately keeps this minimal: `99.00 CNY`. Locale-aware
// currency formatting belongs to Phase 5 (golang.org/x/text/currency).
func formatAmountDisplay(cents int64, currency string) string {
	yuan := float64(cents) / 100.0
	if currency == "" {
		return fmt.Sprintf("%.2f", yuan)
	}
	return fmt.Sprintf("%.2f %s", yuan, currency)
}

// GetMux returns a configured ServeMux with all handlers registered.
func (h *Handlers) GetMux() *asynq.ServeMux {
	mux := asynq.NewServeMux()

	// Register handlers with retry configuration
	mux.HandleFunc(TaskSendVerifyEmail, h.withRetry(h.HandleSendVerifyEmail))
	mux.HandleFunc(TaskSendWelcome, h.withRetry(h.HandleSendWelcome))
	mux.HandleFunc(TaskSendResetPassword, h.withRetry(h.HandleSendResetPassword))
	mux.HandleFunc(TypeAlertNotification, h.withRetry(h.HandleAlertNotification))
	// D5: refund retry — explicit schedule via asynq.ProcessIn, NOT generic backoff.
	mux.HandleFunc(TaskRefundRetry, h.withRetry(h.HandleRefundRetry))
	// D5: refund apology — emitted by attest-side refund-worker once the
	// retry ladder is exhausted. Same queue ("billing"), separate handler.
	mux.HandleFunc(TaskRefundApology, h.withRetry(h.HandleRefundApology))

	return mux
}

// withRetry wraps a handler with retry logic and error handling.
func (h *Handlers) withRetry(handler func(context.Context, *asynq.Task) error) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, task *asynq.Task) error {
		h.logger.Debug("处理邮件任务开始", "task_type", task.Type())

		err := handler(ctx, task)
		if err != nil {
			h.logger.Error("邮件任务处理失败",
				"task_type", task.Type(),
				"error", err,
			)

			// Check if this is a validation error (should not retry)
			if apperr.Is(err, apperr.CodeValidation) {
				return fmt.Errorf("validation error, will not retry: %w", err)
			}

			return err // let asynq handle retry logic
		}

		h.logger.Debug("邮件任务处理完成", "task_type", task.Type())
		return nil
	}
}

// FormatFromAddress builds an RFC 5322 / RFC 2047 compliant `From:` value
// using the locale-specific display name from the i18n catalog.
//
// The SMTP / SES senders construct their own From line today; this helper
// is exported so future call sites (or the eventual cmd-side wiring) can
// pick the localized display name without duplicating QEncoding logic.
//
// Behaviour:
//   - When name is ASCII-only, returns "name <addr>" verbatim.
//   - When name contains non-ASCII bytes, the display portion is
//     RFC 2047-encoded via mime.QEncoding.
//   - addr is returned bare when name is empty.
func FormatFromAddress(locale, addr string) string {
	loc := resolveLocale(locale)
	name := catalogProvider().T(loc, "email.from.name", nil)
	if name == "" || addr == "" {
		return addr
	}
	if isASCII(name) {
		return fmt.Sprintf("%s <%s>", name, addr)
	}
	return fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("utf-8", name), addr)
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

// EnqueueEmail is a typed convenience helper that callers (the API auth
// handler, future MCP enqueue paths, etc.) can use to enqueue an email task
// with locale propagation built in. The function does not import asynq's
// payload struct types directly to avoid coupling callers to the concrete
// task type names; it accepts a discriminated payload via EmailPayload.
//
// Phase 2b ships this helper without wiring it into API handlers yet — the
// API handlers still call asynq.Client.Enqueue with their legacy payload
// shapes. Because every payload struct now defines `Locale string \`json:"locale,omitempty"\``,
// old call sites continue to work (Locale is the zero value → resolveLocale
// falls back to the registry default).
//
// See `docs/prd/I18N-PLAN.md` §6.5 for the long-term migration plan.
type EmailPayload interface {
	taskType() string
	payload() ([]byte, error)
}

// Asynq is the narrow interface EnqueueEmail needs from *asynq.Client. We
// keep it tiny so callers can mock for tests and so we don't import the
// concrete client type into every reverse-dependency.
type Asynq interface {
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// EnqueueEmail builds the asynq task from the supplied payload and submits
// it to the client. Locale is folded into the payload (callers cannot bypass
// it) so producers never forget to set it.
func EnqueueEmail(ctx context.Context, client Asynq, locale string, payload EmailPayload, opts ...asynq.Option) error {
	if client == nil {
		return apperr.Internal("EnqueueEmail: asynq client is nil", nil)
	}
	loc := strings.TrimSpace(locale)
	body, err := payload.payload()
	if err != nil {
		return apperr.Internal("EnqueueEmail: marshal payload", err)
	}
	// Splice locale into the payload JSON if the concrete type didn't already
	// own a Locale field. The struct constructors below all expose Locale, so
	// this hot-path stays a no-op for typed callers; it remains for callers
	// who hand-rolled a payload.
	body, err = ensureLocale(body, loc)
	if err != nil {
		return apperr.Internal("EnqueueEmail: rewrite locale", err)
	}
	task := asynq.NewTask(payload.taskType(), body)
	_, err = client.EnqueueContext(ctx, task, opts...)
	return err
}

// ensureLocale guarantees the marshalled payload contains a "locale" field
// set to loc. If the payload already has a non-empty locale, ensureLocale
// keeps it (caller intent wins). If loc is empty AND no locale exists,
// the payload is returned unchanged so the worker's resolveLocale fallback
// applies.
func ensureLocale(body []byte, loc string) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	if existing, ok := m["locale"].(string); ok && existing != "" {
		return body, nil
	}
	if loc == "" {
		return body, nil
	}
	m["locale"] = loc
	return json.Marshal(m)
}

// --- EmailPayload implementations ---

func (p SendVerifyEmailPayload) taskType() string { return TaskSendVerifyEmail }
func (p SendVerifyEmailPayload) payload() ([]byte, error) {
	return json.Marshal(p)
}

func (p SendWelcomePayload) taskType() string { return TaskSendWelcome }
func (p SendWelcomePayload) payload() ([]byte, error) {
	return json.Marshal(p)
}

func (p SendResetPasswordPayload) taskType() string { return TaskSendResetPassword }
func (p SendResetPasswordPayload) payload() ([]byte, error) {
	return json.Marshal(p)
}

// BuildLocalizedURL appends a locale prefix to the given path according to
// the registry default rule (default locale → no prefix, others → /{loc}/).
// Helper exported so future API-side code can produce links in emails or
// SMS without duplicating the logic.
//
// Example (registry default = "cn"):
//
//	BuildLocalizedURL("https://idcd.com", "cn", "/app/dashboard") = "https://idcd.com/app/dashboard"
//	BuildLocalizedURL("https://idcd.com", "en", "/app/dashboard") = "https://idcd.com/en/app/dashboard"
func BuildLocalizedURL(baseURL, locale, path string) string {
	loc := resolveLocale(locale)
	reg := registryProvider()
	cleanedPath := "/" + strings.TrimLeft(path, "/")
	if loc == reg.DefaultCode() {
		return strings.TrimRight(baseURL, "/") + cleanedPath
	}
	// Construct via net/url to make sure trailing-slash handling is correct.
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme == "" {
		// Fall back to dumb concatenation if baseURL is malformed — the
		// caller is responsible for providing a valid URL, but we don't
		// want this helper to surface as a hard error path.
		return strings.TrimRight(baseURL, "/") + "/" + loc + cleanedPath
	}
	u.Path = "/" + loc + cleanedPath
	return u.String()
}
