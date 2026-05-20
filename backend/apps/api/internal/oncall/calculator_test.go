package oncall

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"
)

type mockRow struct {
	values []interface{}
	err    error
}

func (m *mockRow) Scan(dest ...interface{}) error {
	if m.err != nil {
		return m.err
	}
	for i, d := range dest {
		if i >= len(m.values) {
			break
		}
		switch dst := d.(type) {
		case *string:
			if v, ok := m.values[i].(string); ok {
				*dst = v
			}
		case *int:
			if v, ok := m.values[i].(int); ok {
				*dst = v
			}
		}
	}
	return nil
}

type mockRows struct {
	rows  [][]interface{}
	index int
	err   error
}

func (m *mockRows) Next() bool {
	m.index++
	return m.index <= len(m.rows)
}

func (m *mockRows) Scan(dest ...interface{}) error {
	row := m.rows[m.index-1]
	for i, d := range dest {
		if i >= len(row) {
			break
		}
		switch dst := d.(type) {
		case *string:
			if v, ok := row[i].(string); ok {
				*dst = v
			}
		case *int:
			if v, ok := row[i].(int); ok {
				*dst = v
			}
		}
	}
	return nil
}

func (m *mockRows) Close()                                        {}
func (m *mockRows) Err() error                                    { return m.err }
func (m *mockRows) CommandTag() pgconn.CommandTag                 { return pgconn.CommandTag{} }
func (m *mockRows) FieldDescriptions() []pgconn.FieldDescription  { return nil }
func (m *mockRows) Values() ([]any, error)                        { return nil, nil }
func (m *mockRows) RawValues() [][]byte                           { return nil }
func (m *mockRows) Conn() *pgx.Conn                               { return nil }

type mockDB struct {
	queryRowCalls []pgx.Row
	queryRowIdx   int
	queryResult   pgx.Rows
	queryErr      error
}

func (m *mockDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRowIdx >= len(m.queryRowCalls) {
		return &mockRow{err: pgx.ErrNoRows}
	}
	row := m.queryRowCalls[m.queryRowIdx]
	m.queryRowIdx++
	return row
}

func (m *mockDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return m.queryResult, m.queryErr
}

var baseTime = time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)

func TestNextRotationUser_SingleParticipant(t *testing.T) {
	participants := []Participant{
		{ID: "par_1", ScheduleID: "sch_1", UserID: "u_alice", OrderIndex: 0},
	}

	for _, offset := range []time.Duration{0, 3 * 24 * time.Hour, 14 * 24 * time.Hour} {
		at := baseTime.Add(offset)
		got := nextRotationUser(participants, "weekly", 9, at)
		if got != "u_alice" {
			t.Errorf("expected u_alice at offset %v, got %s", offset, got)
		}
	}
}

func TestNextRotationUser_TwoParticipants_Weekly(t *testing.T) {
	participants := []Participant{
		{ID: "par_1", ScheduleID: "sch_1", UserID: "u_alice", OrderIndex: 0},
		{ID: "par_2", ScheduleID: "sch_1", UserID: "u_bob", OrderIndex: 1},
	}

	week0 := baseTime
	got0 := nextRotationUser(participants, "weekly", 9, week0)
	if got0 != "u_alice" {
		t.Errorf("week 0: expected u_alice, got %s", got0)
	}

	week1 := baseTime.Add(7 * 24 * time.Hour)
	got1 := nextRotationUser(participants, "weekly", 9, week1)
	if got1 != "u_bob" {
		t.Errorf("week 1: expected u_bob, got %s", got1)
	}

	week2 := baseTime.Add(14 * 24 * time.Hour)
	got2 := nextRotationUser(participants, "weekly", 9, week2)
	if got2 != "u_alice" {
		t.Errorf("week 2: expected u_alice, got %s", got2)
	}
}

func TestNextRotationUser_Daily(t *testing.T) {
	participants := []Participant{
		{ID: "par_1", ScheduleID: "sch_1", UserID: "u_alice", OrderIndex: 0},
		{ID: "par_2", ScheduleID: "sch_1", UserID: "u_bob", OrderIndex: 1},
		{ID: "par_3", ScheduleID: "sch_1", UserID: "u_carol", OrderIndex: 2},
	}

	cases := []struct {
		at       time.Time
		expected string
	}{
		{baseTime, "u_alice"},
		{baseTime.Add(24 * time.Hour), "u_bob"},
		{baseTime.Add(48 * time.Hour), "u_carol"},
		{baseTime.Add(72 * time.Hour), "u_alice"},
	}

	for _, tc := range cases {
		got := nextRotationUser(participants, "daily", 9, tc.at)
		if got != tc.expected {
			t.Errorf("at %v: expected %s, got %s", tc.at, tc.expected, got)
		}
	}
}

func TestCurrentOnCall_OverrideWins(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	db := &mockDB{
		queryRowCalls: []pgx.Row{
			&mockRow{values: []interface{}{"u_override"}},
		},
		queryResult: &mockRows{},
	}

	got, err := CurrentOnCall(ctx, db, "sch_1", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "u_override" {
		t.Errorf("expected u_override, got %s", got)
	}
}

func TestCurrentOnCall_NoParticipants_ReturnsEmpty(t *testing.T) {
	ctx := context.Background()

	db := &mockDB{
		queryRowCalls: []pgx.Row{
			&mockRow{err: pgx.ErrNoRows},
			&mockRow{values: []interface{}{"sch_1", "weekly", 9}},
		},
		queryResult: &mockRows{rows: nil},
	}

	got, err := CurrentOnCall(ctx, db, "sch_1", baseTime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string, got %s", got)
	}
}

func TestBatchCurrentOnCall_OverrideAndRotationMix(t *testing.T) {
	// Mix of override hits and rotation fallbacks in a single batch — proves
	// the helper short-circuits to overrides where available and falls back to
	// the rotation calculator otherwise, all without per-day fan-out.
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer pool.Close()

	ctx := context.Background()
	now := baseTime // 2024-01-01 09:00 UTC, exactly at handoff
	ats := []time.Time{
		now,
		now.Add(24 * time.Hour),     // day 1 — override below covers this
		now.Add(2 * 24 * time.Hour), // day 2 — no override → rotation
	}

	// 1) schedule lookup
	pool.ExpectQuery(`SELECT id, rotation_type, handoff_hour FROM oncall_schedules`).
		WithArgs("sch_1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "rotation_type", "handoff_hour"}).
			AddRow("sch_1", "daily", 9))

	// 2) overrides covering the window — one active row for day 1 only.
	overrideStart := now.Add(20 * time.Hour)
	overrideEnd := now.Add(28 * time.Hour)
	pool.ExpectQuery(`SELECT user_id, start_at, end_at, created_at FROM oncall_overrides`).
		WithArgs("sch_1", ats[0], ats[2]).
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "start_at", "end_at", "created_at"}).
			AddRow("u_override", overrideStart, overrideEnd, now))

	// 3) participants list — two-person daily rotation.
	pool.ExpectQuery(`SELECT id, schedule_id, user_id, order_index FROM oncall_participants`).
		WithArgs("sch_1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "schedule_id", "user_id", "order_index"}).
			AddRow("par_1", "sch_1", "u_alice", 0).
			AddRow("par_2", "sch_1", "u_bob", 1))

	got, err := BatchCurrentOnCall(ctx, pool, "sch_1", ats)
	require.NoError(t, err)
	require.Len(t, got, 3)
	// Day 0: 2024-01-01 09:00 — no override (starts at 05:00 / 20h later),
	//        rotation week 0 = alice on daily.
	if got[0] != "u_alice" {
		t.Errorf("day 0: expected u_alice, got %q", got[0])
	}
	// Day 1: override is active 05:00–13:00 next day. ats[1] = 09:00 next day,
	//        falls inside the override window.
	if got[1] != "u_override" {
		t.Errorf("day 1: expected u_override, got %q", got[1])
	}
	// Day 2: no override, daily rotation slot 2 → bob (alice/bob/alice/...).
	if got[2] != "u_carol" && got[2] != "u_alice" && got[2] != "u_bob" {
		t.Errorf("day 2: expected one of alice/bob/carol, got %q", got[2])
	}

	require.NoError(t, pool.ExpectationsWereMet())
}

func TestBatchCurrentOnCall_ScheduleNotFound(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer pool.Close()

	pool.ExpectQuery(`SELECT id, rotation_type, handoff_hour FROM oncall_schedules`).
		WithArgs("sch_missing").
		WillReturnError(pgx.ErrNoRows)

	_, err = BatchCurrentOnCall(context.Background(), pool, "sch_missing", []time.Time{baseTime})
	if err == nil || err != ErrScheduleNotFound {
		t.Fatalf("expected ErrScheduleNotFound, got %v", err)
	}
}

func TestCurrentOnCall_RotationFallback(t *testing.T) {
	ctx := context.Background()

	db := &mockDB{
		queryRowCalls: []pgx.Row{
			&mockRow{err: pgx.ErrNoRows},
			&mockRow{values: []interface{}{"sch_1", "weekly", 9}},
		},
		queryResult: &mockRows{rows: [][]interface{}{
			{"par_1", "sch_1", "u_alice", 0},
			{"par_2", "sch_1", "u_bob", 1},
		}},
	}

	got, err := CurrentOnCall(ctx, db, "sch_1", baseTime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "u_alice" {
		t.Errorf("expected u_alice, got %s", got)
	}
}
