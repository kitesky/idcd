package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kite365/idcd/lib/shared/idgen"
)

const (
	defaultConsecutiveFailures = 3
	typeAlertNotification      = "alert:notification"
	alertQueue                 = "notifier:default"
)

type pgxQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

type poolQuerier struct {
	pool *pgxpool.Pool
}

func (p *poolQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return p.pool.Query(ctx, sql, args...)
}

func (p *poolQuerier) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return p.pool.QueryRow(ctx, sql, args...)
}

func (p *poolQuerier) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return p.pool.Exec(ctx, sql, arguments...)
}

type alertPolicy struct {
	id         string
	channelIDs []string
	recoveryN  int
	enabled    bool
}

type alertChannel struct {
	id          string
	channelType string
	config      []byte
}

type alertEvent struct {
	id        string
	monitorID string
	policyID  string
	status    string
	startedAt time.Time
}

type monitorCheckRow struct {
	status string
}

type alertNotificationPayload struct {
	ChannelType   string `json:"channel_type"`
	ChannelConfig []byte `json:"channel_config"`
	Title         string `json:"title"`
	Body          string `json:"body"`
	URL           string `json:"url"`
	Level         string `json:"level"`
}

type NotificationEnqueuer interface {
	Enqueue(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

type AlertTrigger struct {
	db       pgxQuerier
	enqueuer NotificationEnqueuer
	logger   *slog.Logger
}

func NewAlertTrigger(pool *pgxpool.Pool, enqueuer NotificationEnqueuer, logger *slog.Logger) *AlertTrigger {
	if pool == nil {
		return &AlertTrigger{enqueuer: enqueuer, logger: logger}
	}
	return &AlertTrigger{db: &poolQuerier{pool: pool}, enqueuer: enqueuer, logger: logger}
}

func newAlertTriggerWithQuerier(db pgxQuerier, enqueuer NotificationEnqueuer, logger *slog.Logger) *AlertTrigger {
	return &AlertTrigger{db: db, enqueuer: enqueuer, logger: logger}
}

func (t *AlertTrigger) CheckAndTrigger(ctx context.Context, monitorID string, checkStatus string) {
	if t.db == nil {
		return
	}

	policies, err := t.getAlertPolicies(ctx, monitorID)
	if err != nil {
		t.logger.Error("alert_trigger: get policies", "monitor_id", monitorID, "err", err)
		return
	}

	for _, policy := range policies {
		t.processPolicy(ctx, monitorID, checkStatus, policy)
	}
}

func (t *AlertTrigger) processPolicy(ctx context.Context, monitorID, checkStatus string, policy alertPolicy) {
	n := policy.recoveryN
	if n <= 0 {
		n = defaultConsecutiveFailures
	}

	checks, err := t.getRecentChecks(ctx, monitorID, n)
	if err != nil {
		t.logger.Error("alert_trigger: get recent checks", "monitor_id", monitorID, "err", err)
		return
	}

	firingEvent, err := t.getFiringEvent(ctx, monitorID, policy.id)
	if err != nil {
		t.logger.Error("alert_trigger: get firing event", "monitor_id", monitorID, "err", err)
		return
	}

	if checkStatus == "up" {
		if firingEvent != nil {
			if err := t.resolveEvent(ctx, firingEvent.id, monitorID, policy); err != nil {
				t.logger.Error("alert_trigger: resolve event", "event_id", firingEvent.id, "err", err)
			}
		}
		return
	}

	if len(checks) < n {
		return
	}

	allFailed := true
	for _, c := range checks {
		if c.status == "up" {
			allFailed = false
			break
		}
	}
	if !allFailed {
		return
	}

	if firingEvent != nil {
		return
	}

	event, err := t.insertAlertEvent(ctx, monitorID, policy.id)
	if err != nil {
		t.logger.Error("alert_trigger: insert alert event", "monitor_id", monitorID, "err", err)
		return
	}

	t.sendNotifications(ctx, event.id, monitorID, "firing", policy)
}

func (t *AlertTrigger) resolveEvent(ctx context.Context, eventID, monitorID string, policy alertPolicy) error {
	now := time.Now().UTC()
	_, err := t.db.Exec(ctx, `
		UPDATE alert_events
		SET status = 'resolved', resolved_at = $1
		WHERE id = $2 AND status = 'firing'
	`, now, eventID)
	if err != nil {
		return fmt.Errorf("resolve alert event %s: %w", eventID, err)
	}
	t.sendNotifications(ctx, eventID, monitorID, "resolved", policy)
	return nil
}

func (t *AlertTrigger) sendNotifications(ctx context.Context, eventID, monitorID, status string, policy alertPolicy) {
	for _, chID := range policy.channelIDs {
		ch, err := t.getChannel(ctx, chID)
		if err != nil {
			t.logger.Error("alert_trigger: get channel", "channel_id", chID, "err", err)
			continue
		}

		title, body, level := buildNotificationText(monitorID, status)
		p := alertNotificationPayload{
			ChannelType:   ch.channelType,
			ChannelConfig: ch.config,
			Title:         title,
			Body:          body,
			Level:         level,
		}

		payload, err := json.Marshal(p)
		if err != nil {
			t.logger.Error("alert_trigger: marshal notification payload", "err", err)
			continue
		}

		if t.enqueuer == nil {
			continue
		}

		task := asynq.NewTask(typeAlertNotification, payload)
		if _, err := t.enqueuer.Enqueue(ctx, task, asynq.Queue(alertQueue)); err != nil {
			t.logger.Error("alert_trigger: enqueue notification", "event_id", eventID, "channel_id", chID, "err", err)
		}
	}
}

func buildNotificationText(monitorID, status string) (title, body, level string) {
	switch status {
	case "firing":
		return fmt.Sprintf("监控告警: %s", monitorID),
			fmt.Sprintf("监控 %s 检测到连续失败，已触发告警。", monitorID),
			"critical"
	case "resolved":
		return fmt.Sprintf("告警恢复: %s", monitorID),
			fmt.Sprintf("监控 %s 已恢复正常。", monitorID),
			"info"
	default:
		return fmt.Sprintf("告警通知: %s", monitorID),
			fmt.Sprintf("监控 %s 状态变更: %s", monitorID, status),
			"warning"
	}
}

func (t *AlertTrigger) getAlertPolicies(ctx context.Context, monitorID string) ([]alertPolicy, error) {
	rows, err := t.db.Query(ctx, `
		SELECT id, channel_ids, recovery_n, enabled
		FROM alert_policies
		WHERE monitor_id = $1 AND enabled = true
	`, monitorID)
	if err != nil {
		return nil, fmt.Errorf("query alert_policies: %w", err)
	}
	defer rows.Close()

	var policies []alertPolicy
	for rows.Next() {
		var p alertPolicy
		if err := rows.Scan(&p.id, &p.channelIDs, &p.recoveryN, &p.enabled); err != nil {
			return nil, fmt.Errorf("scan alert_policy: %w", err)
		}
		policies = append(policies, p)
	}
	return policies, rows.Err()
}

func (t *AlertTrigger) getRecentChecks(ctx context.Context, monitorID string, limit int) ([]monitorCheckRow, error) {
	rows, err := t.db.Query(ctx, `
		SELECT status
		FROM monitor_checks
		WHERE monitor_id = $1
		ORDER BY check_at DESC
		LIMIT $2
	`, monitorID, limit)
	if err != nil {
		return nil, fmt.Errorf("query monitor_checks: %w", err)
	}
	defer rows.Close()

	var checks []monitorCheckRow
	for rows.Next() {
		var c monitorCheckRow
		if err := rows.Scan(&c.status); err != nil {
			return nil, fmt.Errorf("scan monitor_check: %w", err)
		}
		checks = append(checks, c)
	}
	return checks, rows.Err()
}

func (t *AlertTrigger) getFiringEvent(ctx context.Context, monitorID, policyID string) (*alertEvent, error) {
	var e alertEvent
	err := t.db.QueryRow(ctx, `
		SELECT id, monitor_id, policy_id, status, started_at
		FROM alert_events
		WHERE monitor_id = $1 AND policy_id = $2 AND status = 'firing'
		LIMIT 1
	`, monitorID, policyID).Scan(&e.id, &e.monitorID, &e.policyID, &e.status, &e.startedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query firing alert_event: %w", err)
	}
	return &e, nil
}

func (t *AlertTrigger) insertAlertEvent(ctx context.Context, monitorID, policyID string) (*alertEvent, error) {
	id := idgen.AlertEvent()
	now := time.Now().UTC()
	_, err := t.db.Exec(ctx, `
		INSERT INTO alert_events (id, monitor_id, policy_id, status, started_at)
		VALUES ($1, $2, $3, 'firing', $4)
	`, id, monitorID, policyID, now)
	if err != nil {
		return nil, fmt.Errorf("insert alert_event: %w", err)
	}
	return &alertEvent{id: id, monitorID: monitorID, policyID: policyID, status: "firing", startedAt: now}, nil
}

func (t *AlertTrigger) getChannel(ctx context.Context, channelID string) (*alertChannel, error) {
	var ch alertChannel
	err := t.db.QueryRow(ctx, `
		SELECT id, type, config
		FROM alert_channels
		WHERE id = $1
	`, channelID).Scan(&ch.id, &ch.channelType, &ch.config)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}
	if err != nil {
		return nil, fmt.Errorf("query alert_channel: %w", err)
	}
	return &ch, nil
}
