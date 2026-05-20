package apiclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestNew_HasAPIKey(t *testing.T) {
	if New("https://api", "").HasAPIKey() {
		t.Fatal("empty apiKey should report HasAPIKey() == false")
	}
	if !New("https://api", "k").HasAPIKey() {
		t.Fatal("non-empty apiKey should report HasAPIKey() == true")
	}
}

func TestPost_HappyPath_SetsHeadersAndDecodesResponse(t *testing.T) {
	var gotAuth, gotCT, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		buf := make([]byte, 256)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "secret-key")
	var out struct {
		OK bool `json:"ok"`
	}
	if err := c.Post(context.Background(), "/v1/echo", map[string]string{"hello": "world"}, &out); err != nil {
		t.Fatalf("Post err: %v", err)
	}
	if !out.OK {
		t.Fatal("expected ok=true in decoded response")
	}
	if gotAuth != "Bearer secret-key" {
		t.Errorf("Authorization header = %q", gotAuth)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type header = %q", gotCT)
	}
	if !strings.Contains(gotBody, "world") {
		t.Errorf("request body lost: %q", gotBody)
	}
}

func TestGet_AppendsQueryParams(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.String()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	params := url.Values{}
	params.Set("a", "1")
	params.Set("b", "two words")
	if err := c.Get(context.Background(), "/v1/q", params, nil); err != nil {
		t.Fatalf("Get err: %v", err)
	}
	if !strings.Contains(gotPath, "a=1") || !strings.Contains(gotPath, "b=two+words") {
		t.Errorf("query params not encoded into path: %q", gotPath)
	}
}

func TestDo_SurfacesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"bad token"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	err := c.Post(context.Background(), "/x", struct{}{}, nil)
	if err == nil {
		t.Fatal("expected error on 401")
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "bad token") {
		t.Errorf("error should include status + message, got: %v", err)
	}
}

func TestDo_FallbackErrorMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`<html>nope</html>`))
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	err := c.Post(context.Background(), "/x", struct{}{}, nil)
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("expected wrapped 500 error, got %v", err)
	}
}

func TestDo_RespectsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until request context is cancelled
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := c.Post(ctx, "/x", struct{}{}, nil)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestNew_TransportTunablesApplied(t *testing.T) {
	c := New("https://api.example", "")
	tr := c.Transport()
	if tr == nil {
		t.Fatal("Transport() returned nil — expected a configured *http.Transport")
	}
	if tr.MaxIdleConns != 100 {
		t.Errorf("MaxIdleConns = %d, want 100", tr.MaxIdleConns)
	}
	if tr.MaxIdleConnsPerHost != 20 {
		t.Errorf("MaxIdleConnsPerHost = %d, want 20 (default 2 starves long-lived MCP workloads)", tr.MaxIdleConnsPerHost)
	}
	if tr.IdleConnTimeout != 90*time.Second {
		t.Errorf("IdleConnTimeout = %v, want 90s", tr.IdleConnTimeout)
	}
	if tr.TLSHandshakeTimeout != 10*time.Second {
		t.Errorf("TLSHandshakeTimeout = %v, want 10s", tr.TLSHandshakeTimeout)
	}
	if tr.ExpectContinueTimeout != 1*time.Second {
		t.Errorf("ExpectContinueTimeout = %v, want 1s", tr.ExpectContinueTimeout)
	}
	if !tr.ForceAttemptHTTP2 {
		t.Error("ForceAttemptHTTP2 should be true")
	}
	// Sanity: the http.Client's Transport is the same one Transport() exposes.
	if c.httpClient.Transport != tr {
		t.Error("httpClient.Transport != Transport() — they must share state")
	}
	if c.httpClient.Timeout != 15*time.Second {
		t.Errorf("httpClient.Timeout = %v, want 15s", c.httpClient.Timeout)
	}
}

func TestClose_IsSafeToCallAndIdempotent(t *testing.T) {
	c := New("https://api.example", "")
	// Two consecutive Close() calls must not panic; CloseIdleConnections
	// on an unused transport is a no-op but still has to be safe.
	c.Close()
	c.Close()
}

// TestDo_UnwrapsDataEnvelope 守 M1: api 统一返回 {"data":..., "request_id":...},
// 早期版本的 do() 直接 unmarshal body 进 out → 字段全零。回归。
func TestDo_UnwrapsDataEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"ip":"1.1.1.1","country":"AU"},"request_id":"req_abc"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	var out struct {
		IP      string `json:"ip"`
		Country string `json:"country"`
	}
	if err := c.Get(context.Background(), "/v1/info/ip", nil, &out); err != nil {
		t.Fatalf("Get err: %v", err)
	}
	if out.IP != "1.1.1.1" || out.Country != "AU" {
		t.Errorf("envelope not unwrapped: %+v", out)
	}
}

// 没 data wrapper(老接口/第三方)时仍能解析,保持兼容
func TestDo_FallbackWhenNoEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ip":"2.2.2.2"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	var out struct {
		IP string `json:"ip"`
	}
	if err := c.Get(context.Background(), "/x", nil, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	if out.IP != "2.2.2.2" {
		t.Errorf("fallback unmarshal failed: %+v", out)
	}
}

// 错误信封带 code + message 时,err 信息要包含两者(便于 LLM/用户定位)
func TestDo_CodedErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"VALIDATION","message":"Missing 'q' parameter","request_id":"req_xx"},"request_id":"req_xx"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	err := c.Get(context.Background(), "/v1/info/ip", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "400") || !strings.Contains(msg, "VALIDATION") || !strings.Contains(msg, "Missing 'q'") {
		t.Errorf("err should carry status+code+message, got: %v", err)
	}
}

func TestDo_NoOutPointerSkipsUnmarshal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not valid json`))
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	// out=nil — must not error even though body is garbage.
	if err := c.Get(context.Background(), "/", nil, nil); err != nil {
		t.Fatalf("expected nil out to skip unmarshal, got err: %v", err)
	}
}
