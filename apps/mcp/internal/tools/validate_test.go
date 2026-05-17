package tools

import (
	"strings"
	"testing"
)

func TestValidateTarget(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"plain hostname", "example.com", "example.com", false},
		{"trim outer whitespace", "  example.com  ", "example.com", false},
		{"empty", "", "", true},
		{"whitespace only", "   ", "", true},
		{"oversize", strings.Repeat("a", maxTargetLen+1), "", true},
		{"embedded CR", "example.com\rGET /admin", "", true},
		{"embedded LF", "example.com\nHost: evil.com", "", true},
		{"embedded tab", "exam\tple.com", "", true},
		{"embedded DEL", "exam\x7fple.com", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateTarget(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestValidateCount(t *testing.T) {
	cases := []struct {
		raw  float64
		def  int
		max  int
		want int
	}{
		{0, 3, 10, 3},
		{-5, 3, 10, 3},
		{5, 3, 10, 5},
		{1000, 3, 10, 10},
	}
	for _, c := range cases {
		got := validateCount(c.raw, c.def, c.max)
		if got != c.want {
			t.Errorf("validateCount(%v, %v, %v) = %v want %v", c.raw, c.def, c.max, got, c.want)
		}
	}
}

func TestValidateURL(t *testing.T) {
	long := "https://" + strings.Repeat("a", maxURLLen)
	if _, err := validateURL(long); err == nil {
		t.Fatal("expected oversize URL to be rejected")
	}
	if _, err := validateURL("https://example.com\rGET /"); err == nil {
		t.Fatal("expected CR-injected URL to be rejected")
	}
	if got, err := validateURL("  https://example.com  "); err != nil || got != "https://example.com" {
		t.Fatalf("expected trimmed URL, got %q err=%v", got, err)
	}
}
