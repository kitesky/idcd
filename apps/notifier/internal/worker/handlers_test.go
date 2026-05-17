package worker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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
	mu                sync.Mutex
	refundedIDs       []string
	failedIDs         []string
	failedRetryCounts []int
	refundedErr       error
	failedErr         error
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
	mu           sync.Mutex
	sentMessages []email.Message
	sendError    error
}

func (m *mockSender) Send(_ context.Context, msg email.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendError != nil {
		return m.sendError
	}
	m.sentMessages = append(m.sentMessages, msg)
	return nil
}

func (m *mockSender) Messages() []email.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]email.Message, len(m.sentMessages))
	copy(out, m.sentMessages)
	return out
}

func setupHandlers(t *testing.T) (*Handlers, *mockSender) {
	// Create mock sender
	sender := &mockSender{}

	// Create templates
	templates, err := template.New()
	if err != nil {
		t.Fatalf("Failed to create templates: %v", err)
	}

	// Discard log output for tests.
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 1}))

	// Create handlers
	handlers := NewHandlers(sender, templates, logger)

	return handlers, sender
}

func TestHandlers_HandleSendVerifyEmail_Success(t *testing.T) {
	handlers, sender := setupHandlers(t)

	cases := []struct {
		name           string
		locale         string
		expectSubject  string
		expectInBody   []string
		notInBody      []string
	}{
		{
			name:          "cn explicit",
			locale:        "cn",
			expectSubject: "【idcd】邮箱验证码",
			expectInBody:  []string{"验证您的邮箱地址", "123456"},
			notInBody:     []string{"Verify your email address"},
		},
		{
			name:          "en explicit",
			locale:        "en",
			expectSubject: "[IDCD] Verify your email address",
			expectInBody:  []string{"Verify your email address", "123456"},
			notInBody:     []string{"验证您的邮箱地址"},
		},
		{
			name:          "empty locale falls back to default (cn)",
			locale:        "",
			expectSubject: "【idcd】邮箱验证码",
			expectInBody:  []string{"验证您的邮箱地址", "123456"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sender.sentMessages = nil
			payload := SendVerifyEmailPayload{
				To:        "user@example.com",
				Code:      "123456",
				ExpiresIn: "10 minutes",
				Locale:    tc.locale,
			}
			payloadBytes, _ := json.Marshal(payload)
			task := asynq.NewTask(TaskSendVerifyEmail, payloadBytes)

			if err := handlers.HandleSendVerifyEmail(context.Background(), task); err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			msgs := sender.Messages()
			if len(msgs) != 1 {
				t.Fatalf("Expected 1 sent message, got: %d", len(msgs))
			}
			msg := msgs[0]
			if msg.To != payload.To {
				t.Errorf("To = %s, want %s", msg.To, payload.To)
			}
			if msg.Subject != tc.expectSubject {
				t.Errorf("Subject = %q, want %q", msg.Subject, tc.expectSubject)
			}
			for _, want := range tc.expectInBody {
				if !strings.Contains(msg.HTML, want) {
					t.Errorf("HTML missing %q", want)
				}
			}
			for _, banned := range tc.notInBody {
				if strings.Contains(msg.HTML, banned) {
					t.Errorf("HTML must not contain %q", banned)
				}
			}
		})
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

	if len(sender.Messages()) != 0 {
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

	cases := []struct {
		locale        string
		expectSubject string
		expectInBody  []string
	}{
		{locale: "cn", expectSubject: "欢迎加入 idcd！", expectInBody: []string{"欢迎加入 idcd", "testuser"}},
		{locale: "en", expectSubject: "Welcome to idcd!", expectInBody: []string{"Welcome to idcd!", "testuser"}},
	}

	for _, tc := range cases {
		t.Run(tc.locale, func(t *testing.T) {
			sender.sentMessages = nil
			payload := SendWelcomePayload{
				To:       "user@example.com",
				Username: "testuser",
				Locale:   tc.locale,
			}
			payloadBytes, _ := json.Marshal(payload)
			task := asynq.NewTask(TaskSendWelcome, payloadBytes)

			if err := handlers.HandleSendWelcome(context.Background(), task); err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			msgs := sender.Messages()
			if len(msgs) != 1 {
				t.Fatalf("Expected 1 sent message, got: %d", len(msgs))
			}
			msg := msgs[0]
			if msg.Subject != tc.expectSubject {
				t.Errorf("Subject = %q, want %q", msg.Subject, tc.expectSubject)
			}
			for _, want := range tc.expectInBody {
				if !strings.Contains(msg.HTML, want) {
					t.Errorf("HTML missing %q", want)
				}
			}
		})
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

	msgs := sender.Messages()
	if len(msgs) != 1 {
		t.Fatalf("Expected 1 sent message, got: %d", len(msgs))
	}

	// Should use email as username fallback
	if !strings.Contains(msgs[0].HTML, payload.To) {
		t.Error("Expected email as username fallback in HTML content")
	}
}

func TestHandlers_HandleSendResetPassword_Success(t *testing.T) {
	handlers, sender := setupHandlers(t)

	cases := []struct {
		locale        string
		expectSubject string
		expectInBody  []string
	}{
		{locale: "cn", expectSubject: "【idcd】密码重置", expectInBody: []string{"密码重置请求"}},
		{locale: "en", expectSubject: "[IDCD] Reset your password", expectInBody: []string{"Password reset request"}},
	}

	for _, tc := range cases {
		t.Run(tc.locale, func(t *testing.T) {
			sender.sentMessages = nil
			payload := SendResetPasswordPayload{
				To:        "user@example.com",
				ResetURL:  "https://idcd.com/reset-password?token=abc123",
				ExpiresIn: "30 分钟",
				Locale:    tc.locale,
			}
			payloadBytes, _ := json.Marshal(payload)
			task := asynq.NewTask(TaskSendResetPassword, payloadBytes)

			if err := handlers.HandleSendResetPassword(context.Background(), task); err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			msgs := sender.Messages()
			if len(msgs) != 1 {
				t.Fatalf("Expected 1 sent message, got: %d", len(msgs))
			}
			msg := msgs[0]
			if msg.Subject != tc.expectSubject {
				t.Errorf("Subject = %q, want %q", msg.Subject, tc.expectSubject)
			}
			if !strings.Contains(msg.HTML, payload.ResetURL) {
				t.Error("HTML must include reset URL")
			}
			for _, want := range tc.expectInBody {
				if !strings.Contains(msg.HTML, want) {
					t.Errorf("HTML missing %q", want)
				}
			}
		})
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

	if len(sender.Messages()) != 0 {
		t.Error("Expected no messages sent for invalid payload")
	}
}

func TestHandlers_withRetry_ValidationError(t *testing.T) {
	handlers, _ := setupHandlers(t)

	// Create a handler that returns a validation error
	handler := func(_ context.Context, _ *asynq.Task) error {
		return apperr.Validation("test validation error", "")
	}

	wrappedHandler := handlers.withRetry(handler)

	task := asynq.NewTask("test:task", nil)
	err := wrappedHandler(context.Background(), task)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	// Should indicate validation error (will not retry)
	if !strings.Contains(err.Error(), "validation error, will not retry") {
		t.Errorf("Expected validation error message, got: %v", err)
	}
}

func TestHandlers_withRetry_OtherError(t *testing.T) {
	handlers, _ := setupHandlers(t)

	// Create a handler that returns a non-validation error
	expectedErr := apperr.Internal("test internal error", nil)
	handler := func(_ context.Context, _ *asynq.Task) error {
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
	if len(sender.Messages()) != 0 {
		t.Errorf("no apology email should be sent on success")
	}
}

func TestHandlers_HandleRefundRetry_FailReschedules(t *testing.T) {
	handlers, sender := setupHandlers(t)
	refunder := &stubRefunder{err: errors.New("paymenthub 502")}
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
	if len(sender.Messages()) != 0 {
		t.Errorf("apology should not fire on first retry failure")
	}
	if len(store.Refunded()) != 0 {
		t.Errorf("payment must not be marked refunded after a failed retry")
	}
}

func TestHandlers_HandleRefundRetry_MaxAttemptsSendsApology(t *testing.T) {
	cases := []struct {
		name             string
		locale           string
		expectedSubject  string
		expectInBody     []string
	}{
		{
			name:            "cn locale",
			locale:          "cn",
			expectedSubject: "【idcd】关于您退款延迟的致歉说明",
			expectInBody:    []string{"致歉", "pay_001", "ph_txn_001", "199.00 CNY"},
		},
		{
			name:            "en locale",
			locale:          "en",
			expectedSubject: "[IDCD] We're sorry about your delayed refund",
			expectInBody:    []string{"sorry", "pay_001", "ph_txn_001", "199.00 CNY"},
		},
		{
			name:            "empty locale falls back to default",
			locale:          "",
			expectedSubject: "【idcd】关于您退款延迟的致歉说明",
			expectInBody:    []string{"致歉"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handlers, sender := setupHandlers(t)
			refunder := &stubRefunder{err: errors.New("paymenthub 500")}
			store := &stubPaymentStore{}
			enq := &stubRetryEnqueuer{}
			handlers = handlers.WithRefundDeps(refunder, store, enq)

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
				Locale:       tc.locale,
			}

			err := handlers.HandleRefundRetry(context.Background(), newRetryTask(t, payload))
			if err != nil {
				t.Fatalf("expected nil err after sending apology, got: %v", err)
			}

			if len(enq.Calls()) != 0 {
				t.Errorf("no reschedule should happen after max attempts, got: %v", enq.Calls())
			}
			ids, counts := store.Failed()
			if len(ids) != 1 || ids[0] != "pay_001" || counts[0] != RefundRetryMaxAttempts {
				t.Errorf("expected MarkRefundFailed(pay_001, %d), got ids=%v counts=%v",
					RefundRetryMaxAttempts, ids, counts)
			}
			if len(store.Refunded()) != 0 {
				t.Errorf("payment must NOT be marked refunded after max attempts fail")
			}

			msgs := sender.Messages()
			if len(msgs) != 1 {
				t.Fatalf("expected 1 apology email, got %d", len(msgs))
			}
			msg := msgs[0]
			if msg.To != "user@example.com" {
				t.Errorf("apology To mismatch: %s", msg.To)
			}
			if msg.Subject != tc.expectedSubject {
				t.Errorf("Subject = %q, want %q", msg.Subject, tc.expectedSubject)
			}
			for _, want := range tc.expectInBody {
				if !strings.Contains(msg.HTML, want) {
					t.Errorf("apology HTML missing %q", want)
				}
			}
		})
	}
}

func TestHandlers_HandleRefundRetry_MaxAttemptsNoEmailSkipsApology(t *testing.T) {
	handlers, sender := setupHandlers(t)
	refunder := &stubRefunder{err: errors.New("paymenthub 500")}
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

	if len(sender.Messages()) != 0 {
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

// ---- helper tests ----

func TestBuildLocalizedURL(t *testing.T) {
	cases := []struct {
		name    string
		base    string
		locale  string
		path    string
		want    string
	}{
		{
			name:   "cn default no prefix",
			base:   "https://idcd.com",
			locale: "cn",
			path:   "/app/dashboard",
			want:   "https://idcd.com/app/dashboard",
		},
		{
			name:   "en gets /en/ prefix",
			base:   "https://idcd.com",
			locale: "en",
			path:   "/app/dashboard",
			want:   "https://idcd.com/en/app/dashboard",
		},
		{
			name:   "empty locale → default → no prefix",
			base:   "https://idcd.com",
			locale: "",
			path:   "/app",
			want:   "https://idcd.com/app",
		},
		{
			name:   "trailing slash on base is normalised",
			base:   "https://idcd.com/",
			locale: "en",
			path:   "app/dashboard",
			want:   "https://idcd.com/en/app/dashboard",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildLocalizedURL(tc.base, tc.locale, tc.path)
			if got != tc.want {
				t.Errorf("BuildLocalizedURL(%q,%q,%q) = %q, want %q",
					tc.base, tc.locale, tc.path, got, tc.want)
			}
		})
	}
}

func TestFormatFromAddress(t *testing.T) {
	cases := []struct {
		name   string
		locale string
		addr   string
		// We don't pin the exact display string (the catalog owns it) but we
		// require deterministic suffix + Q-encoding behaviour for non-ASCII.
		mustContainAddr     bool
		mustContainQEncoded bool
	}{
		{name: "cn (non-ASCII display)", locale: "cn", addr: "noreply@idcd.com", mustContainAddr: true, mustContainQEncoded: true},
		{name: "en (ASCII display)", locale: "en", addr: "noreply@idcd.com", mustContainAddr: true, mustContainQEncoded: false},
		{name: "empty locale → default", locale: "", addr: "noreply@idcd.com", mustContainAddr: true, mustContainQEncoded: true},
		{name: "empty addr returns empty", locale: "en", addr: "", mustContainAddr: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatFromAddress(tc.locale, tc.addr)
			if tc.addr == "" {
				if got != "" {
					t.Errorf("empty addr should return empty, got %q", got)
				}
				return
			}
			if tc.mustContainAddr && !strings.Contains(got, tc.addr) {
				t.Errorf("expected %q to contain %q", got, tc.addr)
			}
			if tc.mustContainQEncoded {
				// RFC 2047 Q-encoded words start with =?utf-8?q? prefix.
				if !strings.Contains(strings.ToLower(got), "=?utf-8?q?") {
					t.Errorf("expected Q-encoded display for non-ASCII, got %q", got)
				}
			}
		})
	}
}

// ---- EnqueueEmail ----

type fakeAsynq struct {
	tasks []*asynq.Task
}

func (f *fakeAsynq) EnqueueContext(_ context.Context, task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	f.tasks = append(f.tasks, task)
	return &asynq.TaskInfo{}, nil
}

func TestEnqueueEmail_SetsLocale(t *testing.T) {
	client := &fakeAsynq{}
	payload := SendVerifyEmailPayload{To: "u@example.com", Code: "123456", ExpiresIn: "10 minutes"}
	if err := EnqueueEmail(context.Background(), client, "en", payload); err != nil {
		t.Fatalf("EnqueueEmail: %v", err)
	}
	if len(client.tasks) != 1 {
		t.Fatalf("expected 1 task enqueued, got %d", len(client.tasks))
	}
	task := client.tasks[0]
	if task.Type() != TaskSendVerifyEmail {
		t.Errorf("task type = %s, want %s", task.Type(), TaskSendVerifyEmail)
	}
	var out map[string]any
	if err := json.Unmarshal(task.Payload(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["locale"] != "en" {
		t.Errorf("payload locale = %v, want en", out["locale"])
	}
	if out["to"] != "u@example.com" {
		t.Errorf("payload to lost in transit")
	}
}

func TestEnqueueEmail_RespectsExplicitPayloadLocale(t *testing.T) {
	client := &fakeAsynq{}
	payload := SendWelcomePayload{To: "u@example.com", Username: "u", Locale: "en"}
	// Caller-arg locale empty, but the payload already has "en" set → keep it.
	if err := EnqueueEmail(context.Background(), client, "", payload); err != nil {
		t.Fatalf("EnqueueEmail: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(client.tasks[0].Payload(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["locale"] != "en" {
		t.Errorf("explicit payload locale should win, got %v", out["locale"])
	}
}

func TestEnqueueEmail_NilClient(t *testing.T) {
	err := EnqueueEmail(context.Background(), nil, "en", SendWelcomePayload{To: "u@example.com"})
	if err == nil {
		t.Fatal("expected error for nil client")
	}
}

// ---- HandleRefundApology tests (D5 / attest-side refund-worker) ----
//
// The apology task payload is now self-contained (the refund-worker
// pre-resolves user_email / ext_order_id / amount via an
// application-level join). These tests therefore exercise the
// render-and-send path directly, plus the fail-open ACK when
// user_email is empty (rare race after a user deletion).

func newApologyTask(t *testing.T, p RefundApologyPayload) *asynq.Task {
	t.Helper()
	body, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal apology payload: %v", err)
	}
	return asynq.NewTask(TaskRefundApology, body)
}

func TestHandlers_HandleRefundApology_Success(t *testing.T) {
	cases := []struct {
		name            string
		locale          string
		expectedSubject string
		expectInBody    []string
	}{
		{
			name:            "cn locale on payload",
			locale:          "cn",
			expectedSubject: "【idcd】关于您退款延迟的致歉说明",
			expectInBody:    []string{"致歉", "v_001", "ord_abc", "99.00 CNY"},
		},
		{
			name:            "en locale on payload",
			locale:          "en",
			expectedSubject: "[IDCD] We're sorry about your delayed refund",
			expectInBody:    []string{"sorry", "v_001", "ord_abc", "99.00 CNY"},
		},
		{
			name:            "empty locale falls back to registry default",
			locale:          "",
			expectedSubject: "【idcd】关于您退款延迟的致歉说明",
			expectInBody:    []string{"致歉"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handlers, sender := setupHandlers(t)

			payload := RefundApologyPayload{
				OrderID:           "v_001",
				UserEmail:         "user@example.com",
				ExtOrderID:     "ord_abc",
				RefundAmountCents: 9900,
				Currency:          "CNY",
				FailureReason:     "max_retries_exhausted",
				Locale:            tc.locale,
			}
			if err := handlers.HandleRefundApology(context.Background(), newApologyTask(t, payload)); err != nil {
				t.Fatalf("expected success, got: %v", err)
			}

			msgs := sender.Messages()
			if len(msgs) != 1 {
				t.Fatalf("expected 1 apology email, got %d", len(msgs))
			}
			msg := msgs[0]
			if msg.To != "user@example.com" {
				t.Errorf("To = %q", msg.To)
			}
			if msg.Subject != tc.expectedSubject {
				t.Errorf("Subject = %q, want %q", msg.Subject, tc.expectedSubject)
			}
			for _, want := range tc.expectInBody {
				if !strings.Contains(msg.HTML, want) {
					t.Errorf("HTML missing %q\nbody=%s", want, msg.HTML)
				}
			}
		})
	}
}

func TestHandlers_HandleRefundApology_InvalidJSON(t *testing.T) {
	handlers, sender := setupHandlers(t)

	task := asynq.NewTask(TaskRefundApology, []byte("{not json"))
	err := handlers.HandleRefundApology(context.Background(), task)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !errors.Is(err, asynq.SkipRetry) {
		t.Errorf("expected asynq.SkipRetry, got: %v", err)
	}
	if len(sender.Messages()) != 0 {
		t.Errorf("no email should be sent on malformed payload")
	}
}

func TestHandlers_HandleRefundApology_MissingOrderID(t *testing.T) {
	handlers, _ := setupHandlers(t)

	payload := RefundApologyPayload{OrderID: "  ", FailureReason: "x"}
	err := handlers.HandleRefundApology(context.Background(), newApologyTask(t, payload))
	if err == nil {
		t.Fatal("expected error for missing order_id")
	}
	if !errors.Is(err, asynq.SkipRetry) {
		t.Errorf("expected asynq.SkipRetry, got: %v", err)
	}
}

// TestHandlers_HandleRefundApology_NoUserEmailAcks covers the D5
// fail-open path: when the producer could not resolve a recipient
// (user_email empty), the handler ACKs with a P0-tagged warning rather
// than retrying forever.
func TestHandlers_HandleRefundApology_NoUserEmailAcks(t *testing.T) {
	handlers, sender := setupHandlers(t)

	payload := RefundApologyPayload{
		OrderID:           "v_001",
		UserEmail:         "", // rare race — user deleted post-failure
		ExtOrderID:     "ord_xyz",
		RefundAmountCents: 1000,
		Currency:          "CNY",
		FailureReason:     "x",
	}
	if err := handlers.HandleRefundApology(context.Background(), newApologyTask(t, payload)); err != nil {
		t.Fatalf("expected fail-open ACK, got: %v", err)
	}
	if len(sender.Messages()) != 0 {
		t.Errorf("no email when recipient is empty")
	}
}

func TestHandlers_HandleRefundApology_EmailSendErrorRetries(t *testing.T) {
	handlers, _ := setupHandlers(t)
	// Rebuild with a failing sender to drive the transient-retry path.
	failing := &mockSender{sendError: errors.New("ses 5xx")}
	templates, err := template.New()
	if err != nil {
		t.Fatalf("template.New: %v", err)
	}
	handlers = NewHandlers(failing, templates, handlers.logger)

	payload := RefundApologyPayload{
		OrderID:           "v_001",
		UserEmail:         "u@example.com",
		ExtOrderID:     "ord_abc",
		RefundAmountCents: 1500,
		Currency:          "CNY",
		FailureReason:     "x",
		Locale:            "cn",
	}
	err = handlers.HandleRefundApology(context.Background(), newApologyTask(t, payload))
	if err == nil {
		t.Fatal("expected error so asynq retries on SES failure")
	}
	if errors.Is(err, asynq.SkipRetry) {
		t.Errorf("email failures must retry, got SkipRetry: %v", err)
	}
	if !strings.Contains(err.Error(), "send email") {
		t.Errorf("error should mention email send path: %v", err)
	}
}

// TestHandlers_HandleRefundApology_PayloadContract pins the wire-level
// JSON keys: the producer (apps/attest/cmd/refund-worker) emits exactly
// these names, and a rename here without a coordinated change there
// would silently break apology delivery in prod.
func TestHandlers_HandleRefundApology_PayloadContract(t *testing.T) {
	handlers, sender := setupHandlers(t)
	rawJSON := []byte(`{
		"order_id":"v_001",
		"user_email":"user@example.com",
		"ext_order_id":"ord_abc",
		"refund_amount_cents":9900,
		"currency":"CNY",
		"failure_reason":"max_retries_exhausted",
		"enqueued_at":"2026-05-17T10:00:00Z",
		"locale":"cn"
	}`)
	task := asynq.NewTask(TaskRefundApology, rawJSON)
	if err := handlers.HandleRefundApology(context.Background(), task); err != nil {
		t.Fatalf("expected success on contract payload, got: %v", err)
	}
	msgs := sender.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 email, got %d", len(msgs))
	}
	if msgs[0].To != "user@example.com" {
		t.Errorf("To = %q", msgs[0].To)
	}
	for _, want := range []string{"ord_abc", "99.00 CNY", "v_001"} {
		if !strings.Contains(msgs[0].HTML, want) {
			t.Errorf("HTML missing %q", want)
		}
	}
}

func TestHandlers_HandleRefundApology_MuxRegistered(t *testing.T) {
	handlers, _ := setupHandlers(t)
	mux := handlers.GetMux()
	if mux == nil {
		t.Fatal("nil mux")
	}
	// Empty user_email → handler ACKs (fail-open) → ProcessTask returns nil.
	task := asynq.NewTask(TaskRefundApology, []byte(`{"order_id":"v_999"}`))
	if err := mux.ProcessTask(context.Background(), task); err != nil {
		t.Errorf("mux did not route %s correctly: %v", TaskRefundApology, err)
	}
}
