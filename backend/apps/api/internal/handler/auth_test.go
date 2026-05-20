package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/lib/auth/password"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
	"github.com/kite365/idcd/lib/db/repository"
)

// jsonUnmarshal is a thin wrapper kept in the test file so it stays close
// to the assertions that consume it. The handler tests only need
// json.Unmarshal in two places, so we deliberately avoid importing the
// encoding/json package at top-of-file alongside production imports.
func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// --- mocks ---

type mockAuthQuerier struct {
	users map[string]idcdmain.User // keyed by email
	otps  map[string]idcdmain.UserOtp
	err   error
}

func newMockAuthQuerier() *mockAuthQuerier {
	return &mockAuthQuerier{
		users: make(map[string]idcdmain.User),
		otps:  make(map[string]idcdmain.UserOtp),
	}
}

func (m *mockAuthQuerier) CreateUser(_ context.Context, arg idcdmain.CreateUserParams) (idcdmain.User, error) {
	if m.err != nil {
		return idcdmain.User{}, m.err
	}
	if _, exists := m.users[arg.Email]; exists {
		return idcdmain.User{}, repository.ErrDuplicate
	}
	u := idcdmain.User{
		ID:           arg.ID,
		Email:        arg.Email,
		PasswordHash: arg.PasswordHash,
		DisplayName:  arg.DisplayName,
		Locale:       arg.Locale,
		Timezone:     arg.Timezone,
		Status:       "active",
	}
	m.users[arg.Email] = u
	return u, nil
}

func (m *mockAuthQuerier) GetUserByEmail(_ context.Context, email string) (idcdmain.User, error) {
	u, ok := m.users[email]
	if !ok {
		return idcdmain.User{}, repository.ErrNotFound
	}
	return u, nil
}

func (m *mockAuthQuerier) GetUserByID(_ context.Context, id string) (idcdmain.User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return idcdmain.User{}, repository.ErrNotFound
}

func (m *mockAuthQuerier) UpdateUserEmailVerified(_ context.Context, id string) (idcdmain.User, error) {
	for email, u := range m.users {
		if u.ID == id {
			now := pgtype.Timestamptz{}
			_ = now.Scan(time.Now())
			u.EmailVerifiedAt = now
			m.users[email] = u
			return u, nil
		}
	}
	return idcdmain.User{}, repository.ErrNotFound
}

func (m *mockAuthQuerier) UpdateUserPasswordHash(_ context.Context, arg idcdmain.UpdateUserPasswordHashParams) error {
	for email, u := range m.users {
		if u.ID == arg.ID {
			u.PasswordHash = arg.PasswordHash
			m.users[email] = u
			return nil
		}
	}
	return repository.ErrNotFound
}

func (m *mockAuthQuerier) UpdateUserLastLogin(_ context.Context, _ idcdmain.UpdateUserLastLoginParams) error {
	return nil
}

func (m *mockAuthQuerier) CreateUserOTP(_ context.Context, arg idcdmain.CreateUserOTPParams) (idcdmain.UserOtp, error) {
	otp := idcdmain.UserOtp{
		ID:        arg.ID,
		UserID:    arg.UserID,
		Type:      arg.Type,
		CodeHash:  arg.CodeHash,
		ExpiresAt: arg.ExpiresAt,
	}
	m.otps[arg.ID] = otp
	return otp, nil
}

func (m *mockAuthQuerier) GetUserOTPByIDAndType(_ context.Context, arg idcdmain.GetUserOTPByIDAndTypeParams) (idcdmain.UserOtp, error) {
	otp, ok := m.otps[arg.ID]
	if !ok || otp.Type != arg.Type {
		return idcdmain.UserOtp{}, errors.New("not found")
	}
	return otp, nil
}

func (m *mockAuthQuerier) IncrementUserOTPAttempts(_ context.Context, id string) error {
	otp, ok := m.otps[id]
	if !ok {
		return nil
	}
	otp.Attempts++
	m.otps[id] = otp
	return nil
}

func (m *mockAuthQuerier) MarkUserOTPUsed(_ context.Context, id string) error {
	otp, ok := m.otps[id]
	if !ok {
		return nil
	}
	now := pgtype.Timestamptz{}
	_ = now.Scan(time.Now())
	otp.UsedAt = now
	m.otps[id] = otp
	return nil
}

func (m *mockAuthQuerier) SoftDeleteUser(_ context.Context, _ string) error {
	return nil
}

func (m *mockAuthQuerier) UpdateUserProfile(_ context.Context, arg idcdmain.UpdateUserProfileParams) (idcdmain.User, error) {
	for email, u := range m.users {
		if u.ID == arg.ID {
			u.DisplayName = arg.DisplayName
			u.Bio = arg.Bio
			m.users[email] = u
			return u, nil
		}
	}
	return idcdmain.User{}, repository.ErrNotFound
}

// mockJWT / mockSession

type mockJWT struct {
	token  string
	locale string // captured by SignWithLocale for i18n test assertions
}

func (m *mockJWT) Sign(_, _ string, _ time.Duration) (string, error) {
	return m.token, nil
}

func (m *mockJWT) SignWithLocale(_, _, locale string, _ time.Duration) (string, error) {
	m.locale = locale
	return m.token, nil
}

type mockSession struct{}

func (m *mockSession) Store(_ context.Context, _, _ string, _ time.Duration) error { return nil }
func (m *mockSession) Delete(_ context.Context, _ string) error                    { return nil }

// mockEnqueuer captures enqueued tasks for assertion in tests.
type mockEnqueuer struct {
	tasks []struct {
		taskType string
		payload  []byte
		queue    string
	}
}

func (m *mockEnqueuer) EnqueueTask(_ context.Context, taskType string, payload []byte, queue string) error {
	m.tasks = append(m.tasks, struct {
		taskType string
		payload  []byte
		queue    string
	}{taskType: taskType, payload: payload, queue: queue})
	return nil
}

// helper

func newTestAuthHandler() *AuthHandler {
	return NewAuthHandler(newMockAuthQuerier(), &mockJWT{token: "tok.en.here"}, &mockSession{}, "test-otp-secret-32bytes-minimum!!")
}

// --- tests ---

func TestRegister_success(t *testing.T) {
	h := newTestAuthHandler()

	body := `{"email":"test@example.com","password":"Password123"}`
	req := httptest.NewRequest("POST", "/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// Token is now delivered via HttpOnly cookie, not in the response body.
	cookie := rr.Result().Cookies()
	var hasTokenCookie bool
	for _, c := range cookie {
		if c.Name == "access_token" && c.Value != "" {
			hasTokenCookie = true
		}
	}
	if !hasTokenCookie {
		t.Errorf("expected access_token cookie, got cookies: %v, body: %s", cookie, rr.Body.String())
	}
}

// TestRegister_localeFromAcceptLanguage asserts the i18n Phase 2c contract:
// registration negotiates the user's short locale code from Accept-Language
// (en-US / zh-CN → en / cn) and persists it on the user row + JWT claim.
func TestRegister_localeFromAcceptLanguage(t *testing.T) {
	cases := []struct {
		name           string
		acceptLanguage string
		wantLocale     string
	}{
		{name: "english", acceptLanguage: "en-US,en;q=0.9", wantLocale: "en"},
		{name: "chinese", acceptLanguage: "zh-CN,zh;q=0.9", wantLocale: "cn"},
		{name: "unsupported_falls_back_to_default", acceptLanguage: "fr-FR", wantLocale: "cn"},
		{name: "empty_falls_back_to_default", acceptLanguage: "", wantLocale: "cn"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := newMockAuthQuerier()
			jwt := &mockJWT{token: "tok"}
			h := NewAuthHandler(mock, jwt, &mockSession{}, "test-otp-secret-32bytes-minimum!!")

			body := `{"email":"` + tc.name + `@example.com","password":"Password123"}`
			req := httptest.NewRequest("POST", "/v1/auth/register", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			if tc.acceptLanguage != "" {
				req.Header.Set("Accept-Language", tc.acceptLanguage)
			}
			rr := httptest.NewRecorder()

			h.Register(rr, req)

			if rr.Code != http.StatusCreated {
				t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
			}
			gotUser, ok := mock.users[tc.name+"@example.com"]
			if !ok {
				t.Fatalf("user not stored")
			}
			if gotUser.Locale != tc.wantLocale {
				t.Errorf("user.Locale = %q, want %q", gotUser.Locale, tc.wantLocale)
			}
			if jwt.locale != tc.wantLocale {
				t.Errorf("JWT signed locale = %q, want %q", jwt.locale, tc.wantLocale)
			}
		})
	}
}

func TestRegister_duplicateEmail(t *testing.T) {
	h := newTestAuthHandler()

	body := `{"email":"test@example.com","password":"Password123"}`
	for _, wantCode := range []int{http.StatusCreated, http.StatusConflict} {
		req := httptest.NewRequest("POST", "/v1/auth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.Register(rr, req)
		if rr.Code != wantCode {
			t.Errorf("expected %d, got %d", wantCode, rr.Code)
		}
	}
}

func TestRegister_weakPassword(t *testing.T) {
	h := newTestAuthHandler()

	body := `{"email":"test@example.com","password":"abc"}`
	req := httptest.NewRequest("POST", "/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestLogin_success(t *testing.T) {
	q := newMockAuthQuerier()
	// Pre-create user with hashed password
	hash, _ := hashPassword("Password123")
	q.users["user@example.com"] = idcdmain.User{
		ID:           "usr_001",
		Email:        "user@example.com",
		PasswordHash: &hash,
		Status:       "active",
		Locale:       "zh-CN",
		Timezone:     "Asia/Shanghai",
	}

	h := NewAuthHandler(q, &mockJWT{token: "tok.en.here"}, &mockSession{}, "test-otp-secret-32bytes-minimum!!")

	body := `{"email":"user@example.com","password":"Password123"}`
	req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Login(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestLogin_wrongPassword(t *testing.T) {
	q := newMockAuthQuerier()
	hash, _ := hashPassword("RightPassword1")
	q.users["user@example.com"] = idcdmain.User{
		ID:           "usr_001",
		Email:        "user@example.com",
		PasswordHash: &hash,
		Status:       "active",
		Locale:       "zh-CN",
		Timezone:     "Asia/Shanghai",
	}

	h := NewAuthHandler(q, &mockJWT{token: "t"}, &mockSession{}, "test-otp-secret-32bytes-minimum!!")

	body := `{"email":"user@example.com","password":"WrongPassword1"}`
	req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Login(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestLogin_unknownEmail(t *testing.T) {
	h := newTestAuthHandler()

	body := `{"email":"nobody@example.com","password":"Password123"}`
	req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Login(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestLogout(t *testing.T) {
	h := newTestAuthHandler()

	req := httptest.NewRequest("POST", "/v1/auth/logout", nil)
	rr := httptest.NewRecorder()

	h.Logout(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestForgotPassword_unknownEmail(t *testing.T) {
	h := newTestAuthHandler()

	body := `{"email":"nobody@example.com"}`
	req := httptest.NewRequest("POST", "/v1/auth/forgot-password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ForgotPassword(rr, req)

	// Should return 200 regardless (avoid email enumeration).
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestResetPassword_success(t *testing.T) {
	q := newMockAuthQuerier()
	hash, _ := hashPassword("OldPassword1")
	q.users["user@example.com"] = idcdmain.User{
		ID:           "usr_001",
		Email:        "user@example.com",
		PasswordHash: &hash,
		Status:       "active",
		Locale:       "zh-CN",
		Timezone:     "Asia/Shanghai",
	}

	h := NewAuthHandler(q, &mockJWT{token: "t"}, &mockSession{}, "test-otp-secret-32bytes-minimum!!")

	// Manually issue an OTP.
	ctx := context.Background()
	otpID, code, err := h.issueOTP(ctx, "usr_001", otpTypeReset, otpTTL)
	if err != nil {
		t.Fatalf("issueOTP: %v", err)
	}

	body := `{"email":"user@example.com","otp_id":"` + otpID + `","code":"` + code + `","new_password":"NewPassword123"}`
	req := httptest.NewRequest("POST", "/v1/auth/reset-password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ResetPassword(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGenerateOTP(t *testing.T) {
	for range 10 {
		code, err := generateOTP()
		if err != nil {
			t.Fatalf("generateOTP: %v", err)
		}
		if len(code) != 6 {
			t.Errorf("OTP length: got %d, want 6", len(code))
		}
		for _, c := range code {
			if c < '0' || c > '9' {
				t.Errorf("OTP contains non-digit: %q", code)
			}
		}
	}
}

func TestRegister_enqueuesVerifyEmail(t *testing.T) {
	eq := &mockEnqueuer{}
	h := newTestAuthHandler().WithEnqueuer(eq)

	body := `{"email":"enqueue@example.com","password":"Password123"}`
	req := httptest.NewRequest("POST", "/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(eq.tasks) != 1 {
		t.Fatalf("expected 1 enqueued task, got %d", len(eq.tasks))
	}
	if eq.tasks[0].taskType != taskSendVerifyEmail {
		t.Errorf("expected task type %q, got %q", taskSendVerifyEmail, eq.tasks[0].taskType)
	}
	if eq.tasks[0].queue != "email" {
		t.Errorf("expected queue %q, got %q", "email", eq.tasks[0].queue)
	}

	// Phase 2a: verify-email payload must carry the user's short locale so
	// the notifier worker picks the right template / subject.
	var payload map[string]any
	if err := jsonUnmarshal(eq.tasks[0].payload, &payload); err != nil {
		t.Fatalf("payload not valid JSON: %v", err)
	}
	if loc, ok := payload["locale"].(string); !ok || loc == "" {
		t.Errorf("expected payload.locale to be set, got %v", payload["locale"])
	}
}

// TestEnqueueVerifyEmail_NegotiatesLocale exercises the Accept-Language path:
// a registration with en-US should produce a payload with locale="en".
func TestEnqueueVerifyEmail_NegotiatesLocale(t *testing.T) {
	eq := &mockEnqueuer{}
	h := newTestAuthHandler().WithEnqueuer(eq)

	body := `{"email":"en-locale@example.com","password":"Password123"}`
	req := httptest.NewRequest("POST", "/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	rr := httptest.NewRecorder()

	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(eq.tasks) != 1 {
		t.Fatalf("expected 1 enqueued task, got %d", len(eq.tasks))
	}
	var payload map[string]any
	if err := jsonUnmarshal(eq.tasks[0].payload, &payload); err != nil {
		t.Fatalf("payload not valid JSON: %v", err)
	}
	if loc, _ := payload["locale"].(string); loc != "en" {
		t.Errorf("expected payload.locale=en, got %q", loc)
	}
}

func TestForgotPassword_enqueuesResetEmail(t *testing.T) {
	q := newMockAuthQuerier()
	hash, _ := hashPassword("OldPassword1")
	q.users["user@example.com"] = idcdmain.User{
		ID:           "usr_001",
		Email:        "user@example.com",
		PasswordHash: &hash,
		Status:       "active",
		Locale:       "zh-CN",
		Timezone:     "Asia/Shanghai",
	}

	eq := &mockEnqueuer{}
	h := NewAuthHandler(q, &mockJWT{token: "t"}, &mockSession{}, "test-otp-secret-32bytes-minimum!!").
		WithEnqueuer(eq)

	body := `{"email":"user@example.com"}`
	req := httptest.NewRequest("POST", "/v1/auth/forgot-password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ForgotPassword(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(eq.tasks) != 1 {
		t.Fatalf("expected 1 enqueued task, got %d", len(eq.tasks))
	}
	if eq.tasks[0].taskType != taskSendResetPassword {
		t.Errorf("expected task type %q, got %q", taskSendResetPassword, eq.tasks[0].taskType)
	}
}

func TestResendVerifyEmail_notVerified(t *testing.T) {
	q := newMockAuthQuerier()
	q.users["user@example.com"] = idcdmain.User{
		ID:     "usr_001",
		Email:  "user@example.com",
		Status: "active",
		Locale: "zh-CN",
	}

	eq := &mockEnqueuer{}
	h := NewAuthHandler(q, &mockJWT{token: "t"}, &mockSession{}, "test-otp-secret-32bytes-minimum!!").
		WithEnqueuer(eq)

	req := httptest.NewRequest("POST", "/v1/auth/resend-verify", nil)
	// Inject user ID into context as authnMW would.
	ctx := context.WithValue(req.Context(), middleware.UserIDContextKey(), "usr_001")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.ResendVerifyEmail(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(eq.tasks) != 1 {
		t.Fatalf("expected 1 enqueued task, got %d", len(eq.tasks))
	}
	if eq.tasks[0].taskType != taskSendVerifyEmail {
		t.Errorf("expected task type %q, got %q", taskSendVerifyEmail, eq.tasks[0].taskType)
	}
}

func TestResendVerifyEmail_alreadyVerified(t *testing.T) {
	q := newMockAuthQuerier()
	now := pgtype.Timestamptz{}
	_ = now.Scan(time.Now())
	q.users["user@example.com"] = idcdmain.User{
		ID:              "usr_001",
		Email:           "user@example.com",
		Status:          "active",
		Locale:          "zh-CN",
		EmailVerifiedAt: now,
	}

	eq := &mockEnqueuer{}
	h := NewAuthHandler(q, &mockJWT{token: "t"}, &mockSession{}, "test-otp-secret-32bytes-minimum!!").
		WithEnqueuer(eq)

	req := httptest.NewRequest("POST", "/v1/auth/resend-verify", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDContextKey(), "usr_001")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.ResendVerifyEmail(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	// No email should be enqueued when already verified.
	if len(eq.tasks) != 0 {
		t.Errorf("expected 0 enqueued tasks for already-verified user, got %d", len(eq.tasks))
	}
}

// hashPassword is a test helper wrapping packages/auth/password.Hash.
func hashPassword(plain string) (string, error) {
	return password.Hash(plain)
}
