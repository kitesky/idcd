package channel

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/kite365/idcd/apps/notifier/internal/email"
)

// mockEmailSender captures sent messages for assertion.
type mockEmailSender struct {
	sent []email.Message
	err  error
}

func (m *mockEmailSender) Send(_ context.Context, msg email.Message) error {
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, msg)
	return nil
}

func TestEmailChannel_Type(t *testing.T) {
	sender := &mockEmailSender{}
	ch, err := NewEmail(EmailConfig{To: "ops@example.com"}, sender)
	if err != nil {
		t.Fatalf("NewEmail: %v", err)
	}
	if ch.Type() != "email" {
		t.Errorf("expected type 'email', got %q", ch.Type())
	}
}

func TestEmailChannel_Send_Firing(t *testing.T) {
	sender := &mockEmailSender{}
	ch, err := NewEmail(EmailConfig{To: "ops@example.com", SubjectPrefix: "[idcd Alert]"}, sender)
	if err != nil {
		t.Fatalf("NewEmail: %v", err)
	}

	p := Payload{
		Title: "API latency high",
		Body:  "P99 latency exceeded 2s threshold.",
		URL:   "https://idcd.com/app/alerts",
		Level: "critical",
	}

	if err := ch.Send(context.Background(), p); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 message sent, got %d", len(sender.sent))
	}
	msg := sender.sent[0]

	if msg.To != "ops@example.com" {
		t.Errorf("To: expected %q, got %q", "ops@example.com", msg.To)
	}
	if !strings.Contains(msg.Subject, "FIRING") {
		t.Errorf("Subject should contain FIRING, got %q", msg.Subject)
	}
	if !strings.Contains(msg.Subject, "API latency high") {
		t.Errorf("Subject should contain monitor name, got %q", msg.Subject)
	}
	if !strings.Contains(msg.Subject, "[idcd Alert]") {
		t.Errorf("Subject should contain prefix, got %q", msg.Subject)
	}
	if !strings.Contains(msg.HTML, "FIRING") {
		t.Error("HTML body should contain FIRING status badge")
	}
	if !strings.Contains(msg.HTML, "API latency high") {
		t.Error("HTML body should contain monitor name")
	}
	if !strings.Contains(msg.HTML, "https://idcd.com/app/alerts") {
		t.Error("HTML body should contain detail URL")
	}
}

func TestEmailChannel_Send_Resolved(t *testing.T) {
	sender := &mockEmailSender{}
	ch, err := NewEmail(EmailConfig{To: "ops@example.com"}, sender)
	if err != nil {
		t.Fatalf("NewEmail: %v", err)
	}

	p := Payload{
		Title: "API latency high",
		Body:  "Monitor recovered.",
		URL:   "https://idcd.com/app/alerts",
		Level: "resolved",
	}

	if err := ch.Send(context.Background(), p); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 message sent, got %d", len(sender.sent))
	}
	msg := sender.sent[0]

	if !strings.Contains(msg.Subject, "RESOLVED") {
		t.Errorf("Subject should contain RESOLVED, got %q", msg.Subject)
	}
	if !strings.Contains(msg.HTML, "RESOLVED") {
		t.Error("HTML body should contain RESOLVED status badge")
	}
}

func TestEmailChannel_Send_DefaultSubjectPrefix(t *testing.T) {
	sender := &mockEmailSender{}
	ch, err := NewEmail(EmailConfig{To: "ops@example.com"}, sender)
	if err != nil {
		t.Fatalf("NewEmail: %v", err)
	}

	p := Payload{Title: "DB down", Level: "critical"}
	if err := ch.Send(context.Background(), p); err != nil {
		t.Fatalf("Send: %v", err)
	}

	msg := sender.sent[0]
	if !strings.Contains(msg.Subject, "[idcd Alert]") {
		t.Errorf("expected default prefix '[idcd Alert]' in subject, got %q", msg.Subject)
	}
}

func TestNewEmail_MissingTo(t *testing.T) {
	sender := &mockEmailSender{}
	_, err := NewEmail(EmailConfig{To: ""}, sender)
	if err == nil {
		t.Fatal("expected error for empty To, got nil")
	}
	if !strings.Contains(err.Error(), "to address") {
		t.Errorf("expected error about 'to address', got %q", err.Error())
	}
}

func TestNewEmail_NilSender(t *testing.T) {
	_, err := NewEmail(EmailConfig{To: "ops@example.com"}, nil)
	if err == nil {
		t.Fatal("expected error for nil sender, got nil")
	}
}

func TestEmailChannel_Send_SenderError(t *testing.T) {
	sendErr := errors.New("SMTP connection refused")
	sender := &mockEmailSender{err: sendErr}
	ch, err := NewEmail(EmailConfig{To: "ops@example.com"}, sender)
	if err != nil {
		t.Fatalf("NewEmail: %v", err)
	}

	p := Payload{Title: "DB down", Level: "critical"}
	err = ch.Send(context.Background(), p)
	if err == nil {
		t.Fatal("expected error from sender, got nil")
	}
	if !strings.Contains(err.Error(), "email channel: send failed") {
		t.Errorf("expected wrapped error, got %q", err.Error())
	}
}

func TestBuildAlertHTML_EscapesSpecialChars(t *testing.T) {
	p := Payload{
		Title: "<script>alert('xss')</script>",
		Body:  "Monitor & status: 'critical' \"issue\"",
		Level: "critical",
	}
	html := buildAlertHTML(p, "FIRING")

	if strings.Contains(html, "<script>") {
		t.Error("HTML should escape <script> tags")
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Error("HTML should contain escaped <script>")
	}
	if strings.Contains(html, "alert('xss')") {
		t.Error("HTML should escape single quotes in title")
	}
}

func TestAlertStatus(t *testing.T) {
	cases := []struct {
		level    string
		expected string
	}{
		{"resolved", "RESOLVED"},
		{"RESOLVED", "RESOLVED"},
		{"critical", "FIRING"},
		{"warning", "FIRING"},
		{"info", "FIRING"},
		{"", "FIRING"},
	}
	for _, c := range cases {
		p := Payload{Level: c.level}
		got := alertStatus(p)
		if got != c.expected {
			t.Errorf("alertStatus(%q) = %q, want %q", c.level, got, c.expected)
		}
	}
}
