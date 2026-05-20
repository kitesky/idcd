package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
)

// syntheticObservationPool returns three deterministic monitor_checks rows
// no matter the args. It backstops orchestrator tests in this package
// that construct minimal Orders (no Target / OwnerID / window) and rely
// on fetchObservations producing realistic data without touching a real
// DB. Used by newHarness in orchestrator_test.go.
type syntheticObservationPool struct{}

func (syntheticObservationPool) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	now := time.Now().UTC().Truncate(time.Second)
	rows := pgxmock.NewRows([]string{"check_at", "node_id", "latency_ms", "status"}).
		AddRow(now.Add(-3*time.Second), "node-cn-bj", int64(42), "up").
		AddRow(now.Add(-2*time.Second), "node-cn-sh", int64(51), "up").
		AddRow(now.Add(-1*time.Second), "node-cn-gz", int64(47), "up")
	return rows.Kind(), nil
}

func (syntheticObservationPool) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return nil
}

func (syntheticObservationPool) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

// ---------------------------------------------------------------------------
// nil order
// ---------------------------------------------------------------------------

func TestFetchObservations_NilOrderRejected(t *testing.T) {
	if _, err := fetchObservations(context.Background(), syntheticObservationPool{}, nil); err == nil {
		t.Fatalf("expected error for nil order")
	}
}

func TestFetchObservations_NilPoolRejected(t *testing.T) {
	order := newObservationOrder(time.Now().UTC())
	_, err := fetchObservations(context.Background(), nil, order)
	if !errors.Is(err, ErrObservationPoolNotConfigured) {
		t.Fatalf("expected ErrObservationPoolNotConfigured, got %v", err)
	}
}

// newObservationOrder builds a minimal Order the cross-schema query
// requires. Other fields are irrelevant to fetchObservations.
func newObservationOrder(t time.Time) *Order {
	return &Order{
		ID:              "vo_test",
		OwnerID:         "u_1",
		Target:          "https://example.com",
		TimeWindowStart: t.Add(-1 * time.Hour),
		TimeWindowEnd:   t,
	}
}

// ---------------------------------------------------------------------------
// happy path: 6 monitor_checks rows (one per node × per check_at) → 6 obs sorted
// ---------------------------------------------------------------------------

func TestFetchObservations_HappyPath_PerNodeRows(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock: %v", err)
	}
	defer mock.Close()

	base := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	order := newObservationOrder(base.Add(1 * time.Hour))
	order.TimeWindowStart = base.Add(-30 * time.Minute)
	order.TimeWindowEnd = base.Add(30 * time.Minute)

	row1At := base.Add(10 * time.Minute)
	row2At := base.Add(5 * time.Minute) // earlier — checks sort

	mock.ExpectQuery(`FROM monitor_checks`).
		WithArgs(order.OwnerID, order.Target, order.TimeWindowStart, order.TimeWindowEnd).
		WillReturnRows(pgxmock.NewRows([]string{"check_at", "node_id", "latency_ms", "status"}).
			AddRow(row1At, "node-cn-bj", int64(42), "up").
			AddRow(row1At, "node-cn-sh", int64(51), "up").
			AddRow(row1At, "node-cn-gz", int64(900), "down").
			AddRow(row2At, "node-cn-bj", int64(40), "up").
			AddRow(row2At, "node-cn-sh", int64(50), "up").
			AddRow(row2At, "node-cn-gz", int64(47), "up"))

	obs, err := fetchObservations(context.Background(), mock, order)
	if err != nil {
		t.Fatalf("fetchObservations: %v", err)
	}
	if len(obs) != 6 {
		t.Fatalf("expected 6 observations, got %d", len(obs))
	}
	for i := 1; i < len(obs); i++ {
		if obs[i].Timestamp.Before(obs[i-1].Timestamp) {
			t.Fatalf("observations not sorted at index %d: %v < %v",
				i, obs[i].Timestamp, obs[i-1].Timestamp)
		}
	}
	for i := 0; i < 3; i++ {
		if !obs[i].Timestamp.Equal(row2At) {
			t.Fatalf("obs[%d] timestamp = %v, want %v", i, obs[i].Timestamp, row2At)
		}
	}
	for i := 3; i < 6; i++ {
		if !obs[i].Timestamp.Equal(row1At) {
			t.Fatalf("obs[%d] timestamp = %v, want %v", i, obs[i].Timestamp, row1At)
		}
	}
	var bj observation
	for _, o := range obs {
		if o.Timestamp.Equal(row1At) && o.NodeID == "node-cn-bj" {
			bj = o
			break
		}
	}
	if bj.Latency != 42*time.Millisecond {
		t.Fatalf("node-cn-bj@row1 latency = %v, want 42ms", bj.Latency)
	}
	if !bj.OK {
		t.Fatalf("node-cn-bj@row1 OK = false, want true")
	}
	// row1 node-cn-gz has status=down → OK should be false
	for _, o := range obs {
		if o.Timestamp.Equal(row1At) && o.NodeID == "node-cn-gz" {
			if o.OK {
				t.Fatalf("expected down → OK=false")
			}
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// zero rows → typed error per docs/prd/18 §3.1 step 1
// ---------------------------------------------------------------------------

func TestFetchObservations_ZeroRowsReturnsError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock: %v", err)
	}
	defer mock.Close()

	order := newObservationOrder(time.Now().UTC())
	mock.ExpectQuery(`FROM monitor_checks`).
		WithArgs(order.OwnerID, order.Target, order.TimeWindowStart, order.TimeWindowEnd).
		WillReturnRows(pgxmock.NewRows([]string{"check_at", "node_id", "latency_ms", "status"}))

	obs, err := fetchObservations(context.Background(), mock, order)
	if err == nil {
		t.Fatalf("expected error for zero rows, got %d obs", len(obs))
	}
	if !strings.Contains(err.Error(), "no data for target") {
		t.Fatalf("expected 'no data for target' in error, got: %v", err)
	}
	if obs != nil {
		t.Fatalf("expected nil slice, got %v", obs)
	}
}

// ---------------------------------------------------------------------------
// query error → wrapped
// ---------------------------------------------------------------------------

func TestFetchObservations_QueryErrorWrapped(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock: %v", err)
	}
	defer mock.Close()

	order := newObservationOrder(time.Now().UTC())
	mock.ExpectQuery(`FROM monitor_checks`).
		WithArgs(order.OwnerID, order.Target, order.TimeWindowStart, order.TimeWindowEnd).
		WillReturnError(errors.New("boom"))

	if _, err := fetchObservations(context.Background(), mock, order); err == nil {
		t.Fatalf("expected query error, got nil")
	} else if !strings.Contains(err.Error(), "query monitor_checks") {
		t.Fatalf("expected wrapped query error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// NewObservationPoolFromEnv: env-var fallbacks + missing both
// ---------------------------------------------------------------------------

func TestNewObservationPoolFromEnv_NoEnvErrors(t *testing.T) {
	t.Setenv("IDCD_MAIN_DB_DSN", "")
	t.Setenv("ATTEST_DB_DSN", "")

	_, err := NewObservationPoolFromEnv(context.Background())
	if err == nil {
		t.Fatalf("expected error when neither DSN env is set")
	}
	if !errors.Is(err, ErrObservationPoolNotConfigured) {
		t.Fatalf("expected ErrObservationPoolNotConfigured, got: %v", err)
	}
}

func TestNewObservationPoolFromEnv_BadDSN(t *testing.T) {
	// pgxpool.New parses the DSN eagerly; an invalid format triggers
	// the wrapped error path.
	t.Setenv("IDCD_MAIN_DB_DSN", "not-a-valid-dsn-#$%^&")
	t.Setenv("ATTEST_DB_DSN", "")

	_, err := NewObservationPoolFromEnv(context.Background())
	if err == nil {
		t.Fatalf("expected error for malformed DSN")
	}
	// Either ErrObservationPoolNotConfigured (if pgxpool tolerated it
	// and returned nil — unlikely) or a wrapped pgxpool error.
	if errors.Is(err, ErrObservationPoolNotConfigured) {
		t.Fatalf("expected pgxpool.New wrapped error, got ErrObservationPoolNotConfigured")
	}
}
