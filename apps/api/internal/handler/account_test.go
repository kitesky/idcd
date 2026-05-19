package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/lib/auth/password"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
	"github.com/kite365/idcd/lib/db/repository"
)

// mockAccountQuerier implements AccountQuerier.
type mockAccountQuerier struct {
	users map[string]idcdmain.User
}

func newMockAccountQuerier(userID string) *mockAccountQuerier {
	name := "Test User"
	return &mockAccountQuerier{
		users: map[string]idcdmain.User{
			userID: {
				ID:          userID,
				Email:       "test@example.com",
				DisplayName: &name,
				Status:      "active",
				Locale:      "zh-CN",
				Timezone:    "Asia/Shanghai",
			},
		},
	}
}

func (m *mockAccountQuerier) GetUserByID(_ context.Context, id string) (idcdmain.User, error) {
	u, ok := m.users[id]
	if !ok {
		return idcdmain.User{}, repository.ErrNotFound
	}
	return u, nil
}

func (m *mockAccountQuerier) UpdateUserProfile(_ context.Context, arg idcdmain.UpdateUserProfileParams) (idcdmain.User, error) {
	u, ok := m.users[arg.ID]
	if !ok {
		return idcdmain.User{}, repository.ErrNotFound
	}
	u.DisplayName = arg.DisplayName
	u.AvatarUrl = arg.AvatarUrl
	u.Bio = arg.Bio
	u.Locale = arg.Locale
	u.Timezone = arg.Timezone
	m.users[arg.ID] = u
	return u, nil
}

func (m *mockAccountQuerier) UpdateUserPasswordHash(_ context.Context, arg idcdmain.UpdateUserPasswordHashParams) error {
	u, ok := m.users[arg.ID]
	if !ok {
		return repository.ErrNotFound
	}
	u.PasswordHash = arg.PasswordHash
	m.users[arg.ID] = u
	return nil
}

func (m *mockAccountQuerier) SoftDeleteUser(_ context.Context, id string) error {
	delete(m.users, id)
	return nil
}

// withUserID injects a user ID into the request context (simulating Authn middleware).
func withUserID(req *http.Request, userID string) *http.Request {
	ctx := context.WithValue(req.Context(), middleware.UserIDContextKey(), userID)
	return req.WithContext(ctx)
}

func TestGetProfile_success(t *testing.T) {
	q := newMockAccountQuerier("usr_001")
	h := NewAccountHandler(q)

	req := httptest.NewRequest("GET", "/v1/account/profile", nil)
	req = withUserID(req, "usr_001")
	rr := httptest.NewRecorder()

	h.GetProfile(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var wrapped struct {
		Data profileResponse `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &wrapped); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if wrapped.Data.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %q", wrapped.Data.Email)
	}
}

func TestGetProfile_noAuth(t *testing.T) {
	h := NewAccountHandler(newMockAccountQuerier("usr_001"))

	req := httptest.NewRequest("GET", "/v1/account/profile", nil)
	// No user ID injected — simulates missing auth.
	rr := httptest.NewRecorder()

	h.GetProfile(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestUpdateProfile_success(t *testing.T) {
	q := newMockAccountQuerier("usr_001")
	h := NewAccountHandler(q)

	bio := "Hello world"
	body := `{"bio":"Hello world"}`
	req := httptest.NewRequest("PATCH", "/v1/account/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, "usr_001")
	rr := httptest.NewRecorder()

	h.UpdateProfile(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var wrapped struct {
		Data profileResponse `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &wrapped); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if wrapped.Data.Bio == nil || *wrapped.Data.Bio != bio {
		t.Errorf("expected bio=%q, got %v", bio, wrapped.Data.Bio)
	}
}

func TestDeleteAccount_success(t *testing.T) {
	q := newMockAccountQuerier("usr_001")
	h := NewAccountHandler(q)

	req := httptest.NewRequest("DELETE", "/v1/account", nil)
	req = withUserID(req, "usr_001")
	rr := httptest.NewRecorder()

	h.DeleteAccount(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestDeleteAccount_noAuth(t *testing.T) {
	h := NewAccountHandler(newMockAccountQuerier("usr_001"))

	req := httptest.NewRequest("DELETE", "/v1/account", nil)
	rr := httptest.NewRecorder()

	h.DeleteAccount(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// newMockAccountQuerierWithPassword creates a mock querier whose user has a real password hash.
func newMockAccountQuerierWithPassword(userID, plainPassword string) *mockAccountQuerier {
	q := newMockAccountQuerier(userID)
	hash, err := password.Hash(plainPassword)
	if err != nil {
		panic("failed to hash test password: " + err.Error())
	}
	u := q.users[userID]
	u.PasswordHash = &hash
	q.users[userID] = u
	return q
}

func TestChangePassword_success(t *testing.T) {
	q := newMockAccountQuerierWithPassword("usr_001", "OldPass1")
	h := NewAccountHandler(q)

	body := `{"current_password":"OldPass1","new_password":"NewPass2"}`
	req := httptest.NewRequest("PATCH", "/v1/account/password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, "usr_001")
	rr := httptest.NewRecorder()

	h.ChangePassword(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var wrapped struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &wrapped); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	// Verify the stored hash now matches the new password.
	u := q.users["usr_001"]
	if u.PasswordHash == nil || !password.Verify("NewPass2", *u.PasswordHash) {
		t.Error("stored hash does not match new password after update")
	}
}

func TestChangePassword_wrongCurrentPassword(t *testing.T) {
	q := newMockAccountQuerierWithPassword("usr_001", "OldPass1")
	h := NewAccountHandler(q)

	body := `{"current_password":"WrongPass9","new_password":"NewPass2"}`
	req := httptest.NewRequest("PATCH", "/v1/account/password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, "usr_001")
	rr := httptest.NewRecorder()

	h.ChangePassword(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestChangePassword_weakNewPassword(t *testing.T) {
	q := newMockAccountQuerierWithPassword("usr_001", "OldPass1")
	h := NewAccountHandler(q)

	// New password has no digits — fails ValidatePassword.
	body := `{"current_password":"OldPass1","new_password":"onlyletters"}`
	req := httptest.NewRequest("PATCH", "/v1/account/password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, "usr_001")
	rr := httptest.NewRecorder()

	h.ChangePassword(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestChangePassword_noAuth(t *testing.T) {
	h := NewAccountHandler(newMockAccountQuerier("usr_001"))

	body := `{"current_password":"OldPass1","new_password":"NewPass2"}`
	req := httptest.NewRequest("PATCH", "/v1/account/password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No user ID injected — simulates missing auth.
	rr := httptest.NewRecorder()

	h.ChangePassword(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// pngMagicHeader is the 8-byte PNG signature (RFC 2083). UploadAvatar runs
// http.DetectContentType on the upload payload to defend against clients that
// label JS/SVG/PHP as image/jpeg, so test fixtures need real magic bytes
// instead of arbitrary noise.
var pngMagicHeader = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

// buildAvatarMultipart constructs a multipart/form-data body with a single
// "avatar" file field of the given MIME type and content. When contentType is
// image/png, the data is prefixed with the PNG signature so the handler's
// magic-bytes sniffer accepts it as a real PNG.
func buildAvatarMultipart(t *testing.T, contentType string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// Create the file part with an explicit Content-Type header.
	h := make(map[string][]string)
	h["Content-Disposition"] = []string{`form-data; name="avatar"; filename="avatar.png"`}
	h["Content-Type"] = []string{contentType}
	part, err := w.CreatePart(h)
	if err != nil {
		t.Fatalf("create multipart part: %v", err)
	}
	if contentType == "image/png" {
		if _, err := part.Write(pngMagicHeader); err != nil {
			t.Fatalf("write png header: %v", err)
		}
	}
	if _, err := io.Copy(part, bytes.NewReader(data)); err != nil {
		t.Fatalf("write multipart data: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return &buf, w.FormDataContentType()
}

func TestUploadAvatar_success(t *testing.T) {
	q := newMockAccountQuerier("usr_001")
	h := NewAccountHandler(q)

	// 1×1 pixel PNG — well under 256 KB when base64-encoded.
	imgData := make([]byte, 1024) // 1 KB of placeholder image bytes
	for i := range imgData {
		imgData[i] = byte(i % 256)
	}

	buf, ct := buildAvatarMultipart(t, "image/png", imgData)
	req := httptest.NewRequest("POST", "/v1/account/avatar", buf)
	req.Header.Set("Content-Type", ct)
	req = withUserID(req, "usr_001")
	rr := httptest.NewRecorder()

	h.UploadAvatar(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var wrapped struct {
		Data uploadAvatarResponse `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &wrapped); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.HasPrefix(wrapped.Data.AvatarURL, "data:image/png;base64,") {
		t.Errorf("expected data URI prefix data:image/png;base64,, got %q", wrapped.Data.AvatarURL[:min(40, len(wrapped.Data.AvatarURL))])
	}

	// Verify DB was updated.
	u := q.users["usr_001"]
	if u.AvatarUrl == nil || *u.AvatarUrl != wrapped.Data.AvatarURL {
		t.Error("avatar_url not persisted to mock DB")
	}
}

func TestUploadAvatar_tooLarge(t *testing.T) {
	q := newMockAccountQuerier("usr_001")
	h := NewAccountHandler(q)

	// Build a payload that exceeds 256 KB after base64 encoding.
	// base64 expands by ~4/3, so ~200 KB raw → ~267 KB base64; add data URI prefix.
	largeData := make([]byte, 200*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	buf, ct := buildAvatarMultipart(t, "image/png", largeData)
	req := httptest.NewRequest("POST", "/v1/account/avatar", buf)
	req.Header.Set("Content-Type", ct)
	req = withUserID(req, "usr_001")
	rr := httptest.NewRecorder()

	h.UploadAvatar(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "too large") {
		t.Errorf("expected 'too large' in error body, got: %s", rr.Body.String())
	}
}

func TestUploadAvatar_invalidType(t *testing.T) {
	q := newMockAccountQuerier("usr_001")
	h := NewAccountHandler(q)

	pdfData := []byte("%PDF-1.4 fake pdf content")
	buf, ct := buildAvatarMultipart(t, "application/pdf", pdfData)
	req := httptest.NewRequest("POST", "/v1/account/avatar", buf)
	req.Header.Set("Content-Type", ct)
	req = withUserID(req, "usr_001")
	rr := httptest.NewRecorder()

	h.UploadAvatar(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "unsupported image type") {
		t.Errorf("expected 'unsupported image type' in error body, got: %s", rr.Body.String())
	}
}

