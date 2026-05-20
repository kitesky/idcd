package oncall

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrScheduleNotFound is returned by CurrentOnCall when the schedule row is
// missing. Callers must check with errors.Is(err, oncall.ErrScheduleNotFound)
// to distinguish 404 from 500 — never compare error strings.
var ErrScheduleNotFound = errors.New("oncall schedule not found")

type QueryRow interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type Participant struct {
	ID         string
	ScheduleID string
	UserID     string
	OrderIndex int
}

type Schedule struct {
	ID           string
	RotationType string
	HandoffHour  int
}

func CurrentOnCall(ctx context.Context, db QueryRow, scheduleID string, at time.Time) (string, error) {
	var overrideUserID string
	err := db.QueryRow(ctx, `
		SELECT user_id FROM oncall_overrides
		WHERE schedule_id = $1 AND start_at <= $2 AND end_at > $2
		ORDER BY created_at DESC
		LIMIT 1
	`, scheduleID, at).Scan(&overrideUserID)
	if err == nil {
		return overrideUserID, nil
	}
	if err != pgx.ErrNoRows {
		return "", fmt.Errorf("query oncall_overrides: %w", err)
	}

	var s Schedule
	err = db.QueryRow(ctx, `
		SELECT id, rotation_type, handoff_hour FROM oncall_schedules WHERE id = $1
	`, scheduleID).Scan(&s.ID, &s.RotationType, &s.HandoffHour)
	if err == pgx.ErrNoRows {
		return "", ErrScheduleNotFound
	}
	if err != nil {
		return "", fmt.Errorf("query oncall_schedules: %w", err)
	}

	rows, err := db.Query(ctx, `
		SELECT id, schedule_id, user_id, order_index FROM oncall_participants
		WHERE schedule_id = $1
		ORDER BY order_index ASC
	`, scheduleID)
	if err != nil {
		return "", fmt.Errorf("query oncall_participants: %w", err)
	}
	defer rows.Close()

	var participants []Participant
	for rows.Next() {
		var p Participant
		if err := rows.Scan(&p.ID, &p.ScheduleID, &p.UserID, &p.OrderIndex); err != nil {
			return "", fmt.Errorf("scan oncall_participant: %w", err)
		}
		participants = append(participants, p)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate oncall_participants: %w", err)
	}

	if len(participants) == 0 {
		return "", nil
	}

	return nextRotationUser(participants, s.RotationType, s.HandoffHour, at), nil
}

// BatchCurrentOnCall returns the on-call user for each requested timestamp in
// a single round-trip's worth of DB work — one schedule lookup, one
// participants scan, one overrides scan covering the full time range.
//
// Callers (notably handler.GetSchedule's 7-day preview) used to call
// CurrentOnCall once per day, which fanned out to ~3 queries per day, i.e.
// ~21 round trips just to render the sidebar. With this helper the same render
// costs three queries total. Returns one user_id per `ats` entry in the same
// order (empty string when no participants are configured for that day).
func BatchCurrentOnCall(ctx context.Context, db QueryRow, scheduleID string, ats []time.Time) ([]string, error) {
	if len(ats) == 0 {
		return nil, nil
	}

	var s Schedule
	if err := db.QueryRow(ctx, `
		SELECT id, rotation_type, handoff_hour FROM oncall_schedules WHERE id = $1
	`, scheduleID).Scan(&s.ID, &s.RotationType, &s.HandoffHour); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrScheduleNotFound
		}
		return nil, fmt.Errorf("query oncall_schedules: %w", err)
	}

	// Compute the [min, max] timestamp window so the override scan only loads
	// rows that could overlap any requested timestamp. Inclusive on min,
	// exclusive on max — matches the per-day comparison below.
	minAt, maxAt := ats[0], ats[0]
	for _, t := range ats[1:] {
		if t.Before(minAt) {
			minAt = t
		}
		if t.After(maxAt) {
			maxAt = t
		}
	}

	type overrideRow struct {
		userID    string
		startAt   time.Time
		endAt     time.Time
		createdAt time.Time
	}
	rows, err := db.Query(ctx, `
		SELECT user_id, start_at, end_at, created_at FROM oncall_overrides
		WHERE schedule_id = $1 AND end_at > $2 AND start_at <= $3
		ORDER BY created_at DESC
	`, scheduleID, minAt, maxAt)
	if err != nil {
		return nil, fmt.Errorf("query oncall_overrides batch: %w", err)
	}
	defer rows.Close()
	var overrides []overrideRow
	for rows.Next() {
		var o overrideRow
		if err := rows.Scan(&o.userID, &o.startAt, &o.endAt, &o.createdAt); err != nil {
			return nil, fmt.Errorf("scan oncall_override: %w", err)
		}
		overrides = append(overrides, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate oncall_overrides batch: %w", err)
	}

	pRows, err := db.Query(ctx, `
		SELECT id, schedule_id, user_id, order_index FROM oncall_participants
		WHERE schedule_id = $1
		ORDER BY order_index ASC
	`, scheduleID)
	if err != nil {
		return nil, fmt.Errorf("query oncall_participants batch: %w", err)
	}
	defer pRows.Close()
	var participants []Participant
	for pRows.Next() {
		var p Participant
		if err := pRows.Scan(&p.ID, &p.ScheduleID, &p.UserID, &p.OrderIndex); err != nil {
			return nil, fmt.Errorf("scan oncall_participant batch: %w", err)
		}
		participants = append(participants, p)
	}
	if err := pRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate oncall_participants batch: %w", err)
	}

	out := make([]string, len(ats))
	for i, at := range ats {
		// Override wins. Overrides are sorted by created_at DESC, so the first
		// match is the most recently created — matches CurrentOnCall's
		// single-row LIMIT 1 ORDER BY created_at DESC semantics.
		matched := false
		for _, o := range overrides {
			if !o.startAt.After(at) && o.endAt.After(at) {
				out[i] = o.userID
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		if len(participants) == 0 {
			out[i] = ""
			continue
		}
		out[i] = nextRotationUser(participants, s.RotationType, s.HandoffHour, at)
	}
	return out, nil
}

var epoch = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

func nextRotationUser(participants []Participant, rotationType string, handoffHour int, at time.Time) string {
	if len(participants) == 0 {
		return ""
	}

	atUTC := at.UTC()
	handoffBase := time.Date(epoch.Year(), epoch.Month(), epoch.Day(), handoffHour, 0, 0, 0, time.UTC)

	var interval time.Duration
	switch rotationType {
	case "daily":
		interval = 24 * time.Hour
	case "weekly":
		interval = 7 * 24 * time.Hour
	default:
		interval = 7 * 24 * time.Hour
	}

	elapsed := atUTC.Sub(handoffBase)
	if elapsed < 0 {
		elapsed = 0
	}

	slotIndex := int(elapsed / interval)
	idx := slotIndex % len(participants)
	return participants[idx].UserID
}
