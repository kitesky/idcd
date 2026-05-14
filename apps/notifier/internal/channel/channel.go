// Package channel defines the Channel interface and Payload type for alert notifications.
// Each channel adapter (webhook, wecom, dingtalk, feishu, etc.) implements this interface.
package channel

import "context"

// Payload contains the notification content to be sent.
type Payload struct {
	Title string // notification title
	Body  string // notification body text
	URL   string // link to details page
	Level string // "critical" | "warning" | "info"
}

// Channel is the common interface all alert channel adapters must implement.
type Channel interface {
	// Send delivers a notification payload.
	Send(ctx context.Context, p Payload) error
	// Type returns the channel type string (e.g. "webhook", "wecom").
	Type() string
}
