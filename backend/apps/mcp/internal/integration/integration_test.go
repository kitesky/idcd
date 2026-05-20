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

// M7(2026-05-20): probe 工具改成 async polling — mock 必须分两个 endpoint
// (POST /v1/probe/* 返回 task_id;GET /v1/probe/tasks/{id} 返回 result)。
// 这里的 mock 也是新合约的最小契约文档:agent 写回的字段名(node_id / avg_ms /
// packet_loss / packets_sent...)是 mcp 工具能识别的标准 schema。
func TestPingWithMockAPI(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/probe/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// envelope shape — apiclient.do() 剥 data wrapper
		w.Write([]byte(`{"data":{"task_id":"pt_test01","status":"queued"},"request_id":"req_x"}`))
	})
	mux.HandleFunc("/v1/probe/tasks/pt_test01", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"task_id":"pt_test01","status":"completed","result":{
			"node_id":"nd_tokyo01",
			"success":true,
			"avg_ms":38.5,
			"min_ms":31.2,
			"max_ms":45.8,
			"packet_loss":0,
			"packets_sent":3,
			"packets_received":3
		}},"request_id":"req_y"}`))
	})

	srv, _ := newServerWithMockAPI(t, mux)
	resp := runRequest(t, srv, `{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"ping","arguments":{"target":"8.8.8.8"}}}`)
	text := getToolCallText(t, resp)

	if !strings.Contains(text, "PING 8.8.8.8") {
		t.Errorf("expected PING header, got: %s", text)
	}
	if !strings.Contains(text, "nd_tokyo01") {
		t.Errorf("expected node id, got: %s", text)
	}
	if !strings.Contains(text, "38.5ms") {
		t.Errorf("expected avg 38.5ms, got: %s", text)
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

// M3(2026-05-20): mcp 工具不再强制要求 IDCD_API_KEY。api 的 /v1/info/* 和
// /v1/probe/* 端点本身允许 anonymous(OptionalAuthnWithTokens),mcp 自己有 PAT
// 鉴权,无需在 tool 层再 enforce 一次 — 那是 contract drift。
//
// 新合约:没 API key 时调用要么在 api 端成功(anonymous 允许),要么返回 ErrToolFailure
// 包装的工具级错误(网络/上游/参数等),不能 panic、不能空 prompt。这里只验证后者。
func TestNoAPIKeyStillReturnsToolError(t *testing.T) {
	// 指向不可达 host,确保所有工具都走到 do() 网络错误分支 — 此时仍要
	// 经 ErrToolFailure 包装成 isError:true 的工具结果,而不是 panic。
	client := apiclient.New("https://127.0.0.1:1", "")
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
			// 关键反向锁:不能再出现 IDCD_API_KEY 字面提示(那是旧的强校验文案)
			if strings.Contains(text, "IDCD_API_KEY") {
				t.Errorf("tool still prompts for IDCD_API_KEY (should have been removed in M3): %s", text)
			}
			// 工具应该返回某种错误描述(网络/参数/上游 5xx),而不是空 prompt
			if text == "" {
				t.Errorf("expected non-empty tool error text, got empty")
			}
		})
	}
}
