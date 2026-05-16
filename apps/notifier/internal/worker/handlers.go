package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kite365/idcd/apps/notifier/internal/channel"
	"github.com/kite365/idcd/apps/notifier/internal/email"
	"github.com/kite365/idcd/apps/notifier/internal/template"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// Task type constants for different email types.
const (
	TaskSendVerifyEmail   = "task:send_verify_email"
	TaskSendWelcome       = "task:send_welcome"
	TaskSendResetPassword = "task:send_reset_password"
	TypeAlertNotification = "alert:notification"

	// TaskRefundRetry is the asynq task type for retrying a failed Paddle
	// refund (D5).  Payload: RefundRetryPayload (JSON).  The task is
	// scheduled with explicit asynq.ProcessIn delays (5min / 30min) and MUST
	// NOT rely on the generic exponential backoff configured in worker.go.
	TaskRefundRetry = "payment:refund_retry"
)

// Refund retry timing constants (D5).
//
// The first retry runs 5 minutes after the original refund.failed webhook.
// The second retry runs 25 minutes after the first (≈30 minutes total).
// After both attempts fail (attempt_count >= 2) we send an apology email
// and leave the payment in 'refund_failed' status for the admin dashboard.
const (
	RefundRetryFirstDelay  = 5 * time.Minute
	RefundRetrySecondDelay = 25 * time.Minute
	RefundRetryMaxAttempts = 2
)

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
}

// PaymentRefunder is the abstraction over the billing provider's Refund API.
// In production, wire an adapter around billing.Provider.RefundPayment.
type PaymentRefunder interface {
	RefundPayment(ctx context.Context, extTxnID string, amountCents int64, reason string) error
}

// PaymentStore is the persistence interface used by the refund retry handler
// to update the payment row after a Paddle call.
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

// SendVerifyEmailPayload holds the payload for sending verification email.
type SendVerifyEmailPayload struct {
	To        string `json:"to"`         // recipient email address
	Code      string `json:"code"`       // 6-digit OTP code
	ExpiresIn string `json:"expires_in"` // human-readable duration
}

// SendWelcomePayload holds the payload for sending welcome email.
type SendWelcomePayload struct {
	To       string `json:"to"`       // recipient email address
	Username string `json:"username"` // user's display name
}

// SendResetPasswordPayload holds the payload for sending password reset email.
type SendResetPasswordPayload struct {
	To        string `json:"to"`         // recipient email address
	ResetURL  string `json:"reset_url"`  // password reset URL
	ExpiresIn string `json:"expires_in"` // human-readable duration
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

	// Validate payload
	if payload.To == "" {
		return apperr.Validation("收件人地址不能为空", "")
	}
	if payload.Code == "" {
		return apperr.Validation("验证码不能为空", "")
	}

	// Render template
	html, err := h.templates.RenderVerifyEmail(template.VerifyEmailData{
		Code:      payload.Code,
		ExpiresIn: payload.ExpiresIn,
	})
	if err != nil {
		return err
	}

	// Send email
	msg := email.Message{
		To:      payload.To,
		Subject: "【idcd】邮箱验证码",
		HTML:    html,
	}

	if err := h.sender.Send(ctx, msg); err != nil {
		h.logger.Error("发送验证邮件失败", "to", payload.To, "error", err)
		return err
	}

	h.logger.Info("验证邮件发送成功", "to", payload.To)
	return nil
}

// HandleSendWelcome processes welcome email tasks.
func (h *Handlers) HandleSendWelcome(ctx context.Context, task *asynq.Task) error {
	var payload SendWelcomePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return apperr.Internal("解析欢迎邮件任务载荷失败", err)
	}

	// Validate payload
	if payload.To == "" {
		return apperr.Validation("收件人地址不能为空", "")
	}
	if payload.Username == "" {
		payload.Username = payload.To // fallback to email as username
	}

	// Render template
	html, err := h.templates.RenderWelcome(template.WelcomeData{
		Username: payload.Username,
	})
	if err != nil {
		return err
	}

	// Send email
	msg := email.Message{
		To:      payload.To,
		Subject: "欢迎加入 idcd！",
		HTML:    html,
	}

	if err := h.sender.Send(ctx, msg); err != nil {
		h.logger.Error("发送欢迎邮件失败", "to", payload.To, "error", err)
		return err
	}

	h.logger.Info("欢迎邮件发送成功", "to", payload.To)
	return nil
}

// HandleSendResetPassword processes password reset email tasks.
func (h *Handlers) HandleSendResetPassword(ctx context.Context, task *asynq.Task) error {
	var payload SendResetPasswordPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return apperr.Internal("解析密码重置邮件任务载荷失败", err)
	}

	// Validate payload
	if payload.To == "" {
		return apperr.Validation("收件人地址不能为空", "")
	}
	if payload.ResetURL == "" {
		return apperr.Validation("重置链接不能为空", "")
	}

	// Render template
	html, err := h.templates.RenderResetPassword(template.ResetPasswordData{
		ResetURL:  payload.ResetURL,
		ExpiresIn: payload.ExpiresIn,
	})
	if err != nil {
		return err
	}

	// Send email
	msg := email.Message{
		To:      payload.To,
		Subject: "【idcd】密码重置",
		HTML:    html,
	}

	if err := h.sender.Send(ctx, msg); err != nil {
		h.logger.Error("发送密码重置邮件失败", "to", payload.To, "error", err)
		return err
	}

	h.logger.Info("密码重置邮件发送成功", "to", payload.To)
	return nil
}

// AlertNotificationPayload holds the payload for sending an alert notification
// via a specific channel.
type AlertNotificationPayload struct {
	ChannelType   string `json:"channel_type"`   // "webhook" | "wecom" | "dingtalk" | "feishu"
	ChannelConfig []byte `json:"channel_config"` // JSON-encoded channel config
	Title         string `json:"title"`
	Body          string `json:"body"`
	URL           string `json:"url"`
	Level         string `json:"level"` // "critical" | "warning" | "info"
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

	if err := ch.Send(ctx, p); err != nil {
		h.logger.Error("告警通知发送失败",
			"channel_type", payload.ChannelType,
			"error", err,
		)
		return err
	}

	h.logger.Info("告警通知发送成功", "channel_type", payload.ChannelType)
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
//  1. Call PaymentRefunder.RefundPayment (Paddle Refund API).
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

	// Attempt the Paddle refund.
	refundErr := h.refunder.RefundPayment(ctx, payload.ExtTxnID, payload.AmountCents, payload.Reason)
	if refundErr == nil {
		// Paddle accepted the refund — persist the new state.
		if err := h.paymentStore.MarkRefunded(ctx, payload.PaymentID); err != nil {
			h.logger.Error("标记 payment 为 refunded 失败",
				"payment_id", payload.PaymentID,
				"error", err,
			)
			// Return error so asynq retries — DB hiccups are transient.
			return fmt.Errorf("mark refunded: %w", err)
		}
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
		return nil
	}

	// Max attempts reached → send apology email + leave row in refund_failed
	// for admin dashboard escalation (D5).
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
// retries have been exhausted.  We deliberately keep the body inline rather
// than adding a fourth go:embed template — keeps the diff small and avoids
// touching the template package's surface area.
func (h *Handlers) sendRefundApologyEmail(ctx context.Context, p RefundRetryPayload) error {
	amountYuan := float64(p.AmountCents) / 100.0
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head><meta charset="UTF-8"><title>退款延迟通知</title></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'PingFang SC', sans-serif; max-width: 560px; margin: 0 auto; padding: 24px; color: #18181b;">
  <h2 style="color: #18181b;">关于您退款延迟的致歉说明</h2>
  <p>尊敬的用户您好，</p>
  <p>我们注意到您订单 <code>%s</code> 的退款（金额 <strong>%.2f %s</strong>）在自动处理过程中遇到了问题，目前尚未成功完成。</p>
  <p>对此给您带来的不便，我们深感抱歉。我们的工程师已收到此次失败告警，并将在 24 小时内人工处理您的退款请求。</p>
  <p>如您有任何疑问，请直接回复本邮件或联系 support@idcd.com，我们会优先处理。</p>
  <p>感谢您的耐心与理解。</p>
  <p>— idcd 团队</p>
  <hr style="border: 0; border-top: 1px solid #e4e4e7; margin: 24px 0;">
  <p style="font-size: 12px; color: #71717a;">订单号：%s ｜ 交易号：%s</p>
</body>
</html>`,
		p.PaymentID, amountYuan, p.Currency, p.PaymentID, p.ExtTxnID,
	)

	msg := email.Message{
		To:      p.UserEmail,
		Subject: "【idcd】关于您退款延迟的致歉说明",
		HTML:    html,
	}
	return h.sender.Send(ctx, msg)
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