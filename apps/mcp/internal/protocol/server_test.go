package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer() *Server {
	srv := NewServer()
	srv.Register(ToolDefinition{
		Name:        "ping",
		Description: "Ping a host",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target": map[string]any{"type": "string"},
				"count":  map[string]any{"type": "integer", "default": 3},
			},
			"required": []string{"target"},
		},
	}, func(ctx context.Context, args map[string]any) (string, error) {
		target, _ := args["target"].(string)
		if target == "" {
			return "", errMissingField("target")
		}
		return "PING " + target + ": ok", nil
	})
	return srv
}

func errMissingField(field string) error {
	return &missingFieldError{field: field}
}

type missingFieldError struct{ field string }

func (e *missingFieldError) Error() string { return e.field + " is required" }

func runRequest(t *testing.T, srv *Server, line string) *Response {
	t.Helper()
	var buf bytes.Buffer
	err := srv.run(context.Background(), strings.NewReader(line+"\n"), &buf)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	output := strings.TrimSpace(buf.String())
	if output == "" {
		return nil
	}
	var resp Response
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("unmarshal response: %v (raw: %s)", err, output)
	}
	return &resp
}

func TestInitialize(t *testing.T) {
	srv := newTestServer()
	resp := runRequest(t, srv, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	if resp == nil || resp.Error != nil {
		t.Fatalf("expected result, got: %+v", resp)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result not a map")
	}
	info, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatal("serverInfo not a map")
	}
	if info["name"] != "idcd-mcp" {
		t.Errorf("serverInfo.name = %v, want idcd-mcp", info["name"])
	}
}

func TestToolsList(t *testing.T) {
	srv := NewServer()

	names := []string{"ping", "http", "dns", "traceroute", "ssl", "diagnose", "ip", "whois"}
	for _, name := range names {
		n := name
		srv.Register(ToolDefinition{Name: n, Description: n, InputSchema: map[string]any{}}, func(ctx context.Context, args map[string]any) (string, error) {
			return n, nil
		})
	}

	resp := runRequest(t, srv, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if resp == nil || resp.Error != nil {
		t.Fatalf("expected result, got: %+v", resp)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result not a map")
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatal("tools not a slice")
	}
	if len(tools) != 8 {
		t.Errorf("tools count = %d, want 8", len(tools))
	}
}

func TestToolsCallPingValid(t *testing.T) {
	srv := newTestServer()
	resp := runRequest(t, srv, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"ping","arguments":{"target":"8.8.8.8"}}}`)
	if resp == nil || resp.Error != nil {
		t.Fatalf("expected result, got: %+v", resp)
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
	if item["type"] != "text" {
		t.Errorf("type = %v, want text", item["type"])
	}
}

func TestToolsCallPingMissingTarget(t *testing.T) {
	srv := newTestServer()
	resp := runRequest(t, srv, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"ping","arguments":{}}}`)
	if resp == nil || resp.Error == nil {
		t.Fatal("expected error response")
	}
	if resp.Error.Code != ErrInternalError {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrInternalError)
	}
}

func TestUnknownMethod(t *testing.T) {
	srv := newTestServer()
	resp := runRequest(t, srv, `{"jsonrpc":"2.0","id":5,"method":"unknown/method"}`)
	if resp == nil || resp.Error == nil {
		t.Fatal("expected error response")
	}
	if resp.Error.Code != ErrMethodNotFound {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrMethodNotFound)
	}
}

func TestInvalidJSON(t *testing.T) {
	srv := newTestServer()
	resp := runRequest(t, srv, `{invalid json}`)
	if resp == nil || resp.Error == nil {
		t.Fatal("expected error response")
	}
	if resp.Error.Code != ErrParseError {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrParseError)
	}
}

func TestInitializedNotification(t *testing.T) {
	srv := newTestServer()
	var buf bytes.Buffer
	err := srv.run(context.Background(), strings.NewReader(`{"jsonrpc":"2.0","method":"initialized"}`+"\n"), &buf)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "" {
		t.Errorf("expected no output for notification, got: %s", buf.String())
	}
}

func TestHTTPTransportMessages(t *testing.T) {
	srv := newTestServer()
	handler := MessagesHandler(srv)

	body := `{"jsonrpc":"2.0","id":20,"method":"tools/call","params":{"name":"ping","arguments":{"target":"1.2.3.4"}}}`
	req := httptest.NewRequest(http.MethodPost, "/messages", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v (raw: %s)", err, w.Body.String())
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}
}

func TestHTTPTransportMethodNotAllowed(t *testing.T) {
	srv := newTestServer()
	handler := MessagesHandler(srv)

	req := httptest.NewRequest(http.MethodGet, "/messages", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}
