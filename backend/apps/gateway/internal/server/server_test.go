// Package server tests cover the chi router wiring, in particular the
// /v1/cert/* reverse-proxy mount added in S2 W8.
//
// We exercise the router directly (s.Router() served via httptest.Server
// + httptest.NewRecorder) rather than booting a full TCP listener — the
// cert-svc upstream is a fake httptest.Server in every test, so the assertions
// stay deterministic and don't depend on the host network stack.
package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/gateway/internal/config"
	"github.com/kite365/idcd/apps/gateway/internal/hub"
	"github.com/kite365/idcd/lib/shared/logger"
	"github.com/kite365/idcd/lib/shared/stream"
)

// newTestServer wires the minimum collaborators a chi router needs to
// answer requests. We deliberately pass nil for NodeAuthPool because the
// /v1/cert/* routes never touch the pgx-backed node auth pool — they go
// straight through the reverse proxy.
func newTestServer(t *testing.T, certSvcURL string) *Server {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	log := logger.Discard()
	h := hub.New(0, log)
	cfg := &config.Config{
		ListenAddr: ":0",
		CertSvcURL: certSvcURL,
	}
	streamCli := stream.New(rdb)
	return New(cfg, h, rdb, nil, streamCli, log)
}

// TestCertSvcProxy_RoutesHitUpstream confirms every advertised cert-svc
// path family (orders, dns-credentials, certs, manage subroutes, and the
// unauthenticated /download endpoint) is reverse-proxied to the cert-svc
// upstream rather than handled locally by the gateway.
func TestCertSvcProxy_RoutesHitUpstream(t *testing.T) {
	var seenPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		w.Header().Set("X-Cert-Svc", "yes")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(upstream.Close)

	srv := newTestServer(t, upstream.URL)
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)

	cases := []struct {
		name   string
		method string
		path   string
	}{
		{"orders_list", http.MethodGet, "/v1/cert/orders"},
		{"orders_create", http.MethodPost, "/v1/cert/orders"},
		{"orders_get", http.MethodGet, "/v1/cert/orders/ord_123"},
		{"orders_retry", http.MethodPost, "/v1/cert/orders/ord_123/retry"},
		{"orders_cancel", http.MethodPost, "/v1/cert/orders/ord_123/cancel"},
		{"orders_manual_ready", http.MethodPost, "/v1/cert/orders/ord_123/manual-ready"},
		{"dns_credentials_list", http.MethodGet, "/v1/cert/dns-credentials"},
		{"dns_credentials_create", http.MethodPost, "/v1/cert/dns-credentials"},
		{"dns_credentials_delete", http.MethodDelete, "/v1/cert/dns-credentials/cred_1"},
		{"certs_list", http.MethodGet, "/v1/cert/certs"},
		{"certs_get", http.MethodGet, "/v1/cert/certs/cert_1"},
		{"certs_revoke", http.MethodPost, "/v1/cert/certs/cert_1/revoke"},
		// /download is mounted OUTSIDE cert-svc's auth middleware (the
		// one-shot token IS the credential); the gateway must not add
		// any extra auth either, just forward the request transparently.
		{"certs_download", http.MethodGet, "/v1/cert/certs/cert_1/download?token=xyz"},
		// /v1/admin/cert/* is the admin surface — the upstream guards it
		// with its own Bearer admin token; gateway only proxies.
		{"admin_orders", http.MethodGet, "/v1/admin/cert/orders"},
		{"admin_force_fail", http.MethodPost, "/v1/admin/cert/orders/ord_1/force-fail"},
		{"admin_ca_quota", http.MethodGet, "/v1/admin/cert/ca-quota"},
		{"admin_dns_health", http.MethodGet, "/v1/admin/cert/dns-health"},
		{"admin_ban", http.MethodPost, "/v1/admin/cert/accounts/1/ban"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seenPath = ""
			req, err := http.NewRequest(tc.method, ts.URL+tc.path, nil)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			resp, err := ts.Client().Do(req)
			if err != nil {
				t.Fatalf("client.Do: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200 from upstream, got %d", resp.StatusCode)
			}
			if resp.Header.Get("X-Cert-Svc") != "yes" {
				t.Errorf("missing X-Cert-Svc header — request did not reach upstream")
			}
			// Reverse proxy must forward the full request path, sans query.
			wantPath := tc.path
			if i := strings.IndexByte(wantPath, '?'); i >= 0 {
				wantPath = wantPath[:i]
			}
			if seenPath != wantPath {
				t.Errorf("upstream saw path %q, want %q", seenPath, wantPath)
			}
		})
	}
}

// TestCertSvcProxy_DoesNotIntercept_OtherRoutes verifies that the
// reverse-proxy mount is scoped to /v1/cert/*. Health, metrics, and agent
// WebSocket routes must continue to be served by the gateway itself.
func TestCertSvcProxy_DoesNotIntercept_OtherRoutes(t *testing.T) {
	hits := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	srv := newTestServer(t, upstream.URL)
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)

	// /health is handled locally by HealthHandler — upstream must NOT be hit.
	resp, err := ts.Client().Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusBadGateway {
		t.Errorf("/health returned 502, proxy should not have intercepted it")
	}
	if hits != 0 {
		t.Errorf("expected upstream hits=0 for non-cert routes, got %d", hits)
	}
}

// TestCertSvcProxy_PreservesAuthorizationHeader is the critical test for
// the gateway's role as a transparent proxy: the user's JWT / session token
// MUST reach cert-svc unmodified, otherwise every authenticated request
// would 401 even with valid credentials.
func TestCertSvcProxy_PreservesAuthorizationHeader(t *testing.T) {
	var (
		gotAuth   string
		gotCookie string
	)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if c, err := r.Cookie("idcd_session"); err == nil {
			gotCookie = c.Value
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	srv := newTestServer(t, upstream.URL)
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/cert/orders", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer test-jwt-token")
	req.AddCookie(&http.Cookie{Name: "idcd_session", Value: "sess-abc"})
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	resp.Body.Close()

	if gotAuth != "Bearer test-jwt-token" {
		t.Errorf("Authorization header lost: got %q", gotAuth)
	}
	if gotCookie != "sess-abc" {
		t.Errorf("idcd_session cookie lost: got %q", gotCookie)
	}
}

// TestCertSvcProxy_UpstreamDown returns 502 — not a Go default error page —
// when cert-svc is unreachable. SREs key alerts off the 502 status, so the
// behavior here is load-bearing.
func TestCertSvcProxy_UpstreamDown(t *testing.T) {
	// Spin up an upstream then immediately close it so Dial fails.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	addr := upstream.URL
	upstream.Close()

	srv := newTestServer(t, addr)
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)

	resp, err := ts.Client().Get(ts.URL + "/v1/cert/orders")
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected 502 when upstream is down, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "cert-svc") {
		t.Errorf("expected error body to mention cert-svc, got %q", string(body))
	}
}

// TestCertSvcProxy_DisabledWhenURLEmpty confirms that leaving cert_svc_url
// blank in the YAML (the S1 / standalone-gateway default) leaves /v1/cert/*
// unmounted. Without this, every request would 502 forever because there's
// no upstream to forward to.
func TestCertSvcProxy_DisabledWhenURLEmpty(t *testing.T) {
	srv := newTestServer(t, "")
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)

	resp, err := ts.Client().Get(ts.URL + "/v1/cert/orders")
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	resp.Body.Close()
	// chi's default 404 — not 502 from a misconfigured proxy.
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 when proxy disabled, got %d", resp.StatusCode)
	}
}

// TestNewCertSvcProxy_InvalidURL surfaces parse errors so misconfigurations
// (e.g. "://bad" or missing scheme) are caught at setupRouter time rather
// than panicking on the first request.
func TestNewCertSvcProxy_InvalidURL(t *testing.T) {
	log := logger.Discard()
	tests := []string{
		"",                    // empty
		"://no-scheme",        // parse error
		"not-a-url-at-all",    // no scheme / host
		"http://",             // no host
	}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if _, err := newCertSvcProxy(raw, log); err == nil {
				t.Errorf("expected error for %q, got nil", raw)
			}
		})
	}
}

// TestNewCertSvcProxy_NilLogger verifies the proxy tolerates a nil logger.
// This matters because tests can construct the proxy bare, and we don't want
// a nil-pointer panic on the error path masking the real upstream failure.
func TestNewCertSvcProxy_NilLogger(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	upstream.Close() // force ErrorHandler path

	proxy, err := newCertSvcProxy(upstream.URL, nil)
	if err != nil {
		t.Fatalf("newCertSvcProxy: %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/cert/orders", nil)
	proxy.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected 502 from ErrorHandler with nil logger, got %d", rec.Code)
	}
}

// silenceSlog returns a discard logger; kept around for parity with other
// server-level tests that may need to assert against captured log lines.
var _ = slog.Default
