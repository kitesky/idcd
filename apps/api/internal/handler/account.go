package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// AccountQuerier is the subset of sqlc Queries used by AccountHandler.
type AccountQuerier interface {
	GetUserByID(ctx context.Context, id string) (idcdmain.User, error)
	UpdateUserProfile(ctx context.Context, arg idcdmain.UpdateUserProfileParams) (idcdmain.User, error)
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
