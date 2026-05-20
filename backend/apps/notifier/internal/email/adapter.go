// Package email provides email sending abstractions.
package email

import "context"

// Message represents an email message to be sent.
type Message struct {
	To      string // recipient email address
	Subject string // email subject line
	HTML    string // HTML content of the email
}

// Sender is the interface for sending email messages.
// Different implementations can provide SMTP, SES, or other email services.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}