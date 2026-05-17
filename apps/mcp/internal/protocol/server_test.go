package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ─────────────────────────────────────────────
// Stdio / handle() tests — unchanged from pre-auth era
// ─────────────────────────────────────────────

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

// ─────────────────────────────────────────────
// HTTP transport tests — auth, CORS, body cap
// ─────────────────────────────────────────────

// validToken is a syntactically-valid MCP token used across the HTTP tests.
const validToken = "idcd_mcp_" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// expiredToken / revokedToken are different rawTokens so their hashes differ.
const expiredToken = "idcd_mcp_" + "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
const revokedToken = "idcd_mcp_" + "11111111111111112222222222222222333333333333333344444444444444"

// buildHTTPConfig returns a config with:
//   - a valid principal for validToken (personal, 1h ahead),
//   - an expired record for expiredToken,
//   - a revoked record for revokedToken,
//   - an https://app.idcd.com CORS allowlist entry.
func buildHTTPConfig(t *testing.T) HTTPConfig {
	t.Helper()
	v := NewStaticTokenValidator(
		StaticTokenRecord{
			RawToken:  validToken,
			TokenID:   "mcpt_test01",
			UserID:    "usr_test",
			Workspace: "wks_test",
			Type:      "personal",
			Scopes:    []string{"tools:call"},
			ExpiresAt: time.Now().Add(1 * time.Hour),
		},
		StaticTokenRecord{
			RawToken:  expiredToken,
			TokenID:   "mcpt_exp01",
			UserID:    "usr_exp",
			Type:      "personal",
			ExpiresAt: time.Now().Add(-1 * time.Minute),
		},
		StaticTokenRecord{
			RawToken:  revokedToken,
			TokenID:   "mcpt_rev01",
			UserID:    "usr_rev",
			Type:      "personal",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			Revoked:   true,
		},
	)
	return HTTPConfig{
		Validator:      v,
		AllowedOrigins: []string{"https://app.idcd.com"},
	}
}

// messagesPost builds an authenticated POST /messages request body.
func messagesPost(t *testing.T, srv *Server, cfg HTTPConfig, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	handler := MessagesHandlerWithConfig(srv, cfg)
	req := httptest.NewRequest(http.MethodPost, "/messages", strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func TestHTTPMessages_OKWithValidToken(t *testing.T) {
	srv := newTestServer()
	cfg := buildHTTPConfig(t)

	body := `{"jsonrpc":"2.0","id":20,"method":"tools/call","params":{"name":"ping","arguments":{"target":"1.2.3.4"}}}`
	w := messagesPost(t, srv, cfg, validToken, body)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v (raw: %s)", err, w.Body.String())
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}
}

func TestHTTPMessages_MissingAuth(t *testing.T) {
	srv := newTestServer()
	cfg := buildHTTPConfig(t)

	w := messagesPost(t, srv, cfg, "", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if got := w.Header().Get("WWW-Authenticate"); !strings.HasPrefix(got, "Bearer") {
		t.Errorf("WWW-Authenticate = %q, want Bearer realm", got)
	}
}

func TestHTTPMessages_MalformedAuth(t *testing.T) {
	srv := newTestServer()
	cfg := buildHTTPConfig(t)

	// Missing Bearer prefix.
	handler := MessagesHandlerWithConfig(srv, cfg)
	req := httptest.NewRequest(http.MethodPost, "/messages", strings.NewReader(`{}`))
	req.Header.Set("Authorization", validToken) // no "Bearer " prefix
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("missing Bearer prefix: status = %d, want 401", w.Code)
	}

	// Wrong prefix for the token value.
	w = messagesPost(t, srv, cfg, "not_an_idcd_token_at_all_long_enough", `{}`)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("wrong prefix: status = %d, want 401", w.Code)
	}

	// Way too short.
	w = messagesPost(t, srv, cfg, "idcd_mcp_x", `{}`)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("too short: status = %d, want 401", w.Code)
	}
}

func TestHTTPMessages_UnknownToken(t *testing.T) {
	srv := newTestServer()
	cfg := buildHTTPConfig(t)

	bogus := "idcd_mcp_" + strings.Repeat("a", 64)
	w := messagesPost(t, srv, cfg, bogus, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestHTTPMessages_ExpiredToken(t *testing.T) {
	srv := newTestServer()
	cfg := buildHTTPConfig(t)

	w := messagesPost(t, srv, cfg, expiredToken, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestHTTPMessages_RevokedToken(t *testing.T) {
	srv := newTestServer()
	cfg := buildHTTPConfig(t)

	w := messagesPost(t, srv, cfg, revokedToken, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestHTTPMessages_BodyTooLarge(t *testing.T) {
	srv := newTestServer()
	cfg := buildHTTPConfig(t)

	// Build a JSON payload >1 MiB. tools/call with a giant target string.
	big := strings.Repeat("A", int(MaxMessageBytes)+1024)
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"ping","arguments":{"target":"` + big + `"}}}`

	w := messagesPost(t, srv, cfg, validToken, body)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", w.Code)
	}
}

func TestHTTPMessages_MethodNotAllowed(t *testing.T) {
	srv := newTestServer()
	cfg := buildHTTPConfig(t)

	handler := MessagesHandlerWithConfig(srv, cfg)
	// GET with valid token: handler should still reject as 405 (auth happens
	// after the method check? — actually we want auth FIRST so we don't
	// leak which routes exist; spec is "auth then method check"). Verify
	// the contract.
	req := httptest.NewRequest(http.MethodGet, "/messages", nil)
	req.Header.Set("Authorization", "Bearer "+validToken)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHTTPMessages_MethodNotAllowedWithoutAuth(t *testing.T) {
	srv := newTestServer()
	cfg := buildHTTPConfig(t)

	handler := MessagesHandlerWithConfig(srv, cfg)
	req := httptest.NewRequest(http.MethodGet, "/messages", nil)
	// No Authorization header → should be 401 (auth happens before method
	// check so we never leak whether GET would have been accepted).
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (auth precedes method check)", w.Code)
	}
}

func TestHTTPMessages_NoValidatorFailsClosed(t *testing.T) {
	srv := newTestServer()
	// Config with allowlist but no validator → every request denied.
	cfg := HTTPConfig{AllowedOrigins: []string{"https://app.idcd.com"}}

	w := messagesPost(t, srv, cfg, validToken, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (fail-closed when no validator)", w.Code)
	}
}

// ─────────────────────────────────────────────
// CORS tests
// ─────────────────────────────────────────────

func TestCORS_PreflightAllowed(t *testing.T) {
	srv := newTestServer()
	cfg := buildHTTPConfig(t)
	handler := MessagesHandlerWithConfig(srv, cfg)

	req := httptest.NewRequest(http.MethodOptions, "/messages", nil)
	req.Header.Set("Origin", "https://app.idcd.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://app.idcd.com" {
		t.Errorf("Allow-Origin = %q, want https://app.idcd.com", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
		t.Errorf("Allow-Methods = %q, want POST in list", got)
	}
}

func TestCORS_PreflightDeniedForUnlistedOrigin(t *testing.T) {
	srv := newTestServer()
	cfg := buildHTTPConfig(t)
	handler := MessagesHandlerWithConfig(srv, cfg)

	req := httptest.NewRequest(http.MethodOptions, "/messages", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want empty for unlisted origin", got)
	}
}

func TestCORS_PreflightDeniedWhenAllowlistEmpty(t *testing.T) {
	srv := newTestServer()
	cfg := HTTPConfig{Validator: buildHTTPConfig(t).Validator} // no AllowedOrigins
	handler := MessagesHandlerWithConfig(srv, cfg)

	req := httptest.NewRequest(http.MethodOptions, "/messages", nil)
	req.Header.Set("Origin", "https://app.idcd.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (empty allowlist → deny all CORS)", w.Code)
	}
}

func TestCORS_NoOriginHeaderSkipsCORS(t *testing.T) {
	srv := newTestServer()
	cfg := buildHTTPConfig(t)

	// Same-origin POST: no Origin header, no Access-Control-* on response.
	w := messagesPost(t, srv, cfg, validToken,
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want empty when no Origin header", got)
	}
}

func TestCORS_AllowCredentialsSuppressedWithWildcard(t *testing.T) {
	srv := newTestServer()
	cfg := HTTPConfig{
		Validator:        buildHTTPConfig(t).Validator,
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
	}
	handler := MessagesHandlerWithConfig(srv, cfg)

	req := httptest.NewRequest(http.MethodOptions, "/messages", nil)
	req.Header.Set("Origin", "https://anywhere.example")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (wildcard mode)", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("Allow-Credentials = %q, want empty when wildcard active", got)
	}
}

// ─────────────────────────────────────────────
// SSE tests
// ─────────────────────────────────────────────

func TestSSE_UnauthRejected(t *testing.T) {
	srv := newTestServer()
	cfg := buildHTTPConfig(t)
	handler := SSEHandlerWithConfig(srv, cfg)

	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestSSE_AuthorisedConnectionFlushes(t *testing.T) {
	srv := newTestServer()
	cfg := buildHTTPConfig(t)
	handler := SSEHandlerWithConfig(srv, cfg)

	// SSE blocks on r.Context().Done(); cancel quickly to return.
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/sse", nil)
	req.Header.Set("Authorization", "Bearer "+validToken)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(w, req)
		close(done)
	}()
	// Give the goroutine a tick to write the headers + endpoint event.
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	if !strings.Contains(w.Body.String(), "event: endpoint") {
		t.Errorf("body missing endpoint event: %q", w.Body.String())
	}
}

// ─────────────────────────────────────────────
// Principal context plumbing
// ─────────────────────────────────────────────

func TestPrincipalReachesToolHandler(t *testing.T) {
	srv := NewServer()
	var seen *Principal
	srv.Register(ToolDefinition{Name: "whoami", Description: "x", InputSchema: map[string]any{}}, func(ctx context.Context, _ map[string]any) (string, error) {
		seen = PrincipalFromContext(ctx)
		return "ok", nil
	})

	cfg := buildHTTPConfig(t)
	body := `{"jsonrpc":"2.0","id":99,"method":"tools/call","params":{"name":"whoami","arguments":{}}}`
	w := messagesPost(t, srv, cfg, validToken, body)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (body=%s)", w.Code, w.Body.String())
	}
	if seen == nil {
		t.Fatal("principal not propagated to tool handler")
	}
	if seen.UserID != "usr_test" {
		t.Errorf("UserID = %q, want usr_test", seen.UserID)
	}
	if seen.WorkspaceID != "wks_test" {
		t.Errorf("WorkspaceID = %q, want wks_test", seen.WorkspaceID)
	}
}

// ─────────────────────────────────────────────
// Global config path (back-compat for cmd/mcp/main.go)
// ─────────────────────────────────────────────

// TestToolsCall_ErrToolFailureBecomesIsError verifies the MCP-spec contract:
// a tool handler returning a wrapped ErrToolFailure must be surfaced as a
// successful Response with Result.IsError=true, NOT as a JSON-RPC protocol
// error. This is what lets the client show "API key missing" to the user
// without treating the call as a transport-level failure.
func TestToolsCall_ErrToolFailureBecomesIsError(t *testing.T) {
	srv := NewServer()
	srv.Register(ToolDefinition{Name: "boom", Description: "x", InputSchema: map[string]any{}},
		func(ctx context.Context, _ map[string]any) (string, error) {
			return "", fmt.Errorf("%w: api key required", ErrToolFailure)
		})

	resp := runRequest(t, srv,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"boom","arguments":{}}}`)
	if resp == nil {
		t.Fatal("nil response")
	}
	if resp.Error != nil {
		t.Fatalf("expected Result, got JSON-RPC error: %+v", resp.Error)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("result not map: %T", resp.Result)
	}
	if v, _ := result["isError"].(bool); !v {
		t.Errorf("isError missing or false; result=%v", result)
	}
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatal("content empty")
	}
	item, _ := content[0].(map[string]any)
	if text, _ := item["text"].(string); !strings.Contains(text, "api key required") {
		t.Errorf("content text = %q, want to contain 'api key required'", text)
	}
	// The sentinel prefix ("mcp tool: failure: ") must be stripped from the
	// user-visible content — it's a routing marker, not a display string.
	if text, _ := item["text"].(string); strings.HasPrefix(text, "mcp tool:") {
		t.Errorf("content text leaks sentinel prefix: %q", text)
	}
}

// TestToolsCall_PlainErrorStaysJSONRPCError guarantees that bare (non-tool)
// errors still surface as JSON-RPC ErrInternalError — we don't want to silently
// hide server-side bugs as "tool failures".
func TestToolsCall_PlainErrorStaysJSONRPCError(t *testing.T) {
	srv := NewServer()
	srv.Register(ToolDefinition{Name: "crash", Description: "x", InputSchema: map[string]any{}},
		func(ctx context.Context, _ map[string]any) (string, error) {
			return "", errors.New("unexpected nil pointer")
		})

	resp := runRequest(t, srv,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"crash","arguments":{}}}`)
	if resp == nil || resp.Error == nil {
		t.Fatalf("expected JSON-RPC error response, got: %+v", resp)
	}
	if resp.Error.Code != ErrInternalError {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrInternalError)
	}
}

func TestLegacyHandlers_UseGlobalConfig(t *testing.T) {
	srv := newTestServer()

	// Reset global config at end so we don't leak between tests.
	t.Cleanup(func() { SetHTTPConfig(HTTPConfig{}) })

	// No global config ⇒ everything 401.
	SetHTTPConfig(HTTPConfig{})
	handler := MessagesHandler(srv)
	req := httptest.NewRequest(http.MethodPost, "/messages", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+validToken)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("no global config: status = %d, want 401", w.Code)
	}

	// Wire global config ⇒ valid token works.
	SetHTTPConfig(buildHTTPConfig(t))
	handler = MessagesHandler(srv)
	req = httptest.NewRequest(http.MethodPost, "/messages",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+validToken)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("with global config: status = %d, want 200 (body=%s)",
			w.Code, w.Body.String())
	}
}
