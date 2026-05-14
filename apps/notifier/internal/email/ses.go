package email

import (
	"context"

	"github.com/kite365/idcd/lib/shared/apperr"
)

// SESConfig holds AWS SES configuration.
type SESConfig struct {
	Region    string `yaml:"region"`     // AWS region (e.g., "us-east-1")
	AccessKey string `yaml:"access_key"` // AWS access key ID
	SecretKey string `yaml:"secret_key"` // AWS secret access key
	From      string `yaml:"from"`       // sender email address
	FromName  string `yaml:"from_name"`  // sender display name
}

// SESSender implements the Sender interface using AWS SES.
// This is a stub implementation for S1 phase.
type SESSender struct {
	config SESConfig
}

// NewSESSender creates a new AWS SES email sender.
func NewSESSender(config SESConfig) *SESSender {
	return &SESSender{config: config}
}

// Send sends an email using AWS SES.
// TODO: Implement actual SES integration when needed.
func (s *SESSender) Send(ctx context.Context, msg Message) error {
	// Stub implementation - always returns an error indicating it's not implemented
	return apperr.Unavailable("SES发送功能尚未实现", nil)
}