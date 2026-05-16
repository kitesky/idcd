package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kite365/idcd/apps/notifier/internal/email"
	"github.com/kite365/idcd/apps/notifier/internal/template"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// ---- refund retry stubs ----

type stubRefunder struct {
	mu    sync.Mutex
	calls []refundCall
	err   error
}

type refundCall struct {
	ExtTxnID    string
	AmountCents int64
	Reason      string
}

func (s *stubRefunder) RefundPayment(_ context.Context, extTxnID string, amountCents int64, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, refundCall{extTxnID, amountCents, reason})
	return s.err
}

func (s *stubRefunder) Calls() []refundCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]refundCall, len(s.calls))
	copy(out, s.calls)
	return out
}

type stubPaymentStore struct {
	mu                  sync.Mutex
	refundedIDs         []string
	failedIDs           []string
	failedRetryCounts   []int
	refundedErr         error
	failedErr           error
}

func (s *stubPaymentStore) MarkRefunded(_ context.Context, paymentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refundedErr != nil {
		return s.refundedErr
	}
	s.refundedIDs = append(s.refundedIDs, paymentID)
	return nil
}

func (s *stubPaymentStore) MarkRefundFailed(_ context.Context, paymentID string, retryCount int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failedErr != nil {
		return s.failedErr
	}
	s.failedIDs = append(s.failedIDs, paymentID)
	s.failedRetryCounts = append(s.failedRetryCounts, retryCount)
	return nil
}

func (s *stubPaymentStore) Refunded() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.refundedIDs))
	copy(out, s.refundedIDs)
	return out
}

func (s *stubPaymentStore) Failed() ([]string, []int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, len(s.failedIDs))
	counts := make([]int, len(s.failedRetryCounts))
	copy(ids, s.failedIDs)
	copy(counts, s.failedRetryCounts)
	return ids, counts
}

type stubRetryEnqueuer struct {
	mu    sync.Mutex
	calls []retryCall
	err   error
}

type retryCall struct {
	Payload RefundRetryPayload
	Delay   time.Duration
}

func (s *stubRetryEnqueuer) EnqueueRefundRetry(_ context.Context, payload RefundRetryPayload, delay time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.calls = append(s.calls, retryCall{payload, delay})
	return nil
}

func (s *stubRetryEnqueuer) Calls() []retryCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]retryCall, len(s.calls))
	copy(out, s.calls)
	return out
}

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

// ---- HandleRefundRetry tests (D5) ----

// newRetryTask builds an asynq task from a RefundRetryPayload.
func newRetryTask(t *testing.T, p RefundRetryPayload) *asynq.Task {
	t.Helper()
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return asynq.NewTask(TaskRefundRetry, b)
}

func TestHandlers_HandleRefundRetry_Success(t *testing.T) {
	handlers, sender := setupHandlers(t)
	refunder := &stubRefunder{}
	store := &stubPaymentStore{}
	enq := &stubRetryEnqueuer{}
	handlers = handlers.WithRefundDeps(refunder, store, enq)

	payload := RefundRetryPayload{
		PaymentID:    "pay_001",
		ExtTxnID:     "ph_txn_001",
		UserID:       "u_user1",
		UserEmail:    "user@example.com",
		AmountCents:  9900,
		Currency:     "CNY",
		Provider:     "payment_hub",
		Reason:       "webhook_refund_failed",
		AttemptCount: 0,
	}

	err := handlers.HandleRefundRetry(context.Background(), newRetryTask(t, payload))
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	calls := refunder.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 refund call, got %d", len(calls))
	}
	if calls[0].ExtTxnID != "ph_txn_001" || calls[0].AmountCents != 9900 {
		t.Errorf("unexpected refund call: %+v", calls[0])
	}
	refunded := store.Refunded()
	if len(refunded) != 1 || refunded[0] != "pay_001" {
		t.Errorf("expected MarkRefunded(pay_001), got: %v", refunded)
	}
	if len(enq.Calls()) != 0 {
		t.Errorf("no rescheduling should happen on success, got: %v", enq.Calls())
	}
	if len(sender.sentMessages) != 0 {
		t.Errorf("no apology email should be sent on success")
	}
}

func TestHandlers_HandleRefundRetry_FailReschedules(t *testing.T) {
	handlers, sender := setupHandlers(t)
	refunder := &stubRefunder{err: errors.New("paddle 502")}
	store := &stubPaymentStore{}
	enq := &stubRetryEnqueuer{}
	handlers = handlers.WithRefundDeps(refunder, store, enq)

	payload := RefundRetryPayload{
		PaymentID:    "pay_001",
		ExtTxnID:     "ph_txn_001",
		UserID:       "u_user1",
		UserEmail:    "user@example.com",
		AmountCents:  9900,
		Currency:     "CNY",
		Provider:     "payment_hub",
		Reason:       "webhook_refund_failed",
		AttemptCount: 0,
	}

	err := handlers.HandleRefundRetry(context.Background(), newRetryTask(t, payload))
	if err != nil {
		t.Fatalf("expected nil err (handler swallows after rescheduling), got: %v", err)
	}

	// Refund attempted.
	if len(refunder.Calls()) != 1 {
		t.Fatalf("expected 1 refund call, got %d", len(refunder.Calls()))
	}

	// One reschedule with the 30min-style delay.
	calls := enq.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 reschedule call, got %d", len(calls))
	}
	if calls[0].Delay != RefundRetrySecondDelay {
		t.Errorf("expected delay %v, got %v", RefundRetrySecondDelay, calls[0].Delay)
	}
	if calls[0].Payload.AttemptCount != 1 {
		t.Errorf("expected next attempt_count=1, got %d", calls[0].Payload.AttemptCount)
	}

	// Failure counter bumped.
	ids, counts := store.Failed()
	if len(ids) != 1 || ids[0] != "pay_001" || counts[0] != 1 {
		t.Errorf("expected MarkRefundFailed(pay_001, 1), got ids=%v counts=%v", ids, counts)
	}

	// No apology yet — we still have one attempt left.
	if len(sender.sentMessages) != 0 {
		t.Errorf("apology should not fire on first retry failure")
	}
	if len(store.Refunded()) != 0 {
		t.Errorf("payment must not be marked refunded after a failed retry")
	}
}

func TestHandlers_HandleRefundRetry_MaxAttemptsSendsApology(t *testing.T) {
	handlers, sender := setupHandlers(t)
	refunder := &stubRefunder{err: errors.New("paddle 500")}
	store := &stubPaymentStore{}
	enq := &stubRetryEnqueuer{}
	handlers = handlers.WithRefundDeps(refunder, store, enq)

	// Already retried once — AttemptCount=1 means this is the second
	// automated try.  nextAttempt becomes 2 → >= RefundRetryMaxAttempts → apology.
	payload := RefundRetryPayload{
		PaymentID:    "pay_001",
		ExtTxnID:     "ph_txn_001",
		UserID:       "u_user1",
		UserEmail:    "user@example.com",
		AmountCents:  19900,
		Currency:     "CNY",
		Provider:     "payment_hub",
		Reason:       "webhook_refund_failed",
		AttemptCount: 1,
	}

	err := handlers.HandleRefundRetry(context.Background(), newRetryTask(t, payload))
	if err != nil {
		t.Fatalf("expected nil err after sending apology, got: %v", err)
	}

	// No further rescheduling.
	if len(enq.Calls()) != 0 {
		t.Errorf("no reschedule should happen after max attempts, got: %v", enq.Calls())
	}

	// Payment left in refund_failed for admin dashboard.
	ids, counts := store.Failed()
	if len(ids) != 1 || ids[0] != "pay_001" || counts[0] != RefundRetryMaxAttempts {
		t.Errorf("expected MarkRefundFailed(pay_001, %d), got ids=%v counts=%v",
			RefundRetryMaxAttempts, ids, counts)
	}
	if len(store.Refunded()) != 0 {
		t.Errorf("payment must NOT be marked refunded after max attempts fail")
	}

	// Apology email sent.
	if len(sender.sentMessages) != 1 {
		t.Fatalf("expected 1 apology email, got %d", len(sender.sentMessages))
	}
	msg := sender.sentMessages[0]
	if msg.To != "user@example.com" {
		t.Errorf("apology To mismatch: %s", msg.To)
	}
	if !strings.Contains(msg.Subject, "致歉") {
		t.Errorf("apology Subject should mention 致歉, got: %s", msg.Subject)
	}
	if !strings.Contains(msg.HTML, "pay_001") || !strings.Contains(msg.HTML, "ph_txn_001") {
		t.Errorf("apology HTML should include payment + ext_txn ids")
	}
}

func TestHandlers_HandleRefundRetry_MaxAttemptsNoEmailSkipsApology(t *testing.T) {
	handlers, sender := setupHandlers(t)
	refunder := &stubRefunder{err: errors.New("paddle 500")}
	store := &stubPaymentStore{}
	enq := &stubRetryEnqueuer{}
	handlers = handlers.WithRefundDeps(refunder, store, enq)

	payload := RefundRetryPayload{
		PaymentID:    "pay_no_email",
		ExtTxnID:     "ph_txn_002",
		UserID:       "u_user1",
		UserEmail:    "", // no email captured upstream
		AmountCents:  9900,
		Currency:     "CNY",
		Provider:     "payment_hub",
		AttemptCount: 1,
	}

	err := handlers.HandleRefundRetry(context.Background(), newRetryTask(t, payload))
	if err != nil {
		t.Fatalf("expected nil err, got: %v", err)
	}

	if len(sender.sentMessages) != 0 {
		t.Errorf("no apology should fire without user email")
	}
	ids, _ := store.Failed()
	if len(ids) != 1 {
		t.Errorf("payment must still be marked refund_failed for admin dashboard")
	}
}

func TestHandlers_HandleRefundRetry_DepsNotWired(t *testing.T) {
	handlers, _ := setupHandlers(t)
	// Intentionally no WithRefundDeps.

	payload := RefundRetryPayload{
		PaymentID:   "pay_001",
		ExtTxnID:    "ph_txn_001",
		AmountCents: 9900,
	}
	err := handlers.HandleRefundRetry(context.Background(), newRetryTask(t, payload))
	if err == nil {
		t.Fatal("expected error when deps not wired")
	}
}

func TestHandlers_HandleRefundRetry_InvalidPayload(t *testing.T) {
	handlers, _ := setupHandlers(t)
	handlers = handlers.WithRefundDeps(&stubRefunder{}, &stubPaymentStore{}, &stubRetryEnqueuer{})

	// Empty payment_id → validation error.
	bad := RefundRetryPayload{ExtTxnID: "x", AmountCents: 100}
	err := handlers.HandleRefundRetry(context.Background(), newRetryTask(t, bad))
	if err == nil || !apperr.Is(err, apperr.CodeValidation) {
		t.Errorf("expected validation error for empty payment_id, got: %v", err)
	}

	// Zero amount → validation error.
	bad2 := RefundRetryPayload{PaymentID: "p", ExtTxnID: "x", AmountCents: 0}
	err = handlers.HandleRefundRetry(context.Background(), newRetryTask(t, bad2))
	if err == nil || !apperr.Is(err, apperr.CodeValidation) {
		t.Errorf("expected validation error for zero amount, got: %v", err)
	}

	// Malformed JSON.
	task := asynq.NewTask(TaskRefundRetry, []byte("not json"))
	err = handlers.HandleRefundRetry(context.Background(), task)
	if err == nil {
		t.Errorf("expected error for malformed JSON")
	}
}

func TestHandlers_HandleRefundRetry_MarkRefundedError(t *testing.T) {
	handlers, _ := setupHandlers(t)
	refunder := &stubRefunder{} // success
	store := &stubPaymentStore{refundedErr: errors.New("db down")}
	enq := &stubRetryEnqueuer{}
	handlers = handlers.WithRefundDeps(refunder, store, enq)

	payload := RefundRetryPayload{
		PaymentID:   "pay_001",
		ExtTxnID:    "ph_txn_001",
		AmountCents: 9900,
		Currency:    "CNY",
		Provider:    "payment_hub",
	}
	err := handlers.HandleRefundRetry(context.Background(), newRetryTask(t, payload))
	if err == nil {
		t.Errorf("expected error so asynq retries when MarkRefunded fails")
	}
}