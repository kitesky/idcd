package service

import (
	"fmt"
	"strings"
	"time"
)

// NotificationData carries the fields needed to render the 4 cert
// notification templates. Fields are optional per-event-type; renderers
// degrade gracefully when something is missing.
type NotificationData struct {
	// EventType is one of cert.issued / cert.failed / cert.expiring /
	// cert.renewal_failed. Set by the watcher when calling the renderer.
	EventType string
	// AccountID owns the cert / order in question. Used to scope follow-up
	// links if downstream notifier wants to deep-link by account.
	AccountID int64
	// CertID is the cert.certs row id (zero when not applicable).
	CertID int64
	// OrderID is the cert.orders row id (zero when not applicable).
	OrderID int64
	// SANs is the list of domain names on the cert. Always shown when
	// non-empty.
	SANs []string
	// CA is the issuing CA identifier ("lets-encrypt", etc).
	CA string
	// NotAfter is the cert expiry timestamp; nil when not applicable.
	NotAfter *time.Time
	// DaysToExpire is the bucket (30/14/7/1) that triggered expiring
	// notifications; zero otherwise.
	DaysToExpire int
	// ErrorMsg is the cause string from acme_request_failed or a renewal
	// failure attempt.
	ErrorMsg string
}

// orderLink returns the user-facing URL for an order detail page.
func orderLink(orderID int64) string {
	if orderID <= 0 {
		return "https://idcd.com/app/cert/orders"
	}
	return fmt.Sprintf("https://idcd.com/app/cert/orders/%d", orderID)
}

// certLink returns the user-facing URL for a cert detail page.
func certLink(certID int64) string {
	if certID <= 0 {
		return "https://idcd.com/app/cert"
	}
	return fmt.Sprintf("https://idcd.com/app/cert/%d", certID)
}

func sanList(sans []string) string {
	if len(sans) == 0 {
		return "(无)"
	}
	return strings.Join(sans, ", ")
}

func formatTime(t *time.Time) string {
	if t == nil {
		return "(未知)"
	}
	return t.UTC().Format("2006-01-02 15:04 UTC")
}

// RenderIssued formats the "证书签发成功" notification.
func RenderIssued(d NotificationData) (subject, body string) {
	sans := sanList(d.SANs)
	subject = fmt.Sprintf("[idcd] 证书签发成功 — %s", sans)
	var sb strings.Builder
	sb.WriteString("您好，\n\n")
	sb.WriteString("您申请的 SSL/TLS 证书已签发成功：\n\n")
	fmt.Fprintf(&sb, "  域名 (SAN)：%s\n", sans)
	if d.CA != "" {
		fmt.Fprintf(&sb, "  签发 CA：%s\n", d.CA)
	}
	if d.NotAfter != nil {
		fmt.Fprintf(&sb, "  有效期至：%s\n", formatTime(d.NotAfter))
	}
	if d.CertID > 0 {
		fmt.Fprintf(&sb, "  证书 ID：%d\n", d.CertID)
	}
	sb.WriteString("\n您可以前往控制台下载证书：\n")
	sb.WriteString(certLink(d.CertID))
	sb.WriteString("\n\n— idcd 证书平台\n")
	body = sb.String()
	return
}

// RenderFailed formats the "证书签发失败" notification.
func RenderFailed(d NotificationData) (subject, body string) {
	sans := sanList(d.SANs)
	subject = fmt.Sprintf("[idcd] 证书签发失败 — %s", sans)
	var sb strings.Builder
	sb.WriteString("您好，\n\n")
	sb.WriteString("很抱歉，您申请的证书签发失败：\n\n")
	fmt.Fprintf(&sb, "  域名 (SAN)：%s\n", sans)
	if d.CA != "" {
		fmt.Fprintf(&sb, "  签发 CA：%s\n", d.CA)
	}
	if d.OrderID > 0 {
		fmt.Fprintf(&sb, "  订单 ID：%d\n", d.OrderID)
	}
	if d.ErrorMsg != "" {
		fmt.Fprintf(&sb, "  失败原因：%s\n", d.ErrorMsg)
	}
	sb.WriteString("\n您可以在控制台查看详情并重试：\n")
	sb.WriteString(orderLink(d.OrderID))
	sb.WriteString("\n\n— idcd 证书平台\n")
	body = sb.String()
	return
}

// RenderExpiring formats the "证书即将到期" notification.
func RenderExpiring(d NotificationData) (subject, body string) {
	sans := sanList(d.SANs)
	subject = fmt.Sprintf("[idcd] 证书将在 %d 天后到期 — %s", d.DaysToExpire, sans)
	var sb strings.Builder
	sb.WriteString("您好，\n\n")
	fmt.Fprintf(&sb, "您的证书将在 %d 天后到期，请及时续期：\n\n", d.DaysToExpire)
	fmt.Fprintf(&sb, "  域名 (SAN)：%s\n", sans)
	if d.CA != "" {
		fmt.Fprintf(&sb, "  签发 CA：%s\n", d.CA)
	}
	if d.NotAfter != nil {
		fmt.Fprintf(&sb, "  到期时间：%s\n", formatTime(d.NotAfter))
	}
	if d.CertID > 0 {
		fmt.Fprintf(&sb, "  证书 ID：%d\n", d.CertID)
	}
	sb.WriteString("\nidcd 已自动安排续期任务；如您使用了手动 DNS 模式，请确保 DNS 凭证仍然有效：\n")
	sb.WriteString(certLink(d.CertID))
	sb.WriteString("\n\n— idcd 证书平台\n")
	body = sb.String()
	return
}

// RenderRenewalFailed formats the "证书续期失败" notification.
func RenderRenewalFailed(d NotificationData) (subject, body string) {
	sans := sanList(d.SANs)
	subject = fmt.Sprintf("[idcd] 证书续期失败 — %s", sans)
	var sb strings.Builder
	sb.WriteString("您好，\n\n")
	sb.WriteString("您的证书自动续期任务多次失败，请尽快人工介入：\n\n")
	fmt.Fprintf(&sb, "  域名 (SAN)：%s\n", sans)
	if d.CA != "" {
		fmt.Fprintf(&sb, "  签发 CA：%s\n", d.CA)
	}
	if d.NotAfter != nil {
		fmt.Fprintf(&sb, "  当前到期时间：%s\n", formatTime(d.NotAfter))
	}
	if d.CertID > 0 {
		fmt.Fprintf(&sb, "  证书 ID：%d\n", d.CertID)
	}
	if d.ErrorMsg != "" {
		fmt.Fprintf(&sb, "  最近一次失败原因：%s\n", d.ErrorMsg)
	}
	sb.WriteString("\n请前往控制台检查 DNS 凭证 / 验证方式：\n")
	sb.WriteString(certLink(d.CertID))
	sb.WriteString("\n\n— idcd 证书平台\n")
	body = sb.String()
	return
}

// RenderNotification dispatches to the appropriate renderer by EventType.
// Returns ("", "") when EventType does not match a known template.
func RenderNotification(d NotificationData) (subject, body string) {
	switch d.EventType {
	case EventCertIssued:
		return RenderIssued(d)
	case EventCertFailed:
		return RenderFailed(d)
	case EventCertExpiring:
		return RenderExpiring(d)
	case EventCertRenewalFailed:
		return RenderRenewalFailed(d)
	}
	return "", ""
}
