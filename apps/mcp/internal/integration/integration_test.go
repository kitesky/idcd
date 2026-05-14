package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kite365/idcd/apps/mcp/internal/apiclient"
	"github.com/kite365/idcd/apps/mcp/internal/protocol"
	"github.com/kite365/idcd/apps/mcp/internal/tools"
)

func newServerWithMockAPI(t *testing.T, mux *http.ServeMux) (*protocol.Server, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	client := apiclient.New(ts.URL, "test-key")
	srv := protocol.NewServer()
	tools.RegisterAll(srv, client)
	return srv, ts
}

func runRequest(t *testing.T, srv *protocol.Server, line string) *protocol.Response {
	t.Helper()
	var buf bytes.Buffer
	err := srv.RunIO(context.Background(), strings.NewReader(line+"\n"), &buf)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	output := strings.TrimSpace(buf.String())
	if output == "" {
		return nil
	}
	var resp protocol.Response
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("unmarshal response: %v (raw: %s)", err, output)
	}
	return &resp
}

func getToolCallText(t *testing.T, resp *protocol.Response) string {
	t.Helper()
	if resp == nil || resp.Error != nil {
		t.Fatalf("expected success, got: %+v", resp)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result not a map")
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatal("content missing or empty")
	}
	item, ok := content[0].(map[string]any)
	if !ok {
		t.Fatal("content[0] not a map")
	}
	text, _ := item["text"].(string)
	return text
}

func TestPingWithMockAPI(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/probe/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"target": "8.8.8.8",
			"results": [
				{"node": "Tokyo", "country": "JP", "avg_ms": 32, "loss_pct": 0, "success": true},
				{"node": "Singapore", "country": "SG", "avg_ms": 45, "loss_pct": 0, "success": true}
			],
			"avg_ms": 38,
			"loss_pct": 0
		}`))
	})

	srv, _ := newServerWithMockAPI(t, mux)
	resp := runRequest(t, srv, `{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"ping","arguments":{"target":"8.8.8.8"}}}`)
	text := getToolCallText(t, resp)

	if !strings.Contains(text, "PING 8.8.8.8") {
		t.Errorf("expected PING header, got: %s", text)
	}
	if !strings.Contains(text, "Tokyo") {
		t.Errorf("expected Tokyo node, got: %s", text)
	}
	if !strings.Contains(text, "38ms") {
		t.Errorf("expected avg 38ms, got: %s", text)
	}
	if !strings.Contains(text, "0%") {
		t.Errorf("expected 0%% loss, got: %s", text)
	}
}

func TestIPWithMockAPI(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/info/ip", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"ip": "1.1.1.1",
			"asn": "AS13335",
			"country": "US",
			"city": "Los Angeles",
			"isp": "Cloudflare, Inc."
		}`))
	})

	srv, _ := newServerWithMockAPI(t, mux)
	resp := runRequest(t, srv, `{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"ip","arguments":{"address":"1.1.1.1"}}}`)
	text := getToolCallText(t, resp)

	if !strings.Contains(text, "1.1.1.1") {
		t.Errorf("expected IP in output, got: %s", text)
	}
	if !strings.Contains(text, "AS13335") {
		t.Errorf("expected ASN, got: %s", text)
	}
	if !strings.Contains(text, "Los Angeles") {
		t.Errorf("expected city, got: %s", text)
	}
	if !strings.Contains(text, "Cloudflare") {
		t.Errorf("expected ISP, got: %s", text)
	}
}

func TestDNSWithMockAPI(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/info/dns", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"domain": "example.com",
			"type": "A",
			"records": [
				{"value": "104.21.1.1", "ttl": 300},
				{"value": "172.67.1.1", "ttl": 300}
			]
		}`))
	})

	srv, _ := newServerWithMockAPI(t, mux)
	resp := runRequest(t, srv, `{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"dns","arguments":{"domain":"example.com"}}}`)
	text := getToolCallText(t, resp)

	if !strings.Contains(text, "example.com") {
		t.Errorf("expected domain, got: %s", text)
	}
	if !strings.Contains(text, "104.21.1.1") {
		t.Errorf("expected first A record, got: %s", text)
	}
	if !strings.Contains(text, "TTL: 300") {
		t.Errorf("expected TTL, got: %s", text)
	}
}

func TestNoAPIKeyReturnsPrompt(t *testing.T) {
	client := apiclient.New("https://api.idcd.com", "")
	srv := protocol.NewServer()
	tools.RegisterAll(srv, client)

	cases := []struct {
		name string
		args string
	}{
		{"ping", `{"target":"8.8.8.8"}`},
		{"ip", `{"address":"1.1.1.1"}`},
		{"dns", `{"domain":"example.com"}`},
		{"ssl", `{"host":"example.com"}`},
		{"whois", `{"query":"example.com"}`},
		{"http", `{"url":"https://example.com"}`},
		{"traceroute", `{"target":"8.8.8.8"}`},
		{"diagnose", `{"target":"example.com"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			line := `{"jsonrpc":"2.0","id":99,"method":"tools/call","params":{"name":"` + tc.name + `","arguments":` + tc.args + `}}`
			resp := runRequest(t, srv, line)
			text := getToolCallText(t, resp)
			if !strings.Contains(text, "IDCD_API_KEY") {
				t.Errorf("expected API key prompt for %s, got: %s", tc.name, text)
			}
		})
	}
}
