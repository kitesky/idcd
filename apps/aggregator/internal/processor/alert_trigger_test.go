package processor

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

type mockEnqueuer struct {
	calls []*asynq.Task
	err   error
}

func (m *mockEnqueuer) Enqueue(_ context.Context, task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	m.calls = append(m.calls, task)
	return nil, m.err
}

func newTestTrigger(t *testing.T) (pgxmock.PgxPoolIface, *mockEnqueuer, *AlertTrigger) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool: %v", err)
	}
	enq := &mockEnqueuer{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	trigger := newAlertTriggerWithQuerier(mock, enq, logger)
	return mock, enq, trigger
}

func TestAlertTrigger_ThreeConsecutiveFailures_CreatesFiringEvent(t *testing.T) {
	mock, enq, trigger := newTestTrigger(t)
	ctx := context.Background()
	monitorID := "m_test001"
	policyID := "pol_test001"
	channelID := "ch_test001"

	mock.ExpectQuery("SELECT id, channel_ids, recovery_n, enabled FROM alert_policies").
		WithArgs(monitorID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "channel_ids", "recovery_n", "enabled"}).
			AddRow(policyID, []string{channelID}, 3, true))

	mock.ExpectQuery("SELECT status FROM monitor_checks").
		WithArgs(monitorID, 3).
		WillReturnRows(pgxmock.NewRows([]string{"status"}).
			AddRow("down").
			AddRow("down").
			AddRow("down"))

	mock.ExpectQuery("SELECT id, monitor_id, policy_id, status, started_at FROM alert_events").
		WithArgs(monitorID, policyID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "monitor_id", "policy_id", "status", "started_at"}))

	mock.ExpectExec("INSERT INTO alert_events").
		WithArgs(pgxmock.AnyArg(), monitorID, policyID, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mock.ExpectQuery("SELECT id, type, config FROM alert_channels").
		WithArgs(channelID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "type", "config"}).
			AddRow(channelID, "webhook", []byte(`{"url":"https://example.com"}`)))

	trigger.CheckAndTrigger(ctx, monitorID, "down")

	if len(enq.calls) != 1 {
		t.Errorf("expected 1 notification enqueued, got %d", len(enq.calls))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestAlertTrigger_LessThanNFailures_NoEvent(t *testing.T) {
	mock, enq, trigger := newTestTrigger(t)
	ctx := context.Background()
	monitorID := "m_test002"
	policyID := "pol_test002"
	channelID := "ch_test002"

	mock.ExpectQuery("SELECT id, channel_ids, recovery_n, enabled FROM alert_policies").
		WithArgs(monitorID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "channel_ids", "recovery_n", "enabled"}).
			AddRow(policyID, []string{channelID}, 3, true))

	mock.ExpectQuery("SELECT status FROM monitor_checks").
		WithArgs(monitorID, 3).
		WillReturnRows(pgxmock.NewRows([]string{"status"}).
			AddRow("down").
			AddRow("down"))

	mock.ExpectQuery("SELECT id, monitor_id, policy_id, status, started_at FROM alert_events").
		WithArgs(monitorID, policyID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "monitor_id", "policy_id", "status", "started_at"}))

	trigger.CheckAndTrigger(ctx, monitorID, "down")

	if len(enq.calls) != 0 {
		t.Errorf("expected 0 notifications, got %d", len(enq.calls))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestAlertTrigger_Recovery_ResolvesEvent(t *testing.T) {
	mock, enq, trigger := newTestTrigger(t)
	ctx := context.Background()
	monitorID := "m_test003"
	policyID := "pol_test003"
	channelID := "ch_test003"
	eventID := "ae_existing001"
	startedAt := time.Now().Add(-5 * time.Minute)

	mock.ExpectQuery("SELECT id, channel_ids, recovery_n, enabled FROM alert_policies").
		WithArgs(monitorID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "channel_ids", "recovery_n", "enabled"}).
			AddRow(policyID, []string{channelID}, 3, true))

	mock.ExpectQuery("SELECT status FROM monitor_checks").
		WithArgs(monitorID, 3).
		WillReturnRows(pgxmock.NewRows([]string{"status"}).
			AddRow("up").
			AddRow("up").
			AddRow("up"))

	mock.ExpectQuery("SELECT id, monitor_id, policy_id, status, started_at FROM alert_events").
		WithArgs(monitorID, policyID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "monitor_id", "policy_id", "status", "started_at"}).
			AddRow(eventID, monitorID, policyID, "firing", startedAt))

	mock.ExpectExec("UPDATE alert_events").
		WithArgs(pgxmock.AnyArg(), eventID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mock.ExpectQuery("SELECT id, type, config FROM alert_channels").
		WithArgs(channelID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "type", "config"}).
			AddRow(channelID, "webhook", []byte(`{"url":"https://example.com"}`)))

	trigger.CheckAndTrigger(ctx, monitorID, "up")

	if len(enq.calls) != 1 {
		t.Errorf("expected 1 resolved notification, got %d", len(enq.calls))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestAlertTrigger_Idempotent_NoSecondFiringEvent(t *testing.T) {
	mock, enq, trigger := newTestTrigger(t)
	ctx := context.Background()
	monitorID := "m_test004"
	policyID := "pol_test004"
	channelID := "ch_test004"
	eventID := "ae_existing002"
	startedAt := time.Now().Add(-10 * time.Minute)

	mock.ExpectQuery("SELECT id, channel_ids, recovery_n, enabled FROM alert_policies").
		WithArgs(monitorID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "channel_ids", "recovery_n", "enabled"}).
			AddRow(policyID, []string{channelID}, 3, true))

	mock.ExpectQuery("SELECT status FROM monitor_checks").
		WithArgs(monitorID, 3).
		WillReturnRows(pgxmock.NewRows([]string{"status"}).
			AddRow("down").
			AddRow("down").
			AddRow("down"))

	mock.ExpectQuery("SELECT id, monitor_id, policy_id, status, started_at FROM alert_events").
		WithArgs(monitorID, policyID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "monitor_id", "policy_id", "status", "started_at"}).
			AddRow(eventID, monitorID, policyID, "firing", startedAt))

	trigger.CheckAndTrigger(ctx, monitorID, "down")

	if len(enq.calls) != 0 {
		t.Errorf("expected 0 notifications (idempotent), got %d", len(enq.calls))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
