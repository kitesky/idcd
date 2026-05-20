package handler

import (
	"testing"
)

func TestNormaliseSAN(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"basic lower", "example.com", "example.com", false},
		{"uppercase", "Example.COM", "example.com", false},
		{"trailing dot", "example.com.", "example.com", false},
		{"wildcard", "*.example.com", "*.example.com", false},
		{"idn", "管理员.example.com", "xn--jsr105e4gf.example.com", false},
		{"empty", "", "", true},
		{"single label", "example", "", true},
		{"reserved local", "thing.local", "", true},
		{"reserved internal", "thing.internal", "", true},
		{"reserved test", "thing.test", "", true},
		{"reserved localhost", "localhost", "", true},
		{"ip", "10.0.0.1", "", true},
		{"wildcard mid", "foo.*.example.com", "", true},
		{"wildcard alone", "*.", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := normaliseSAN(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("want error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestDedupePreserveOrder(t *testing.T) {
	in := []string{"a", "b", "a", "c", "b"}
	got := dedupePreserveOrder(in)
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

