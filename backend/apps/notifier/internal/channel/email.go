package channel

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/kite365/idcd/apps/notifier/internal/email"
)

// EmailConfig holds configuration for the email alert channel.
// Stored as JSON in the alertchannel.config JSONB column.
type EmailConfig struct {
	To            string `json:"to"`
	SubjectPrefix string `json:"subject_prefix"`
}

// EmailChannel sends alert notifications via email using the shared email.Sender.
type EmailChannel struct {
	cfg    EmailConfig
	sender email.Sender
}

// NewEmail creates an EmailChannel.
// Returns an error if the required "to" field is missing.
func NewEmail(cfg EmailConfig, sender email.Sender) (*EmailChannel, error) {
	if strings.TrimSpace(cfg.To) == "" {
		return nil, fmt.Errorf("email channel: to address must not be empty")
	}
	if sender == nil {
		return nil, fmt.Errorf("email channel: sender must not be nil")
	}
	return &EmailChannel{cfg: cfg, sender: sender}, nil
}

// Type implements Channel.
func (e *EmailChannel) Type() string { return "email" }

// Send implements Channel. Builds an HTML alert email and delivers it via the
// configured email.Sender.
func (e *EmailChannel) Send(ctx context.Context, p Payload) error {
	status := alertStatus(p)
	prefix := e.cfg.SubjectPrefix
	if prefix == "" {
		prefix = "[idcd Alert]"
	}
	subject := fmt.Sprintf("%s %s: %s", prefix, status, p.Title)
	html := buildAlertHTML(p, status)

	msg := email.Message{
		To:      e.cfg.To,
		Subject: subject,
		HTML:    html,
	}
	if err := e.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("email channel: send failed: %w", err)
	}
	return nil
}

// alertStatus derives the human-readable status label from the payload level.
// A level of "" or any value that isn't explicitly "resolved" maps to "FIRING".
func alertStatus(p Payload) string {
	if strings.EqualFold(p.Level, "resolved") {
		return "RESOLVED"
	}
	return "FIRING"
}

// buildAlertHTML renders a simple HTML email body for an alert notification.
func buildAlertHTML(p Payload, status string) string {
	badgeColor := "#dc2626" // red for FIRING
	if status == "RESOLVED" {
		badgeColor = "#16a34a" // green for RESOLVED
	}
	// Sanitise the detail URL before embedding it in an href.  The raw value
	// originates from monitor configuration so a malicious operator (or an
	// upstream that mis-handles input) could otherwise smuggle `javascript:`,
	// `data:`, or attribute-breaking content into the alert email.  We accept
	// only http(s) URLs and fall back to the alerts dashboard.  The result is
	// further attribute-escaped before being written to keep `"`, `<`, etc.
	// from breaking out of the href context.
	detailURL := safeDetailURL(p.URL)

	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>idcd 告警通知</title>
</head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:PingFang SC,'Microsoft YaHei',sans-serif;">
<table width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;padding:32px 0;">
  <tr>
    <td align="center">
      <table width="560" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:8px;overflow:hidden;box-shadow:0 1px 4px rgba(0,0,0,.08);">
        <!-- Header -->
        <tr>
          <td style="background:#0f172a;padding:24px 32px;">
            <span style="color:#ffffff;font-size:20px;font-weight:700;letter-spacing:.5px;">idcd</span>
            <span style="color:#94a3b8;font-size:14px;margin-left:8px;">监控告警</span>
          </td>
        </tr>
        <!-- Status badge + title -->
        <tr>
          <td style="padding:32px 32px 16px;">
            <span style="display:inline-block;background:`)
	b.WriteString(badgeColor)
	b.WriteString(`;color:#fff;font-size:13px;font-weight:600;padding:3px 12px;border-radius:99px;letter-spacing:.5px;">`)
	b.WriteString(status)
	b.WriteString(`</span>
            <h2 style="margin:12px 0 0;font-size:20px;color:#0f172a;line-height:1.4;">`)
	b.WriteString(htmlEscape(p.Title))
	b.WriteString(`</h2>
          </td>
        </tr>
        <!-- Body -->
        <tr>
          <td style="padding:0 32px 24px;">
            <p style="margin:0;color:#475569;font-size:15px;line-height:1.7;">`)
	bodyText := p.Body
	if bodyText == "" {
		bodyText = "监控检测到异常，请尽快查看。"
	}
	b.WriteString(htmlEscape(bodyText))
	b.WriteString(`</p>
          </td>
        </tr>
        <!-- CTA button -->
        <tr>
          <td style="padding:0 32px 32px;">
            <a href="`)
	b.WriteString(htmlEscape(detailURL))
	b.WriteString(`" style="display:inline-block;background:#2563eb;color:#ffffff;text-decoration:none;font-size:14px;font-weight:600;padding:10px 24px;border-radius:6px;">查看告警详情</a>
          </td>
        </tr>
        <!-- Divider -->
        <tr>
          <td style="border-top:1px solid #e2e8f0;padding:20px 32px;">
            <p style="margin:0;color:#94a3b8;font-size:12px;line-height:1.6;">
              此邮件由 <strong>idcd</strong> 告警系统自动发送，请勿直接回复。<br>
              如需管理告警通知，请访问 <a href="https://idcd.com/app/alerts" style="color:#2563eb;">idcd 控制台</a>。
            </p>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>
</body>
</html>`)
	return b.String()
}

// htmlEscape escapes the five special HTML characters.
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&#34;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// defaultAlertURL is returned by safeDetailURL when the payload URL is empty
// or fails the http/https scheme allow-list.  Exported in test-friendly form
// via the symbol but kept package-private — alert email recipients always
// land on the alerts dashboard.
const defaultAlertURL = "https://idcd.com/app/alerts"

// safeDetailURL coerces an untrusted payload URL into a value that is safe to
// drop into an HTML href attribute.  Rules:
//
//   - empty string → default alerts dashboard URL.
//   - any parse error → default URL.
//   - scheme must be http or https (case-insensitive); anything else
//     (`javascript:`, `data:`, `vbscript:`, `file:`, `tel:` typos, …) is
//     rejected and the default URL is used.
//   - scheme-relative URLs (`//evil.com/x`) are rejected — they would inherit
//     the user agent's scheme and could land on `javascript:` in some clients.
//
// Callers still HTML-escape the return value before writing it into the
// attribute; safeDetailURL handles scheme filtering, htmlEscape handles
// attribute-context characters.
func safeDetailURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultAlertURL
	}
	u, err := url.Parse(raw)
	if err != nil {
		return defaultAlertURL
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return defaultAlertURL
	}
	if u.Host == "" {
		return defaultAlertURL
	}
	return raw
}
