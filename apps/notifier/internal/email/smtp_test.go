package email

import (
	"context"
	"testing"

	"github.com/kite365/idcd/packages/shared/apperr"
)

// mockSMTPSender for testing SMTP functionality
type mockSMTPSender struct {
	config   SMTPConfig
	sentMsgs []Message
	sendErr  error
}

func newMockSMTPSender(config SMTPConfig) *mockSMTPSender {
	return &mockSMTPSender{
		config:   config,
		sentMsgs: make([]Message, 0),
	}
}

func (m *mockSMTPSender) Send(ctx context.Context, msg Message) error {
	if m.sendErr != nil {
		return m.sendErr
	}

	// Validate message like real SMTP sender would
	sender := &SMTPSender{config: m.config}
	if err := sender.validateMessage(msg); err != nil {
		return err
	}

	m.sentMsgs = append(m.sentMsgs, msg)
	return nil
}

func TestSMTPSender_Send_Success(t *testing.T) {
	config := SMTPConfig{
		Host:     "smtp.gmail.com",
		Port:     587,
		Username: "test@gmail.com",
		Password: "password",
		From:     "noreply@idcd.com",
		FromName: "idcd",
	}

	mock := newMockSMTPSender(config)

	msg := Message{
		To:      "user@example.com",
		Subject: "Test Subject",
		HTML:    "<h1>Test HTML Content</h1>",
	}

	err := mock.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(mock.sentMsgs) != 1 {
		t.Fatalf("Expected 1 sent message, got: %d", len(mock.sentMsgs))
	}

	sentMsg := mock.sentMsgs[0]
	if sentMsg.To != msg.To {
		t.Errorf("Expected To: %s, got: %s", msg.To, sentMsg.To)
	}
	if sentMsg.Subject != msg.Subject {
		t.Errorf("Expected Subject: %s, got: %s", msg.Subject, sentMsg.Subject)
	}
	if sentMsg.HTML != msg.HTML {
		t.Errorf("Expected HTML: %s, got: %s", msg.HTML, sentMsg.HTML)
	}
}

func TestSMTPSender_ValidateMessage(t *testing.T) {
	config := SMTPConfig{
		Host: "smtp.gmail.com",
		Port: 587,
		From: "noreply@idcd.com",
	}

	sender := NewSMTPSender(config)

	tests := []struct {
		name    string
		msg     Message
		wantErr bool
		errCode apperr.Code
	}{
		{
			name: "valid message",
			msg: Message{
				To:      "user@example.com",
				Subject: "Test",
				HTML:    "<p>Content</p>",
			},
			wantErr: false,
		},
		{
			name: "missing recipient",
			msg: Message{
				To:      "",
				Subject: "Test",
				HTML:    "<p>Content</p>",
			},
			wantErr: true,
			errCode: apperr.CodeValidation,
		},
		{
			name: "missing subject",
			msg: Message{
				To:      "user@example.com",
				Subject: "",
				HTML:    "<p>Content</p>",
			},
			wantErr: true,
			errCode: apperr.CodeValidation,
		},
		{
			name: "missing content",
			msg: Message{
				To:      "user@example.com",
				Subject: "Test",
				HTML:    "",
			},
			wantErr: true,
			errCode: apperr.CodeValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sender.validateMessage(tt.msg)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if !apperr.Is(err, tt.errCode) {
					t.Errorf("Expected error code %s, got %v", tt.errCode, err)
				}
			} else if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

func TestSMTPSender_BuildFromAddress(t *testing.T) {
	tests := []struct {
		name     string
		config   SMTPConfig
		expected string
	}{
		{
			name: "with display name",
			config: SMTPConfig{
				From:     "noreply@idcd.com",
				FromName: "idcd",
			},
			expected: "idcd <noreply@idcd.com>",
		},
		{
			name: "without display name",
			config: SMTPConfig{
				From:     "noreply@idcd.com",
				FromName: "",
			},
			expected: "noreply@idcd.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := NewSMTPSender(tt.config)
			result := sender.buildFromAddress()

			if result != tt.expected {
				t.Errorf("Expected: %s, got: %s", tt.expected, result)
			}
		})
	}
}

func TestSMTPSender_BuildMessage(t *testing.T) {
	sender := NewSMTPSender(SMTPConfig{})

	from := "idcd <noreply@idcd.com>"
	msg := Message{
		To:      "user@example.com",
		Subject: "Test Subject",
		HTML:    "<h1>Test Content</h1>",
	}

	result := sender.buildMessage(from, msg)
	resultStr := string(result)

	// Check essential headers
	expectedHeaders := []string{
		"From: idcd <noreply@idcd.com>",
		"To: user@example.com",
		"Subject: Test Subject",
		"Content-Type: text/html; charset=UTF-8",
		"MIME-Version: 1.0",
	}

	for _, header := range expectedHeaders {
		if !contains(resultStr, header) {
			t.Errorf("Expected header %q not found in message", header)
		}
	}

	// Check body content
	if !contains(resultStr, "<h1>Test Content</h1>") {
		t.Error("HTML content not found in message body")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}