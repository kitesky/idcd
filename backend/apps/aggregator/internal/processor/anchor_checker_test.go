package processor

import (
	"context"
	"testing"

	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func newTestAnchorChecker(t *testing.T) (pgxmock.PgxPoolIface, *AnchorChecker) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool: %v", err)
	}
	return mock, newAnchorCheckerWithQuerier(mock)
}

func TestAnchorChecker_nilPool_noOp(t *testing.T) {
	a := &AnchorChecker{}
	err := a.CheckDeviation(context.Background(), "mon_001", 500.0, true)
	if err != nil {
		t.Errorf("expected nil error with nil pool, got %v", err)
	}
}

func TestAnchorChecker_noBaseline_skips(t *testing.T) {
	mock, a := newTestAnchorChecker(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT id, p95_latency, success_rate, sample_count FROM monitor_baselines").
		WithArgs("mon_001").
		WillReturnError(pgxErrNoRows())

	if err := a.CheckDeviation(ctx, "mon_001", 999.0, false); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestAnchorChecker_insufficientSamples_skips(t *testing.T) {
	mock, a := newTestAnchorChecker(t)
	ctx := context.Background()
	p95 := 100.0
	sr := 0.99

	mock.ExpectQuery("SELECT id, p95_latency, success_rate, sample_count FROM monitor_baselines").
		WithArgs("mon_002").
		WillReturnRows(pgxmock.NewRows([]string{"id", "p95_latency", "success_rate", "sample_count"}).
			AddRow("bln_abc", &p95, &sr, 5))

	if err := a.CheckDeviation(ctx, "mon_002", 500.0, false); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestAnchorChecker_latencyOver2x_createsWarning(t *testing.T) {
	mock, a := newTestAnchorChecker(t)
	ctx := context.Background()
	p95 := 100.0
	sr := 0.99

	mock.ExpectQuery("SELECT id, p95_latency, success_rate, sample_count FROM monitor_baselines").
		WithArgs("mon_003").
		WillReturnRows(pgxmock.NewRows([]string{"id", "p95_latency", "success_rate", "sample_count"}).
			AddRow("bln_001", &p95, &sr, 100))

	// 250ms / 100ms = 2.5x → warning
	mock.ExpectQuery("SELECT COUNT").
		WithArgs("mon_003", "latency_spike").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectExec("INSERT INTO anchor_deviations").
		WithArgs(anyArg(), "mon_003", "bln_001", "latency_spike", anyArg(), anyArg(), anyArg(), "warning", anyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	// success_rate: 1.0 >= 0.99*0.95=0.9405 → no deviation, resolve.
	mock.ExpectExec("UPDATE anchor_deviations").
		WithArgs(anyArg(), "mon_003", "success_rate_drop").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	if err := a.CheckDeviation(ctx, "mon_003", 250.0, true); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestAnchorChecker_latencyOver3x_createsCritical(t *testing.T) {
	mock, a := newTestAnchorChecker(t)
	ctx := context.Background()
	p95 := 100.0
	sr := 0.99

	mock.ExpectQuery("SELECT id, p95_latency, success_rate, sample_count FROM monitor_baselines").
		WithArgs("mon_004").
		WillReturnRows(pgxmock.NewRows([]string{"id", "p95_latency", "success_rate", "sample_count"}).
			AddRow("bln_002", &p95, &sr, 200))

	// 350ms / 100ms = 3.5x → critical
	mock.ExpectQuery("SELECT COUNT").
		WithArgs("mon_004", "latency_spike").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectExec("INSERT INTO anchor_deviations").
		WithArgs(anyArg(), "mon_004", "bln_002", "latency_spike", anyArg(), anyArg(), anyArg(), "critical", anyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	// success_rate: 1.0 is fine → resolve.
	mock.ExpectExec("UPDATE anchor_deviations").
		WithArgs(anyArg(), "mon_004", "success_rate_drop").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	if err := a.CheckDeviation(ctx, "mon_004", 350.0, true); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestAnchorChecker_normalLatency_noDeviation(t *testing.T) {
	mock, a := newTestAnchorChecker(t)
	ctx := context.Background()
	p95 := 200.0
	sr := 0.99

	mock.ExpectQuery("SELECT id, p95_latency, success_rate, sample_count FROM monitor_baselines").
		WithArgs("mon_005").
		WillReturnRows(pgxmock.NewRows([]string{"id", "p95_latency", "success_rate", "sample_count"}).
			AddRow("bln_003", &p95, &sr, 50))

	// 150ms / 200ms = 0.75x → resolve latency spike.
	mock.ExpectExec("UPDATE anchor_deviations").
		WithArgs(anyArg(), "mon_005", "latency_spike").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	// success=true, 1.0 >= 0.99*0.95=0.9405 → resolve success_rate_drop.
	mock.ExpectExec("UPDATE anchor_deviations").
		WithArgs(anyArg(), "mon_005", "success_rate_drop").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	if err := a.CheckDeviation(ctx, "mon_005", 150.0, true); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestAnchorChecker_successRateDrop_createsWarning(t *testing.T) {
	mock, a := newTestAnchorChecker(t)
	ctx := context.Background()
	p95 := 200.0
	sr := 0.99

	mock.ExpectQuery("SELECT id, p95_latency, success_rate, sample_count FROM monitor_baselines").
		WithArgs("mon_006").
		WillReturnRows(pgxmock.NewRows([]string{"id", "p95_latency", "success_rate", "sample_count"}).
			AddRow("bln_004", &p95, &sr, 50))

	// 100ms / 200ms = 0.5x → resolve latency spike.
	mock.ExpectExec("UPDATE anchor_deviations").
		WithArgs(anyArg(), "mon_006", "latency_spike").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	// success=false → current=0.0 < 0.99*0.95=0.9405 → warning.
	mock.ExpectQuery("SELECT COUNT").
		WithArgs("mon_006", "success_rate_drop").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectExec("INSERT INTO anchor_deviations").
		WithArgs(anyArg(), "mon_006", "bln_004", "success_rate_drop", anyArg(), anyArg(), anyArg(), "warning", anyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := a.CheckDeviation(ctx, "mon_006", 100.0, false); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestAnchorChecker_openDeviationExists_skipsInsert(t *testing.T) {
	mock, a := newTestAnchorChecker(t)
	ctx := context.Background()
	p95 := 100.0
	sr := 0.99

	mock.ExpectQuery("SELECT id, p95_latency, success_rate, sample_count FROM monitor_baselines").
		WithArgs("mon_007").
		WillReturnRows(pgxmock.NewRows([]string{"id", "p95_latency", "success_rate", "sample_count"}).
			AddRow("bln_005", &p95, &sr, 100))

	// Latency 350ms → critical, but open deviation already exists — no INSERT.
	mock.ExpectQuery("SELECT COUNT").
		WithArgs("mon_007", "latency_spike").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	// success_rate: 1.0 is fine → resolve.
	mock.ExpectExec("UPDATE anchor_deviations").
		WithArgs(anyArg(), "mon_007", "success_rate_drop").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	if err := a.CheckDeviation(ctx, "mon_007", 350.0, true); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
