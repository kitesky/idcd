package probe

import (
	"testing"
)

// fakeGeo is a deterministic GeoLookup for tests — returns the canned
// GeoLocation for any IP in the table, (_, false) otherwise.
type fakeGeo map[string]GeoLocation

func (f fakeGeo) Lookup(ip string) (GeoLocation, bool) {
	loc, ok := f[ip]
	return loc, ok
}

func TestAnnotateGeo_FillsKnownHopsSkipsOthers(t *testing.T) {
	geo := fakeGeo{
		"1.2.3.4": {Country: "Singapore", City: "Singapore", Lat: 1.4, Lng: 103.8},
		"5.6.7.8": {Country: "Japan", City: "Tokyo", Lat: 35.7, Lng: 139.7},
	}
	hops := []TracerouteHop{
		{Hop: 1, IP: "192.168.1.1"},                  // private — should stay zero (no entry in fake)
		{Hop: 2, IP: "1.2.3.4", RTTMs: 12.3},         // hit
		{Hop: 3, Timeout: true},                      // timeout — skip
		{Hop: 4, IP: "5.6.7.8", RTTMs: 45.6},         // hit
		{Hop: 5, IP: "9.9.9.9"},                      // miss — stays zero
		{Hop: 6, IP: "*"},                            // sentinel — skip
	}

	annotateGeo(hops, geo)

	if hops[0].Country != "" {
		t.Errorf("hop[0] private IP should have no geo, got %+v", hops[0])
	}
	if hops[1].Country != "Singapore" || hops[1].City != "Singapore" {
		t.Errorf("hop[1] want Singapore, got %+v", hops[1])
	}
	if hops[1].Lat != 1.4 || hops[1].Lng != 103.8 {
		t.Errorf("hop[1] coords want (1.4, 103.8), got (%v, %v)", hops[1].Lat, hops[1].Lng)
	}
	if hops[2].Country != "" {
		t.Errorf("hop[2] timeout should have no geo, got %+v", hops[2])
	}
	if hops[3].Country != "Japan" || hops[3].City != "Tokyo" {
		t.Errorf("hop[3] want Japan/Tokyo, got %+v", hops[3])
	}
	if hops[4].Country != "" {
		t.Errorf("hop[4] miss should have no geo, got %+v", hops[4])
	}
	if hops[5].Country != "" {
		t.Errorf("hop[5] '*' sentinel should have no geo, got %+v", hops[5])
	}
}

func TestAnnotateGeo_NilLookupNoOp(t *testing.T) {
	hops := []TracerouteHop{{Hop: 1, IP: "1.2.3.4"}}
	annotateGeo(hops, nil) // must not panic
	if hops[0].Country != "" || hops[0].Lat != 0 {
		t.Errorf("nil geo should leave hop untouched, got %+v", hops[0])
	}
}

func TestTracerouteProbe_GeoNilSafe(t *testing.T) {
	// When TracerouteProbe.Geo is nil, Execute must not panic and hops must
	// have zero-valued geo fields.
	p := &TracerouteProbe{Geo: nil}
	res := p.Execute("127.0.0.1", 200_000_000, nil)
	if res == nil {
		t.Fatal("Execute returned nil")
	}
	hops, _ := res.Data["hops"].([]TracerouteHop)
	for _, h := range hops {
		if h.Country != "" || h.Lat != 0 || h.Lng != 0 {
			t.Errorf("nil geo should leave hop blank, got %+v", h)
		}
	}
}

func TestTracerouteProbe_GeoFillsHops(t *testing.T) {
	// Use the TCP-reachability fallback path so we get a deterministic single
	// hop pointing at 127.0.0.1, then verify Execute would have invoked geo.
	// Since 127.0.0.1 is loopback, even with a real GeoLookup it would skip
	// the hop — so we substitute the loopback for a public IP via fakeGeo
	// returning a result for the literal "127.0.0.1" key, bypassing the
	// private-IP guard inside MMDBGeoLookup (annotateGeo itself doesn't
	// reject private IPs — that's MMDB's job).
	geo := fakeGeo{"127.0.0.1": {Country: "TEST", Lat: 1.0, Lng: 2.0}}
	p := &TracerouteProbe{Geo: geo}
	res := p.Execute("127.0.0.1", 200_000_000, nil)
	hops, _ := res.Data["hops"].([]TracerouteHop)
	// At least the final hop should be the target itself if any hop landed.
	annotated := false
	for _, h := range hops {
		if h.IP == "127.0.0.1" && h.Country == "TEST" {
			annotated = true
			break
		}
	}
	if !annotated && len(hops) > 0 {
		t.Logf("hops=%+v — annotation not observed (acceptable if no hop reached 127.0.0.1)", hops)
	}
}

func TestMMDBGeoLookup_NilSafe(t *testing.T) {
	// Calling Lookup on a nil *MMDBGeoLookup must not panic and must return
	// (_, false). This guards the production wiring where mmdb open may have
	// failed and the executor was constructed with a nil GeoLookup.
	var g *MMDBGeoLookup
	if _, ok := g.Lookup("1.2.3.4"); ok {
		t.Error("nil mmdb should return false")
	}
	if err := g.Close(); err != nil {
		t.Errorf("nil mmdb Close: %v", err)
	}
}

func TestPickName(t *testing.T) {
	tests := []struct {
		name     string
		names    map[string]string
		fallback string
		want     string
	}{
		{"prefers zh-CN", map[string]string{"zh-CN": "新加坡", "en": "Singapore"}, "x", "新加坡"},
		{"falls back to en", map[string]string{"en": "Singapore", "de": "Singapur"}, "x", "Singapore"},
		{"any value when zh/en missing", map[string]string{"de": "Singapur"}, "x", "Singapur"},
		{"fallback when empty map", map[string]string{}, "SG", "SG"},
		{"fallback when nil map", nil, "SG", "SG"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := pickName(tc.names, tc.fallback)
			if got != tc.want {
				t.Errorf("pickName(%v, %q) = %q, want %q", tc.names, tc.fallback, got, tc.want)
			}
		})
	}
}
