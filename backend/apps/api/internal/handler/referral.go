package handler

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
)

type ReferralPool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type ReferralHandler struct {
	pool ReferralPool
}

func NewReferralHandler(pool ReferralPool) *ReferralHandler {
	return &ReferralHandler{pool: pool}
}

type referralCodeResponse struct {
	Code      string `json:"code"`
	UsesCount int    `json:"uses_count"`
	URL       string `json:"url"`
}

type referralRewardItem struct {
	ID           string     `json:"id"`
	ReferredID   string     `json:"referred_id"`
	Code         string     `json:"code"`
	Status       string     `json:"status"`
	RewardAmount string     `json:"reward_amount"`
	CreditedAt   *time.Time `json:"credited_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type referralRewardsResponse struct {
	Rewards       []referralRewardItem `json:"rewards"`
	TotalPending  string               `json:"total_pending"`
	TotalCredited string               `json:"total_credited"`
}

func generateReferralCode() (string, error) {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}
	var sb strings.Builder
	sb.WriteString("IDCD-")
	for _, v := range b {
		sb.WriteByte(chars[int(v)%len(chars)])
	}
	return sb.String(), nil
}

func (h *ReferralHandler) GetOrCreateCode(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var existingID, existingCode string
	var usesCount int
	err := h.pool.QueryRow(ctx,
		`SELECT id, code, uses_count FROM referral_codes WHERE user_id = $1`,
		userID,
	).Scan(&existingID, &existingCode, &usesCount)

	if err == nil {
		response.JSON(w, r, http.StatusOK, referralCodeResponse{
			Code:      existingCode,
			UsesCount: usesCount,
			URL:       "https://idcd.com/?ref=" + existingCode,
		})
		return
	}

	code, err := generateReferralCode()
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to generate code", err))
		return
	}

	id := idgen.New("ref_")
	if _, err := h.pool.Exec(ctx,
		`INSERT INTO referral_codes (id, user_id, code) VALUES ($1, $2, $3)`,
		id, userID, code,
	); err != nil {
		response.Error(w, r, apperr.Internal("failed to create referral code", err))
		return
	}

	response.JSON(w, r, http.StatusCreated, referralCodeResponse{
		Code:      code,
		UsesCount: 0,
		URL:       "https://idcd.com/?ref=" + code,
	})
}

func (h *ReferralHandler) GetCode(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var code string
	var usesCount int
	err := h.pool.QueryRow(ctx,
		`SELECT code, uses_count FROM referral_codes WHERE user_id = $1`,
		userID,
	).Scan(&code, &usesCount)
	if err != nil {
		response.Error(w, r, apperr.NotFound("referral code not found"))
		return
	}

	response.JSON(w, r, http.StatusOK, referralCodeResponse{
		Code:      code,
		UsesCount: usesCount,
		URL:       "https://idcd.com/?ref=" + code,
	})
}

func (h *ReferralHandler) ListRewards(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	rows, err := h.pool.Query(ctx, `
		SELECT id, referred_id, code, status, reward_amount, credited_at, created_at
		FROM referral_rewards
		WHERE referrer_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query rewards", err))
		return
	}
	defer rows.Close()

	rewards := []referralRewardItem{}
	var totalPending, totalCredited float64

	for rows.Next() {
		var item referralRewardItem
		var amount float64
		var creditedAt *time.Time
		if err := rows.Scan(
			&item.ID, &item.ReferredID, &item.Code, &item.Status,
			&amount, &creditedAt, &item.CreatedAt,
		); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan reward", err))
			return
		}
		item.RewardAmount = fmt.Sprintf("%.2f", amount)
		item.CreditedAt = creditedAt
		rewards = append(rewards, item)
		switch item.Status {
		case "pending":
			totalPending += amount
		case "credited":
			totalCredited += amount
		}
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to iterate rewards", err))
		return
	}

	response.JSON(w, r, http.StatusOK, referralRewardsResponse{
		Rewards:       rewards,
		TotalPending:  fmt.Sprintf("%.2f", totalPending),
		TotalCredited: fmt.Sprintf("%.2f", totalCredited),
	})
}
