package pagination_test

import (
	"testing"

	"github.com/kite365/idcd/lib/shared/pagination"
)

func TestClamp(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{"zero falls back to default", 0, pagination.DefaultPageSize},
		{"negative falls back to default", -5, pagination.DefaultPageSize},
		{"under max passes through", 42, 42},
		{"at max passes through", pagination.MaxPageSize, pagination.MaxPageSize},
		{"over max clamps down", pagination.MaxPageSize + 1, pagination.MaxPageSize},
		{"huge value clamps down", 1_000_000, pagination.MaxPageSize},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := pagination.Clamp(tc.input); got != tc.want {
				t.Errorf("Clamp(%d) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestClampWith(t *testing.T) {
	tests := []struct {
		name                   string
		input, defSize, maxVal int
		want                   int
	}{
		{"admin default kicks in", 0, pagination.AdminDefaultPageSize, pagination.MaxPageSize, pagination.AdminDefaultPageSize},
		{"repo cap allows larger pull", 150, pagination.AdminDefaultPageSize, pagination.RepoMaxPageSize, 150},
		{"repo cap still bounds extreme values", 1_000, pagination.AdminDefaultPageSize, pagination.RepoMaxPageSize, pagination.RepoMaxPageSize},
		{"negative with custom default", -1, 5, 25, 5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := pagination.ClampWith(tc.input, tc.defSize, tc.maxVal)
			if got != tc.want {
				t.Errorf("ClampWith(%d, %d, %d) = %d, want %d",
					tc.input, tc.defSize, tc.maxVal, got, tc.want)
			}
		})
	}
}

// Guard: documented constants stay in a sane relationship to each other.
// 若产品决定突破这些关系（如 Repo < Max），请同时更新这个测试和包注释。
func TestConstantInvariants(t *testing.T) {
	if pagination.DefaultPageSize > pagination.MaxPageSize {
		t.Fatalf("DefaultPageSize (%d) must be <= MaxPageSize (%d)",
			pagination.DefaultPageSize, pagination.MaxPageSize)
	}
	if pagination.AdminDefaultPageSize > pagination.RepoMaxPageSize {
		t.Fatalf("AdminDefaultPageSize (%d) must be <= RepoMaxPageSize (%d)",
			pagination.AdminDefaultPageSize, pagination.RepoMaxPageSize)
	}
	if pagination.MaxPageSize > pagination.RepoMaxPageSize {
		t.Fatalf("MaxPageSize (%d) must be <= RepoMaxPageSize (%d)",
			pagination.MaxPageSize, pagination.RepoMaxPageSize)
	}
}
