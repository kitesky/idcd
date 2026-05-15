package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
)

type statusPageUserPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type StatusPageUserHandler struct {
	pool statusPageUserPool
}

func NewStatusPageUserHandler(pool statusPageUserPool) *StatusPageUserHandler {
	return &StatusPageUserHandler{pool: pool}
}

type statusPageItem struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	IsPublic      bool   `json:"is_public"`
	OverallStatus string `json:"overall_status"`
	CreatedAt     string `json:"created_at"`
}

type createStatusPageReq struct {
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	IsPublic bool   `json:"is_public"`
}

func (h *StatusPageUserHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	rows, err := h.pool.Query(ctx,
		`SELECT id, name, slug, created_at FROM status_pages WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list status pages", err))
		return
	}
	defer rows.Close()

	items := []statusPageItem{}
	for rows.Next() {
		var item statusPageItem
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.Name, &item.Slug, &createdAt); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan status page", err))
			return
		}
		item.IsPublic = true
		item.OverallStatus = "operational"
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to iterate status pages", err))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]any{"status_pages": items})
}

func (h *StatusPageUserHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	var req createStatusPageReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", ""))
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Slug = strings.TrimSpace(req.Slug)
	if req.Name == "" || req.Slug == "" {
		response.Error(w, r, apperr.Validation("name and slug are required", ""))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	id := idgen.New("sp_")
	var createdAt time.Time
	err := h.pool.QueryRow(ctx,
		`INSERT INTO status_pages (id, user_id, slug, name, branding)
		 VALUES ($1, $2, $3, $4, TRUE)
		 RETURNING created_at`,
		id, userID, req.Slug, req.Name,
	).Scan(&createdAt)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			response.Error(w, r, apperr.Conflict("slug already taken"))
			return
		}
		response.Error(w, r, apperr.Internal("failed to create status page", err))
		return
	}

	item := statusPageItem{
		ID:            id,
		Name:          req.Name,
		Slug:          req.Slug,
		IsPublic:      true,
		OverallStatus: "operational",
		CreatedAt:     createdAt.UTC().Format(time.RFC3339),
	}
	response.JSON(w, r, http.StatusCreated, map[string]any{"status_page": item})
}

func (h *StatusPageUserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, r, apperr.Validation("missing id", ""))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	tag, err := h.pool.Exec(ctx,
		`DELETE FROM status_pages WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to delete status page", err))
		return
	}
	if tag.RowsAffected() == 0 {
		response.Error(w, r, apperr.NotFound("status page not found"))
		return
	}

	response.JSON(w, r, http.StatusNoContent, nil)
}

