package route53

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kite365/idcd/lib/cert/dns"
)

const (
	fakeAKID   = "AKIATESTACCESSKEY"    // 17 chars, in [16,128]
	fakeSecret = "fake-secret-access-key-0123456789" // >= 20 chars
)

// ---- New / Kind / ValidateCredential ---------------------------------------

func TestNew_NonNil(t *testing.T) {
	if got := New(Config{}); got == nil {
		t.Fatalf("New returned nil")
	}
}

func TestKind(t *testing.T) {
	if k := New(Config{}).Kind(); k != dns.KindRoute53 {
		t.Fatalf("Kind = %q, want route53", k)
	}
}

func TestKindString(t *testing.T) {
	if string(dns.KindRoute53) != "route53" {
		t.Fatalf("KindRoute53 string = %q, want route53", dns.KindRoute53)
	}
}

func TestValidateCredential(t *testing.T) {
	p := New(Config{})
	cases := []struct {
		name string
		cred map[string]string
		want error
	}{
		{"empty map", map[string]string{}, dns.ErrInvalidCredential},
		{"missing secret", map[string]string{"access_key_id": fakeAKID}, dns.ErrInvalidCredential},
		{"missing akid", map[string]string{"secret_access_key": fakeSecret}, dns.ErrInvalidCredential},
		{"akid too short", map[string]string{"access_key_id": "AKIA123", "secret_access_key": fakeSecret}, dns.ErrInvalidCredential},
		{"akid too long", map[string]string{"access_key_id": strings.Repeat("A", 129), "secret_access_key": fakeSecret}, dns.ErrInvalidCredential},
		{"secret too short", map[string]string{"access_key_id": fakeAKID, "secret_access_key": "abc"}, dns.ErrInvalidCredential},
		{"empty akid", map[string]string{"access_key_id": "", "secret_access_key": fakeSecret}, dns.ErrInvalidCredential},
		{"empty secret", map[string]string{"access_key_id": fakeAKID, "secret_access_key": ""}, dns.ErrInvalidCredential},
		{"ok", map[string]string{"access_key_id": fakeAKID, "secret_access_key": fakeSecret}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := p.ValidateCredential(c.cred)
			if c.want == nil && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if c.want != nil && !errors.Is(err, c.want) {
				t.Fatalf("want %v, got %v", c.want, err)
			}
		})
	}
}

// ---- defaults applied -------------------------------------------------------

func TestNew_AppliesDefaults(t *testing.T) {
	p := New(Config{}).(*r53Provider)
	if p.cfg.PropagationTimeout != defaultPropagationTimeout {
		t.Fatalf("PropagationTimeout = %v, want %v", p.cfg.PropagationTimeout, defaultPropagationTimeout)
	}
	if p.cfg.PollingInterval != defaultPollingInterval {
		t.Fatalf("PollingInterval = %v, want %v", p.cfg.PollingInterval, defaultPollingInterval)
	}
	if p.cfg.TTL != defaultTTL {
		t.Fatalf("TTL = %d, want %d", p.cfg.TTL, defaultTTL)
	}
}

func TestNew_OverridesPreserved(t *testing.T) {
	cfg := Config{
		PropagationTimeout: 30 * time.Second,
		PollingInterval:    1 * time.Second,
		TTL:                300,
	}
	p := New(cfg).(*r53Provider)
	if p.cfg.PropagationTimeout != 30*time.Second {
		t.Fatalf("PropagationTimeout override lost")
	}
	if p.cfg.PollingInterval != 1*time.Second {
		t.Fatalf("PollingInterval override lost")
	}
	if p.cfg.TTL != 300 {
		t.Fatalf("TTL override lost")
	}
}

// ---- BuildSolver ------------------------------------------------------------

func TestBuildSolver_BadCred(t *testing.T) {
	p := New(Config{})
	_, err := p.BuildSolver(context.Background(), map[string]string{}, nil)
	if !errors.Is(err, dns.ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential, got %v", err)
	}
}

func TestBuildSolver_Ok(t *testing.T) {
	p := New(Config{})
	solver, err := p.BuildSolver(context.Background(), map[string]string{
		"access_key_id":     fakeAKID,
		"secret_access_key": fakeSecret,
		"region":            "eu-west-1",
		"hosted_zone_id":    "Z123ABC",
	}, []string{"example.com"})
	if err != nil {
		t.Fatalf("BuildSolver: %v", err)
	}
	if solver == nil {
		t.Fatalf("solver is nil")
	}
	if got := solver.Timeout(); got != defaultPropagationTimeout {
		t.Fatalf("solver.Timeout = %v, want %v", got, defaultPropagationTimeout)
	}
}

func TestBuildSolver_DefaultRegion(t *testing.T) {
	// 不显式传 region 也应该能构造（走 us-east-1 默认）。
	p := New(Config{})
	if _, err := p.BuildSolver(context.Background(), map[string]string{
		"access_key_id":     fakeAKID,
		"secret_access_key": fakeSecret,
	}, nil); err != nil {
		t.Fatalf("BuildSolver: %v", err)
	}
}

// ---- HealthCheck (mock route53 endpoint) -----------------------------------

func TestHealthCheck_BadCred(t *testing.T) {
	p := New(Config{})
	err := p.HealthCheck(context.Background(), map[string]string{})
	if !errors.Is(err, dns.ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential, got %v", err)
	}
}

func TestHealthCheck_Success(t *testing.T) {
	srv := newFakeR53(t)
	defer srv.Close()
	p := New(Config{BaseEndpoint: srv.URL, MaxRetries: 1})
	err := p.HealthCheck(context.Background(), map[string]string{
		"access_key_id":     fakeAKID,
		"secret_access_key": fakeSecret,
	})
	if err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
}

func TestHealthCheck_AccessDenied(t *testing.T) {
	srv := newFakeR53AccessDenied(t)
	defer srv.Close()
	p := New(Config{BaseEndpoint: srv.URL, MaxRetries: 1})
	err := p.HealthCheck(context.Background(), map[string]string{
		"access_key_id":     fakeAKID,
		"secret_access_key": fakeSecret,
	})
	if !errors.Is(err, dns.ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential, got %v", err)
	}
}

func TestHealthCheck_Throttling(t *testing.T) {
	srv := newFakeR53Throttling(t)
	defer srv.Close()
	p := New(Config{BaseEndpoint: srv.URL, MaxRetries: 1})
	err := p.HealthCheck(context.Background(), map[string]string{
		"access_key_id":     fakeAKID,
		"secret_access_key": fakeSecret,
	})
	if !errors.Is(err, dns.ErrUpstreamUnavailable) {
		t.Fatalf("want ErrUpstreamUnavailable, got %v", err)
	}
}

func TestHealthCheck_ServerError(t *testing.T) {
	srv := newFakeR53Status(t, 500, "<Error><Code>InternalError</Code><Message>boom</Message></Error>")
	defer srv.Close()
	p := New(Config{BaseEndpoint: srv.URL, MaxRetries: 1})
	err := p.HealthCheck(context.Background(), map[string]string{
		"access_key_id":     fakeAKID,
		"secret_access_key": fakeSecret,
	})
	if !errors.Is(err, dns.ErrUpstreamUnavailable) {
		t.Fatalf("want ErrUpstreamUnavailable, got %v", err)
	}
}

// ---- Solver Present / CleanUp ----------------------------------------------

func TestSolver_PresentCleanUp(t *testing.T) {
	srv := newFakeR53(t)
	defer srv.Close()
	p := New(Config{BaseEndpoint: srv.URL, MaxRetries: 1})
	solver, err := p.BuildSolver(context.Background(), map[string]string{
		"access_key_id":     fakeAKID,
		"secret_access_key": fakeSecret,
		"hosted_zone_id":    "Z123ABC",
	}, nil)
	if err != nil {
		t.Fatalf("BuildSolver: %v", err)
	}

	const fqdn = "_acme-challenge.example.com."
	const value = "test-txt-value"

	if err := solver.Present(context.Background(), fqdn, value); err != nil {
		t.Fatalf("Present: %v", err)
	}
	if err := solver.CleanUp(context.Background(), fqdn, value); err != nil {
		t.Fatalf("CleanUp: %v", err)
	}
}

func TestSolver_Present_AutoZone(t *testing.T) {
	// 不传 hosted_zone_id：solver 应当走 ListHostedZonesByName 自动检测。
	srv := newFakeR53(t)
	defer srv.Close()
	p := New(Config{BaseEndpoint: srv.URL, MaxRetries: 1})
	solver, err := p.BuildSolver(context.Background(), map[string]string{
		"access_key_id":     fakeAKID,
		"secret_access_key": fakeSecret,
	}, nil)
	if err != nil {
		t.Fatalf("BuildSolver: %v", err)
	}
	if err := solver.Present(context.Background(), "_acme-challenge.example.com.", "v"); err != nil {
		t.Fatalf("Present (auto zone): %v", err)
	}
}

func TestSolver_Present_NoZone(t *testing.T) {
	// fake server 已知 example.com；查 unknown.test 应当返回 ErrZoneNotFound。
	srv := newFakeR53(t)
	defer srv.Close()
	p := New(Config{BaseEndpoint: srv.URL, MaxRetries: 1})
	solver, err := p.BuildSolver(context.Background(), map[string]string{
		"access_key_id":     fakeAKID,
		"secret_access_key": fakeSecret,
	}, nil)
	if err != nil {
		t.Fatalf("BuildSolver: %v", err)
	}
	err = solver.Present(context.Background(), "_acme-challenge.unknown.test.", "v")
	if !errors.Is(err, dns.ErrZoneNotFound) {
		t.Fatalf("want ErrZoneNotFound, got %v", err)
	}
}

func TestSolver_Present_TooShortFQDN(t *testing.T) {
	srv := newFakeR53(t)
	defer srv.Close()
	p := New(Config{BaseEndpoint: srv.URL, MaxRetries: 1})
	solver, err := p.BuildSolver(context.Background(), map[string]string{
		"access_key_id":     fakeAKID,
		"secret_access_key": fakeSecret,
	}, nil)
	if err != nil {
		t.Fatalf("BuildSolver: %v", err)
	}
	err = solver.Present(context.Background(), "localhost.", "v")
	if !errors.Is(err, dns.ErrZoneNotFound) {
		t.Fatalf("want ErrZoneNotFound, got %v", err)
	}
}

func TestSolver_Present_AccessDenied(t *testing.T) {
	srv := newFakeR53AccessDenied(t)
	defer srv.Close()
	p := New(Config{BaseEndpoint: srv.URL, MaxRetries: 1})
	solver, err := p.BuildSolver(context.Background(), map[string]string{
		"access_key_id":     fakeAKID,
		"secret_access_key": fakeSecret,
		"hosted_zone_id":    "Z123ABC",
	}, nil)
	if err != nil {
		t.Fatalf("BuildSolver: %v", err)
	}
	err = solver.Present(context.Background(), "_acme-challenge.example.com.", "v")
	if !errors.Is(err, dns.ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential, got %v", err)
	}
}

func TestSolver_CleanUp_AccessDenied(t *testing.T) {
	srv := newFakeR53AccessDenied(t)
	defer srv.Close()
	p := New(Config{BaseEndpoint: srv.URL, MaxRetries: 1})
	solver, err := p.BuildSolver(context.Background(), map[string]string{
		"access_key_id":     fakeAKID,
		"secret_access_key": fakeSecret,
		"hosted_zone_id":    "Z123ABC",
	}, nil)
	if err != nil {
		t.Fatalf("BuildSolver: %v", err)
	}
	err = solver.CleanUp(context.Background(), "_acme-challenge.example.com.", "v")
	if !errors.Is(err, dns.ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential, got %v", err)
	}
}

// ---- mapAWSErr direct unit tests -------------------------------------------

func TestMapAWSErr_Nil(t *testing.T) {
	if got := mapAWSErr(nil); got != nil {
		t.Fatalf("nil err mapped to %v", got)
	}
}

func TestMapAWSErr_GenericNetwork(t *testing.T) {
	err := mapAWSErr(fmt.Errorf("dial tcp: connection refused"))
	if !errors.Is(err, dns.ErrUpstreamUnavailable) {
		t.Fatalf("want ErrUpstreamUnavailable, got %v", err)
	}
}

func TestMapAWSErr_HTTPStatus403(t *testing.T) {
	err := mapAWSErr(fmt.Errorf("dial returned http 403 forbidden"))
	if !errors.Is(err, dns.ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential, got %v", err)
	}
}

func TestMapAWSErr_HTTPStatus401(t *testing.T) {
	err := mapAWSErr(fmt.Errorf("recv: http 401"))
	if !errors.Is(err, dns.ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential, got %v", err)
	}
}

// ---- fake AWS Route 53 endpoint --------------------------------------------

// fakeR53Server emulates just enough of the Route 53 HTTP API (XML) to drive
// the AWS SDK end-to-end in tests. Only the endpoints we exercise are wired.
type fakeR53Server struct {
	mu sync.Mutex
	t  *testing.T

	hostedZones map[string]string // zone name (with trailing dot) -> id
}

func newFakeR53(t *testing.T) *httptest.Server {
	f := &fakeR53Server{
		t: t,
		hostedZones: map[string]string{
			"example.com.": "Z123ABC",
		},
	}
	return httptest.NewServer(http.HandlerFunc(f.serve))
}

func (f *fakeR53Server) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Drain body but ignore: we don't parse the change batch in detail.
	_, _ = io.Copy(io.Discard, r.Body)
	_ = r.Body.Close()

	w.Header().Set("Content-Type", "application/xml")
	path := r.URL.Path

	switch {
	case strings.HasSuffix(path, "/hostedzone") && r.Method == http.MethodGet && r.URL.Query().Get("dnsname") == "":
		// ListHostedZones
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>
<ListHostedZonesResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/">
  <HostedZones>
    <HostedZone>
      <Id>/hostedzone/Z123ABC</Id>
      <Name>example.com.</Name>
      <Config><PrivateZone>false</PrivateZone></Config>
    </HostedZone>
  </HostedZones>
  <IsTruncated>false</IsTruncated>
  <MaxItems>1</MaxItems>
</ListHostedZonesResponse>`)
		return

	case strings.HasSuffix(path, "/hostedzonesbyname") && r.Method == http.MethodGet:
		dnsName := r.URL.Query().Get("dnsname")
		want := strings.TrimSuffix(dnsName, ".") + "."
		id, ok := f.hostedZones[want]
		// Even if no match, return list with whatever zones are >= request;
		// the SDK + our code filter by exact name. We just emit one entry
		// if present.
		fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<ListHostedZonesByNameResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/">
  <HostedZones>`)
		if ok {
			fmt.Fprintf(w, `
    <HostedZone>
      <Id>/hostedzone/%s</Id>
      <Name>%s</Name>
      <Config><PrivateZone>false</PrivateZone></Config>
    </HostedZone>`, id, want)
		}
		fmt.Fprintf(w, `
  </HostedZones>
  <DNSName>%s</DNSName>
  <IsTruncated>false</IsTruncated>
  <MaxItems>100</MaxItems>
</ListHostedZonesByNameResponse>`, dnsName)
		return

	case strings.Contains(path, "/hostedzone/") && strings.HasSuffix(path, "/rrset") && r.Method == http.MethodPost:
		// ChangeResourceRecordSets
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>
<ChangeResourceRecordSetsResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/">
  <ChangeInfo>
    <Id>/change/C100</Id>
    <Status>INSYNC</Status>
    <SubmittedAt>2026-05-17T00:00:00Z</SubmittedAt>
  </ChangeInfo>
</ChangeResourceRecordSetsResponse>`)
		return
	}

	// Unknown path: 404 (helps debugging if SDK calls something unexpected).
	http.Error(w, "fakeR53: not found "+r.Method+" "+path, http.StatusNotFound)
}

// newFakeR53AccessDenied returns a server that always responds with an
// AccessDenied API error envelope (HTTP 403 + ErrorResponse XML).
func newFakeR53AccessDenied(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, errEnvelope("AccessDenied", "User is not authorized"))
	}))
}

func newFakeR53Throttling(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, errEnvelope("Throttling", "Rate exceeded"))
	}))
}

func newFakeR53Status(t *testing.T, status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
}

// errEnvelope formats a Route 53 ErrorResponse XML body that the SDK decodes
// into a smithy.APIError. The Type/Code/Message tags follow the standard
// REST-XML protocol envelope used by Route 53.
func errEnvelope(code, msg string) string {
	type errBody struct {
		XMLName xml.Name `xml:"ErrorResponse"`
		Error   struct {
			Type    string `xml:"Type"`
			Code    string `xml:"Code"`
			Message string `xml:"Message"`
		} `xml:"Error"`
		RequestId string `xml:"RequestId"`
	}
	b := errBody{}
	b.Error.Type = "Sender"
	b.Error.Code = code
	b.Error.Message = msg
	b.RequestId = "req-fake"
	out, _ := xml.MarshalIndent(b, "", "  ")
	return xml.Header + string(out)
}

// ---- defaults wiring: TTL / poll seen through BuildSolver ------------------

func TestSolver_TTLWired(t *testing.T) {
	p := New(Config{TTL: 600}).(*r53Provider)
	if p.cfg.TTL != 600 {
		t.Fatalf("TTL not wired")
	}
	sol, err := p.BuildSolver(context.Background(), map[string]string{
		"access_key_id":     fakeAKID,
		"secret_access_key": fakeSecret,
		"hosted_zone_id":    "Z",
	}, nil)
	if err != nil {
		t.Fatalf("BuildSolver: %v", err)
	}
	got := sol.(*r53Solver).ttl
	if got != 600 {
		t.Fatalf("solver ttl = %d, want 600", got)
	}
}

