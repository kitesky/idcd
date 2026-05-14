package handler

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"math/big"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
)

const betaCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
const betaCodeLen = 8

// BetaInvitationHandler handles beta invitation endpoints.
type BetaInvitationHandler struct {
	pool AdminPool
}

// NewBetaInvitationHandler creates a BetaInvitationHandler.
func NewBetaInvitationHandler(pool AdminPool) *BetaInvitationHandler {
	return &BetaInvitationHandler{pool: pool}
}

// betaInvitationRow is the DB scan target for beta_invitations.
type betaInvitationRow struct {
	ID          string
	Code        string
	Email       *string
	Status      string
	RequestedBy *string
	ApprovedBy  *string
	UsedBy      *string
	UsedAt      *time.Time
	ExpiresAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type betaInvitationResponse struct {
	ID          string  `json:"id"`
	Code        string  `json:"code"`
	Email       *string `json:"email,omitempty"`
	Status      string  `json:"status"`
	RequestedBy *string `json:"requested_by,omitempty"`
	ApprovedBy  *string `json:"approved_by,omitempty"`
	UsedBy      *string `json:"used_by,omitempty"`
	UsedAt      *string `json:"used_at,omitempty"`
	ExpiresAt   *string `json:"expires_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func toResponse(row betaInvitationRow) betaInvitationResponse {
	r := betaInvitationResponse{
		ID:          row.ID,
		Code:        row.Code,
		Email:       row.Email,
		Status:      row.Status,
		RequestedBy: row.RequestedBy,
		ApprovedBy:  row.ApprovedBy,
		UsedBy:      row.UsedBy,
		CreatedAt:   row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   row.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if row.UsedAt != nil {
		s := row.UsedAt.UTC().Format(time.RFC3339)
		r.UsedAt = &s
	}
	if row.ExpiresAt != nil {
		s := row.ExpiresAt.UTC().Format(time.RFC3339)
		r.ExpiresAt = &s
	}
	return r
}

func generateBetaCode() (string, error) {
	b := make([]byte, betaCodeLen)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(betaCodeAlphabet))))
		if err != nil {
			return "", err
		}
		b[i] = betaCodeAlphabet[n.Int64()]
	}
	return string(b), nil
}

// RequestBeta handles POST /v1/beta/request — user requests beta access.
func (h *BetaInvitationHandler) RequestBeta(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	var existing string
	err := h.pool.QueryRow(ctx,
		`SELECT id FROM beta_invitations WHERE requested_by = $1 AND status IN ('pending','approved') LIMIT 1`,
		userID,
	).Scan(&existing)
	if err == nil {
		response.Error(w, r, apperr.Conflict("beta request already exists"))
		return
	}

	id := idgen.New("bid_")
	now := time.Now().UTC()
	var row betaInvitationRow
	err = h.pool.QueryRow(ctx,
		`INSERT INTO beta_invitations (id, code, status, requested_by, created_at, updated_at)
		 VALUES ($1, $2, 'pending', $3, $4, $4)
		 RETURNING id, code, email, status, requested_by, approved_by, used_by, used_at, expires_at, created_at, updated_at`,
		id, "", userID, now,
	).Scan(&row.ID, &row.Code, &row.Email, &row.Status, &row.RequestedBy, &row.ApprovedBy,
		&row.UsedBy, &row.UsedAt, &row.ExpiresAt, &row.CreatedAt, &row.UpdatedAt)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to create beta request", err))
		return
	}

	response.JSON(w, r, http.StatusCreated, toResponse(row))
}

// GetBetaStatus handles GET /v1/beta/status — user checks their beta status.
func (h *BetaInvitationHandler) GetBetaStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	var row betaInvitationRow
	err := h.pool.QueryRow(ctx,
		`SELECT id, code, email, status, requested_by, approved_by, used_by, used_at, expires_at, created_at, updated_at
		 FROM beta_invitations
		 WHERE requested_by = $1 OR used_by = $1
		 ORDER BY created_at DESC
		 LIMIT 1`,
		userID,
	).Scan(&row.ID, &row.Code, &row.Email, &row.Status, &row.RequestedBy, &row.ApprovedBy,
		&row.UsedBy, &row.UsedAt, &row.ExpiresAt, &row.CreatedAt, &row.UpdatedAt)
	if err != nil {
		response.Error(w, r, apperr.NotFound("no beta invitation found"))
		return
	}

	response.JSON(w, r, http.StatusOK, toResponse(row))
}

// RedeemBeta handles POST /v1/beta/redeem — user redeems an invitation code.
func (h *BetaInvitationHandler) RedeemBeta(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Code == "" {
		response.Error(w, r, apperr.Validation("code is required", ""))
		return
	}

	var row betaInvitationRow
	err := h.pool.QueryRow(ctx,
		`SELECT id, code, email, status, requested_by, approved_by, used_by, used_at, expires_at, created_at, updated_at
		 FROM beta_invitations
		 WHERE code = $1`,
		body.Code,
	).Scan(&row.ID, &row.Code, &row.Email, &row.Status, &row.RequestedBy, &row.ApprovedBy,
		&row.UsedBy, &row.UsedAt, &row.ExpiresAt, &row.CreatedAt, &row.UpdatedAt)
	if err != nil {
		response.Error(w, r, apperr.NotFound("invitation code not found"))
		return
	}

	if row.Status != "approved" {
		response.Error(w, r, apperr.Conflict("invitation code is not available for redemption"))
		return
	}

	if row.ExpiresAt != nil && row.ExpiresAt.Before(time.Now()) {
		response.Error(w, r, apperr.Conflict("invitation code has expired"))
		return
	}

	if row.Email != nil && *row.Email != "" {
		var userEmail string
		err := h.pool.QueryRow(ctx, `SELECT email FROM users WHERE id = $1`, userID).Scan(&userEmail)
		if err != nil || userEmail != *row.Email {
			response.Error(w, r, apperr.Forbidden("invitation code is restricted to a specific email"))
			return
		}
	}

	now := time.Now().UTC()
	var updated betaInvitationRow
	err = h.pool.QueryRow(ctx,
		`UPDATE beta_invitations
		 SET status = 'used', used_by = $1, used_at = $2, updated_at = $2
		 WHERE id = $3
		 RETURNING id, code, email, status, requested_by, approved_by, used_by, used_at, expires_at, created_at, updated_at`,
		userID, now, row.ID,
	).Scan(&updated.ID, &updated.Code, &updated.Email, &updated.Status, &updated.RequestedBy,
		&updated.ApprovedBy, &updated.UsedBy, &updated.UsedAt, &updated.ExpiresAt, &updated.CreatedAt, &updated.UpdatedAt)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to redeem invitation", err))
		return
	}

	response.JSON(w, r, http.StatusOK, toResponse(updated))
}

// AdminListBetaInvitations handles GET /v1/admin/beta-invitations.
func (h *BetaInvitationHandler) AdminListBetaInvitations(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	statusFilter := r.URL.Query().Get("status")

	var rows pgx.Rows
	var err error

	if statusFilter != "" {
		rows, err = h.pool.Query(ctx,
			`SELECT id, code, email, status, requested_by, approved_by, used_by, used_at, expires_at, created_at, updated_at
			 FROM beta_invitations
			 WHERE status = $1
			 ORDER BY created_at DESC`,
			statusFilter,
		)
	} else {
		rows, err = h.pool.Query(ctx,
			`SELECT id, code, email, status, requested_by, approved_by, used_by, used_at, expires_at, created_at, updated_at
			 FROM beta_invitations
			 ORDER BY created_at DESC`,
		)
	}
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list beta invitations", err))
		return
	}
	defer rows.Close()

	invitations := make([]betaInvitationResponse, 0)
	for rows.Next() {
		var row betaInvitationRow
		if err := rows.Scan(&row.ID, &row.Code, &row.Email, &row.Status, &row.RequestedBy,
			&row.ApprovedBy, &row.UsedBy, &row.UsedAt, &row.ExpiresAt, &row.CreatedAt, &row.UpdatedAt); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan invitation row", err))
			return
		}
		invitations = append(invitations, toResponse(row))
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("invitation row iteration error", err))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]any{
		"invitations": invitations,
		"total":       len(invitations),
	})
}

// AdminCreateBetaInvitation handles POST /v1/admin/beta-invitations.
func (h *BetaInvitationHandler) AdminCreateBetaInvitation(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var body struct {
		Email       string `json:"email"`
		ExpiresDays int    `json:"expires_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", ""))
		return
	}

	code, err := generateBetaCode()
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to generate invitation code", err))
		return
	}

	id := idgen.New("bid_")
	now := time.Now().UTC()

	var emailArg *string
	if body.Email != "" {
		emailArg = &body.Email
	}

	var expiresAt *time.Time
	if body.ExpiresDays > 0 {
		t := now.AddDate(0, 0, body.ExpiresDays)
		expiresAt = &t
	}

	var row betaInvitationRow
	err = h.pool.QueryRow(ctx,
		`INSERT INTO beta_invitations (id, code, email, status, expires_at, created_at, updated_at)
		 VALUES ($1, $2, $3, 'approved', $4, $5, $5)
		 RETURNING id, code, email, status, requested_by, approved_by, used_by, used_at, expires_at, created_at, updated_at`,
		id, code, emailArg, expiresAt, now,
	).Scan(&row.ID, &row.Code, &row.Email, &row.Status, &row.RequestedBy, &row.ApprovedBy,
		&row.UsedBy, &row.UsedAt, &row.ExpiresAt, &row.CreatedAt, &row.UpdatedAt)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to create invitation", err))
		return
	}

	response.JSON(w, r, http.StatusCreated, toResponse(row))
}

// AdminUpdateBetaInvitation handles PATCH /v1/admin/beta-invitations/{id}.
func (h *BetaInvitationHandler) AdminUpdateBetaInvitation(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	invitationID := chi.URLParam(r, "id")
	if invitationID == "" {
		response.Error(w, r, apperr.Validation("id is required", ""))
		return
	}

	var body struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Action == "" {
		response.Error(w, r, apperr.Validation("action is required", ""))
		return
	}

	var newStatus string
	switch body.Action {
	case "approve":
		newStatus = "approved"
	case "revoke":
		newStatus = "revoked"
	default:
		response.Error(w, r, apperr.Validation("action must be 'approve' or 'revoke'", ""))
		return
	}

	now := time.Now().UTC()
	var row betaInvitationRow

	if newStatus == "approved" {
		code, err := generateBetaCode()
		if err != nil {
			response.Error(w, r, apperr.Internal("failed to generate invitation code", err))
			return
		}
		err = h.pool.QueryRow(ctx,
			`UPDATE beta_invitations
			 SET status = $1, code = $2, updated_at = $3
			 WHERE id = $4
			 RETURNING id, code, email, status, requested_by, approved_by, used_by, used_at, expires_at, created_at, updated_at`,
			newStatus, code, now, invitationID,
		).Scan(&row.ID, &row.Code, &row.Email, &row.Status, &row.RequestedBy, &row.ApprovedBy,
			&row.UsedBy, &row.UsedAt, &row.ExpiresAt, &row.CreatedAt, &row.UpdatedAt)
		if err != nil {
			response.Error(w, r, apperr.NotFound("invitation not found"))
			return
		}
	} else {
		err := h.pool.QueryRow(ctx,
			`UPDATE beta_invitations
			 SET status = $1, updated_at = $2
			 WHERE id = $3
			 RETURNING id, code, email, status, requested_by, approved_by, used_by, used_at, expires_at, created_at, updated_at`,
			newStatus, now, invitationID,
		).Scan(&row.ID, &row.Code, &row.Email, &row.Status, &row.RequestedBy, &row.ApprovedBy,
			&row.UsedBy, &row.UsedAt, &row.ExpiresAt, &row.CreatedAt, &row.UpdatedAt)
		if err != nil {
			response.Error(w, r, apperr.NotFound("invitation not found"))
			return
		}
	}

	response.JSON(w, r, http.StatusOK, toResponse(row))
}
