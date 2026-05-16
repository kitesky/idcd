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

// TestBuildAlertHTML_SanitisesURL covers the P1#14 fix: a malicious payload
// URL must never end up executable inside the rendered href.  We exercise the
// scheme allow-list plus the attribute-context escape in one place.
func TestBuildAlertHTML_SanitisesURL(t *testing.T) {
	cases := []struct {
		name     string
		url      string
		// mustContain checks for substrings that should appear in the rendered HTML
		mustContain []string
		// mustNotContain checks for substrings that must NOT appear (raw,
		// unescaped) anywhere in the rendered HTML.
		mustNotContain []string
	}{
		{
			name:           "javascript: scheme rejected → default URL",
			url:            "javascript:alert(1)",
			mustContain:    []string{"https://idcd.com/app/alerts"},
			mustNotContain: []string{"javascript:alert", `href="javascript:`},
		},
		{
			name:           "data: scheme rejected",
			url:            "data:text/html,<script>alert(1)</script>",
			mustContain:    []string{"https://idcd.com/app/alerts"},
			mustNotContain: []string{"data:text/html", "<script>alert(1)</script>"},
		},
		{
			name:           "vbscript: scheme rejected",
			url:            "vbscript:msgbox(1)",
			mustContain:    []string{"https://idcd.com/app/alerts"},
			mustNotContain: []string{"vbscript:"},
		},
		{
			name:           "scheme-relative rejected",
			url:            "//evil.example.com/x",
			mustContain:    []string{"https://idcd.com/app/alerts"},
			mustNotContain: []string{"//evil.example.com"},
		},
		{
			name:           "attribute-breaking quote escaped",
			url:            `https://example.com/x"><script>alert(1)</script>`,
			// Quote must be HTML-escaped.  Note: html/template-style escapes
			// look like &#34; — the literal `"` must NOT appear unescaped in
			// the href attribute.
			mustContain:    []string{"&#34;"},
			mustNotContain: []string{`x"><script>`, `<script>alert(1)</script>`},
		},
		{
			name: "uppercase HTTPS accepted",
			url:  "HTTPS://example.com/alerts/123",
			// The footer always links back to the alerts dashboard so we
			// only assert that the supplied URL is preserved in the CTA
			// href — we do NOT assert absence of the default URL.
			mustContain:    []string{`href="HTTPS://example.com/alerts/123"`},
			mustNotContain: []string{},
		},
		{
			name:           "empty URL → default",
			url:            "",
			mustContain:    []string{"https://idcd.com/app/alerts"},
			mustNotContain: []string{},
		},
		{
			name:           "whitespace-only URL → default",
			url:            "   ",
			mustContain:    []string{"https://idcd.com/app/alerts"},
			mustNotContain: []string{},
		},
		{
			name:           "control-char URL rejected (parse error)",
			url:            "http://example.com/\x00abc",
			mustContain:    []string{"https://idcd.com/app/alerts"},
			mustNotContain: []string{"\x00abc"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := Payload{
				Title: "DB down",
				Body:  "ignored",
				URL:   tc.url,
				Level: "critical",
			}
			html := buildAlertHTML(p, "FIRING")
			for _, want := range tc.mustContain {
				if !strings.Contains(html, want) {
					t.Errorf("expected %q in HTML, got: %s", want, html)
				}
			}
			for _, banned := range tc.mustNotContain {
				if strings.Contains(html, banned) {
					t.Errorf("did NOT expect %q in HTML, got: %s", banned, html)
				}
			}
		})
	}
}

// TestSafeDetailURL exercises the URL filter directly so future callers can
// rely on it for similar sanitisation needs.
func TestSafeDetailURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", defaultAlertURL},
		{"   ", defaultAlertURL},
		{"https://idcd.com/app/alerts/abc", "https://idcd.com/app/alerts/abc"},
		{"http://example.com/", "http://example.com/"},
		{"javascript:alert(1)", defaultAlertURL},
		{"JAVASCRIPT:alert(1)", defaultAlertURL},
		{"data:text/html,foo", defaultAlertURL},
		{"vbscript:x", defaultAlertURL},
		{"//evil.com/x", defaultAlertURL},
		{"mailto:a@b.c", defaultAlertURL},
		{"ftp://example.com/", defaultAlertURL},
		{"https://", defaultAlertURL},          // no host
		{"http://\x00bad", defaultAlertURL},   // parse error path
	}
	for _, c := range cases {
		got := safeDetailURL(c.in)
		if got != c.want {
			t.Errorf("safeDetailURL(%q) = %q, want %q", c.in, got, c.want)
		}
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
