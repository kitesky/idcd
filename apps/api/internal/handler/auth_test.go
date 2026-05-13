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

	"github.com/kite365/idcd/packages/auth/password"
	"github.com/kite365/idcd/packages/db/gen/idcdmain"
	"github.com/kite365/idcd/packages/db/repository"
)

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

type mockJWT struct{ token string }

func (m *mockJWT) Sign(_, _ string, _ time.Duration) (string, error) {
	return m.token, nil
}

type mockSession struct{}

func (m *mockSession) Store(_ context.Context, _, _ string, _ time.Duration) error { return nil }
func (m *mockSession) Delete(_ context.Context, _ string) error                    { return nil }

// helper

func newTestAuthHandler() *AuthHandler {
	return NewAuthHandler(newMockAuthQuerier(), &mockJWT{token: "tok.en.here"}, &mockSession{})
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

	// response.JSON wraps data in {"data": {...}, "request_id": "..."}
	var wrapped struct {
		Data authResponse `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &wrapped); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if wrapped.Data.AccessToken == "" {
		t.Errorf("expected access_token in response, body: %s", rr.Body.String())
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

	h := NewAuthHandler(q, &mockJWT{token: "tok.en.here"}, &mockSession{})

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

	h := NewAuthHandler(q, &mockJWT{token: "t"}, &mockSession{})

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

	h := NewAuthHandler(q, &mockJWT{token: "t"}, &mockSession{})

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

// hashPassword is a test helper wrapping packages/auth/password.Hash.
func hashPassword(plain string) (string, error) {
	return password.Hash(plain)
}
