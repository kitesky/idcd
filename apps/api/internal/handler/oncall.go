package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/oncall"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
)

type OncallPool interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type OncallHandler struct {
	pool OncallPool
}

func NewOncallHandler(pool OncallPool) *OncallHandler {
	return &OncallHandler{pool: pool}
}

type oncallScheduleResponse struct {
	ID           string `json:"id"`
	TeamID       string `json:"team_id"`
	Name         string `json:"name"`
	RotationType string `json:"rotation_type"`
	HandoffHour  int    `json:"handoff_hour"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type oncallParticipantResponse struct {
	ID         string `json:"id"`
	ScheduleID string `json:"schedule_id"`
	UserID     string `json:"user_id"`
	OrderIndex int    `json:"order_index"`
}

type oncallOverrideResponse struct {
	ID         string `json:"id"`
	ScheduleID string `json:"schedule_id"`
	UserID     string `json:"user_id"`
	StartAt    string `json:"start_at"`
	EndAt      string `json:"end_at"`
	CreatedBy  string `json:"created_by"`
	CreatedAt  string `json:"created_at"`
}

type createScheduleRequest struct {
	TeamID       string `json:"team_id"`
	Name         string `json:"name"`
	RotationType string `json:"rotation_type"`
	HandoffHour  *int   `json:"handoff_hour"`
}

type addParticipantRequest struct {
	UserID     string `json:"user_id"`
	OrderIndex int    `json:"order_index"`
}

type createOverrideRequest struct {
	UserID  string `json:"user_id"`
	StartAt string `json:"start_at"`
	EndAt   string `json:"end_at"`
}

// teamRoleForUser returns the membership role ("owner"/"admin"/"member") for
// userID in teamID. Returns ("", pgx.ErrNoRows) when the user is not a member.
func (h *OncallHandler) teamRoleForUser(ctx context.Context, teamID, userID string) (string, error) {
	var role string
	err := h.pool.QueryRow(ctx,
		`SELECT role FROM team_memberships WHERE team_id = $1 AND user_id = $2`,
		teamID, userID,
	).Scan(&role)
	return role, err
}

// scheduleTeamID resolves the team_id for an oncall schedule. pgx.ErrNoRows
// means the schedule does not exist.
func (h *OncallHandler) scheduleTeamID(ctx context.Context, scheduleID string) (string, error) {
	var teamID string
	err := h.pool.QueryRow(ctx,
		`SELECT team_id FROM oncall_schedules WHERE id = $1`, scheduleID,
	).Scan(&teamID)
	return teamID, err
}

// requireScheduleMember verifies that userID belongs to the schedule's team.
// Returns nil on success; writes the appropriate error response and returns
// a non-nil error when the caller should abort.
func (h *OncallHandler) requireScheduleMember(ctx context.Context, scheduleID, userID string) (teamID string, appErr *apperr.Error) {
	tid, err := h.scheduleTeamID(ctx, scheduleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperr.NotFound("oncall schedule not found")
		}
		return "", apperr.Internal("failed to load oncall schedule", err)
	}
	if _, err := h.teamRoleForUser(ctx, tid, userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperr.NotFound("oncall schedule not found")
		}
		return "", apperr.Internal("failed to verify team membership", err)
	}
	return tid, nil
}

// requireScheduleAdmin is like requireScheduleMember but also enforces
// owner/admin role.
func (h *OncallHandler) requireScheduleAdmin(ctx context.Context, scheduleID, userID string) (teamID string, appErr *apperr.Error) {
	tid, err := h.scheduleTeamID(ctx, scheduleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperr.NotFound("oncall schedule not found")
		}
		return "", apperr.Internal("failed to load oncall schedule", err)
	}
	role, err := h.teamRoleForUser(ctx, tid, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperr.NotFound("oncall schedule not found")
		}
		return "", apperr.Internal("failed to verify team role", err)
	}
	if role != "owner" && role != "admin" {
		return "", apperr.Forbidden("team owner or admin required")
	}
	return tid, nil
}

func (h *OncallHandler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	var req createScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid JSON request body", ""))
		return
	}
	if req.TeamID == "" {
		response.Error(w, r, apperr.Validation("team_id is required", "team_id"))
		return
	}
	if req.Name == "" {
		response.Error(w, r, apperr.Validation("name is required", "name"))
		return
	}
	rotationType := req.RotationType
	if rotationType == "" {
		rotationType = "weekly"
	}
	if rotationType != "daily" && rotationType != "weekly" && rotationType != "custom" {
		response.Error(w, r, apperr.Validation("rotation_type must be daily, weekly, or custom", "rotation_type"))
		return
	}
	handoffHour := 9
	if req.HandoffHour != nil {
		handoffHour = *req.HandoffHour
	}

	// Authorize: caller must be an owner or admin of the target team. Without
	// this any logged-in user could write oncall data into someone else's team.
	role, err := h.teamRoleForUser(ctx, req.TeamID, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(w, r, apperr.Forbidden("not a member of the target team"))
			return
		}
		response.Error(w, r, apperr.Internal("failed to verify team membership", err))
		return
	}
	if role != "owner" && role != "admin" {
		response.Error(w, r, apperr.Forbidden("team owner or admin required"))
		return
	}

	id := idgen.OncallSchedule()
	now := time.Now().UTC()

	_, err = h.pool.Exec(ctx, `
		INSERT INTO oncall_schedules (id, team_id, name, rotation_type, handoff_hour, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)`,
		id, req.TeamID, req.Name, rotationType, handoffHour, now,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to create oncall schedule", err))
		return
	}

	resp := oncallScheduleResponse{
		ID:           id,
		TeamID:       req.TeamID,
		Name:         req.Name,
		RotationType: rotationType,
		HandoffHour:  handoffHour,
		CreatedAt:    now.Format(time.RFC3339),
		UpdatedAt:    now.Format(time.RFC3339),
	}
	response.JSON(w, r, http.StatusCreated, resp)
}

func (h *OncallHandler) ListSchedules(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	rows, err := h.pool.Query(ctx, `
		SELECT os.id, os.team_id, os.name, os.rotation_type, os.handoff_hour, os.created_at, os.updated_at
		FROM oncall_schedules os
		WHERE os.team_id IN (
			SELECT team_id FROM team_memberships WHERE user_id = $1
		)
		ORDER BY os.created_at DESC`, userID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list oncall schedules", err))
		return
	}
	defer rows.Close()

	var items []oncallScheduleResponse
	for rows.Next() {
		var item oncallScheduleResponse
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&item.ID, &item.TeamID, &item.Name, &item.RotationType, &item.HandoffHour, &createdAt, &updatedAt); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan oncall schedule", err))
			return
		}
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		item.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to iterate oncall schedules", err))
		return
	}
	if items == nil {
		items = []oncallScheduleResponse{}
	}
	response.JSON(w, r, http.StatusOK, map[string]any{"items": items})
}

type scheduleDetailResponse struct {
	oncallScheduleResponse
	CurrentOnCall string                      `json:"current_on_call"`
	Preview       []previewDay                `json:"preview"`
	Participants  []oncallParticipantResponse `json:"participants"`
}

type previewDay struct {
	Date   string `json:"date"`
	UserID string `json:"user_id"`
}

func (h *OncallHandler) GetSchedule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	id := chi.URLParam(r, "id")

	// Authorize first to avoid leaking schedule existence to non-members.
	if _, appErr := h.requireScheduleMember(ctx, id, userID); appErr != nil {
		response.Error(w, r, appErr)
		return
	}

	var sched oncallScheduleResponse
	var createdAt, updatedAt time.Time
	err := h.pool.QueryRow(ctx, `
		SELECT id, team_id, name, rotation_type, handoff_hour, created_at, updated_at
		FROM oncall_schedules WHERE id = $1`, id).
		Scan(&sched.ID, &sched.TeamID, &sched.Name, &sched.RotationType, &sched.HandoffHour, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(w, r, apperr.NotFound("oncall schedule not found"))
		} else {
			response.Error(w, r, apperr.Internal("failed to fetch oncall schedule", err))
		}
		return
	}
	sched.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	sched.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)

	rows, err := h.pool.Query(ctx, `
		SELECT id, schedule_id, user_id, order_index
		FROM oncall_participants WHERE schedule_id = $1
		ORDER BY order_index ASC`, id)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to fetch participants", err))
		return
	}
	defer rows.Close()

	var participants []oncallParticipantResponse
	for rows.Next() {
		var p oncallParticipantResponse
		if err := rows.Scan(&p.ID, &p.ScheduleID, &p.UserID, &p.OrderIndex); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan participant", err))
			return
		}
		participants = append(participants, p)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to iterate participants", err))
		return
	}
	if participants == nil {
		participants = []oncallParticipantResponse{}
	}

	now := time.Now().UTC()
	currentOnCall, currentErr := oncall.CurrentOnCall(ctx, h.pool, id, now)
	if currentErr != nil && !errors.Is(currentErr, oncall.ErrScheduleNotFound) {
		response.Error(w, r, apperr.Internal("failed to compute current on-call", currentErr))
		return
	}

	var preview []previewDay
	for i := range 7 {
		day := now.AddDate(0, 0, i)
		dayUserID, dayErr := oncall.CurrentOnCall(ctx, h.pool, id, day)
		if dayErr != nil && !errors.Is(dayErr, oncall.ErrScheduleNotFound) {
			response.Error(w, r, apperr.Internal("failed to compute on-call preview", dayErr))
			return
		}
		preview = append(preview, previewDay{
			Date:   day.Format("2006-01-02"),
			UserID: dayUserID,
		})
	}

	detail := scheduleDetailResponse{
		oncallScheduleResponse: sched,
		CurrentOnCall:          currentOnCall,
		Preview:                preview,
		Participants:           participants,
	}
	response.JSON(w, r, http.StatusOK, detail)
}

func (h *OncallHandler) AddParticipant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	scheduleID := chi.URLParam(r, "id")

	var req addParticipantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid JSON request body", ""))
		return
	}
	if req.UserID == "" {
		response.Error(w, r, apperr.Validation("user_id is required", "user_id"))
		return
	}

	if _, appErr := h.requireScheduleAdmin(ctx, scheduleID, userID); appErr != nil {
		response.Error(w, r, appErr)
		return
	}

	id := idgen.OncallParticipant()
	_, err := h.pool.Exec(ctx, `
		INSERT INTO oncall_participants (id, schedule_id, user_id, order_index)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (schedule_id, order_index) DO NOTHING`,
		id, scheduleID, req.UserID, req.OrderIndex,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to add participant", err))
		return
	}

	resp := oncallParticipantResponse{
		ID:         id,
		ScheduleID: scheduleID,
		UserID:     req.UserID,
		OrderIndex: req.OrderIndex,
	}
	response.JSON(w, r, http.StatusCreated, resp)
}

func (h *OncallHandler) RemoveParticipant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	scheduleID := chi.URLParam(r, "id")
	participantUserID := chi.URLParam(r, "user_id")

	if _, appErr := h.requireScheduleAdmin(ctx, scheduleID, userID); appErr != nil {
		response.Error(w, r, appErr)
		return
	}

	tag, err := h.pool.Exec(ctx, `
		DELETE FROM oncall_participants WHERE schedule_id = $1 AND user_id = $2`,
		scheduleID, participantUserID,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to remove participant", err))
		return
	}
	if tag.RowsAffected() == 0 {
		response.Error(w, r, apperr.NotFound("participant not found"))
		return
	}

	response.JSON(w, r, http.StatusNoContent, nil)
}

func (h *OncallHandler) CreateOverride(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	createdBy := middleware.UserIDFromContext(ctx)
	if createdBy == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	scheduleID := chi.URLParam(r, "id")

	var req createOverrideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid JSON request body", ""))
		return
	}
	if req.UserID == "" {
		response.Error(w, r, apperr.Validation("user_id is required", "user_id"))
		return
	}
	startAt, err := time.Parse(time.RFC3339, req.StartAt)
	if err != nil {
		response.Error(w, r, apperr.Validation("start_at must be RFC3339", "start_at"))
		return
	}
	endAt, err := time.Parse(time.RFC3339, req.EndAt)
	if err != nil {
		response.Error(w, r, apperr.Validation("end_at must be RFC3339", "end_at"))
		return
	}
	if !endAt.After(startAt) {
		response.Error(w, r, apperr.Validation("end_at must be after start_at", "end_at"))
		return
	}

	if _, appErr := h.requireScheduleAdmin(ctx, scheduleID, createdBy); appErr != nil {
		response.Error(w, r, appErr)
		return
	}

	id := idgen.OncallOverride()
	now := time.Now().UTC()

	_, err = h.pool.Exec(ctx, `
		INSERT INTO oncall_overrides (id, schedule_id, user_id, start_at, end_at, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, scheduleID, req.UserID, startAt.UTC(), endAt.UTC(), createdBy, now,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to create override", err))
		return
	}

	resp := oncallOverrideResponse{
		ID:         id,
		ScheduleID: scheduleID,
		UserID:     req.UserID,
		StartAt:    startAt.UTC().Format(time.RFC3339),
		EndAt:      endAt.UTC().Format(time.RFC3339),
		CreatedBy:  createdBy,
		CreatedAt:  now.Format(time.RFC3339),
	}
	response.JSON(w, r, http.StatusCreated, resp)
}

func (h *OncallHandler) ListOverrides(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	scheduleID := chi.URLParam(r, "id")

	if _, appErr := h.requireScheduleMember(ctx, scheduleID, userID); appErr != nil {
		response.Error(w, r, appErr)
		return
	}

	activeOnly := r.URL.Query().Get("active") == "true"

	var rows pgx.Rows
	var err error
	if activeOnly {
		now := time.Now().UTC()
		rows, err = h.pool.Query(ctx, `
			SELECT id, schedule_id, user_id, start_at, end_at, created_by, created_at
			FROM oncall_overrides
			WHERE schedule_id = $1 AND start_at <= $2 AND end_at >= $2
			ORDER BY start_at ASC`, scheduleID, now)
	} else {
		rows, err = h.pool.Query(ctx, `
			SELECT id, schedule_id, user_id, start_at, end_at, created_by, created_at
			FROM oncall_overrides
			WHERE schedule_id = $1
			ORDER BY start_at ASC`, scheduleID)
	}
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list overrides", err))
		return
	}
	defer rows.Close()

	var overrides []oncallOverrideResponse
	for rows.Next() {
		var o oncallOverrideResponse
		var startAt, endAt, createdAt time.Time
		if err := rows.Scan(&o.ID, &o.ScheduleID, &o.UserID, &startAt, &endAt, &o.CreatedBy, &createdAt); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan override", err))
			return
		}
		o.StartAt = startAt.UTC().Format(time.RFC3339)
		o.EndAt = endAt.UTC().Format(time.RFC3339)
		o.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		overrides = append(overrides, o)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to iterate overrides", err))
		return
	}
	if overrides == nil {
		overrides = []oncallOverrideResponse{}
	}

	// response.JSON already wraps payload in {data: ...}; an extra "data" key
	// here would surface to clients as {data:{data:{...}}}.
	response.JSON(w, r, http.StatusOK, map[string]any{"overrides": overrides})
}

func (h *OncallHandler) DeleteOverride(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	scheduleID := chi.URLParam(r, "id")
	overrideID := chi.URLParam(r, "override_id")

	if _, appErr := h.requireScheduleAdmin(ctx, scheduleID, userID); appErr != nil {
		response.Error(w, r, appErr)
		return
	}

	tag, err := h.pool.Exec(ctx, `
		DELETE FROM oncall_overrides WHERE id = $1 AND schedule_id = $2`,
		overrideID, scheduleID,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to delete override", err))
		return
	}
	if tag.RowsAffected() == 0 {
		response.Error(w, r, apperr.NotFound("override not found"))
		return
	}

	response.JSON(w, r, http.StatusNoContent, nil)
}

func (h *OncallHandler) GetCurrentOnCall(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	scheduleID := chi.URLParam(r, "id")

	if _, appErr := h.requireScheduleMember(ctx, scheduleID, userID); appErr != nil {
		response.Error(w, r, appErr)
		return
	}

	now := time.Now().UTC()
	currentUserID, err := oncall.CurrentOnCall(ctx, h.pool, scheduleID, now)
	if err != nil {
		if errors.Is(err, oncall.ErrScheduleNotFound) {
			response.Error(w, r, apperr.NotFound("oncall schedule not found"))
			return
		}
		response.Error(w, r, apperr.Internal("failed to determine current on-call", err))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]string{"user_id": currentUserID})
}
