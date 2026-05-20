package dnspod

import (
	"context"
	"encoding/json"
	"errors"
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
	fakeSecretID  = "AKIDxxxxxxxxxxxxxxxxxx"   // 22 chars
	fakeSecretKey = "abcdef0123456789abcdef01" // 24 chars
)

func okCred() map[string]string {
	return map[string]string{
		"secret_id":  fakeSecretID,
		"secret_key": fakeSecretKey,
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
		{"missing secret_key", map[string]string{"secret_id": fakeSecretID}, dns.ErrInvalidCredential},
		{"empty secret_id", map[string]string{"secret_id": "", "secret_key": fakeSecretKey}, dns.ErrInvalidCredential},
		{"empty secret_key", map[string]string{"secret_id": fakeSecretID, "secret_key": ""}, dns.ErrInvalidCredential},
		{"short secret_id", map[string]string{"secret_id": "abc", "secret_key": fakeSecretKey}, dns.ErrInvalidCredential},
		{"short secret_key", map[string]string{"secret_id": fakeSecretID, "secret_key": "abc"}, dns.ErrInvalidCredential},
		{"ok", okCred(), nil},
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

func TestNewReturnsNonNil(t *testing.T) {
	if New(Config{}) == nil {
		t.Fatal("New returned nil")
	}
}

func TestKindIsDNSPod(t *testing.T) {
	if New(Config{}).Kind() != dns.KindDNSPod {
		t.Fatalf("wrong kind")
	}
	if dns.KindDNSPod != "dnspod" {
		t.Fatalf("KindDNSPod constant changed: %q", dns.KindDNSPod)
	}
}

func TestConfigDefaults(t *testing.T) {
	p := New(Config{}).(*dpProvider)
	if p.propagationTimeout != defaultPropagationTimeout {
		t.Errorf("default propagation timeout = %v", p.propagationTimeout)
	}
	if p.pollingInterval != defaultPollingInterval {
		t.Errorf("default polling interval = %v", p.pollingInterval)
	}
	if p.ttl != defaultTTL {
		t.Errorf("default ttl = %d", p.ttl)
	}
	if p.baseURL != defaultBaseURL {
		t.Errorf("default baseURL = %q", p.baseURL)
	}
}

func TestConfigOverrides(t *testing.T) {
	p := New(Config{
		BaseURL:            "http://custom/",
		PropagationTimeout: 17 * time.Second,
		PollingInterval:    13 * time.Second,
		TTL:                42,
	}).(*dpProvider)
	if p.baseURL != "http://custom" {
		t.Errorf("baseURL = %q", p.baseURL)
	}
	if p.propagationTimeout != 17*time.Second {
		t.Errorf("propagation = %v", p.propagationTimeout)
	}
	if p.pollingInterval != 13*time.Second {
		t.Errorf("polling = %v", p.pollingInterval)
	}
	if p.ttl != 42 {
		t.Errorf("ttl = %d", p.ttl)
	}
}

// ---- BuildSolver bad cred ---------------------------------------------------

func TestBuildSolver_BadCred(t *testing.T) {
	p := New(Config{})
	_, err := p.BuildSolver(context.Background(), map[string]string{}, nil)
	if !errors.Is(err, dns.ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential, got %v", err)
	}
}

func TestBuildSolver_OK(t *testing.T) {
	p := New(Config{})
	solver, err := p.BuildSolver(context.Background(), okCred(), nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if solver == nil {
		t.Fatal("nil solver")
	}
	if solver.Timeout() != defaultPropagationTimeout {
		t.Errorf("solver.Timeout = %v", solver.Timeout())
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
			body:    `{"Response":{"DomainList":[],"RequestId":"abc"}}`,
			wantErr: nil,
		},
		{
			name:    "http 401",
			status:  401,
			body:    `{"Response":{}}`,
			wantErr: dns.ErrInvalidCredential,
		},
		{
			name:    "http 500",
			status:  500,
			body:    `oops`,
			wantErr: dns.ErrUpstreamUnavailable,
		},
		{
			name:    "api auth failure",
			status:  200,
			body:    `{"Response":{"Error":{"Code":"AuthFailure.SignatureFailure","Message":"bad sig"},"RequestId":"x"}}`,
			wantErr: dns.ErrInvalidCredential,
		},
		{
			name:    "api unauthorized op",
			status:  200,
			body:    `{"Response":{"Error":{"Code":"UnauthorizedOperation","Message":"no perm"},"RequestId":"x"}}`,
			wantErr: dns.ErrInvalidCredential,
		},
		{
			name:    "api internal error",
			status:  200,
			body:    `{"Response":{"Error":{"Code":"InternalError","Message":"boom"},"RequestId":"x"}}`,
			wantErr: dns.ErrUpstreamUnavailable,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Fatalf("unexpected method %s", r.Method)
				}
				if got := r.Header.Get("X-TC-Action"); got != "DescribeDomainList" {
					t.Fatalf("bad X-TC-Action: %s", got)
				}
				if auth := r.Header.Get("Authorization"); !strings.HasPrefix(auth, "TC3-HMAC-SHA256 ") {
					t.Fatalf("bad Authorization header: %s", auth)
				}
				w.WriteHeader(c.status)
				_, _ = w.Write([]byte(c.body))
			}))
			defer srv.Close()
			p := New(Config{BaseURL: srv.URL})
			err := p.HealthCheck(context.Background(), okCred())
			if c.wantErr == nil && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Fatalf("want %v, got %v", c.wantErr, err)
			}
		})
	}
}

// ---- BuildSolver + Present + CleanUp ----------------------------------------

// fakeDNSPodServer mocks just enough of the DNSPod v3 API for our tests.
type fakeDNSPodServer struct {
	mu       sync.Mutex
	zones    map[string]bool                 // zone name set
	records  map[string]map[uint64]apiRecord // zone -> recordID -> record
	nextRec  uint64
	authFail bool
	t        *testing.T
}

type apiRecord struct {
	Sub   string
	Type  string
	Line  string
	Value string
	TTL   uint64
}

func newFakeDNSPod(t *testing.T) (*fakeDNSPodServer, *httptest.Server) {
	f := &fakeDNSPodServer{
		zones: map[string]bool{
			"example.com": true,
		},
		records: map[string]map[uint64]apiRecord{
			"example.com": {},
		},
		t: t,
	}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	return f, srv
}

func (f *fakeDNSPodServer) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")

	if f.authFail {
		f.writeError(w, "AuthFailure.SignatureFailure", "bad sig")
		return
	}

	action := r.Header.Get("X-TC-Action")
	body, _ := io.ReadAll(r.Body)

	switch action {
	case "DescribeDomainList":
		var req describeDomainListReq
		_ = json.Unmarshal(body, &req)
		resp := describeDomainListResp{RequestId: "rid"}
		for z := range f.zones {
			if req.Keyword == "" || z == req.Keyword {
				resp.DomainList = append(resp.DomainList, domainListItem{
					DomainId: 1, Name: z, Punycode: z,
				})
			}
		}
		f.writeOK(w, resp)
		return

	case "CreateRecord":
		var req createRecordReq
		_ = json.Unmarshal(body, &req)
		if !f.zones[req.Domain] {
			f.writeError(w, "InvalidParameter.DomainNotExists", "no zone")
			return
		}
		f.nextRec++
		id := f.nextRec
		if f.records[req.Domain] == nil {
			f.records[req.Domain] = map[uint64]apiRecord{}
		}
		f.records[req.Domain][id] = apiRecord{
			Sub: req.SubDomain, Type: req.RecordType, Line: req.RecordLine,
			Value: req.Value, TTL: req.TTL,
		}
		f.writeOK(w, map[string]any{"RequestId": "rid", "RecordId": id})
		return

	case "DescribeRecordList":
		var req describeRecordListReq
		_ = json.Unmarshal(body, &req)
		if !f.zones[req.Domain] {
			f.writeError(w, "ResourceNotFound.NoDataOfDomain", "no zone")
			return
		}
		resp := describeRecordListResp{RequestId: "rid"}
		for id, rec := range f.records[req.Domain] {
			if req.Subdomain != "" && rec.Sub != req.Subdomain {
				continue
			}
			if req.RecordType != "" && rec.Type != req.RecordType {
				continue
			}
			resp.RecordList = append(resp.RecordList, recordListItem{
				RecordId: id, Value: rec.Value, Name: rec.Sub, Type: rec.Type,
			})
		}
		if len(resp.RecordList) == 0 {
			f.writeError(w, "ResourceNotFound.NoDataOfRecord", "no record")
			return
		}
		f.writeOK(w, resp)
		return

	case "DeleteRecord":
		var req deleteRecordReq
		_ = json.Unmarshal(body, &req)
		if _, ok := f.records[req.Domain][req.RecordId]; !ok {
			f.writeError(w, "ResourceNotFound.NoDataOfRecord", "no record")
			return
		}
		delete(f.records[req.Domain], req.RecordId)
		f.writeOK(w, map[string]any{"RequestId": "rid"})
		return
	}
	http.NotFound(w, r)
}

func (f *fakeDNSPodServer) writeOK(w http.ResponseWriter, response any) {
	_ = json.NewEncoder(w).Encode(map[string]any{"Response": response})
}

func (f *fakeDNSPodServer) writeError(w http.ResponseWriter, code, msg string) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"Response": map[string]any{
			"Error":     map[string]any{"Code": code, "Message": msg},
			"RequestId": "rid",
		},
	})
}

func TestSolver_PresentCleanUp(t *testing.T) {
	fake, srv := newFakeDNSPod(t)
	defer srv.Close()
	p := New(Config{BaseURL: srv.URL})
	solver, err := p.BuildSolver(context.Background(), okCred(), []string{"example.com"})
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
	gotRecs := len(fake.records["example.com"])
	var savedRec apiRecord
	for _, r := range fake.records["example.com"] {
		savedRec = r
	}
	fake.mu.Unlock()
	if gotRecs != 1 {
		t.Fatalf("want 1 record after present, got %d", gotRecs)
	}
	if savedRec.Sub != "_acme-challenge" {
		t.Errorf("sub = %q, want _acme-challenge", savedRec.Sub)
	}
	if savedRec.Value != value {
		t.Errorf("value = %q, want %q", savedRec.Value, value)
	}
	if savedRec.Type != "TXT" {
		t.Errorf("type = %q", savedRec.Type)
	}

	if err := solver.CleanUp(context.Background(), fqdn, value); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	fake.mu.Lock()
	gotRecs = len(fake.records["example.com"])
	fake.mu.Unlock()
	if gotRecs != 0 {
		t.Fatalf("want 0 records after cleanup, got %d", gotRecs)
	}
}

func TestSolver_Present_NestedSubdomain(t *testing.T) {
	// fqdn = _acme-challenge.www.example.com.，zone = example.com
	fake, srv := newFakeDNSPod(t)
	defer srv.Close()
	p := New(Config{BaseURL: srv.URL})
	solver, err := p.BuildSolver(context.Background(), okCred(), nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := solver.Present(context.Background(), "_acme-challenge.www.example.com.", "v"); err != nil {
		t.Fatalf("present: %v", err)
	}
	fake.mu.Lock()
	var sub string
	for _, r := range fake.records["example.com"] {
		sub = r.Sub
	}
	fake.mu.Unlock()
	if sub != "_acme-challenge.www" {
		t.Errorf("sub = %q, want _acme-challenge.www", sub)
	}
}

func TestSolver_Present_NoZone(t *testing.T) {
	_, srv := newFakeDNSPod(t)
	defer srv.Close()
	p := New(Config{BaseURL: srv.URL})
	solver, err := p.BuildSolver(context.Background(), okCred(), nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	err = solver.Present(context.Background(), "_acme-challenge.unknown.test.", "v")
	if !errors.Is(err, dns.ErrZoneNotFound) {
		t.Fatalf("want ErrZoneNotFound, got %v", err)
	}
}

func TestSolver_Present_TooShortFQDN(t *testing.T) {
	_, srv := newFakeDNSPod(t)
	defer srv.Close()
	p := New(Config{BaseURL: srv.URL})
	solver, err := p.BuildSolver(context.Background(), okCred(), nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	err = solver.Present(context.Background(), "localhost.", "v")
	if !errors.Is(err, dns.ErrZoneNotFound) {
		t.Fatalf("want ErrZoneNotFound for too-short fqdn, got %v", err)
	}
}

func TestSolver_Present_AuthFailure(t *testing.T) {
	fake, srv := newFakeDNSPod(t)
	defer srv.Close()
	fake.mu.Lock()
	fake.authFail = true
	fake.mu.Unlock()
	p := New(Config{BaseURL: srv.URL})
	solver, err := p.BuildSolver(context.Background(), okCred(), nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	err = solver.Present(context.Background(), "_acme-challenge.example.com.", "v")
	if !errors.Is(err, dns.ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential, got %v", err)
	}
}

func TestSolver_CleanUp_NoData(t *testing.T) {
	// 删除时 zone 存在但没有匹配 record：DescribeRecordList 返回 NoDataOfRecord，
	// 我们应当把它视为空列表，CleanUp 不报错。
	_, srv := newFakeDNSPod(t)
	defer srv.Close()
	p := New(Config{BaseURL: srv.URL})
	solver, err := p.BuildSolver(context.Background(), okCred(), nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	// 没有 Present 过，直接 CleanUp。
	if err := solver.CleanUp(context.Background(), "_acme-challenge.example.com.", "v"); err != nil {
		t.Fatalf("cleanup on empty: %v", err)
	}
}

func TestSolver_CleanUp_QuotedValue(t *testing.T) {
	// 模拟 DNSPod 把 value 存成 "v1" 带引号；CleanUp 应当也能匹配。
	fake, srv := newFakeDNSPod(t)
	defer srv.Close()
	fake.mu.Lock()
	fake.nextRec++
	id := fake.nextRec
	fake.records["example.com"][id] = apiRecord{
		Sub: "_acme-challenge", Type: "TXT", Line: "默认",
		Value: "\"v1\"", TTL: 600,
	}
	fake.mu.Unlock()

	p := New(Config{BaseURL: srv.URL})
	solver, err := p.BuildSolver(context.Background(), okCred(), nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := solver.CleanUp(context.Background(), "_acme-challenge.example.com.", "v1"); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	fake.mu.Lock()
	_, still := fake.records["example.com"][id]
	fake.mu.Unlock()
	if still {
		t.Fatal("quoted record not cleaned up")
	}
}

// ---- helpers ---------------------------------------------------------------

func TestExtractSubDomain(t *testing.T) {
	cases := []struct {
		fqdn, zone, want string
		wantErr          bool
	}{
		{"_acme-challenge.example.com.", "example.com", "_acme-challenge", false},
		{"_acme-challenge.www.example.com.", "example.com", "_acme-challenge.www", false},
		{"example.com.", "example.com", "@", false},
		{"foo.bar.test.", "example.com", "", true},
	}
	for _, c := range cases {
		got, err := extractSubDomain(c.fqdn, c.zone)
		if c.wantErr {
			if err == nil {
				t.Errorf("extractSubDomain(%q, %q) want error, got %q", c.fqdn, c.zone, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("extractSubDomain(%q, %q): %v", c.fqdn, c.zone, err)
			continue
		}
		if got != c.want {
			t.Errorf("extractSubDomain(%q, %q) = %q, want %q", c.fqdn, c.zone, got, c.want)
		}
	}
}

func TestHostOnly(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://dnspod.tencentcloudapi.com", "dnspod.tencentcloudapi.com"},
		{"http://127.0.0.1:8080", "127.0.0.1:8080"},
		{"https://dnspod.tencentcloudapi.com/", "dnspod.tencentcloudapi.com"},
		{"dnspod.tencentcloudapi.com/foo", "dnspod.tencentcloudapi.com"},
	}
	for _, c := range cases {
		if got := hostOnly(c.in); got != c.want {
			t.Errorf("hostOnly(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSnippet(t *testing.T) {
	short := []byte("hello")
	if snippet(short) != "hello" {
		t.Fail()
	}
	long := make([]byte, 300)
	for i := range long {
		long[i] = 'a'
	}
	s := snippet(long)
	if !strings.HasSuffix(s, "...") || len(s) != 203 {
		t.Errorf("snippet length = %d, last = %q", len(s), s[len(s)-3:])
	}
}

func TestTC3Sign_Deterministic(t *testing.T) {
	// 用 Tencent Cloud 官方示例核对签名格式。这里只做自洽：
	// 相同参数两次签名结果必须一致；不同 secret 必须产生不同 signature。
	p1 := tc3SignParams{
		secretID:  fakeSecretID,
		secretKey: fakeSecretKey,
		service:   "dnspod",
		host:      "dnspod.tencentcloudapi.com",
		action:    "DescribeDomainList",
		body:      []byte(`{"Limit":1}`),
		date:      "2026-05-17",
		timestamp: "1779148800",
	}
	a := tc3Sign(p1)
	b := tc3Sign(p1)
	if a != b {
		t.Fatal("non-deterministic sign")
	}
	if !strings.HasPrefix(a, "TC3-HMAC-SHA256 Credential=") {
		t.Fatalf("bad header format: %s", a)
	}
	if !strings.Contains(a, "SignedHeaders=content-type;host;x-tc-action") {
		t.Fatalf("missing SignedHeaders: %s", a)
	}
	if !strings.Contains(a, "Signature=") {
		t.Fatalf("missing Signature: %s", a)
	}
	p2 := p1
	p2.secretKey = "different-key-xxxxxxxxxxxxxxxxxx"
	c := tc3Sign(p2)
	if c == a {
		t.Fatal("different keys produced same signature")
	}
}

func TestMapAPIError(t *testing.T) {
	cases := []struct {
		code string
		want error
	}{
		{"AuthFailure", dns.ErrInvalidCredential},
		{"AuthFailure.SignatureFailure", dns.ErrInvalidCredential},
		{"UnauthorizedOperation", dns.ErrInvalidCredential},
		{"ResourceNotFound.NoDataOfRecord", dns.ErrZoneNotFound},
		{"InvalidParameter.DomainNotExists", dns.ErrZoneNotFound},
		{"InvalidParameterValue.DomainNotExists", dns.ErrZoneNotFound},
		{"InternalError", dns.ErrUpstreamUnavailable},
		{"LimitExceeded", dns.ErrUpstreamUnavailable},
	}
	for _, c := range cases {
		err := mapAPIError(&apiError{Code: c.code, Message: "msg"})
		if !errors.Is(err, c.want) {
			t.Errorf("mapAPIError(%q) → %v, want %v", c.code, err, c.want)
		}
	}
}

func TestMapHTTPStatus(t *testing.T) {
	cases := []struct {
		code int
		want error
	}{
		{200, nil},
		{299, nil},
		{401, dns.ErrInvalidCredential},
		{403, dns.ErrInvalidCredential},
		{404, dns.ErrZoneNotFound},
		{400, dns.ErrUpstreamUnavailable},
		{500, dns.ErrUpstreamUnavailable},
		{503, dns.ErrUpstreamUnavailable},
	}
	for _, c := range cases {
		err := mapHTTPStatus(c.code, []byte("body"))
		if c.want == nil {
			if err != nil {
				t.Errorf("status %d: got %v, want nil", c.code, err)
			}
			continue
		}
		if !errors.Is(err, c.want) {
			t.Errorf("status %d: got %v, want %v", c.code, err, c.want)
		}
	}
}
