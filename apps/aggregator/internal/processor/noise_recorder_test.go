package processor

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestRecordNoise_Fire(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool: %v", err)
	}
	ctx := context.Background()

	mock.ExpectExec("INSERT INTO alert_noise_stats").
		WithArgs(pgxmock.AnyArg(), "u_test", pgxmock.AnyArg(), "m_test").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := RecordNoise(ctx, mock, "u_test", "m_test", "fire", time.Now()); err != nil {
		t.Fatalf("RecordNoise fire: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRecordNoise_ResolveFlap(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool: %v", err)
	}
	ctx := context.Background()

	fireTime := time.Now().Add(-2 * time.Minute)

	mock.ExpectExec("INSERT INTO alert_noise_stats").
		WithArgs(pgxmock.AnyArg(), "u_test", pgxmock.AnyArg(), "m_test").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := RecordNoise(ctx, mock, "u_test", "m_test", "resolve", fireTime); err != nil {
		t.Fatalf("RecordNoise resolve flap: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRecordNoise_ResolveNoFlap(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool: %v", err)
	}
	ctx := context.Background()

	fireTime := time.Now().Add(-30 * time.Minute)

	mock.ExpectExec("INSERT INTO alert_noise_stats").
		WithArgs(pgxmock.AnyArg(), "u_test", pgxmock.AnyArg(), "m_test").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := RecordNoise(ctx, mock, "u_test", "m_test", "resolve", fireTime); err != nil {
		t.Fatalf("RecordNoise resolve no-flap: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRecordNoise_DifferentMonitors_IndependentRows(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool: %v", err)
	}
	ctx := context.Background()

	mock.ExpectExec("INSERT INTO alert_noise_stats").
		WithArgs(pgxmock.AnyArg(), "u_test", pgxmock.AnyArg(), "m_alpha").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mock.ExpectExec("INSERT INTO alert_noise_stats").
		WithArgs(pgxmock.AnyArg(), "u_test", pgxmock.AnyArg(), "m_beta").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := RecordNoise(ctx, mock, "u_test", "m_alpha", "fire", time.Now()); err != nil {
		t.Fatalf("RecordNoise m_alpha: %v", err)
	}
	if err := RecordNoise(ctx, mock, "u_test", "m_beta", "fire", time.Now()); err != nil {
		t.Fatalf("RecordNoise m_beta: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRecordNoise_UnknownEventType(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool: %v", err)
	}
	ctx := context.Background()
	err = RecordNoise(ctx, mock, "u_test", "m_test", "unknown", time.Now())
	if err == nil {
		t.Fatal("expected error for unknown eventType")
	}
}

func newNoiseTestTrigger(t *testing.T) (pgxmock.PgxPoolIface, *AlertTrigger) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool: %v", err)
	}
	enq := &mockEnqueuer{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	trigger := newAlertTriggerWithQuerier(mock, enq, logger)
	return mock, trigger
}
