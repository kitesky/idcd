package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/postmortem"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
)

type PostmortemPool interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type PostmortemHandler struct {
	pool PostmortemPool
}

func NewPostmortemHandler(pool PostmortemPool) *PostmortemHandler {
	return &PostmortemHandler{pool: pool}
}

type PostmortemResponse struct {
	ID           string                     `json:"id"`
	AlertEventID string                     `json:"alert_event_id"`
	MonitorID    string                     `json:"monitor_id"`
	UserID       string                     `json:"user_id"`
	Title        string                     `json:"title"`
	Status       string                     `json:"status"`
	Severity     string                     `json:"severity"`
	Impact       string                     `json:"impact"`
	Timeline     []postmortem.TimelineEntry `json:"timeline"`
	RootCause    string                     `json:"root_cause"`
	Resolution   string                     `json:"resolution"`
	ActionItems  []postmortem.ActionItem    `json:"action_items"`
	CreatedAt    string                     `json:"created_at"`
	UpdatedAt    string                     `json:"updated_at"`
}

type UpdatePostmortemRequest struct {
	Title       *string `json:"title"`
	Status      *string `json:"status"`
	Severity    *string `json:"severity"`
	Impact      *string `json:"impact"`
	RootCause   *string `json:"root_cause"`
	Resolution  *string `json:"resolution"`
}

type IncidentListItem struct {
	EventID    string  `json:"event_id"`
	MonitorID  string  `json:"monitor_id"`
	Status     string  `json:"status"`
	StartedAt  string  `json:"started_at"`
	ResolvedAt *string `json:"resolved_at,omitempty"`
	HasDraft   bool    `json:"has_draft"`
}

func (h *PostmortemHandler) Draft(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	eventID := chi.URLParam(r, "event_id")

	var (
		monitorID  string
		startedAt  time.Time
		resolvedAt *time.Time
		metadata   []byte
	)
	err := h.pool.QueryRow(ctx, `
		SELECT ae.monitor_id, ae.started_at, ae.resolved_at, ae.metadata
		FROM alert_events ae
		WHERE ae.id = $1
		  AND EXISTS (
		    SELECT 1 FROM alert_policies ap
		    WHERE ap.id = ae.policy_id AND ap.user_id = $2
		  )`, eventID, userID).Scan(&monitorID, &startedAt, &resolvedAt, &metadata)
	if errors.Is(err, pgx.ErrNoRows) {
		response.Error(w, r, apperr.NotFound("alert event not found"))
		return
	}
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to fetch alert event", err))
		return
	}

	var monitorName, monitorType string
	_ = h.pool.QueryRow(ctx, `
		SELECT name, type FROM monitors WHERE id = $1`, monitorID).Scan(&monitorName, &monitorType)
	if monitorName == "" {
		monitorName = monitorID
	}
	if monitorType == "" {
		monitorType = "http"
	}

	_ = metadata

	var dur time.Duration
	if resolvedAt != nil {
		dur = resolvedAt.Sub(startedAt)
	} else {
		dur = time.Since(startedAt)
	}

	draft := postmortem.GenerateDraft(postmortem.DraftInput{
		MonitorName: monitorName,
		MonitorType: monitorType,
		StartedAt:   startedAt,
		ResolvedAt:  resolvedAt,
		Duration:    dur,
	})

	timelineJSON, err := json.Marshal(draft.Timeline)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to marshal timeline", err))
		return
	}
	actionItemsJSON, err := json.Marshal(draft.ActionItems)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to marshal action items", err))
		return
	}

	id := idgen.New("pm_")
	now := time.Now()

	_, err = h.pool.Exec(ctx, `
		INSERT INTO incident_postmortems
		  (id, alert_event_id, monitor_id, user_id, title, status, severity,
		   impact, timeline, root_cause, resolution, action_items, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,'draft',$6,$7,$8,$9,$10,$11,$12,$12)
		ON CONFLICT (alert_event_id) DO UPDATE SET
		  title=$5, severity=$6, impact=$7, timeline=$8,
		  root_cause=$9, resolution=$10, action_items=$11, updated_at=$12`,
		id, eventID, monitorID, userID,
		draft.Title, draft.Severity, draft.Impact,
		timelineJSON, draft.RootCause, draft.Resolution,
		actionItemsJSON, now,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to save postmortem draft", err))
		return
	}

	resp := PostmortemResponse{
		ID:           id,
		AlertEventID: eventID,
		MonitorID:    monitorID,
		UserID:       userID,
		Title:        draft.Title,
		Status:       "draft",
		Severity:     draft.Severity,
		Impact:       draft.Impact,
		Timeline:     draft.Timeline,
		RootCause:    draft.RootCause,
		Resolution:   draft.Resolution,
		ActionItems:  draft.ActionItems,
		CreatedAt:    now.UTC().Format(time.RFC3339),
		UpdatedAt:    now.UTC().Format(time.RFC3339),
	}
	response.JSON(w, r, http.StatusCreated, resp)
}

func (h *PostmortemHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	eventID := chi.URLParam(r, "event_id")

	var pm PostmortemResponse
	var timelineJSON, actionItemsJSON []byte
	var createdAt, updatedAt time.Time

	err := h.pool.QueryRow(ctx, `
		SELECT id, alert_event_id, monitor_id, user_id, title, status, severity,
		       impact, timeline, root_cause, resolution, action_items, created_at, updated_at
		FROM incident_postmortems
		WHERE alert_event_id = $1 AND user_id = $2`, eventID, userID).
		Scan(&pm.ID, &pm.AlertEventID, &pm.MonitorID, &pm.UserID,
			&pm.Title, &pm.Status, &pm.Severity, &pm.Impact,
			&timelineJSON, &pm.RootCause, &pm.Resolution,
			&actionItemsJSON, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		response.Error(w, r, apperr.NotFound("postmortem not found"))
		return
	}
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to fetch postmortem", err))
		return
	}

	_ = json.Unmarshal(timelineJSON, &pm.Timeline)
	_ = json.Unmarshal(actionItemsJSON, &pm.ActionItems)
	pm.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	pm.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)

	response.JSON(w, r, http.StatusOK, pm)
}

func (h *PostmortemHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	eventID := chi.URLParam(r, "event_id")

	var req UpdatePostmortemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid JSON request body", ""))
		return
	}

	var pm PostmortemResponse
	var timelineJSON, actionItemsJSON []byte
	var createdAt, updatedAt time.Time

	err := h.pool.QueryRow(ctx, `
		SELECT id, alert_event_id, monitor_id, user_id, title, status, severity,
		       impact, timeline, root_cause, resolution, action_items, created_at, updated_at
		FROM incident_postmortems
		WHERE alert_event_id = $1 AND user_id = $2`, eventID, userID).
		Scan(&pm.ID, &pm.AlertEventID, &pm.MonitorID, &pm.UserID,
			&pm.Title, &pm.Status, &pm.Severity, &pm.Impact,
			&timelineJSON, &pm.RootCause, &pm.Resolution,
			&actionItemsJSON, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		response.Error(w, r, apperr.NotFound("postmortem not found"))
		return
	}
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to fetch postmortem", err))
		return
	}

	if req.Title != nil {
		pm.Title = *req.Title
	}
	if req.Status != nil {
		pm.Status = *req.Status
	}
	if req.Severity != nil {
		pm.Severity = *req.Severity
	}
	if req.Impact != nil {
		pm.Impact = *req.Impact
	}
	if req.RootCause != nil {
		pm.RootCause = *req.RootCause
	}
	if req.Resolution != nil {
		pm.Resolution = *req.Resolution
	}

	now := time.Now()
	_, err = h.pool.Exec(ctx, `
		UPDATE incident_postmortems
		SET title=$1, status=$2, severity=$3, impact=$4,
		    root_cause=$5, resolution=$6, updated_at=$7
		WHERE alert_event_id=$8 AND user_id=$9`,
		pm.Title, pm.Status, pm.Severity, pm.Impact,
		pm.RootCause, pm.Resolution, now,
		eventID, userID,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to update postmortem", err))
		return
	}

	_ = json.Unmarshal(timelineJSON, &pm.Timeline)
	_ = json.Unmarshal(actionItemsJSON, &pm.ActionItems)
	pm.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	pm.UpdatedAt = now.UTC().Format(time.RFC3339)

	response.JSON(w, r, http.StatusOK, pm)
}

func (h *PostmortemHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	q := r.URL.Query()
	limit := 50
	if l := q.Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}

	rows, err := h.pool.Query(ctx, `
		SELECT ae.id, ae.monitor_id, ae.status, ae.started_at, ae.resolved_at,
		       (EXISTS (SELECT 1 FROM incident_postmortems pm WHERE pm.alert_event_id = ae.id)) AS has_draft
		FROM alert_events ae
		WHERE EXISTS (
		    SELECT 1 FROM alert_policies ap
		    WHERE ap.id = ae.policy_id AND ap.user_id = $1
		)
		ORDER BY ae.started_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list incidents", err))
		return
	}
	defer rows.Close()

	var items []IncidentListItem
	for rows.Next() {
		var item IncidentListItem
		var startedAt time.Time
		var resolvedAt *time.Time
		if err := rows.Scan(&item.EventID, &item.MonitorID, &item.Status,
			&startedAt, &resolvedAt, &item.HasDraft); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan incident", err))
			return
		}
		item.StartedAt = startedAt.UTC().Format(time.RFC3339)
		if resolvedAt != nil {
			s := resolvedAt.UTC().Format(time.RFC3339)
			item.ResolvedAt = &s
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to iterate incidents", err))
		return
	}

	if items == nil {
		items = []IncidentListItem{}
	}
	response.JSON(w, r, http.StatusOK, map[string]any{"items": items})
}
