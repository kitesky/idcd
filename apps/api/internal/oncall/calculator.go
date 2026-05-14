package oncall

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

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
		return "", fmt.Errorf("schedule not found: %s", scheduleID)
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
