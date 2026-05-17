package cloudflare

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/kite365/idcd/lib/cert/dns"
)

const fakeToken = "test-token-0123456789ab" // 22 chars, passes length check

// ---- ValidateCredential -----------------------------------------------------

func TestValidateCredential(t *testing.T) {
	p := New(Config{})
	cases := []struct {
		name string
		cred map[string]string
		want error
	}{
		{"empty map", map[string]string{}, dns.ErrInvalidCredential},
		{"empty token", map[string]string{"api_token": ""}, dns.ErrInvalidCredential},
		{"too short", map[string]string{"api_token": "abc"}, dns.ErrInvalidCredential},
		{"ok", map[string]string{"api_token": fakeToken}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := p.ValidateCredential(c.cred)
			if c.want == nil && err != nil {
				t.Fatalf("want nil err, got %v", err)
			}
			if c.want != nil && !errors.Is(err, c.want) {
				t.Fatalf("want %v, got %v", c.want, err)
			}
		})
	}
}

// ---- HealthCheck ------------------------------------------------------------

func TestHealthCheck(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		wantErr error
	}{
		{
			name:    "ok",
			status:  200,
			body:    `{"success":true,"result":{"id":"x","status":"active"}}`,
			wantErr: nil,
		},
		{
			name:    "unauthorized",
			status:  401,
			body:    `{"success":false}`,
			wantErr: dns.ErrInvalidCredential,
		},
		{
			name:    "server error",
			status:  500,
			body:    `oops`,
			wantErr: dns.ErrUpstreamUnavailable,
		},
		{
			name:    "inactive token",
			status:  200,
			body:    `{"success":true,"result":{"id":"x","status":"disabled"}}`,
			wantErr: dns.ErrInvalidCredential,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/user/tokens/verify" {
					t.Fatalf("unexpected path %s", r.URL.Path)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer "+fakeToken {
					t.Fatalf("bad auth header: %s", got)
				}
				w.WriteHeader(c.status)
				_, _ = w.Write([]byte(c.body))
			}))
			defer srv.Close()
			p := New(Config{BaseURL: srv.URL})
			err := p.HealthCheck(context.Background(), map[string]string{"api_token": fakeToken})
			if c.wantErr == nil && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Fatalf("want %v, got %v", c.wantErr, err)
			}
		})
	}
}

func TestHealthCheck_BadCred(t *testing.T) {
	p := New(Config{})
	err := p.HealthCheck(context.Background(), map[string]string{})
	if !errors.Is(err, dns.ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential, got %v", err)
	}
}

// ---- BuildSolver + Present + CleanUp ----------------------------------------

// fakeCloudflareServer mocks just enough of the v4 API for our tests.
type fakeCloudflareServer struct {
	mu        sync.Mutex
	zoneByDom map[string]string                  // zone name -> id
	records   map[string]map[string]dnsRecord    // zoneID -> recordID -> record
	nextRec   int
	t         *testing.T
}

func newFakeCloudflare(t *testing.T) (*fakeCloudflareServer, *httptest.Server) {
	f := &fakeCloudflareServer{
		zoneByDom: map[string]string{
			"example.com": "zone-example",
		},
		records: map[string]map[string]dnsRecord{
			"zone-example": {},
		},
		t: t,
	}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	return f, srv
}

func (f *fakeCloudflareServer) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")

	switch {
	case r.URL.Path == "/zones" && r.Method == http.MethodGet:
		name := r.URL.Query().Get("name")
		zoneID, ok := f.zoneByDom[name]
		if !ok {
			_ = json.NewEncoder(w).Encode(zonesListResp{Success: true, Result: nil})
			return
		}
		resp := zonesListResp{Success: true}
		resp.Result = append(resp.Result, struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}{ID: zoneID, Name: name})
		_ = json.NewEncoder(w).Encode(resp)
		return

	case strings.HasPrefix(r.URL.Path, "/zones/") && strings.HasSuffix(r.URL.Path, "/dns_records") && r.Method == http.MethodPost:
		zoneID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/zones/"), "/dns_records")
		var rec dnsRecord
		if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		f.nextRec++
		id := newRecID(f.nextRec)
		if f.records[zoneID] == nil {
			f.records[zoneID] = map[string]dnsRecord{}
		}
		f.records[zoneID][id] = rec
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "result": rec})
		return

	case strings.HasPrefix(r.URL.Path, "/zones/") && strings.HasSuffix(r.URL.Path, "/dns_records") && r.Method == http.MethodGet:
		zoneID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/zones/"), "/dns_records")
		filterName := r.URL.Query().Get("name")
		resp := dnsRecordListResp{Success: true}
		for id, rec := range f.records[zoneID] {
			if filterName != "" && rec.Name != filterName {
				continue
			}
			resp.Result = append(resp.Result, dnsRecordListItem{
				ID: id, Name: rec.Name, Content: rec.Content,
			})
		}
		_ = json.NewEncoder(w).Encode(resp)
		return

	case strings.HasPrefix(r.URL.Path, "/zones/") && r.Method == http.MethodDelete:
		// /zones/<zoneID>/dns_records/<recID>
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/zones/"), "/")
		if len(parts) != 3 || parts[1] != "dns_records" {
			http.NotFound(w, r)
			return
		}
		zoneID, recID := parts[0], parts[2]
		if _, ok := f.records[zoneID][recID]; !ok {
			http.NotFound(w, r)
			return
		}
		delete(f.records[zoneID], recID)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		return
	}
	http.NotFound(w, r)
}

func newRecID(n int) string { return "rec-" + itoa(n) }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func TestSolver_PresentCleanUp(t *testing.T) {
	fake, srv := newFakeCloudflare(t)
	defer srv.Close()
	p := New(Config{BaseURL: srv.URL})
	solver, err := p.BuildSolver(context.Background(), map[string]string{"api_token": fakeToken}, []string{"example.com"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if solver.Timeout() != defaultPropagationTimeout {
		t.Fatalf("unexpected timeout %v", solver.Timeout())
	}

	const fqdn = "_acme-challenge.example.com."
	const value = "test-txt-value"

	if err := solver.Present(context.Background(), fqdn, value); err != nil {
		t.Fatalf("present: %v", err)
	}
	fake.mu.Lock()
	gotRecs := len(fake.records["zone-example"])
	fake.mu.Unlock()
	if gotRecs != 1 {
		t.Fatalf("want 1 record after present, got %d", gotRecs)
	}

	if err := solver.CleanUp(context.Background(), fqdn, value); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	fake.mu.Lock()
	gotRecs = len(fake.records["zone-example"])
	fake.mu.Unlock()
	if gotRecs != 0 {
		t.Fatalf("want 0 records after cleanup, got %d", gotRecs)
	}
}

func TestSolver_Present_NestedSubdomain(t *testing.T) {
	// fqdn 是 _acme-challenge.www.example.com.，zone 是 example.com。
	// findZoneID 应当走从最具体向上找 apex 的路径。
	_, srv := newFakeCloudflare(t)
	defer srv.Close()
	p := New(Config{BaseURL: srv.URL})
	solver, err := p.BuildSolver(context.Background(), map[string]string{"api_token": fakeToken}, nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := solver.Present(context.Background(), "_acme-challenge.www.example.com.", "v"); err != nil {
		t.Fatalf("present: %v", err)
	}
}

func TestSolver_Present_NoZone(t *testing.T) {
	_, srv := newFakeCloudflare(t)
	defer srv.Close()
	p := New(Config{BaseURL: srv.URL})
	solver, err := p.BuildSolver(context.Background(), map[string]string{"api_token": fakeToken}, nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	err = solver.Present(context.Background(), "_acme-challenge.unknown.test.", "v")
	if !errors.Is(err, dns.ErrZoneNotFound) {
		t.Fatalf("want ErrZoneNotFound, got %v", err)
	}
}

func TestSolver_Present_TooShortFQDN(t *testing.T) {
	_, srv := newFakeCloudflare(t)
	defer srv.Close()
	p := New(Config{BaseURL: srv.URL})
	solver, err := p.BuildSolver(context.Background(), map[string]string{"api_token": fakeToken}, nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	err = solver.Present(context.Background(), "localhost.", "v")
	if !errors.Is(err, dns.ErrZoneNotFound) {
		t.Fatalf("want ErrZoneNotFound for too-short fqdn, got %v", err)
	}
}

func TestSolver_Present_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"success":false}`))
	}))
	defer srv.Close()
	p := New(Config{BaseURL: srv.URL})
	solver, err := p.BuildSolver(context.Background(), map[string]string{"api_token": fakeToken}, nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	err = solver.Present(context.Background(), "_acme-challenge.example.com.", "v")
	if !errors.Is(err, dns.ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential, got %v", err)
	}
}

func TestBuildSolver_BadCred(t *testing.T) {
	p := New(Config{})
	_, err := p.BuildSolver(context.Background(), map[string]string{}, nil)
	if !errors.Is(err, dns.ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential, got %v", err)
	}
}

func TestKindIsCloudflare(t *testing.T) {
	if New(Config{}).Kind() != dns.KindCloudflare {
		t.Fatalf("wrong kind")
	}
}

// quote-wrapped content path
func TestListMatching_QuotedContent(t *testing.T) {
	fake, srv := newFakeCloudflare(t)
	defer srv.Close()
	// pre-insert a TXT record where content is already quote-wrapped (cloudflare
	// sometimes returns it that way).
	fake.records["zone-example"]["rec-quoted"] = dnsRecord{
		Type: "TXT", Name: "_acme-challenge.example.com", Content: "\"v1\"",
	}
	p := New(Config{BaseURL: srv.URL})
	solver, err := p.BuildSolver(context.Background(), map[string]string{"api_token": fakeToken}, nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := solver.CleanUp(context.Background(), "_acme-challenge.example.com.", "v1"); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	fake.mu.Lock()
	_, stillThere := fake.records["zone-example"]["rec-quoted"]
	fake.mu.Unlock()
	if stillThere {
		t.Fatalf("quoted record not cleaned up")
	}
}
