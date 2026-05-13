package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/hibiken/asynq"

	"github.com/kite365/idcd/apps/notifier/internal/email"
	"github.com/kite365/idcd/apps/notifier/internal/template"
	"github.com/kite365/idcd/packages/shared/apperr"
)

// mockSender implements email.Sender for testing
type mockSender struct {
	sentMessages []email.Message
	sendError    error
}

func (m *mockSender) Send(ctx context.Context, msg email.Message) error {
	if m.sendError != nil {
		return m.sendError
	}
	m.sentMessages = append(m.sentMessages, msg)
	return nil
}

func (m *mockSender) reset() {
	m.sentMessages = nil
	m.sendError = nil
}

func setupHandlers(t *testing.T) (*Handlers, *mockSender) {
	// Create mock sender
	sender := &mockSender{}

	// Create templates
	templates, err := template.New()
	if err != nil {
		t.Fatalf("Failed to create templates: %v", err)
	}

	// Create logger (discard output for tests)
	logger := slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError + 1}))

	// Create handlers
	handlers := NewHandlers(sender, templates, logger)

	return handlers, sender
}

func TestHandlers_HandleSendVerifyEmail_Success(t *testing.T) {
	handlers, sender := setupHandlers(t)

	payload := SendVerifyEmailPayload{
		To:        "user@example.com",
		Code:      "123456",
		ExpiresIn: "10 分钟",
	}

	payloadBytes, _ := json.Marshal(payload)
	task := asynq.NewTask(TaskSendVerifyEmail, payloadBytes)

	err := handlers.HandleSendVerifyEmail(context.Background(), task)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(sender.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got: %d", len(sender.sentMessages))
	}

	msg := sender.sentMessages[0]
	if msg.To != payload.To {
		t.Errorf("Expected To: %s, got: %s", payload.To, msg.To)
	}
	if msg.Subject != "【idcd】邮箱验证码" {
		t.Errorf("Expected Subject: 【idcd】邮箱验证码, got: %s", msg.Subject)
	}
	if msg.HTML == "" {
		t.Error("Expected non-empty HTML content")
	}

	// Check if code is in the HTML
	if !contains(msg.HTML, payload.Code) {
		t.Error("Expected verification code in HTML content")
	}
}

func TestHandlers_HandleSendVerifyEmail_InvalidPayload(t *testing.T) {
	handlers, sender := setupHandlers(t)

	// Test with invalid JSON
	task := asynq.NewTask(TaskSendVerifyEmail, []byte("invalid json"))

	err := handlers.HandleSendVerifyEmail(context.Background(), task)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}

	if len(sender.sentMessages) != 0 {
		t.Error("Expected no messages sent for invalid payload")
	}

	// Test with empty recipient
	payload := SendVerifyEmailPayload{
		To:        "",
		Code:      "123456",
		ExpiresIn: "10 分钟",
	}

	payloadBytes, _ := json.Marshal(payload)
	task = asynq.NewTask(TaskSendVerifyEmail, payloadBytes)

	err = handlers.HandleSendVerifyEmail(context.Background(), task)
	if err == nil {
		t.Error("Expected error for empty recipient, got nil")
	}
	if !apperr.Is(err, apperr.CodeValidation) {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

func TestHandlers_HandleSendWelcome_Success(t *testing.T) {
	handlers, sender := setupHandlers(t)

	payload := SendWelcomePayload{
		To:       "user@example.com",
		Username: "testuser",
	}

	payloadBytes, _ := json.Marshal(payload)
	task := asynq.NewTask(TaskSendWelcome, payloadBytes)

	err := handlers.HandleSendWelcome(context.Background(), task)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(sender.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got: %d", len(sender.sentMessages))
	}

	msg := sender.sentMessages[0]
	if msg.To != payload.To {
		t.Errorf("Expected To: %s, got: %s", payload.To, msg.To)
	}
	if msg.Subject != "欢迎加入 idcd！" {
		t.Errorf("Expected Subject: 欢迎加入 idcd！, got: %s", msg.Subject)
	}
	if msg.HTML == "" {
		t.Error("Expected non-empty HTML content")
	}

	// Check if username is in the HTML
	if !contains(msg.HTML, payload.Username) {
		t.Error("Expected username in HTML content")
	}
}

func TestHandlers_HandleSendWelcome_EmptyUsername(t *testing.T) {
	handlers, sender := setupHandlers(t)

	payload := SendWelcomePayload{
		To:       "user@example.com",
		Username: "", // empty username should fallback to email
	}

	payloadBytes, _ := json.Marshal(payload)
	task := asynq.NewTask(TaskSendWelcome, payloadBytes)

	err := handlers.HandleSendWelcome(context.Background(), task)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(sender.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got: %d", len(sender.sentMessages))
	}

	msg := sender.sentMessages[0]
	// Should use email as username fallback
	if !contains(msg.HTML, payload.To) {
		t.Error("Expected email as username fallback in HTML content")
	}
}

func TestHandlers_HandleSendResetPassword_Success(t *testing.T) {
	handlers, sender := setupHandlers(t)

	payload := SendResetPasswordPayload{
		To:        "user@example.com",
		ResetURL:  "https://idcd.com/reset-password?token=abc123",
		ExpiresIn: "30 分钟",
	}

	payloadBytes, _ := json.Marshal(payload)
	task := asynq.NewTask(TaskSendResetPassword, payloadBytes)

	err := handlers.HandleSendResetPassword(context.Background(), task)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(sender.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got: %d", len(sender.sentMessages))
	}

	msg := sender.sentMessages[0]
	if msg.To != payload.To {
		t.Errorf("Expected To: %s, got: %s", payload.To, msg.To)
	}
	if msg.Subject != "【idcd】密码重置" {
		t.Errorf("Expected Subject: 【idcd】密码重置, got: %s", msg.Subject)
	}
	if msg.HTML == "" {
		t.Error("Expected non-empty HTML content")
	}

	// Check if reset URL is in the HTML
	if !contains(msg.HTML, payload.ResetURL) {
		t.Error("Expected reset URL in HTML content")
	}
}

func TestHandlers_HandleSendResetPassword_InvalidPayload(t *testing.T) {
	handlers, sender := setupHandlers(t)

	// Test with empty reset URL
	payload := SendResetPasswordPayload{
		To:        "user@example.com",
		ResetURL:  "", // empty URL should cause validation error
		ExpiresIn: "30 分钟",
	}

	payloadBytes, _ := json.Marshal(payload)
	task := asynq.NewTask(TaskSendResetPassword, payloadBytes)

	err := handlers.HandleSendResetPassword(context.Background(), task)
	if err == nil {
		t.Error("Expected error for empty reset URL, got nil")
	}
	if !apperr.Is(err, apperr.CodeValidation) {
		t.Errorf("Expected validation error, got: %v", err)
	}

	if len(sender.sentMessages) != 0 {
		t.Error("Expected no messages sent for invalid payload")
	}
}

func TestHandlers_withRetry_ValidationError(t *testing.T) {
	handlers, _ := setupHandlers(t)

	// Create a handler that returns a validation error
	handler := func(ctx context.Context, task *asynq.Task) error {
		return apperr.Validation("test validation error", "")
	}

	wrappedHandler := handlers.withRetry(handler)

	task := asynq.NewTask("test:task", nil)
	err := wrappedHandler(context.Background(), task)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	// Should indicate validation error (will not retry)
	if !contains(err.Error(), "validation error, will not retry") {
		t.Errorf("Expected validation error message, got: %v", err)
	}
}

func TestHandlers_withRetry_OtherError(t *testing.T) {
	handlers, _ := setupHandlers(t)

	// Create a handler that returns a non-validation error
	expectedErr := apperr.Internal("test internal error", nil)
	handler := func(ctx context.Context, task *asynq.Task) error {
		return expectedErr
	}

	wrappedHandler := handlers.withRetry(handler)

	task := asynq.NewTask("test:task", nil)
	err := wrappedHandler(context.Background(), task)

	if err != expectedErr {
		t.Errorf("Expected original error %v, got: %v", expectedErr, err)
	}
}

func TestHandlers_GetMux(t *testing.T) {
	handlers, _ := setupHandlers(t)

	mux := handlers.GetMux()
	if mux == nil {
		t.Error("Expected non-nil ServeMux")
	}

	// We can't easily test the registered handlers without more complex setup,
	// but we can verify the mux was created
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}