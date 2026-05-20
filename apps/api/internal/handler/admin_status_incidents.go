package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// AdminStatusIncidentsHandler powers the small admin CRUD used by operators
// to log incidents that should appear on idcd.com/status. Public read uses
// PublicStatusHandler.Incidents instead — this handler is admin-only and lives
// behind AdminAuthMiddleware.
//
// We don't bother with optimistic-locking / soft delete / audit log here —
// status_incidents has at most a few dozen rows per year and the audit story
// for the admin panel is already covered by middleware request logs.
type AdminStatusIncidentsHandler struct {
	pool *pgxpool.Pool
}

// NewAdminStatusIncidentsHandler wires the handler to the main pool.
func NewAdminStatusIncidentsHandler(pool *pgxpool.Pool) *AdminStatusIncidentsHandler {
	return &AdminStatusIncidentsHandler{pool: pool}
}

// adminIncident is the JSON shape used by both list and detail endpoints
// (and the request body of POST/PATCH — fields not in the body are left
// unchanged on PATCH).
type adminIncident struct {
	ID         int64      `json:"id,omitempty"`
	ServiceKey string     `json:"service_key,omitempty"`
	StartedAt  time.Time  `json:"started_at"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
	Severity   string     `json:"severity"`
	Title      string     `json:"title"`
	Summary    string     `json:"summary,omitempty"`
	Related    []string   `json:"related,omitempty"`
	CreatedAt  time.Time  `json:"created_at,omitempty"`
	UpdatedAt  time.Time  `json:"updated_at,omitempty"`
}

var allowedSeverity = map[string]bool{
	"degradation":    true,
	"partial_outage": true,
	"outage":         true,
	"maintenance":    true,
}

// List handles GET /admin/status-incidents. Returns most-recent-first,
// no pagination — staying intentionally simple because we don't expect
// more than a hundred entries.
func (h *AdminStatusIncidentsHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, service_key, started_at, ended_at, severity, title,
		       COALESCE(summary, ''), COALESCE(related, ARRAY[]::TEXT[]),
		       created_at, updated_at
		  FROM status_incidents
		 ORDER BY started_at DESC
		 LIMIT 500
	`)
	if err != nil {
		response.Error(w, r, apperr.Internal("list incidents", err))
		return
	}
	defer rows.Close()

	out := []adminIncident{}
	for rows.Next() {
		var inc adminIncident
		if err := rows.Scan(
			&inc.ID, &inc.ServiceKey, &inc.StartedAt, &inc.EndedAt,
			&inc.Severity, &inc.Title, &inc.Summary, &inc.Related,
			&inc.CreatedAt, &inc.UpdatedAt,
		); err != nil {
			response.Error(w, r, apperr.Internal("scan incident", err))
			return
		}
		out = append(out, inc)
	}
	response.JSON(w, r, http.StatusOK, map[string]any{"incidents": out})
}

// Create handles POST /admin/status-incidents. Returns the freshly
// inserted row including the generated id.
func (h *AdminStatusIncidentsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body adminIncident
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, r, apperr.Validationf("invalid JSON body"))
		return
	}
	if err := validateIncident(&body); err != nil {
		response.Error(w, r, err)
		return
	}

	row := h.pool.QueryRow(r.Context(), `
		INSERT INTO status_incidents
		  (service_key, started_at, ended_at, severity, title, summary, related)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7)
		RETURNING id, created_at, updated_at
	`, body.ServiceKey, body.StartedAt, body.EndedAt, body.Severity,
		body.Title, body.Summary, body.Related)

	if err := row.Scan(&body.ID, &body.CreatedAt, &body.UpdatedAt); err != nil {
		response.Error(w, r, apperr.Internal("insert incident", err))
		return
	}
	response.JSON(w, r, http.StatusCreated, body)
}

// Update handles PATCH /admin/status-incidents/{id}. Only fields present in
// the request body are touched — null EndedAt explicitly resolves to NULL
// in DB (i.e. clearing a previously-set end time is supported).
func (h *AdminStatusIncidentsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.Error(w, r, apperr.Validationf("invalid id"))
		return
	}

	// Decode into a map first so we can tell apart "field absent" from
	// "field explicitly null". The strongly-typed struct loses that
	// distinction for *time.Time which we actually care about (clearing
	// ended_at to mark an incident reopened).
	raw := map[string]json.RawMessage{}
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		response.Error(w, r, apperr.Validationf("invalid JSON body"))
		return
	}

	sets := []string{"updated_at = NOW()"}
	args := []any{}
	idx := 1

	// Track parsed values so we can run the same cross-field invariant
	// (ended_at >= started_at) before the DB CHECK fires it back as a 500.
	var (
		newStartedAt *time.Time
		newEndedAt   *time.Time
	)

	addStr := func(field, jsonKey string) error {
		v, ok := raw[jsonKey]
		if !ok {
			return nil
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return apperr.Validationf("%s must be string", jsonKey)
		}
		sets = append(sets, field+" = $"+strconv.Itoa(idx))
		args = append(args, s)
		idx++
		return nil
	}
	addStrArr := func(field, jsonKey string) error {
		v, ok := raw[jsonKey]
		if !ok {
			return nil
		}
		var arr []string
		if err := json.Unmarshal(v, &arr); err != nil {
			return apperr.Validationf("%s must be string array", jsonKey)
		}
		sets = append(sets, field+" = $"+strconv.Itoa(idx))
		args = append(args, arr)
		idx++
		return nil
	}

	if err := addStr("service_key", "service_key"); err != nil {
		response.Error(w, r, err)
		return
	}
	if v, ok := raw["started_at"]; ok {
		var t time.Time
		if err := json.Unmarshal(v, &t); err != nil {
			response.Error(w, r, apperr.Validationf("started_at must be RFC3339 timestamp"))
			return
		}
		sets = append(sets, "started_at = $"+strconv.Itoa(idx))
		args = append(args, t)
		idx++
		newStartedAt = &t
	}
	if v, ok := raw["ended_at"]; ok {
		if string(v) == "null" {
			sets = append(sets, "ended_at = NULL")
		} else {
			var t time.Time
			if err := json.Unmarshal(v, &t); err != nil {
				response.Error(w, r, apperr.Validationf("ended_at must be RFC3339 timestamp or null"))
				return
			}
			sets = append(sets, "ended_at = $"+strconv.Itoa(idx))
			args = append(args, t)
			idx++
			newEndedAt = &t
		}
	}
	if v, ok := raw["severity"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			response.Error(w, r, apperr.Validationf("severity must be string"))
			return
		}
		if !allowedSeverity[s] {
			response.Error(w, r, apperr.Validationf("severity must be one of degradation|partial_outage|outage|maintenance"))
			return
		}
		sets = append(sets, "severity = $"+strconv.Itoa(idx))
		args = append(args, s)
		idx++
	}
	if err := addStr("title", "title"); err != nil {
		response.Error(w, r, err)
		return
	}
	if err := addStr("summary", "summary"); err != nil {
		response.Error(w, r, err)
		return
	}
	if err := addStrArr("related", "related"); err != nil {
		response.Error(w, r, err)
		return
	}

	if len(sets) == 1 {
		// Only updated_at would change → nothing to do.
		response.JSON(w, r, http.StatusOK, map[string]any{"ok": true, "noop": true})
		return
	}

	// Cross-field check: if both started_at and ended_at are being set in this
	// PATCH, enforce ended_at >= started_at as a 400 instead of leaving it to
	// the status_incidents_time_check CHECK constraint (which surfaces as 500).
	// When only one side is being touched, we don't read the other from DB —
	// the CHECK still backs us up, just less nicely.
	if newStartedAt != nil && newEndedAt != nil && newEndedAt.Before(*newStartedAt) {
		response.Error(w, r, apperr.Validationf("ended_at must be at or after started_at"))
		return
	}

	args = append(args, id)
	query := "UPDATE status_incidents SET " + strings.Join(sets, ", ") +
		" WHERE id = $" + strconv.Itoa(idx) + " RETURNING id"

	var returnedID int64
	if err := h.pool.QueryRow(r.Context(), query, args...).Scan(&returnedID); err != nil {
		if err == pgx.ErrNoRows {
			response.Error(w, r, apperr.NotFound("incident not found"))
			return
		}
		response.Error(w, r, apperr.Internal("update incident", err))
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]any{"ok": true, "id": returnedID})
}

// Delete handles DELETE /admin/status-incidents/{id}. Hard delete — there is
// no "trash" concept for the dozen entries we expect over a year. If you
// need to recover one, restore from DB snapshot.
func (h *AdminStatusIncidentsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.Error(w, r, apperr.Validationf("invalid id"))
		return
	}
	tag, err := h.pool.Exec(r.Context(), `DELETE FROM status_incidents WHERE id = $1`, id)
	if err != nil {
		response.Error(w, r, apperr.Internal("delete incident", err))
		return
	}
	if tag.RowsAffected() == 0 {
		response.Error(w, r, apperr.NotFound("incident not found"))
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]any{"ok": true})
}

// validateIncident enforces the not-null + enum constraints we want surfaced
// as 400s before they hit the DB CHECK constraints.
func validateIncident(b *adminIncident) error {
	if strings.TrimSpace(b.ServiceKey) == "" {
		return apperr.Validationf("service_key is required")
	}
	if strings.TrimSpace(b.Title) == "" {
		return apperr.Validationf("title is required")
	}
	if b.StartedAt.IsZero() {
		return apperr.Validationf("started_at is required")
	}
	if !allowedSeverity[b.Severity] {
		return apperr.Validationf("severity must be one of degradation|partial_outage|outage|maintenance")
	}
	if b.EndedAt != nil && b.EndedAt.Before(b.StartedAt) {
		return apperr.Validationf("ended_at must be after started_at")
	}
	return nil
}
