package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

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
)

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
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(sender email.Sender, templates *template.Templates, logger *slog.Logger) *Handlers {
	return &Handlers{
		sender:    sender,
		templates: templates,
		logger:    logger,
	}
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

	ch, err := buildChannel(payload.ChannelType, payload.ChannelConfig)
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
func buildChannel(channelType string, cfgJSON []byte) (channel.Channel, error) {
	switch channelType {
	case "webhook":
		var cfg channel.WebhookConfig
		if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
			return nil, fmt.Errorf("unmarshal webhook config: %w", err)
		}
		return channel.NewWebhook(cfg), nil
	case "wecom":
		var cfg channel.WecomConfig
		if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
			return nil, fmt.Errorf("unmarshal wecom config: %w", err)
		}
		return channel.NewWecom(cfg), nil
	case "dingtalk":
		var cfg channel.DingtalkConfig
		if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
			return nil, fmt.Errorf("unmarshal dingtalk config: %w", err)
		}
		return channel.NewDingtalk(cfg), nil
	case "feishu":
		var cfg channel.FeishuConfig
		if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
			return nil, fmt.Errorf("unmarshal feishu config: %w", err)
		}
		return channel.NewFeishu(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported channel type: %s", channelType)
	}
}

// GetMux returns a configured ServeMux with all handlers registered.
func (h *Handlers) GetMux() *asynq.ServeMux {
	mux := asynq.NewServeMux()

	// Register handlers with retry configuration
	mux.HandleFunc(TaskSendVerifyEmail, h.withRetry(h.HandleSendVerifyEmail))
	mux.HandleFunc(TaskSendWelcome, h.withRetry(h.HandleSendWelcome))
	mux.HandleFunc(TaskSendResetPassword, h.withRetry(h.HandleSendResetPassword))
	mux.HandleFunc(TypeAlertNotification, h.withRetry(h.HandleAlertNotification))

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