package handler

import (
	"testing"
	"time"
)

// validateIncident is the only branchy logic worth covering without a real
// DB — it gates malformed admin POSTs before they hit the DB CHECK constraint
// (which would surface as 500 instead of 400). Keep these in sync with the
// spec in admin_status_incidents.go.
func TestValidateIncident(t *testing.T) {
	t.Parallel()

	now := time.Now()
	earlier := now.Add(-1 * time.Hour)
	later := now.Add(1 * time.Hour)

	cases := []struct {
		name    string
		give    adminIncident
		wantErr bool
	}{
		{
			name: "happy path: all required fields valid",
			give: adminIncident{
				ServiceKey: "api",
				StartedAt:  now,
				Severity:   "outage",
				Title:      "DB primary failover",
			},
			wantErr: false,
		},
		{
			name: "happy path with ended_at after started_at",
			give: adminIncident{
				ServiceKey: "api",
				StartedAt:  earlier,
				EndedAt:    &now,
				Severity:   "degradation",
				Title:      "slow queries",
			},
			wantErr: false,
		},
		{
			name:    "missing service_key → 400",
			give:    adminIncident{StartedAt: now, Severity: "outage", Title: "x"},
			wantErr: true,
		},
		{
			name:    "blank title (whitespace) → 400",
			give:    adminIncident{ServiceKey: "api", StartedAt: now, Severity: "outage", Title: "   "},
			wantErr: true,
		},
		{
			name:    "zero started_at → 400",
			give:    adminIncident{ServiceKey: "api", Severity: "outage", Title: "x"},
			wantErr: true,
		},
		{
			name:    "invalid severity → 400",
			give:    adminIncident{ServiceKey: "api", StartedAt: now, Severity: "panic", Title: "x"},
			wantErr: true,
		},
		{
			name: "ended_at before started_at → 400",
			give: adminIncident{
				ServiceKey: "api", StartedAt: now, EndedAt: &earlier,
				Severity: "outage", Title: "x",
			},
			wantErr: true,
		},
		{
			name: "ended_at = started_at is allowed (zero-duration incident)",
			give: adminIncident{
				ServiceKey: "api", StartedAt: now, EndedAt: &now,
				Severity: "outage", Title: "x",
			},
			wantErr: false,
		},
		{
			name: "ended_at after started_at is allowed",
			give: adminIncident{
				ServiceKey: "api", StartedAt: now, EndedAt: &later,
				Severity: "outage", Title: "x",
			},
			wantErr: false,
		},
		{
			name: "each allowed severity passes",
			give: adminIncident{
				ServiceKey: "api", StartedAt: now,
				Severity: "maintenance", Title: "x",
			},
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateIncident(&tc.give)
			gotErr := err != nil
			if gotErr != tc.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}
