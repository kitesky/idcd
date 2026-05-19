package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
)

// Input length limits for team-scoped string fields. Applied at the handler
// layer so a hostile caller can't trigger a 50MB row write and chew through
// DB / log budget before Postgres' column constraints reject it.
const (
	maxTeamNameLen        = 64
	minTeamSlugLen        = 3
	maxTeamSlugLen        = 32
	maxInvitationEmailLen = 256
)

// teamSlugRe enforces the URL-safe slug format: lowercase alphanumerics and
// inner dashes only, no leading/trailing dash. The DB unique index handles
// collision; this regex is the upfront contract that rejects characters
// (uppercase, underscores, slashes, etc.) Postgres would silently accept.
var teamSlugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type TeamPool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Begin(ctx context.Context) (pgx.Tx, error)
}

type TeamHandler struct {
	pool TeamPool
}

func NewTeamHandler(pool TeamPool) *TeamHandler {
	return &TeamHandler{pool: pool}
}

type teamResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Plan      string `json:"plan"`
	OwnerID   string `json:"owner_id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type memberResponse struct {
	ID         string  `json:"id"`
	TeamID     string  `json:"team_id"`
	UserID     string  `json:"user_id"`
	Email      string  `json:"email"`
	Role       string  `json:"role"`
	InvitedBy  *string `json:"invited_by,omitempty"`
	JoinedAt   string  `json:"joined_at"`
}

type invitationResponse struct {
	ID         string `json:"id"`
	TeamID     string `json:"team_id"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	InvitedBy  string `json:"invited_by"`
	Status     string `json:"status"`
	ExpiresAt  string `json:"expires_at"`
	CreatedAt  string `json:"created_at"`
}

type createTeamRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type updateTeamRequest struct {
	Name string `json:"name"`
}

type createInvitationRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type updateMemberRoleRequest struct {
	Role string `json:"role"`
}

type acceptInvitationRequest struct {
	Token string `json:"token"`
}

func generateInvitationToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (h *TeamHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	var req createTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.Name == "" || req.Slug == "" {
		response.Error(w, r, apperr.Validation("name and slug are required", ""))
		return
	}
	if len(req.Name) > maxTeamNameLen {
		response.Error(w, r, apperr.Validation("name is too long", "name"))
		return
	}
	if len(req.Slug) < minTeamSlugLen || len(req.Slug) > maxTeamSlugLen {
		response.Error(w, r, apperr.Validation("slug must be 3-32 characters", "slug"))
		return
	}
	if !teamSlugRe.MatchString(req.Slug) {
		response.Error(w, r, apperr.Validation("slug must be lowercase alphanumerics with optional inner dashes", "slug"))
		return
	}

	teamID := idgen.New("team_")
	var name, slug, plan, ownerID string
	var createdAt, updatedAt time.Time

	// Wrap both INSERTs in a transaction — orphaned teams (no owner membership) must not persist.
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to begin transaction", err))
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	err = tx.QueryRow(r.Context(),
		`INSERT INTO teams (id, name, slug, plan, owner_id)
		 VALUES ($1, $2, $3, 'free', $4)
		 RETURNING id, name, slug, plan, owner_id, created_at, updated_at`,
		teamID, req.Name, req.Slug, userID,
	).Scan(&teamID, &name, &slug, &plan, &ownerID, &createdAt, &updatedAt)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			response.Error(w, r, apperr.Validation("slug already taken", ""))
			return
		}
		response.Error(w, r, apperr.Internal("failed to create team", err))
		return
	}

	memberID := idgen.New("tmb_")
	_, err = tx.Exec(r.Context(),
		`INSERT INTO team_memberships (id, team_id, user_id, role)
		 VALUES ($1, $2, $3, 'owner')`,
		memberID, teamID, userID,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to create membership", err))
		return
	}

	if err = tx.Commit(r.Context()); err != nil {
		response.Error(w, r, apperr.Internal("failed to commit team creation", err))
		return
	}

	response.JSON(w, r, http.StatusCreated, map[string]any{
		"team": teamResponse{
			ID:        teamID,
			Name:      name,
			Slug:      slug,
			Plan:      plan,
			OwnerID:   ownerID,
			CreatedAt: createdAt.UTC().Format(time.RFC3339),
			UpdatedAt: updatedAt.UTC().Format(time.RFC3339),
		},
	})
}

func (h *TeamHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT t.id, t.name, t.slug, t.plan, t.owner_id, t.created_at, t.updated_at
		 FROM teams t
		 JOIN team_memberships tm ON tm.team_id = t.id
		 WHERE tm.user_id = $1
		 ORDER BY t.created_at DESC`,
		userID,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list teams", err))
		return
	}
	defer rows.Close()

	items := make([]teamResponse, 0)
	for rows.Next() {
		var id, name, slug, plan, ownerID string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &name, &slug, &plan, &ownerID, &createdAt, &updatedAt); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan team", err))
			return
		}
		items = append(items, teamResponse{
			ID:        id,
			Name:      name,
			Slug:      slug,
			Plan:      plan,
			OwnerID:   ownerID,
			CreatedAt: createdAt.UTC().Format(time.RFC3339),
			UpdatedAt: updatedAt.UTC().Format(time.RFC3339),
		})
	}

	response.JSON(w, r, http.StatusOK, map[string]any{"teams": items})
}

func (h *TeamHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	teamID := chi.URLParam(r, "id")

	var isMember bool
	err := h.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM team_memberships WHERE team_id = $1 AND user_id = $2)`,
		teamID, userID,
	).Scan(&isMember)
	if err != nil || !isMember {
		response.Error(w, r, apperr.Forbidden("not a team member"))
		return
	}

	var id, name, slug, plan, ownerID string
	var createdAt, updatedAt time.Time
	err = h.pool.QueryRow(r.Context(),
		`SELECT id, name, slug, plan, owner_id, created_at, updated_at FROM teams WHERE id = $1`,
		teamID,
	).Scan(&id, &name, &slug, &plan, &ownerID, &createdAt, &updatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			response.Error(w, r, apperr.NotFound("team not found"))
			return
		}
		response.Error(w, r, apperr.Internal("failed to get team", err))
		return
	}

	members, err := h.listMembersInternal(r.Context(), teamID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list members", err))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]any{
		"team": teamResponse{
			ID:        id,
			Name:      name,
			Slug:      slug,
			Plan:      plan,
			OwnerID:   ownerID,
			CreatedAt: createdAt.UTC().Format(time.RFC3339),
			UpdatedAt: updatedAt.UTC().Format(time.RFC3339),
		},
		"members": members,
	})
}

func (h *TeamHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	teamID := chi.URLParam(r, "id")

	role, err := h.getMemberRole(r.Context(), teamID, userID)
	if err != nil || (role != "owner" && role != "admin") {
		response.Error(w, r, apperr.Forbidden("owner or admin required"))
		return
	}

	var req updateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.Name == "" {
		response.Error(w, r, apperr.Validation("name is required", ""))
		return
	}
	if len(req.Name) > maxTeamNameLen {
		response.Error(w, r, apperr.Validation("name is too long", "name"))
		return
	}

	var id, name, slug, plan, ownerID string
	var createdAt, updatedAt time.Time
	err = h.pool.QueryRow(r.Context(),
		`UPDATE teams SET name = $1, updated_at = NOW()
		 WHERE id = $2
		 RETURNING id, name, slug, plan, owner_id, created_at, updated_at`,
		req.Name, teamID,
	).Scan(&id, &name, &slug, &plan, &ownerID, &createdAt, &updatedAt)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to update team", err))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]any{
		"team": teamResponse{
			ID:        id,
			Name:      name,
			Slug:      slug,
			Plan:      plan,
			OwnerID:   ownerID,
			CreatedAt: createdAt.UTC().Format(time.RFC3339),
			UpdatedAt: updatedAt.UTC().Format(time.RFC3339),
		},
	})
}

func (h *TeamHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	teamID := chi.URLParam(r, "id")

	role, err := h.getMemberRole(r.Context(), teamID, userID)
	if err != nil || role != "owner" {
		response.Error(w, r, apperr.Forbidden("owner required"))
		return
	}

	// Single transaction — without this, a failure on the second/third DELETE
	// could leave orphaned memberships or invitations that point at a team
	// row that no longer exists.
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to begin transaction", err))
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	if _, err := tx.Exec(r.Context(), `DELETE FROM team_invitations WHERE team_id = $1`, teamID); err != nil {
		response.Error(w, r, apperr.Internal("failed to delete invitations", err))
		return
	}
	if _, err := tx.Exec(r.Context(), `DELETE FROM team_memberships WHERE team_id = $1`, teamID); err != nil {
		response.Error(w, r, apperr.Internal("failed to delete memberships", err))
		return
	}
	if _, err := tx.Exec(r.Context(), `DELETE FROM teams WHERE id = $1`, teamID); err != nil {
		response.Error(w, r, apperr.Internal("failed to delete team", err))
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		response.Error(w, r, apperr.Internal("failed to commit team deletion", err))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]string{"message": "team deleted"})
}

func (h *TeamHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	teamID := chi.URLParam(r, "id")

	var isMember bool
	err := h.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM team_memberships WHERE team_id = $1 AND user_id = $2)`,
		teamID, userID,
	).Scan(&isMember)
	if err != nil || !isMember {
		response.Error(w, r, apperr.Forbidden("not a team member"))
		return
	}

	members, err := h.listMembersInternal(r.Context(), teamID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list members", err))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]any{"members": members})
}

func (h *TeamHandler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	teamID := chi.URLParam(r, "id")
	targetUserID := chi.URLParam(r, "user_id")

	role, err := h.getMemberRole(r.Context(), teamID, userID)
	if err != nil || role != "owner" {
		response.Error(w, r, apperr.Forbidden("owner required"))
		return
	}

	var req updateMemberRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.Role != "owner" && req.Role != "admin" && req.Role != "member" {
		response.Error(w, r, apperr.Validation("role must be owner, admin, or member", ""))
		return
	}

	// Resolve the team's current owner so we can reject "owner downgrades
	// themselves" (which would lock the team out of admin-only ops) and so a
	// real transfer keeps teams.owner_id in sync with the membership table.
	var currentOwnerID string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT owner_id FROM teams WHERE id = $1`, teamID,
	).Scan(&currentOwnerID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(w, r, apperr.NotFound("team not found"))
			return
		}
		response.Error(w, r, apperr.Internal("failed to fetch team", err))
		return
	}

	// Block self-demotion. Without this an owner can hand themselves "member"
	// and the team has no one able to call owner-only endpoints afterwards.
	if targetUserID == currentOwnerID && req.Role != "owner" {
		response.Error(w, r, apperr.Validation("owner cannot demote themselves; transfer ownership first", "role"))
		return
	}

	// Promoting someone else to owner = ownership transfer. Do the membership
	// flip, the previous owner's demotion to admin, and the teams.owner_id
	// update inside one transaction so we never expose a state with two owners
	// or a team whose owner_id no longer matches a 'owner' membership row.
	if req.Role == "owner" && targetUserID != currentOwnerID {
		tx, err := h.pool.Begin(r.Context())
		if err != nil {
			response.Error(w, r, apperr.Internal("failed to begin transaction", err))
			return
		}
		defer func() { _ = tx.Rollback(r.Context()) }()

		tag, err := tx.Exec(r.Context(),
			`UPDATE team_memberships SET role = 'owner' WHERE team_id = $1 AND user_id = $2`,
			teamID, targetUserID,
		)
		if err != nil {
			response.Error(w, r, apperr.Internal("failed to promote new owner", err))
			return
		}
		if tag.RowsAffected() == 0 {
			response.Error(w, r, apperr.NotFound("member not found"))
			return
		}

		if _, err := tx.Exec(r.Context(),
			`UPDATE team_memberships SET role = 'admin' WHERE team_id = $1 AND user_id = $2`,
			teamID, currentOwnerID,
		); err != nil {
			response.Error(w, r, apperr.Internal("failed to demote previous owner", err))
			return
		}

		if _, err := tx.Exec(r.Context(),
			`UPDATE teams SET owner_id = $1, updated_at = NOW() WHERE id = $2`,
			targetUserID, teamID,
		); err != nil {
			response.Error(w, r, apperr.Internal("failed to update team owner", err))
			return
		}

		if err := tx.Commit(r.Context()); err != nil {
			response.Error(w, r, apperr.Internal("failed to commit ownership transfer", err))
			return
		}

		auditLog(r, "audit.team.ownership_transferred",
			"team_id", teamID,
			"from_user_id", currentOwnerID,
			"to_user_id", targetUserID,
		)
		response.JSON(w, r, http.StatusOK, map[string]string{"message": "ownership transferred"})
		return
	}

	tag, err := h.pool.Exec(r.Context(),
		`UPDATE team_memberships SET role = $1 WHERE team_id = $2 AND user_id = $3`,
		req.Role, teamID, targetUserID,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to update role", err))
		return
	}
	if tag.RowsAffected() == 0 {
		response.Error(w, r, apperr.NotFound("member not found"))
		return
	}

	auditLog(r, "audit.team.role_updated",
		"team_id", teamID,
		"target_user_id", targetUserID,
		"new_role", req.Role,
	)
	response.JSON(w, r, http.StatusOK, map[string]string{"message": "role updated"})
}

func (h *TeamHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	teamID := chi.URLParam(r, "id")
	targetUserID := chi.URLParam(r, "user_id")

	isSelf := userID == targetUserID
	if !isSelf {
		role, err := h.getMemberRole(r.Context(), teamID, userID)
		if err != nil || (role != "owner" && role != "admin") {
			response.Error(w, r, apperr.Forbidden("owner or admin required"))
			return
		}
	}

	// Owner cannot leave the team — transfer ownership first. The previous
	// version swallowed this query's error with `_ =`, which meant a missing
	// team row left ownerID="" and silently bypassed the owner guard. Treat
	// pgx.ErrNoRows as a real "team gone" 404 and any other error as 500 so
	// we never DELETE memberships against a phantom team.
	var ownerID string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT owner_id FROM teams WHERE id = $1`, teamID,
	).Scan(&ownerID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(w, r, apperr.NotFound("team not found"))
			return
		}
		response.Error(w, r, apperr.Internal("failed to fetch team", err))
		return
	}
	if targetUserID == ownerID {
		response.Error(w, r, apperr.Validation("owner cannot leave the team; transfer ownership first", ""))
		return
	}

	tag, err := h.pool.Exec(r.Context(),
		`DELETE FROM team_memberships WHERE team_id = $1 AND user_id = $2`,
		teamID, targetUserID,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to remove member", err))
		return
	}
	if tag.RowsAffected() == 0 {
		response.Error(w, r, apperr.NotFound("member not found"))
		return
	}

	auditLog(r, "audit.team.member_removed",
		"team_id", teamID,
		"target_user_id", targetUserID,
		"self_leave", isSelf,
	)
	response.JSON(w, r, http.StatusOK, map[string]string{"message": "member removed"})
}

func (h *TeamHandler) ListInvitations(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	teamID := chi.URLParam(r, "id")

	var isMember bool
	err := h.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM team_memberships WHERE team_id = $1 AND user_id = $2)`,
		teamID, userID,
	).Scan(&isMember)
	if err != nil || !isMember {
		response.Error(w, r, apperr.Forbidden("not a team member"))
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT id, team_id, email, role, invited_by, status, expires_at, created_at
		 FROM team_invitations WHERE team_id = $1 AND status = 'pending' ORDER BY created_at DESC`,
		teamID,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list invitations", err))
		return
	}
	defer rows.Close()

	items := make([]invitationResponse, 0)
	for rows.Next() {
		var id, tid, email, role, invitedBy, status string
		var expiresAt, createdAt time.Time
		if err := rows.Scan(&id, &tid, &email, &role, &invitedBy, &status, &expiresAt, &createdAt); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan invitation", err))
			return
		}
		items = append(items, invitationResponse{
			ID:        id,
			TeamID:    tid,
			Email:     email,
			Role:      role,
			InvitedBy: invitedBy,
			Status:    status,
			ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
			CreatedAt: createdAt.UTC().Format(time.RFC3339),
		})
	}

	response.JSON(w, r, http.StatusOK, map[string]any{"invitations": items})
}

func (h *TeamHandler) CreateInvitation(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	teamID := chi.URLParam(r, "id")

	role, err := h.getMemberRole(r.Context(), teamID, userID)
	if err != nil || (role != "owner" && role != "admin") {
		response.Error(w, r, apperr.Forbidden("owner or admin required"))
		return
	}

	var req createInvitationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.Email == "" {
		response.Error(w, r, apperr.Validation("email is required", ""))
		return
	}
	if len(req.Email) > maxInvitationEmailLen {
		response.Error(w, r, apperr.Validation("email is too long", "email"))
		return
	}
	// RFC 5322 / 6532 validation. mail.ParseAddress accepts the same syntax
	// our outbound mailer will eventually feed into SMTP, so anything it
	// rejects here would have failed delivery anyway — but unvalidated input
	// would still have been persisted and rendered in the team UI.
	if _, err := mail.ParseAddress(req.Email); err != nil {
		response.Error(w, r, apperr.Validation("email is not a valid address", "email"))
		return
	}
	invRole := req.Role
	if invRole == "" {
		invRole = "member"
	}
	if invRole != "admin" && invRole != "member" {
		// owner role is reserved for transfer-ownership flow, not invitations.
		response.Error(w, r, apperr.Validation("role must be admin or member", "role"))
		return
	}

	token, err := generateInvitationToken()
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to generate token", err))
		return
	}

	invID := idgen.New("tinv_")
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	var id, tid, email, iRole, invitedBy, status string
	var expAt, createdAt time.Time
	err = h.pool.QueryRow(r.Context(),
		`INSERT INTO team_invitations (id, team_id, email, role, token, invited_by, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, team_id, email, role, invited_by, status, expires_at, created_at`,
		invID, teamID, req.Email, invRole, token, userID, expiresAt,
	).Scan(&id, &tid, &email, &iRole, &invitedBy, &status, &expAt, &createdAt)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to create invitation", err))
		return
	}

	response.JSON(w, r, http.StatusCreated, map[string]any{
		"invitation": invitationResponse{
			ID:        id,
			TeamID:    tid,
			Email:     email,
			Role:      iRole,
			InvitedBy: invitedBy,
			Status:    status,
			ExpiresAt: expAt.UTC().Format(time.RFC3339),
			CreatedAt: createdAt.UTC().Format(time.RFC3339),
		},
	})
}

func (h *TeamHandler) RevokeInvitation(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	teamID := chi.URLParam(r, "id")
	invID := chi.URLParam(r, "inv_id")

	role, err := h.getMemberRole(r.Context(), teamID, userID)
	if err != nil || (role != "owner" && role != "admin") {
		response.Error(w, r, apperr.Forbidden("owner or admin required"))
		return
	}

	tag, err := h.pool.Exec(r.Context(),
		`UPDATE team_invitations SET status = 'expired' WHERE id = $1 AND team_id = $2`,
		invID, teamID,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to revoke invitation", err))
		return
	}
	if tag.RowsAffected() == 0 {
		response.Error(w, r, apperr.NotFound("invitation not found"))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]string{"message": "invitation revoked"})
}

func (h *TeamHandler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	var req acceptInvitationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.Token == "" {
		response.Error(w, r, apperr.Validation("token is required", ""))
		return
	}

	// Wrap UPDATE+INSERT in a single transaction. Previously the two ran on the
	// raw pool — if the INSERT failed (DB hiccup, FK violation, anything), the
	// UPDATE had already marked the invitation 'accepted' and could not be
	// retried, leaving the user permanently outside the team with a dead token.
	// With a transaction, INSERT failure rolls the UPDATE back so the user can
	// re-attempt or admin can revoke/reissue cleanly.
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to begin transaction", err))
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	// Atomically consume the invitation — prevents two concurrent accepts with the same token
	// AND eliminates the race where the expiry check happens after the row is marked accepted.
	var invID, teamID, role, invitedBy string
	err = tx.QueryRow(r.Context(),
		`UPDATE team_invitations
		 SET status = 'accepted'
		 WHERE token = $1 AND status = 'pending' AND expires_at > NOW()
		 RETURNING id, team_id, role, invited_by`,
		req.Token,
	).Scan(&invID, &teamID, &role, &invitedBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Could be: token unknown, already used, OR expired.
			// Probe the row to give a clearer error to the user.
			var exists bool
			_ = h.pool.QueryRow(r.Context(),
				`SELECT EXISTS (SELECT 1 FROM team_invitations WHERE token = $1 AND expires_at <= NOW())`,
				req.Token,
			).Scan(&exists)
			if exists {
				response.Error(w, r, apperr.Validation("invitation has expired", ""))
				return
			}
			response.Error(w, r, apperr.NotFound("invitation not found or already used"))
			return
		}
		response.Error(w, r, apperr.Internal("failed to accept invitation", err))
		return
	}

	memberID := idgen.New("tmb_")
	_, err = tx.Exec(r.Context(),
		`INSERT INTO team_memberships (id, team_id, user_id, role, invited_by)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (team_id, user_id) DO NOTHING`,
		memberID, teamID, userID, role, invitedBy,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to create membership", err))
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		response.Error(w, r, apperr.Internal("failed to commit invitation acceptance", err))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]string{"team_id": teamID, "role": role})
}

func (h *TeamHandler) getMemberRole(ctx context.Context, teamID, userID string) (string, error) {
	var role string
	err := h.pool.QueryRow(ctx,
		`SELECT role FROM team_memberships WHERE team_id = $1 AND user_id = $2`,
		teamID, userID,
	).Scan(&role)
	if err != nil {
		return "", err
	}
	return role, nil
}

func (h *TeamHandler) listMembersInternal(ctx context.Context, teamID string) ([]memberResponse, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT tm.id, tm.team_id, tm.user_id, u.email, tm.role, tm.invited_by, tm.joined_at
		 FROM team_memberships tm
		 JOIN "user" u ON u.id = tm.user_id
		 WHERE tm.team_id = $1 ORDER BY tm.joined_at ASC`,
		teamID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]memberResponse, 0)
	for rows.Next() {
		var id, tid, uid, email, role string
		var invitedBy *string
		var joinedAt time.Time
		if err := rows.Scan(&id, &tid, &uid, &email, &role, &invitedBy, &joinedAt); err != nil {
			return nil, err
		}
		members = append(members, memberResponse{
			ID:        id,
			TeamID:    tid,
			UserID:    uid,
			Email:     email,
			Role:      role,
			InvitedBy: invitedBy,
			JoinedAt:  joinedAt.UTC().Format(time.RFC3339),
		})
	}
	return members, nil
}
