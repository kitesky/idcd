package processor

import (
	"context"
	"testing"
	"time"

	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func newTestBaselineUpdater(t *testing.T) (pgxmock.PgxPoolIface, *BaselineUpdater) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool: %v", err)
	}
	return mock, newBaselineUpdaterWithQuerier(mock)
}

func anyArg() interface{} {
	return pgxmock.AnyArg()
}

func TestBaselineUpdater_nilPool_noOp(t *testing.T) {
	b := &BaselineUpdater{}
	err := b.UpdateBaseline(context.Background(), "mon_001")
	if err != nil {
		t.Errorf("expected nil error with nil pool, got %v", err)
	}
}

func TestBaselineUpdater_noExistingBaseline_computesAndInserts(t *testing.T) {
	mock, b := newTestBaselineUpdater(t)
	ctx := context.Background()
	monitorID := "mon_001"

	mock.ExpectQuery("SELECT sample_count, computed_at FROM monitor_baselines").
		WithArgs(monitorID).
		WillReturnError(pgxErrNoRows())

	p50 := 100.0
	p95 := 200.0
	p99 := 300.0
	sr := 0.98
	mock.ExpectQuery("SELECT").
		WithArgs(monitorID).
		WillReturnRows(pgxmock.NewRows([]string{"p50", "p95", "p99", "success_rate", "sample_count"}).
			AddRow(&p50, &p95, &p99, &sr, 500))

	mock.ExpectExec("INSERT INTO monitor_baselines").
		WithArgs(anyArg(), monitorID, anyArg(), anyArg(), anyArg(), anyArg(), anyArg(), anyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := b.UpdateBaseline(ctx, monitorID); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestBaselineUpdater_existingRecentBaseline_skips(t *testing.T) {
	mock, b := newTestBaselineUpdater(t)
	ctx := context.Background()
	monitorID := "mon_002"

	recent := time.Now().Add(-1 * time.Hour)
	mock.ExpectQuery("SELECT sample_count, computed_at FROM monitor_baselines").
		WithArgs(monitorID).
		WillReturnRows(pgxmock.NewRows([]string{"sample_count", "computed_at"}).
			AddRow(50, &recent))

	if err := b.UpdateBaseline(ctx, monitorID); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestBaselineUpdater_existingRecentBaseline_atBoundary_runs(t *testing.T) {
	mock, b := newTestBaselineUpdater(t)
	ctx := context.Background()
	monitorID := "mon_003"

	recent := time.Now().Add(-1 * time.Hour)
	mock.ExpectQuery("SELECT sample_count, computed_at FROM monitor_baselines").
		WithArgs(monitorID).
		WillReturnRows(pgxmock.NewRows([]string{"sample_count", "computed_at"}).
			AddRow(100, &recent))

	p50 := 50.0
	p95 := 150.0
	p99 := 250.0
	sr := 0.95
	mock.ExpectQuery("SELECT").
		WithArgs(monitorID).
		WillReturnRows(pgxmock.NewRows([]string{"p50", "p95", "p99", "success_rate", "sample_count"}).
			AddRow(&p50, &p95, &p99, &sr, 100))

	mock.ExpectExec("INSERT INTO monitor_baselines").
		WithArgs(anyArg(), monitorID, anyArg(), anyArg(), anyArg(), anyArg(), anyArg(), anyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := b.UpdateBaseline(ctx, monitorID); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestBaselineUpdater_emptyChecks_zeroSampleCount(t *testing.T) {
	mock, b := newTestBaselineUpdater(t)
	ctx := context.Background()
	monitorID := "mon_004"

	mock.ExpectQuery("SELECT sample_count, computed_at FROM monitor_baselines").
		WithArgs(monitorID).
		WillReturnError(pgxErrNoRows())

	mock.ExpectQuery("SELECT").
		WithArgs(monitorID).
		WillReturnRows(pgxmock.NewRows([]string{"p50", "p95", "p99", "success_rate", "sample_count"}).
			AddRow(nil, nil, nil, nil, 0))

	mock.ExpectExec("INSERT INTO monitor_baselines").
		WithArgs(anyArg(), monitorID, anyArg(), anyArg(), anyArg(), anyArg(), anyArg(), anyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := b.UpdateBaseline(ctx, monitorID); err != nil {
		t.Errorf("unexpected error with zero sample count: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestBaselineUpdater_idempotent_onConflictUpdate(t *testing.T) {
	mock, b := newTestBaselineUpdater(t)
	ctx := context.Background()
	monitorID := "mon_005"

	mock.ExpectQuery("SELECT sample_count, computed_at FROM monitor_baselines").
		WithArgs(monitorID).
		WillReturnError(pgxErrNoRows())

	p95 := 100.0
	mock.ExpectQuery("SELECT").
		WithArgs(monitorID).
		WillReturnRows(pgxmock.NewRows([]string{"p50", "p95", "p99", "success_rate", "sample_count"}).
			AddRow(nil, &p95, nil, nil, 10))

	mock.ExpectExec("INSERT INTO monitor_baselines").
		WithArgs(anyArg(), monitorID, anyArg(), anyArg(), anyArg(), anyArg(), anyArg(), anyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := b.UpdateBaseline(ctx, monitorID); err != nil {
		t.Errorf("first call error: %v", err)
	}

	old := time.Now().Add(-7 * time.Hour)
	mock.ExpectQuery("SELECT sample_count, computed_at FROM monitor_baselines").
		WithArgs(monitorID).
		WillReturnRows(pgxmock.NewRows([]string{"sample_count", "computed_at"}).
			AddRow(10, &old))

	mock.ExpectQuery("SELECT").
		WithArgs(monitorID).
		WillReturnRows(pgxmock.NewRows([]string{"p50", "p95", "p99", "success_rate", "sample_count"}).
			AddRow(nil, &p95, nil, nil, 20))

	mock.ExpectExec("INSERT INTO monitor_baselines").
		WithArgs(anyArg(), monitorID, anyArg(), anyArg(), anyArg(), anyArg(), anyArg(), anyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := b.UpdateBaseline(ctx, monitorID); err != nil {
		t.Errorf("second call error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
