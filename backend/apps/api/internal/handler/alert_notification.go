package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// AlertNotificationHandler implements GET /v1/alert-channels/{id}/notifications.
type AlertNotificationHandler struct {
	pool AlertPool
}

// NewAlertNotificationHandler creates an AlertNotificationHandler.
func NewAlertNotificationHandler(pool AlertPool) *AlertNotificationHandler {
	return &AlertNotificationHandler{pool: pool}
}

// AlertNotificationResponse is the JSON representation of an alert_notification row.
type AlertNotificationResponse struct {
	ID           string  `json:"id"`
	AlertEventID string  `json:"alert_event_id"`
	ChannelID    string  `json:"channel_id"`
	Status       string  `json:"status"`
	Error        *string `json:"error"`
	SentAt       *string `json:"sent_at"`
	CreatedAt    string  `json:"created_at"`
}

// ListNotificationsResponse is the response body for the list notifications endpoint.
type ListNotificationsResponse struct {
	Notifications []AlertNotificationResponse `json:"notifications"`
	Total         int64                       `json:"total"`
}

// List handles GET /v1/alert-channels/{id}/notifications.
func (h *AlertNotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	channelID := chi.URLParam(r, "id")

	var ownerID string
	err := h.pool.QueryRow(ctx,
		`SELECT user_id FROM alert_channels WHERE id = $1`,
		channelID,
	).Scan(&ownerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(w, r, apperr.NotFound("alert channel not found"))
		} else {
			response.Error(w, r, apperr.Internal("failed to fetch alert channel", err))
		}
		return
	}
	if ownerID != userID {
		response.Error(w, r, apperr.Forbidden("access denied"))
		return
	}

	q := r.URL.Query()

	limit := 20
	if l := q.Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	offset := 0
	if o := q.Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	var total int64
	err = h.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM alert_notifications WHERE channel_id = $1`,
		channelID,
	).Scan(&total)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to count notifications", err))
		return
	}

	rows, err := h.pool.Query(ctx, `
		SELECT an.id, an.event_id, an.channel_id, an.status, an.error, an.sent_at, an.created_at
		FROM alert_notifications an
		JOIN alert_channels ac ON ac.id = an.channel_id
		WHERE an.channel_id = $1 AND ac.user_id = $2
		ORDER BY an.created_at DESC
		LIMIT $3 OFFSET $4`,
		channelID, userID, limit, offset,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list notifications", err))
		return
	}
	defer rows.Close()

	items := make([]AlertNotificationResponse, 0)
	for rows.Next() {
		var item AlertNotificationResponse
		var sentAt *time.Time
		var createdAt time.Time
		if err := rows.Scan(
			&item.ID, &item.AlertEventID, &item.ChannelID, &item.Status,
			&item.Error, &sentAt, &createdAt,
		); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan notification", err))
			return
		}
		if sentAt != nil {
			t := sentAt.UTC().Format(time.RFC3339)
			item.SentAt = &t
		}
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to iterate notifications", err))
		return
	}

	response.JSON(w, r, http.StatusOK, ListNotificationsResponse{
		Notifications: items,
		Total:         total,
	})
}
