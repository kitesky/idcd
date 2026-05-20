package job

import (
	"testing"
	"time"
)

// untilNextRun is the only branchy bit of the aggregator that's worth
// guarding without standing up a real DB. The SQL aggregation is integration-
// tested separately (run by hand against dev DB; see scripts/dev-status-test.sh).
func TestUntilNextRun(t *testing.T) {
	t.Parallel()

	// Fix "now" at 2026-05-20 12:34:56 UTC. runAtMinute=5 means the next
	// run is tomorrow at 00:05 UTC.
	fixedNow := time.Date(2026, 5, 20, 12, 34, 56, 0, time.UTC)

	cases := []struct {
		name        string
		runAtMinute int
		want        time.Duration
	}{
		{
			name:        "today's window passed → tomorrow",
			runAtMinute: 5,
			// next 00:05 UTC = 2026-05-21 00:05 → 11h 30m 4s from 12:34:56
			want: 11*time.Hour + 30*time.Minute + 4*time.Second,
		},
		{
			name:        "later minute today still passed → tomorrow",
			runAtMinute: 30,
			want:        11*time.Hour + 55*time.Minute + 4*time.Second,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := NewDailyAggregator(nil, DailyOptions{
				RunAtMinute: tc.runAtMinute,
				Clock:       func() time.Time { return fixedNow },
			})
			got := a.untilNextRun()
			if got != tc.want {
				t.Fatalf("untilNextRun: got %v, want %v", got, tc.want)
			}
		})
	}
}

// untilNextRun called BEFORE today's window should target today, not tomorrow.
func TestUntilNextRunBeforeWindow(t *testing.T) {
	t.Parallel()
	// 2026-05-20 00:01 UTC, runAtMinute=5 → 4 minutes until today's 00:05.
	now := time.Date(2026, 5, 20, 0, 1, 0, 0, time.UTC)
	a := NewDailyAggregator(nil, DailyOptions{
		RunAtMinute: 5,
		Clock:       func() time.Time { return now },
	})
	got := a.untilNextRun()
	want := 4 * time.Minute
	if got != want {
		t.Fatalf("untilNextRun before window: got %v, want %v", got, want)
	}
}

// Defaults: zero RunAtMinute → 5. Negative/out-of-range → 5.
func TestDailyAggregatorDefaults(t *testing.T) {
	t.Parallel()
	cases := []struct {
		give int
		want int
	}{
		{0, 5},
		{-1, 5},
		{60, 5},
		{99, 5},
		{15, 15}, // valid passes through
		{1, 1},
	}
	for _, tc := range cases {
		a := NewDailyAggregator(nil, DailyOptions{RunAtMinute: tc.give})
		if a.runAtMinute != tc.want {
			t.Errorf("RunAtMinute=%d: got %d, want %d", tc.give, a.runAtMinute, tc.want)
		}
	}
}
