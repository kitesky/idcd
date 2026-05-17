package aliyun

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	teatea "github.com/alibabacloud-go/tea/tea"

	"github.com/kite365/idcd/lib/cert/dns"
)

const (
	fakeAK = "LTAI4Gxxxxxxxxxxxxxxxxxx" // 24 chars
	fakeSK = "xxxxxxxxxxxxxxxxxxxxxxx0" // 24 chars
)

func validCred() map[string]string {
	return map[string]string{
		"access_key_id":     fakeAK,
		"access_key_secret": fakeSK,
		"region_id":         "cn-hangzhou",
	}
}

// ---- basic API surface ------------------------------------------------------

func TestKindIsAliyun(t *testing.T) {
	if New(Config{}).Kind() != dns.KindAliyun {
		t.Fatalf("expected Kind() == aliyun")
	}
}

func TestNewReturnsNonNil(t *testing.T) {
	if p := New(Config{}); p == nil {
		t.Fatalf("expected non-nil provider")
	}
}

func TestNewAppliesDefaults(t *testing.T) {
	p := New(Config{}).(*aliProvider)
	if p.cfg.PropagationTimeout != defaultPropagationTimeout {
		t.Fatalf("propagation default wrong: %v", p.cfg.PropagationTimeout)
	}
	if p.cfg.PollingInterval != defaultPollingInterval {
		t.Fatalf("polling default wrong: %v", p.cfg.PollingInterval)
	}
	if p.cfg.TTL != defaultTTL {
		t.Fatalf("ttl default wrong: %d", p.cfg.TTL)
	}
	if p.cfg.newClient == nil {
		t.Fatalf("newClient default missing")
	}
}

func TestNewKeepsExplicitConfig(t *testing.T) {
	cfg := Config{
		PropagationTimeout: 7 * time.Minute,
		PollingInterval:    11 * time.Second,
		TTL:                900,
	}
	p := New(cfg).(*aliProvider)
	if p.cfg.PropagationTimeout != 7*time.Minute {
		t.Fatalf("propagation override lost")
	}
	if p.cfg.PollingInterval != 11*time.Second {
		t.Fatalf("polling override lost")
	}
	if p.cfg.TTL != 900 {
		t.Fatalf("ttl override lost")
	}
}

// ---- ValidateCredential -----------------------------------------------------

func TestValidateCredential(t *testing.T) {
	p := New(Config{})
	cases := []struct {
		name string
		cred map[string]string
		want error
	}{
		{"empty map", map[string]string{}, dns.ErrInvalidCredential},
		{"missing ak", map[string]string{"access_key_secret": fakeSK}, dns.ErrInvalidCredential},
		{"missing sk", map[string]string{"access_key_id": fakeAK}, dns.ErrInvalidCredential},
		{"blank ak", map[string]string{"access_key_id": "   ", "access_key_secret": fakeSK}, dns.ErrInvalidCredential},
		{"blank sk", map[string]string{"access_key_id": fakeAK, "access_key_secret": "  "}, dns.ErrInvalidCredential},
		{"short ak", map[string]string{"access_key_id": "abc", "access_key_secret": fakeSK}, dns.ErrInvalidCredential},
		{"short sk", map[string]string{"access_key_id": fakeAK, "access_key_secret": "abc"}, dns.ErrInvalidCredential},
		{"ok", validCred(), nil},
		{"ok-no-region", map[string]string{"access_key_id": fakeAK, "access_key_secret": fakeSK}, nil},
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

// ---- regionOrDefault --------------------------------------------------------

func TestRegionOrDefault(t *testing.T) {
	if got := regionOrDefault(map[string]string{}); got != defaultRegionID {
		t.Fatalf("empty map: got %s want %s", got, defaultRegionID)
	}
	if got := regionOrDefault(map[string]string{"region_id": "  "}); got != defaultRegionID {
		t.Fatalf("blank region: got %s", got)
	}
	if got := regionOrDefault(map[string]string{"region_id": "us-east-1"}); got != "us-east-1" {
		t.Fatalf("explicit: got %s", got)
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

func TestBuildSolver_OK(t *testing.T) {
	stub := &stubClient{domains: []string{"example.com"}}
	p := New(Config{
		newClient: func(_ map[string]string) (txtClient, error) { return stub, nil },
	})
	solver, err := p.BuildSolver(context.Background(), validCred(), []string{"example.com"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if solver == nil {
		t.Fatalf("nil solver")
	}
	if got := solver.Timeout(); got != defaultPropagationTimeout {
		t.Fatalf("timeout: got %v want %v", got, defaultPropagationTimeout)
	}
}

func TestBuildSolver_ClientFactoryError(t *testing.T) {
	p := New(Config{
		newClient: func(_ map[string]string) (txtClient, error) {
			return nil, fmt.Errorf("%w: boom", dns.ErrUpstreamUnavailable)
		},
	})
	_, err := p.BuildSolver(context.Background(), validCred(), nil)
	if !errors.Is(err, dns.ErrUpstreamUnavailable) {
		t.Fatalf("want ErrUpstreamUnavailable, got %v", err)
	}
}

// ---- HealthCheck ------------------------------------------------------------

func TestHealthCheck_BadCred(t *testing.T) {
	p := New(Config{})
	err := p.HealthCheck(context.Background(), map[string]string{})
	if !errors.Is(err, dns.ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential, got %v", err)
	}
}

func TestHealthCheck_OK(t *testing.T) {
	stub := &stubClient{domains: []string{"example.com"}}
	p := New(Config{
		newClient: func(_ map[string]string) (txtClient, error) { return stub, nil },
	})
	if err := p.HealthCheck(context.Background(), validCred()); err != nil {
		t.Fatalf("healthcheck: %v", err)
	}
	if stub.describeCount != 1 {
		t.Fatalf("expected 1 describe call, got %d", stub.describeCount)
	}
}

func TestHealthCheck_AuthError(t *testing.T) {
	stub := &stubClient{describeErr: fakeSDKErr(403, "Forbidden.RAM", "no perm")}
	p := New(Config{
		newClient: func(_ map[string]string) (txtClient, error) { return stub, nil },
	})
	err := p.HealthCheck(context.Background(), validCred())
	if !errors.Is(err, dns.ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential, got %v", err)
	}
}

func TestHealthCheck_UpstreamError(t *testing.T) {
	stub := &stubClient{describeErr: fakeSDKErr(500, "ServiceUnavailable", "down")}
	p := New(Config{
		newClient: func(_ map[string]string) (txtClient, error) { return stub, nil },
	})
	err := p.HealthCheck(context.Background(), validCred())
	if !errors.Is(err, dns.ErrUpstreamUnavailable) {
		t.Fatalf("want ErrUpstreamUnavailable, got %v", err)
	}
}

func TestHealthCheck_ClientFactoryError(t *testing.T) {
	p := New(Config{
		newClient: func(_ map[string]string) (txtClient, error) {
			return nil, fmt.Errorf("%w: boom", dns.ErrUpstreamUnavailable)
		},
	})
	err := p.HealthCheck(context.Background(), validCred())
	if !errors.Is(err, dns.ErrUpstreamUnavailable) {
		t.Fatalf("want ErrUpstreamUnavailable, got %v", err)
	}
}

// ---- Solver Present/CleanUp/Timeout -----------------------------------------

func TestSolver_Present_CleanUp_Happy(t *testing.T) {
	stub := &stubClient{domains: []string{"example.com"}}
	solver := newSolver(t, stub)
	const fqdn = "_acme-challenge.example.com."
	const value = "txt-value-xyz"
	if err := solver.Present(context.Background(), fqdn, value); err != nil {
		t.Fatalf("present: %v", err)
	}
	if got := stub.txtCount(); got != 1 {
		t.Fatalf("after Present: want 1 record, got %d", got)
	}
	if err := solver.CleanUp(context.Background(), fqdn, value); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if got := stub.txtCount(); got != 0 {
		t.Fatalf("after CleanUp: want 0 records, got %d", got)
	}
}

func TestSolver_Present_NestedSubdomain(t *testing.T) {
	stub := &stubClient{domains: []string{"example.com"}}
	solver := newSolver(t, stub)
	// _acme-challenge.www.example.com → zone=example.com, rr=_acme-challenge.www
	if err := solver.Present(context.Background(), "_acme-challenge.www.example.com.", "v"); err != nil {
		t.Fatalf("present: %v", err)
	}
	rec, ok := stub.lastRec()
	if !ok {
		t.Fatalf("no record stored")
	}
	if rec.zone != "example.com" {
		t.Fatalf("zone: got %s want example.com", rec.zone)
	}
	if rec.rr != "_acme-challenge.www" {
		t.Fatalf("rr: got %s want _acme-challenge.www", rec.rr)
	}
	if rec.ttl != defaultTTL {
		t.Fatalf("ttl: got %d want %d", rec.ttl, defaultTTL)
	}
}

func TestSolver_Present_NoZone(t *testing.T) {
	stub := &stubClient{domains: []string{"otherdomain.com"}}
	solver := newSolver(t, stub)
	err := solver.Present(context.Background(), "_acme-challenge.unknown.test.", "v")
	if !errors.Is(err, dns.ErrZoneNotFound) {
		t.Fatalf("want ErrZoneNotFound, got %v", err)
	}
}

func TestSolver_Present_ApexZone(t *testing.T) {
	// fqdn = _acme-challenge.example.com. → rr = _acme-challenge
	stub := &stubClient{domains: []string{"example.com"}}
	solver := newSolver(t, stub)
	if err := solver.Present(context.Background(), "_acme-challenge.example.com.", "vv"); err != nil {
		t.Fatalf("present: %v", err)
	}
	rec, _ := stub.lastRec()
	if rec.rr != "_acme-challenge" {
		t.Fatalf("rr: got %s", rec.rr)
	}
}

func TestSolver_Present_EmptyDomains(t *testing.T) {
	stub := &stubClient{domains: nil}
	solver := newSolver(t, stub)
	err := solver.Present(context.Background(), "_acme-challenge.example.com.", "v")
	if !errors.Is(err, dns.ErrZoneNotFound) {
		t.Fatalf("want ErrZoneNotFound, got %v", err)
	}
}

func TestSolver_Present_TooShortFQDN(t *testing.T) {
	stub := &stubClient{domains: []string{"example.com"}}
	solver := newSolver(t, stub)
	err := solver.Present(context.Background(), "localhost.", "v")
	if !errors.Is(err, dns.ErrZoneNotFound) {
		t.Fatalf("want ErrZoneNotFound, got %v", err)
	}
}

func TestSolver_Present_AddError(t *testing.T) {
	stub := &stubClient{
		domains: []string{"example.com"},
		addErr:  fakeSDKErr(0, "InvalidAccessKeyId.NotFound", "no key"),
	}
	solver := newSolver(t, stub)
	err := solver.Present(context.Background(), "_acme-challenge.example.com.", "v")
	if !errors.Is(err, dns.ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential, got %v", err)
	}
}

func TestSolver_CleanUp_NoMatch(t *testing.T) {
	// CleanUp on something that was never added: list returns empty, no
	// records to delete — should not error.
	stub := &stubClient{domains: []string{"example.com"}}
	solver := newSolver(t, stub)
	if err := solver.CleanUp(context.Background(), "_acme-challenge.example.com.", "ghost"); err != nil {
		t.Fatalf("cleanup empty: %v", err)
	}
}

func TestSolver_CleanUp_ListError(t *testing.T) {
	stub := &stubClient{
		domains: []string{"example.com"},
		listErr: fakeSDKErr(500, "InternalError", "oops"),
	}
	solver := newSolver(t, stub)
	err := solver.CleanUp(context.Background(), "_acme-challenge.example.com.", "v")
	if !errors.Is(err, dns.ErrUpstreamUnavailable) {
		t.Fatalf("want ErrUpstreamUnavailable, got %v", err)
	}
}

func TestSolver_CleanUp_DeleteError(t *testing.T) {
	stub := &stubClient{domains: []string{"example.com"}}
	// pre-populate one matching record
	stub.preAdd("example.com", "_acme-challenge", "v", "rec-1")
	stub.delErr = fakeSDKErr(403, "Forbidden", "denied")
	solver := newSolver(t, stub)
	err := solver.CleanUp(context.Background(), "_acme-challenge.example.com.", "v")
	if !errors.Is(err, dns.ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential, got %v", err)
	}
}

func TestSolver_Timeout(t *testing.T) {
	stub := &stubClient{domains: []string{"example.com"}}
	p := New(Config{
		PropagationTimeout: 42 * time.Second,
		newClient:          func(_ map[string]string) (txtClient, error) { return stub, nil },
	})
	solver, err := p.BuildSolver(context.Background(), validCred(), nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := solver.Timeout(); got != 42*time.Second {
		t.Fatalf("timeout: got %v want 42s", got)
	}
}

// ---- error mapping ----------------------------------------------------------

func TestMapSDKError_Nil(t *testing.T) {
	if err := mapSDKError(nil); err != nil {
		t.Fatalf("nil → got %v", err)
	}
}

func TestMapSDKError_StatusBuckets(t *testing.T) {
	cases := []struct {
		name   string
		status int
		code   string
		want   error
	}{
		{"401", 401, "Unauthorized", dns.ErrInvalidCredential},
		{"403", 403, "Forbidden", dns.ErrInvalidCredential},
		{"404", 404, "DomainNotExists", dns.ErrZoneNotFound},
		{"500", 500, "InternalError", dns.ErrUpstreamUnavailable},
		{"502", 502, "BadGateway", dns.ErrUpstreamUnavailable},
		{"200 with auth code", 0, "SignatureDoesNotMatch", dns.ErrInvalidCredential},
		{"200 with zone code", 0, "DomainRecordNotBelongToUser", dns.ErrZoneNotFound},
		{"200 with unknown code", 0, "RandomThing", dns.ErrUpstreamUnavailable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := mapSDKError(fakeSDKErr(c.status, c.code, "msg"))
			if !errors.Is(err, c.want) {
				t.Fatalf("want %v, got %v", c.want, err)
			}
		})
	}
}

func TestMapSDKError_ContextCanceled(t *testing.T) {
	if err := mapSDKError(context.Canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("ctx.Canceled lost: %v", err)
	}
	if err := mapSDKError(context.DeadlineExceeded); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("ctx.DeadlineExceeded lost: %v", err)
	}
}

func TestMapSDKError_PlainError(t *testing.T) {
	err := mapSDKError(errors.New("io: connection refused"))
	if !errors.Is(err, dns.ErrUpstreamUnavailable) {
		t.Fatalf("plain err: want ErrUpstreamUnavailable, got %v", err)
	}
}

func TestSafeMsg(t *testing.T) {
	if safeMsg(nil) != "" {
		t.Fatalf("nil sdkerr → expected empty")
	}
	if safeMsg(&teatea.SDKError{}) != "" {
		t.Fatalf("nil message → expected empty")
	}
	short := "short"
	if safeMsg(&teatea.SDKError{Message: &short}) != "short" {
		t.Fatalf("short pass-through")
	}
	long := strings.Repeat("a", 500)
	if got := safeMsg(&teatea.SDKError{Message: &long}); len(got) <= 200 || !strings.HasSuffix(got, "...") {
		t.Fatalf("long should truncate w/ ellipsis, got len=%d", len(got))
	}
}

func TestIsAuthErrorCode(t *testing.T) {
	if !isAuthErrorCode("InvalidAccessKeyId") {
		t.Fatalf("InvalidAccessKeyId should be auth")
	}
	if !isAuthErrorCode("Forbidden.RAM") {
		t.Fatalf("Forbidden.RAM should be auth")
	}
	if isAuthErrorCode("RandomOther") {
		t.Fatalf("RandomOther not auth")
	}
}

func TestIsZoneErrorCode(t *testing.T) {
	if !isZoneErrorCode("DomainNotExists") {
		t.Fatalf("DomainNotExists should be zone")
	}
	if isZoneErrorCode("RandomOther") {
		t.Fatalf("RandomOther not zone")
	}
}

// ---- realClientFactory smoke ------------------------------------------------

func TestRealClientFactory_Builds(t *testing.T) {
	// Smoke test: production factory must not return error for a well-formed
	// credential (it doesn't talk to network on construction).
	c, err := realClientFactory(validCred())
	if err != nil {
		t.Fatalf("realClientFactory: %v", err)
	}
	if c == nil {
		t.Fatalf("nil client from realClientFactory")
	}
	if _, ok := c.(*realClient); !ok {
		t.Fatalf("expected *realClient, got %T", c)
	}
}

// ---- helpers ----------------------------------------------------------------

func newSolver(t *testing.T, stub *stubClient) *aliSolver {
	t.Helper()
	p := New(Config{
		newClient: func(_ map[string]string) (txtClient, error) { return stub, nil },
	})
	solver, err := p.BuildSolver(context.Background(), validCred(), nil)
	if err != nil {
		t.Fatalf("build solver: %v", err)
	}
	return solver.(*aliSolver)
}

// fakeSDKErr constructs a *tea.SDKError with the given status/code/message.
func fakeSDKErr(status int, code, msg string) *teatea.SDKError {
	e := &teatea.SDKError{}
	if status != 0 {
		s := status
		e.StatusCode = &s
	}
	c := code
	e.Code = &c
	m := msg
	e.Message = &m
	return e
}

// stubClient implements txtClient for unit tests.
type stubClient struct {
	mu sync.Mutex

	domains []string

	// errors to inject
	describeErr error
	addErr      error
	listErr     error
	delErr      error

	describeCount int
	nextID        int
	records       []stubRec
}

type stubRec struct {
	id    string
	zone  string
	rr    string
	value string
	ttl   int
}

func (s *stubClient) describeDomains(_ context.Context) error {
	s.mu.Lock()
	s.describeCount++
	err := s.describeErr
	s.mu.Unlock()
	return mapSDKError(err)
}

func (s *stubClient) resolveZone(_ context.Context, fqdn string) (string, string, error) {
	if s.describeErr != nil {
		// describeDomains underneath
	}
	name := strings.TrimSuffix(fqdn, ".")
	labels := strings.Split(name, ".")
	if len(labels) < 2 {
		return "", "", fmt.Errorf("%w: too short", dns.ErrZoneNotFound)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.domains) == 0 {
		return "", "", fmt.Errorf("%w: no domains", dns.ErrZoneNotFound)
	}
	for i := 0; i < len(labels)-1; i++ {
		candidate := strings.Join(labels[i:], ".")
		for _, z := range s.domains {
			if strings.EqualFold(candidate, z) {
				rr := strings.TrimSuffix(strings.Join(labels[:i], "."), ".")
				if rr == "" {
					rr = "@"
				}
				return z, rr, nil
			}
		}
	}
	return "", "", fmt.Errorf("%w: %s", dns.ErrZoneNotFound, fqdn)
}

func (s *stubClient) addTXT(_ context.Context, zone, rr, value string, ttl int) error {
	if s.addErr != nil {
		return mapSDKError(s.addErr)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	s.records = append(s.records, stubRec{
		id:    fmt.Sprintf("rec-%d", s.nextID),
		zone:  zone, rr: rr, value: value, ttl: ttl,
	})
	return nil
}

func (s *stubClient) listMatchingTXT(_ context.Context, zone, rr, value string) ([]string, error) {
	if s.listErr != nil {
		return nil, mapSDKError(s.listErr)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []string
	for _, r := range s.records {
		if r.zone == zone && strings.EqualFold(r.rr, rr) && r.value == value {
			ids = append(ids, r.id)
		}
	}
	return ids, nil
}

func (s *stubClient) deleteRecord(_ context.Context, id string) error {
	if s.delErr != nil {
		return mapSDKError(s.delErr)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.records[:0]
	for _, r := range s.records {
		if r.id != id {
			out = append(out, r)
		}
	}
	s.records = out
	return nil
}

func (s *stubClient) txtCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.records)
}

func (s *stubClient) lastRec() (stubRec, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.records) == 0 {
		return stubRec{}, false
	}
	return s.records[len(s.records)-1], true
}

func (s *stubClient) preAdd(zone, rr, value, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, stubRec{id: id, zone: zone, rr: rr, value: value, ttl: 600})
}
