package handler

import (
	"testing"
)

// statusFromInt is the contract between the DB SMALLINT codes and the
// frontend's ServiceStatus enum strings. The codes are spec
// (lib/db/migrations/idcd_main/00050_status_uptime.sql) — guard them.
func TestStatusFromInt(t *testing.T) {
	t.Parallel()
	cases := map[int16]string{
		1: statusStrOperational,
		2: statusStrDegraded,
		3: statusStrOutage,
		4: statusStrMaintenance,
		// Unknown values must NOT silently green-wash — fall back to outage.
		0:  statusStrOutage,
		5:  statusStrOutage,
		99: statusStrOutage,
		-1: statusStrOutage,
	}
	for in, want := range cases {
		if got := statusFromInt(in); got != want {
			t.Errorf("statusFromInt(%d) = %q, want %q", in, got, want)
		}
	}
}

// computeOverall implements the "any-worst" rollup with one twist: maintenance
// counts as degraded for the top banner (so users see "something is off" not
// "all systems operational" during a planned window). These are spec.
func TestComputeOverall(t *testing.T) {
	t.Parallel()

	svc := func(s string) PublicStatusService { return PublicStatusService{CurrentStatus: s} }
	group := func(online, total int) PublicStatusNodeCountryGroup {
		return PublicStatusNodeCountryGroup{OnlineCount: online, TotalCount: total}
	}

	cases := []struct {
		name     string
		services []PublicStatusService
		groups   []PublicStatusNodeCountryGroup
		want     string
	}{
		{
			name:     "empty everywhere → operational",
			services: nil, groups: nil, want: statusStrOperational,
		},
		{
			name: "all services operational, all nodes online → operational",
			services: []PublicStatusService{svc(statusStrOperational), svc(statusStrOperational)},
			groups:   []PublicStatusNodeCountryGroup{group(3, 3), group(1, 1)},
			want:     statusStrOperational,
		},
		{
			name: "one service degraded → degraded",
			services: []PublicStatusService{svc(statusStrOperational), svc(statusStrDegraded)},
			groups:   nil,
			want:     statusStrDegraded,
		},
		{
			name: "one service outage → outage (degraded is overridden)",
			services: []PublicStatusService{svc(statusStrDegraded), svc(statusStrOutage)},
			groups:   nil,
			want:     statusStrOutage,
		},
		{
			name: "all nodes in a country offline → outage",
			services: []PublicStatusService{svc(statusStrOperational)},
			groups:   []PublicStatusNodeCountryGroup{group(0, 3)},
			want:     statusStrOutage,
		},
		{
			name: "partial node outage (2/3 online) → degraded",
			services: []PublicStatusService{svc(statusStrOperational)},
			groups:   []PublicStatusNodeCountryGroup{group(2, 3)},
			want:     statusStrDegraded,
		},
		{
			name: "service in maintenance → degraded (banner shows trouble)",
			services: []PublicStatusService{svc(statusStrMaintenance)},
			groups:   nil,
			want:     statusStrDegraded,
		},
		{
			name: "country with TotalCount=0 (no nodes enrolled) is ignored",
			services: []PublicStatusService{svc(statusStrOperational)},
			groups:   []PublicStatusNodeCountryGroup{group(0, 0)},
			want:     statusStrOperational,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeOverall(tc.services, tc.groups)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
