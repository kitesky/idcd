package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/auth/password"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// AccountQuerier is the subset of sqlc Queries used by AccountHandler.
type AccountQuerier interface {
	GetUserByID(ctx context.Context, id string) (idcdmain.User, error)
	UpdateUserProfile(ctx context.Context, arg idcdmain.UpdateUserProfileParams) (idcdmain.User, error)
	UpdateUserPasswordHash(ctx context.Context, arg idcdmain.UpdateUserPasswordHashParams) error
	SoftDeleteUser(ctx context.Context, id string) error
}

// AccountHandler implements account management endpoints.
type AccountHandler struct {
	q AccountQuerier
}

// NewAccountHandler creates an AccountHandler.
func NewAccountHandler(q AccountQuerier) *AccountHandler {
	return &AccountHandler{q: q}
}

// profileResponse is the public representation of a user.
type profileResponse struct {
	ID          string  `json:"id"`
	Email       string  `json:"email"`
	DisplayName *string `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
	Bio         *string `json:"bio"`
	Locale      string  `json:"locale"`
	Timezone    string  `json:"timezone"`
	Status      string  `json:"status"`
}

func toProfileResponse(u idcdmain.User) profileResponse {
	return profileResponse{
		ID:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		AvatarURL:   u.AvatarUrl,
		Bio:         u.Bio,
		Locale:      u.Locale,
		Timezone:    u.Timezone,
		Status:      u.Status,
	}
}

// GetProfile handles GET /v1/account/profile.
func (h *AccountHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	user, err := h.q.GetUserByID(r.Context(), userID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to fetch profile", err))
		return
	}

	response.JSON(w, r, http.StatusOK, toProfileResponse(user))
}

// updateProfileRequest contains optional profile fields.
type updateProfileRequest struct {
	DisplayName *string `json:"display_name"`
	Bio         *string `json:"bio"`
	Locale      *string `json:"locale"`
	Timezone    *string `json:"timezone"`
}

// UpdateProfile handles PATCH /v1/account/profile.
func (h *AccountHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}

	// Fetch current values to fill in unchanged fields.
	current, err := h.q.GetUserByID(r.Context(), userID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to fetch profile", err))
		return
	}

	locale := current.Locale
	if req.Locale != nil {
		locale = *req.Locale
	}
	timezone := current.Timezone
	if req.Timezone != nil {
		timezone = *req.Timezone
	}

	displayName := current.DisplayName
	if req.DisplayName != nil {
		displayName = req.DisplayName
	}
	bio := current.Bio
	if req.Bio != nil {
		bio = req.Bio
	}

	updated, err := h.q.UpdateUserProfile(r.Context(), idcdmain.UpdateUserProfileParams{
		ID:          userID,
		DisplayName: displayName,
		AvatarUrl:   current.AvatarUrl, // not updatable via this endpoint
		Bio:         bio,
		Locale:      locale,
		Timezone:    timezone,
	})
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to update profile", err))
		return
	}

	response.JSON(w, r, http.StatusOK, toProfileResponse(updated))
}

// maxAvatarDataURIBytes is the maximum size of the base64-encoded data URI stored in DB.
const maxAvatarDataURIBytes = 256 * 1024 // 256 KB

// allowedAvatarTypes lists the accepted image MIME types.
var allowedAvatarTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
}

// uploadAvatarResponse is returned after a successful avatar upload.
type uploadAvatarResponse struct {
	AvatarURL string `json:"avatar_url"`
}

// UploadAvatar handles POST /v1/account/avatar.
// It accepts a multipart/form-data upload (field "avatar"), validates the MIME
// type, encodes the image as a base64 data URI, and stores it in the DB.
// S1 simplification: no object storage — data URI goes directly into avatar_url.
func (h *AccountHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	// Limit total body to 5 MB before parsing.
	const maxUploadBytes = 5 << 20 // 5 MB
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		response.Error(w, r, apperr.Validation("request body too large or malformed", err.Error()))
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		response.Error(w, r, apperr.Validation("avatar field is required", err.Error()))
		return
	}
	defer file.Close()

	// Validate Content-Type from the multipart header.
	contentType := header.Header.Get("Content-Type")
	if !allowedAvatarTypes[contentType] {
		response.Error(w, r, apperr.Validation(
			fmt.Sprintf("unsupported image type %q, allowed: jpeg, png, gif, webp", contentType), ""))
		return
	}

	// Read file bytes.
	imgBytes, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to read uploaded file", err))
		return
	}

	// Build data URI.
	encoded := base64.StdEncoding.EncodeToString(imgBytes)
	dataURI := fmt.Sprintf("data:%s;base64,%s", contentType, encoded)

	// Enforce 256 KB limit on the stored data URI.
	if len(dataURI) > maxAvatarDataURIBytes {
		response.Error(w, r, apperr.Validation("image too large after encoding, max 256KB", ""))
		return
	}

	// Fetch current profile to preserve all other fields.
	current, err := h.q.GetUserByID(r.Context(), userID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to fetch user profile", err))
		return
	}

	// Reuse UpdateUserProfile — it accepts AvatarUrl as part of the params.
	_, err = h.q.UpdateUserProfile(r.Context(), idcdmain.UpdateUserProfileParams{
		ID:          userID,
		DisplayName: current.DisplayName,
		AvatarUrl:   &dataURI,
		Bio:         current.Bio,
		Locale:      current.Locale,
		Timezone:    current.Timezone,
	})
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to save avatar", err))
		return
	}

	response.JSON(w, r, http.StatusOK, uploadAvatarResponse{AvatarURL: dataURI})
}

// DeleteAccount handles DELETE /v1/account — initiates 30-day soft delete.
func (h *AccountHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	if err := h.q.SoftDeleteUser(r.Context(), userID); err != nil {
		response.Error(w, r, apperr.Internal("failed to schedule account deletion", err))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]string{
		"message": "account scheduled for deletion in 30 days",
	})
}

// changePasswordRequest contains the current and new passwords.
type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangePassword handles PATCH /v1/account/password.
func (h *AccountHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		response.Error(w, r, apperr.Validation("current_password and new_password are required", ""))
		return
	}

	user, err := h.q.GetUserByID(r.Context(), userID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to fetch user", err))
		return
	}

	if user.PasswordHash == nil || !password.Verify(req.CurrentPassword, *user.PasswordHash) {
		response.Error(w, r, apperr.Unauthorized("current password is incorrect"))
		return
	}

	if err := password.ValidatePassword(req.NewPassword, user.Email); err != nil {
		response.Error(w, r, apperr.Validation(err.Error(), ""))
		return
	}

	hash, err := password.Hash(req.NewPassword)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to hash password", err))
		return
	}

	if err := h.q.UpdateUserPasswordHash(r.Context(), idcdmain.UpdateUserPasswordHashParams{
		ID:           userID,
		PasswordHash: &hash,
	}); err != nil {
		response.Error(w, r, apperr.Internal("failed to update password", err))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]string{"message": "password updated"})
}
