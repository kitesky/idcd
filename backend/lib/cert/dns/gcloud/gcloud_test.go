package gcloud

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kite365/idcd/lib/cert/dns"
)

// testPrivateKey is generated once per test process — google.JWTConfigFromJSON
// only validates PEM/PKCS#8 structure, and httptest intercepts the token
// exchange before any real signature is verified. Generating at runtime
// keeps a PEM-shaped string out of source control where secret scanners
// flag it.
var testPrivateKey = mustGenerateTestPrivateKey()

func mustGenerateTestPrivateKey() string {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(fmt.Sprintf("gen test key: %v", err))
	}
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		panic(fmt.Sprintf("marshal test key: %v", err))
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

// buildSA 返回一个合法 service account JSON。
// tokenURI 决定 oauth2 找谁取 token —— 测试 httptest 既要 serve Cloud DNS
// 路径，也要 serve token 路径。
func buildSA(t *testing.T, tokenURI string) string {
	t.Helper()
	sa := map[string]string{
		"type":           "service_account",
		"project_id":     "test-project",
		"private_key_id": "test-kid",
		"private_key":    testPrivateKey,
		"client_email":   "test@test-project.iam.gserviceaccount.com",
		"client_id":      "1234567890",
		"token_uri":      tokenURI,
	}
	b, err := json.Marshal(sa)
	if err != nil {
		t.Fatalf("marshal sa: %v", err)
	}
	return string(b)
}

// ---- New / Kind -------------------------------------------------------------

func TestNew_NonNil(t *testing.T) {
	if p := New(Config{}); p == nil {
		t.Fatal("New returned nil")
	}
}

func TestKindIsGCloud(t *testing.T) {
	if New(Config{}).Kind() != dns.KindGCloud {
		t.Fatalf("wrong kind")
	}
}

func TestNew_AppliesDefaults(t *testing.T) {
	p := New(Config{}).(*gcloudProvider)
	if p.cfg.PropagationTimeout != defaultPropagationTimeout {
		t.Fatalf("propagation timeout default: %v", p.cfg.PropagationTimeout)
	}
	if p.cfg.PollingInterval != defaultPollingInterval {
		t.Fatalf("polling interval default: %v", p.cfg.PollingInterval)
	}
	if p.cfg.TTL != defaultTTL {
		t.Fatalf("ttl default: %d", p.cfg.TTL)
	}
}

func TestNew_RespectsConfig(t *testing.T) {
	cfg := Config{
		PropagationTimeout: 1 * time.Minute,
		PollingInterval:    1 * time.Second,
		TTL:                42,
	}
	p := New(cfg).(*gcloudProvider)
	if p.cfg.PropagationTimeout != 1*time.Minute || p.cfg.PollingInterval != 1*time.Second || p.cfg.TTL != 42 {
		t.Fatalf("config not respected: %+v", p.cfg)
	}
}

// ---- ValidateCredential -----------------------------------------------------

func TestValidateCredential(t *testing.T) {
	validSA := buildSA(t, "https://oauth2.googleapis.com/token")

	// 用 map 拼变种 SA。
	saWithout := func(field string) string {
		var m map[string]string
		_ = json.Unmarshal([]byte(validSA), &m)
		delete(m, field)
		b, _ := json.Marshal(m)
		return string(b)
	}

	cases := []struct {
		name string
		cred map[string]string
		want error
	}{
		{"empty map", map[string]string{}, dns.ErrInvalidCredential},
		{"empty sa json", map[string]string{"service_account_json": ""}, dns.ErrInvalidCredential},
		{"whitespace sa json", map[string]string{"service_account_json": "   "}, dns.ErrInvalidCredential},
		{"not json", map[string]string{"service_account_json": "not-json"}, dns.ErrInvalidCredential},
		{"missing client_email", map[string]string{"service_account_json": saWithout("client_email")}, dns.ErrInvalidCredential},
		{"missing private_key", map[string]string{"service_account_json": saWithout("private_key")}, dns.ErrInvalidCredential},
		{"missing project_id", map[string]string{"service_account_json": saWithout("project_id")}, dns.ErrInvalidCredential},
		{"empty project_id override", map[string]string{"service_account_json": validSA, "project_id": "   "}, dns.ErrInvalidCredential},
		{"ok minimal", map[string]string{"service_account_json": validSA}, nil},
		{"ok with project override", map[string]string{"service_account_json": validSA, "project_id": "override"}, nil},
	}

	p := New(Config{})
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

// ---- BuildSolver ------------------------------------------------------------

func TestBuildSolver_BadCred(t *testing.T) {
	p := New(Config{})
	_, err := p.BuildSolver(context.Background(), map[string]string{}, nil)
	if !errors.Is(err, dns.ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential, got %v", err)
	}
}

func TestBuildSolver_OK(t *testing.T) {
	// 不会发起任何网络调用：BuildSolver 只构造 client，不调 API。
	cred := map[string]string{
		"service_account_json": buildSA(t, "https://oauth2.googleapis.com/token"),
	}
	p := New(Config{PropagationTimeout: 30 * time.Second})
	solver, err := p.BuildSolver(context.Background(), cred, []string{"example.com"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if solver == nil {
		t.Fatal("nil solver")
	}
	if solver.Timeout() != 30*time.Second {
		t.Fatalf("timeout: %v", solver.Timeout())
	}
}

func TestBuildSolver_ProjectFallsBackToSAField(t *testing.T) {
	cred := map[string]string{
		"service_account_json": buildSA(t, "https://oauth2.googleapis.com/token"),
	}
	p := New(Config{}).(*gcloudProvider)
	solver, err := p.BuildSolver(context.Background(), cred, nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	gs := solver.(*gcloudSolver)
	if gs.project != "test-project" {
		t.Fatalf("project mismatch: %q", gs.project)
	}
}

func TestBuildSolver_ProjectOverride(t *testing.T) {
	cred := map[string]string{
		"service_account_json": buildSA(t, "https://oauth2.googleapis.com/token"),
		"project_id":           "other-project",
	}
	p := New(Config{}).(*gcloudProvider)
	solver, err := p.BuildSolver(context.Background(), cred, nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	gs := solver.(*gcloudSolver)
	if gs.project != "other-project" {
		t.Fatalf("override project not used: %q", gs.project)
	}
}

// ---- HealthCheck ------------------------------------------------------------

// fakeGCloud 拦截 oauth2 token + Cloud DNS API 两端流量。
type fakeGCloud struct {
	mu          sync.Mutex
	zonesStatus int    // ManagedZones.List 返回的 HTTP code
	zonesBody   string // 对应 body（默认空 list）

	// 录制收到的请求路径
	paths    []string
	tokenHit atomic.Int32
}

func newFakeGCloud() (*fakeGCloud, *httptest.Server) {
	f := &fakeGCloud{
		zonesStatus: 200,
		zonesBody:   `{"managedZones":[]}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	return f, srv
}

func (f *fakeGCloud) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.paths = append(f.paths, r.URL.Path)
	zonesStatus := f.zonesStatus
	zonesBody := f.zonesBody
	f.mu.Unlock()

	switch {
	case r.URL.Path == "/token":
		f.tokenHit.Add(1)
		w.Header().Set("Content-Type", "application/json")
		// JWTConfig 期望标准 oauth2 access token 响应。
		_, _ = w.Write([]byte(`{"access_token":"fake","token_type":"Bearer","expires_in":3600}`))
		return

	case strings.HasSuffix(r.URL.Path, "/managedZones") && r.Method == http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(zonesStatus)
		_, _ = w.Write([]byte(zonesBody))
		return
	}
	http.NotFound(w, r)
}

func TestHealthCheck_BadCred(t *testing.T) {
	p := New(Config{})
	err := p.HealthCheck(context.Background(), map[string]string{})
	if !errors.Is(err, dns.ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential, got %v", err)
	}
}

func TestHealthCheck_OK(t *testing.T) {
	fake, srv := newFakeGCloud()
	defer srv.Close()
	p := New(Config{Endpoint: srv.URL})
	cred := map[string]string{
		"service_account_json": buildSA(t, srv.URL+"/token"),
	}
	if err := p.HealthCheck(context.Background(), cred); err != nil {
		t.Fatalf("healthcheck: %v", err)
	}
	if fake.tokenHit.Load() == 0 {
		t.Fatalf("expected oauth2 token endpoint hit")
	}
}

func TestHealthCheck_Forbidden(t *testing.T) {
	fake, srv := newFakeGCloud()
	defer srv.Close()
	fake.mu.Lock()
	fake.zonesStatus = 403
	fake.zonesBody = `{"error":{"code":403,"message":"forbidden"}}`
	fake.mu.Unlock()

	p := New(Config{Endpoint: srv.URL})
	cred := map[string]string{
		"service_account_json": buildSA(t, srv.URL+"/token"),
	}
	err := p.HealthCheck(context.Background(), cred)
	if !errors.Is(err, dns.ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential, got %v", err)
	}
}

func TestHealthCheck_Unauthorized(t *testing.T) {
	fake, srv := newFakeGCloud()
	defer srv.Close()
	fake.mu.Lock()
	fake.zonesStatus = 401
	fake.zonesBody = `{"error":{"code":401,"message":"unauthorized"}}`
	fake.mu.Unlock()

	p := New(Config{Endpoint: srv.URL})
	cred := map[string]string{
		"service_account_json": buildSA(t, srv.URL+"/token"),
	}
	err := p.HealthCheck(context.Background(), cred)
	if !errors.Is(err, dns.ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential, got %v", err)
	}
}

func TestHealthCheck_ServerError(t *testing.T) {
	fake, srv := newFakeGCloud()
	defer srv.Close()
	fake.mu.Lock()
	fake.zonesStatus = 500
	fake.zonesBody = `{"error":{"code":500,"message":"boom"}}`
	fake.mu.Unlock()

	p := New(Config{Endpoint: srv.URL})
	cred := map[string]string{
		"service_account_json": buildSA(t, srv.URL+"/token"),
	}
	err := p.HealthCheck(context.Background(), cred)
	if !errors.Is(err, dns.ErrUpstreamUnavailable) {
		t.Fatalf("want ErrUpstreamUnavailable, got %v", err)
	}
}

// ---- Solver Present / CleanUp / Timeout -------------------------------------

type fakeFullGCloud struct {
	mu      sync.Mutex
	zoneID  string // managedZone Name 字段（Cloud DNS 的 zone Name 通常是 "zone-foo"，不是 DNS name）
	dnsName string // managed zone 的 dnsName（带末尾点）
	// rrsets keyed by (rrName, rrType)
	rrsets map[string]*rrEntry

	changeID atomic.Int64
}

type rrEntry struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	TTL     int64    `json:"ttl"`
	Rrdatas []string `json:"rrdatas"`
}

func newFakeFullGCloud() (*fakeFullGCloud, *httptest.Server) {
	f := &fakeFullGCloud{
		zoneID:  "zone-example",
		dnsName: "example.com.",
		rrsets:  map[string]*rrEntry{},
	}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	return f, srv
}

func rrKey(name, typ string) string { return name + "|" + typ }

func (f *fakeFullGCloud) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.URL.Path == "/token" {
		_, _ = w.Write([]byte(`{"access_token":"fake","token_type":"Bearer","expires_in":3600}`))
		return
	}

	// dns/v1/projects/{project}/managedZones — list zones
	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/managedZones") {
		f.mu.Lock()
		defer f.mu.Unlock()
		wantDNSName := r.URL.Query().Get("dnsName")
		resp := struct {
			ManagedZones []map[string]any `json:"managedZones"`
		}{}
		if wantDNSName == f.dnsName || wantDNSName == "" {
			resp.ManagedZones = append(resp.ManagedZones, map[string]any{
				"name":       f.zoneID,
				"dnsName":    f.dnsName,
				"visibility": "public",
			})
		}
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	// .../managedZones/<zoneID>/rrsets — list records
	if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/rrsets") {
		f.mu.Lock()
		defer f.mu.Unlock()
		filterName := r.URL.Query().Get("name")
		filterType := r.URL.Query().Get("type")
		resp := struct {
			Rrsets []*rrEntry `json:"rrsets"`
		}{}
		for _, rr := range f.rrsets {
			if filterName != "" && rr.Name != filterName {
				continue
			}
			if filterType != "" && rr.Type != filterType {
				continue
			}
			resp.Rrsets = append(resp.Rrsets, rr)
		}
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	// .../managedZones/<zoneID>/changes — apply change
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/changes") {
		var change struct {
			Additions []*rrEntry `json:"additions"`
			Deletions []*rrEntry `json:"deletions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&change); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		f.mu.Lock()
		for _, d := range change.Deletions {
			delete(f.rrsets, rrKey(d.Name, d.Type))
		}
		for _, a := range change.Additions {
			cp := *a
			f.rrsets[rrKey(a.Name, a.Type)] = &cp
		}
		f.mu.Unlock()
		id := f.changeID.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     fmt.Sprintf("ch-%d", id),
			"status": "done",
		})
		return
	}

	http.NotFound(w, r)
}

func newProviderForFake(srv *httptest.Server) dns.Provider {
	return New(Config{Endpoint: srv.URL, PropagationTimeout: 5 * time.Second})
}

func TestSolver_Timeout(t *testing.T) {
	_, srv := newFakeFullGCloud()
	defer srv.Close()
	p := newProviderForFake(srv)
	cred := map[string]string{
		"service_account_json": buildSA(t, srv.URL+"/token"),
	}
	solver, err := p.BuildSolver(context.Background(), cred, nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if solver.Timeout() != 5*time.Second {
		t.Fatalf("timeout: %v", solver.Timeout())
	}
}

func TestSolver_PresentCleanUp(t *testing.T) {
	fake, srv := newFakeFullGCloud()
	defer srv.Close()
	p := newProviderForFake(srv)
	cred := map[string]string{
		"service_account_json": buildSA(t, srv.URL+"/token"),
	}
	solver, err := p.BuildSolver(context.Background(), cred, []string{"example.com"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	const fqdn = "_acme-challenge.example.com."
	const value = "test-value"

	if err := solver.Present(context.Background(), fqdn, value); err != nil {
		t.Fatalf("present: %v", err)
	}
	fake.mu.Lock()
	rec, ok := fake.rrsets[rrKey(fqdn, "TXT")]
	fake.mu.Unlock()
	if !ok {
		t.Fatalf("expected TXT record present")
	}
	if len(rec.Rrdatas) != 1 || rec.Rrdatas[0] != `"`+value+`"` {
		t.Fatalf("unexpected rrdatas: %v", rec.Rrdatas)
	}

	if err := solver.CleanUp(context.Background(), fqdn, value); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	fake.mu.Lock()
	_, stillThere := fake.rrsets[rrKey(fqdn, "TXT")]
	fake.mu.Unlock()
	if stillThere {
		t.Fatalf("expected TXT removed after cleanup")
	}
}

func TestSolver_Present_NoZone(t *testing.T) {
	fake, srv := newFakeFullGCloud()
	defer srv.Close()
	fake.mu.Lock()
	fake.dnsName = "other.test." // 不匹配 example.com
	fake.mu.Unlock()

	p := newProviderForFake(srv)
	cred := map[string]string{
		"service_account_json": buildSA(t, srv.URL+"/token"),
	}
	solver, err := p.BuildSolver(context.Background(), cred, nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	err = solver.Present(context.Background(), "_acme-challenge.example.com.", "v")
	if !errors.Is(err, dns.ErrZoneNotFound) {
		t.Fatalf("want ErrZoneNotFound, got %v", err)
	}
}

func TestSolver_Present_TooShortFQDN(t *testing.T) {
	_, srv := newFakeFullGCloud()
	defer srv.Close()
	p := newProviderForFake(srv)
	cred := map[string]string{
		"service_account_json": buildSA(t, srv.URL+"/token"),
	}
	solver, err := p.BuildSolver(context.Background(), cred, nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	err = solver.Present(context.Background(), "localhost.", "v")
	if !errors.Is(err, dns.ErrZoneNotFound) {
		t.Fatalf("want ErrZoneNotFound, got %v", err)
	}
}

func TestSolver_CleanUp_PreservesOtherValues(t *testing.T) {
	fake, srv := newFakeFullGCloud()
	defer srv.Close()

	// 预置一条带两个 rrdata 的 TXT 记录，模拟同名 TXT 上有其它无关 challenge。
	fake.mu.Lock()
	fake.rrsets[rrKey("_acme-challenge.example.com.", "TXT")] = &rrEntry{
		Name:    "_acme-challenge.example.com.",
		Type:    "TXT",
		TTL:     120,
		Rrdatas: []string{`"keep-me"`, `"to-remove"`},
	}
	fake.mu.Unlock()

	p := newProviderForFake(srv)
	cred := map[string]string{
		"service_account_json": buildSA(t, srv.URL+"/token"),
	}
	solver, err := p.BuildSolver(context.Background(), cred, nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := solver.CleanUp(context.Background(), "_acme-challenge.example.com.", "to-remove"); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	fake.mu.Lock()
	rec, ok := fake.rrsets[rrKey("_acme-challenge.example.com.", "TXT")]
	fake.mu.Unlock()
	if !ok {
		t.Fatalf("expected record to still exist with remaining rrdata")
	}
	if len(rec.Rrdatas) != 1 || rec.Rrdatas[0] != `"keep-me"` {
		t.Fatalf("unexpected remaining rrdatas: %v", rec.Rrdatas)
	}
}

// ---- helpers / smoke --------------------------------------------------------

func TestDnsFqdn(t *testing.T) {
	if got := dnsFqdn("example.com"); got != "example.com." {
		t.Fatalf("got %q", got)
	}
	if got := dnsFqdn("example.com."); got != "example.com." {
		t.Fatalf("got %q", got)
	}
}

func TestQuoteTXT(t *testing.T) {
	if got := quoteTXT("hello"); got != `"hello"` {
		t.Fatalf("got %q", got)
	}
	if got := quoteTXT(`"hello"`); got != `"hello"` {
		t.Fatalf("got %q", got)
	}
}

func TestResolveProject(t *testing.T) {
	sa := serviceAccountKey{ProjectID: "from-sa"}
	if got := resolveProject(map[string]string{}, sa); got != "from-sa" {
		t.Fatalf("got %q", got)
	}
	if got := resolveProject(map[string]string{"project_id": "override"}, sa); got != "override" {
		t.Fatalf("got %q", got)
	}
	if got := resolveProject(map[string]string{"project_id": "   "}, sa); got != "from-sa" {
		t.Fatalf("got %q for whitespace override", got)
	}
}
