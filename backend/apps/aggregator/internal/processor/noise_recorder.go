package processor

import (
	"context"
	"fmt"
	"time"

	"github.com/kite365/idcd/lib/shared/idgen"
)

// ExecQuerier is satisfied by pgxpool.Pool and pgxmock.
type ExecQuerier interface {
	pgxQuerier
}

// RecordNoise updates daily noise stats after each alert event state change.
// eventType must be "fire" or "resolve".
// fireTime is the time the alert started firing (used for flap detection on resolve).
func RecordNoise(ctx context.Context, db ExecQuerier, userID, monitorID, eventType string, fireTime time.Time) error {
	today := time.Now().UTC().Format("2006-01-02")
	id := idgen.New("ans_")

	switch eventType {
	case "fire":
		_, err := db.Exec(ctx, `
			INSERT INTO alert_noise_stats (id, user_id, date, monitor_id, total_firings)
			VALUES ($1, $2, $3, $4, 1)
			ON CONFLICT (user_id, date, monitor_id) DO UPDATE
			SET total_firings = alert_noise_stats.total_firings + 1
		`, id, userID, today, monitorID)
		if err != nil {
			return fmt.Errorf("record noise fire: %w", err)
		}

	case "resolve":
		isFlap := time.Since(fireTime) < 5*time.Minute
		if isFlap {
			_, err := db.Exec(ctx, `
				INSERT INTO alert_noise_stats (id, user_id, date, monitor_id, total_resolved, flap_count)
				VALUES ($1, $2, $3, $4, 1, 1)
				ON CONFLICT (user_id, date, monitor_id) DO UPDATE
				SET total_resolved = alert_noise_stats.total_resolved + 1,
				    flap_count     = alert_noise_stats.flap_count + 1
			`, id, userID, today, monitorID)
			if err != nil {
				return fmt.Errorf("record noise resolve flap: %w", err)
			}
		} else {
			_, err := db.Exec(ctx, `
				INSERT INTO alert_noise_stats (id, user_id, date, monitor_id, total_resolved)
				VALUES ($1, $2, $3, $4, 1)
				ON CONFLICT (user_id, date, monitor_id) DO UPDATE
				SET total_resolved = alert_noise_stats.total_resolved + 1
			`, id, userID, today, monitorID)
			if err != nil {
				return fmt.Errorf("record noise resolve: %w", err)
			}
		}

	default:
		return fmt.Errorf("unknown eventType: %s", eventType)
	}

	return nil
}
