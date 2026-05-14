package oncall

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
