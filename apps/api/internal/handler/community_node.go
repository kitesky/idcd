package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
)

type CommunityNodePool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type CommunityNodeHandler struct {
	pool CommunityNodePool
}

func NewCommunityNodeHandler(pool CommunityNodePool) *CommunityNodeHandler {
	return &CommunityNodeHandler{pool: pool}
}

type applyNodeRequest struct {
	Hostname      string `json:"hostname"`
	IPAddress     string `json:"ip_address"`
	Country       string `json:"country"`
	City          string `json:"city"`
	ISP           string `json:"isp"`
	BandwidthMbps *int   `json:"bandwidth_mbps"`
	Motivation    string `json:"motivation"`
}

type nodeApplicationResponse struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Hostname  string     `json:"hostname"`
	IPAddress string     `json:"ip_address"`
	Country   string     `json:"country"`
	City      *string    `json:"city,omitempty"`
	ISP       *string    `json:"isp,omitempty"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type pointsHistoryItem struct {
	ID        string    `json:"id"`
	Amount    int       `json:"amount"`
	Balance   int       `json:"balance"`
	Reason    string    `json:"reason"`
	RefID     *string   `json:"ref_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type pointsResponse struct {
	Balance int                 `json:"balance"`
	History []pointsHistoryItem `json:"history"`
}

type redeemRequest struct {
	RewardType string `json:"reward_type"`
	Points     int    `json:"points"`
}

type redeemResponse struct {
	ID           string    `json:"id"`
	PointsSpent  int       `json:"points_spent"`
	RewardType   string    `json:"reward_type"`
	RewardAmount int       `json:"reward_amount"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

type adminUpdateRequest struct {
	Action string `json:"action"`
	Note   string `json:"note"`
}

func (h *CommunityNodeHandler) Apply(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	var req applyNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", ""))
		return
	}

	if req.Hostname == "" || req.IPAddress == "" || req.Country == "" {
		response.Error(w, r, apperr.Validation("hostname, ip_address, and country are required", ""))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	id := idgen.New("na_")
	now := time.Now()

	_, err := h.pool.Exec(ctx, `
		INSERT INTO node_applications
			(id, user_id, hostname, ip_address, country, city, isp, bandwidth_mbps, motivation, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'pending', $10, $10)
	`, id, userID, req.Hostname, req.IPAddress, req.Country,
		nullableString(req.City), nullableString(req.ISP), req.BandwidthMbps,
		nullableString(req.Motivation), now)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to create application", err))
		return
	}

	city := req.City
	isp := req.ISP
	resp := nodeApplicationResponse{
		ID:        id,
		UserID:    userID,
		Hostname:  req.Hostname,
		IPAddress: req.IPAddress,
		Country:   req.Country,
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if city != "" {
		resp.City = &city
	}
	if isp != "" {
		resp.ISP = &isp
	}

	response.JSON(w, r, http.StatusCreated, resp)
}

func (h *CommunityNodeHandler) MyApplications(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	rows, err := h.pool.Query(ctx, `
		SELECT id, user_id, hostname, ip_address, country, city, isp, status, created_at, updated_at
		FROM node_applications
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query applications", err))
		return
	}
	defer rows.Close()

	apps := []nodeApplicationResponse{}
	for rows.Next() {
		var a nodeApplicationResponse
		if err := rows.Scan(
			&a.ID, &a.UserID, &a.Hostname, &a.IPAddress, &a.Country,
			&a.City, &a.ISP, &a.Status, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan application", err))
			return
		}
		apps = append(apps, a)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to read applications", err))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]any{"applications": apps})
}

func (h *CommunityNodeHandler) GetPoints(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	rows, err := h.pool.Query(ctx, `
		SELECT id, amount, balance, reason, ref_id, created_at
		FROM node_points
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 50
	`, userID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query points", err))
		return
	}
	defer rows.Close()

	history := []pointsHistoryItem{}
	var balance int
	for rows.Next() {
		var item pointsHistoryItem
		if err := rows.Scan(&item.ID, &item.Amount, &item.Balance, &item.Reason, &item.RefID, &item.CreatedAt); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan points", err))
			return
		}
		history = append(history, item)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to read points", err))
		return
	}

	if len(history) > 0 {
		balance = history[0].Balance
	}

	response.JSON(w, r, http.StatusOK, pointsResponse{
		Balance: balance,
		History: history,
	})
}

func (h *CommunityNodeHandler) Redeem(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	var req redeemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", ""))
		return
	}

	if req.Points <= 0 || req.RewardType == "" {
		response.Error(w, r, apperr.Validation("points and reward_type are required", ""))
		return
	}

	rewardAmount := rewardAmountFor(req.RewardType, req.Points)
	if rewardAmount < 0 {
		response.Error(w, r, apperr.Validation("invalid reward_type", ""))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var currentBalance int
	err := h.pool.QueryRow(ctx, `
		SELECT COALESCE((SELECT balance FROM node_points WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1), 0)
	`, userID).Scan(&currentBalance)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to check balance", err))
		return
	}

	if currentBalance < req.Points {
		response.Error(w, r, apperr.Validation("insufficient points balance", ""))
		return
	}

	newBalance := currentBalance - req.Points
	now := time.Now()
	pointsID := idgen.New("pts_")
	redeemID := idgen.New("red_")

	_, err = h.pool.Exec(ctx, `
		INSERT INTO node_points (id, user_id, amount, balance, reason, ref_id, created_at)
		VALUES ($1, $2, $3, $4, 'redemption', $5, $6)
	`, pointsID, userID, -req.Points, newBalance, redeemID, now)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to deduct points", err))
		return
	}

	_, err = h.pool.Exec(ctx, `
		INSERT INTO point_redemptions (id, user_id, points_spent, reward_type, reward_amount, status, created_at)
		VALUES ($1, $2, $3, $4, $5, 'pending', $6)
	`, redeemID, userID, req.Points, req.RewardType, rewardAmount, now)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to create redemption", err))
		return
	}

	response.JSON(w, r, http.StatusCreated, redeemResponse{
		ID:           redeemID,
		PointsSpent:  req.Points,
		RewardType:   req.RewardType,
		RewardAmount: rewardAmount,
		Status:       "pending",
		CreatedAt:    now,
	})
}

func (h *CommunityNodeHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	status := r.URL.Query().Get("status")

	query := `
		SELECT id, user_id, hostname, ip_address, country, city, isp, status, created_at, updated_at
		FROM node_applications
	`
	args := []any{}
	if status != "" {
		query += " WHERE status = $1"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC LIMIT 200"

	rows, err := h.pool.Query(ctx, query, args...)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query applications", err))
		return
	}
	defer rows.Close()

	apps := []nodeApplicationResponse{}
	for rows.Next() {
		var a nodeApplicationResponse
		if err := rows.Scan(
			&a.ID, &a.UserID, &a.Hostname, &a.IPAddress, &a.Country,
			&a.City, &a.ISP, &a.Status, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan application", err))
			return
		}
		apps = append(apps, a)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to read applications", err))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]any{"applications": apps})
}

func (h *CommunityNodeHandler) AdminUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, r, apperr.Validation("missing application id", ""))
		return
	}

	var req adminUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", ""))
		return
	}

	if req.Action != "approve" && req.Action != "reject" && req.Action != "activate" {
		response.Error(w, r, apperr.Validation("action must be approve, reject, or activate", ""))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	now := time.Now()
	var tag pgconn.CommandTag
	var err error

	switch req.Action {
	case "approve":
		tag, err = h.pool.Exec(ctx, `
			UPDATE node_applications
			SET status = 'probation', review_note = $2, probation_started_at = $3, updated_at = $3
			WHERE id = $1
		`, id, req.Note, now)
	case "reject":
		tag, err = h.pool.Exec(ctx, `
			UPDATE node_applications
			SET status = 'rejected', review_note = $2, updated_at = $3
			WHERE id = $1
		`, id, req.Note, now)
	case "activate":
		tag, err = h.pool.Exec(ctx, `
			UPDATE node_applications
			SET status = 'active', activated_at = $2, updated_at = $2
			WHERE id = $1
		`, id, now)
	}

	if err != nil {
		response.Error(w, r, apperr.Internal("failed to update application", err))
		return
	}

	if tag.RowsAffected() == 0 {
		response.Error(w, r, apperr.NotFound("application not found"))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]any{"id": id, "action": req.Action, "updated_at": now})
}

func rewardAmountFor(rewardType string, points int) int {
	switch rewardType {
	case "api_calls":
		return (points / 500) * 1000
	case "monitors":
		return points / 1000
	case "storage":
		return points / 500
	default:
		return -1
	}
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
