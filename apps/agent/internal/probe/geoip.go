package probe

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/oschwald/maxminddb-golang/v2"
)

// GeoLocation is the geo info attached to a traceroute/MTR hop.
// Lat/Lng are 0 when the IP is private or unresolved — call sites should
// check Country != "" before treating the location as plottable.
type GeoLocation struct {
	Country string  `json:"country,omitempty"`
	City    string  `json:"city,omitempty"`
	Lat     float64 `json:"lat,omitempty"`
	Lng     float64 `json:"lng,omitempty"`
}

// GeoLookup resolves an IP to a GeoLocation. Implementations must be safe
// for concurrent use — traceroute/MTR fan out lookups across hops.
//
// Returning (GeoLocation{}, false) for unknown / private IPs is preferred
// over an error: a missing geo is not a probe failure.
type GeoLookup interface {
	Lookup(ip string) (GeoLocation, bool)
}

// MMDBGeoLookup reads a MaxMind GeoLite2-City database from disk.
type MMDBGeoLookup struct {
	reader *maxminddb.Reader
}

// OpenMMDB loads the GeoLite2-City mmdb file. Caller owns Close().
func OpenMMDB(path string) (*MMDBGeoLookup, error) {
	r, err := maxminddb.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open geoip mmdb %q: %w", path, err)
	}
	return &MMDBGeoLookup{reader: r}, nil
}

// Close releases the mmdb file handle.
func (g *MMDBGeoLookup) Close() error {
	if g == nil || g.reader == nil {
		return nil
	}
	return g.reader.Close()
}

// mmdbCityRecord is the minimum subset of GeoLite2-City needed for the map.
// Field names match MaxMind's mmdb labels exactly.
type mmdbCityRecord struct {
	Country struct {
		Names   map[string]string `maxminddb:"names"`
		IsoCode string            `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
}

// Lookup queries the mmdb. Returns (_, false) when the IP is invalid, private,
// or not present in the database — never returns an error so call sites stay
// simple (a failed lookup is not a probe error).
func (g *MMDBGeoLookup) Lookup(ip string) (GeoLocation, bool) {
	if g == nil || g.reader == nil || ip == "" {
		return GeoLocation{}, false
	}
	parsed, err := netip.ParseAddr(ip)
	if err != nil {
		// Fall back through net.ParseIP for forms maxmind/v2 doesn't accept.
		nip := net.ParseIP(ip)
		if nip == nil {
			return GeoLocation{}, false
		}
		parsed, _ = netip.AddrFromSlice(nip.To16())
	}
	if !parsed.IsValid() || parsed.IsPrivate() || parsed.IsLoopback() || parsed.IsUnspecified() {
		return GeoLocation{}, false
	}

	var rec mmdbCityRecord
	if err := g.reader.Lookup(parsed).Decode(&rec); err != nil {
		return GeoLocation{}, false
	}

	loc := GeoLocation{
		Country: pickName(rec.Country.Names, rec.Country.IsoCode),
		City:    pickName(rec.City.Names, ""),
		Lat:     rec.Location.Latitude,
		Lng:     rec.Location.Longitude,
	}
	if loc.Country == "" && loc.Lat == 0 && loc.Lng == 0 {
		return GeoLocation{}, false
	}
	return loc, true
}

// annotateGeo fills Country/City/Lat/Lng on each non-timeout, non-private hop.
// Mutates hops in place; missing geo is left as zero values (handled in JSON
// by omitempty so the wire payload doesn't carry empty fields).
func annotateGeo(hops []TracerouteHop, geo GeoLookup) {
	if geo == nil {
		return
	}
	for i := range hops {
		h := &hops[i]
		if h.Timeout || h.IP == "" || h.IP == "*" {
			continue
		}
		loc, ok := geo.Lookup(h.IP)
		if !ok {
			continue
		}
		h.Country = loc.Country
		h.City = loc.City
		h.Lat = loc.Lat
		h.Lng = loc.Lng
	}
}

// pickName prefers Chinese (zh-CN), then English, then any value, finally fallback.
func pickName(names map[string]string, fallback string) string {
	for _, k := range []string{"zh-CN", "en"} {
		if v, ok := names[k]; ok && v != "" {
			return v
		}
	}
	for _, v := range names {
		if v != "" {
			return v
		}
	}
	return fallback
}
