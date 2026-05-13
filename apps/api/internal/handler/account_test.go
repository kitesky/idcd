package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kite365/idcd/apps/api/internal/middleware"
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
	u.Bio = arg.Bio
	u.Locale = arg.Locale
	u.Timezone = arg.Timezone
	m.users[arg.ID] = u
	return u, nil
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
