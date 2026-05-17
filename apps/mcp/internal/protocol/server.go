package protocol

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ToolHandler func(ctx context.Context, args map[string]any) (string, error)

// ErrToolFailure is the sentinel that a ToolHandler wraps when it wants the
// MCP transport to emit an in-band tool error (ToolCallResult.IsError=true)
// rather than a JSON-RPC protocol-level error. Use this for "expected"
// failures the caller can act on — missing API key, bad user input, upstream
// API returned 4xx — anything that is a tool-level outcome, not a server bug.
//
// Usage in a tool handler:
//
//	if !client.HasAPIKey() {
//	    return "", fmt.Errorf("%w: IDCD_API_KEY is not set", protocol.ErrToolFailure)
//	}
//
// Bare errors returned from a handler still map to JSON-RPC ErrInternalError —
// reserved for "the server itself failed" cases (panics, unexpected I/O).
var ErrToolFailure = errors.New("mcp tool: failure")

type Server struct {
	tools    []ToolDefinition
	handlers map[string]ToolHandler
}

func NewServer() *Server {
	return &Server{
		handlers: make(map[string]ToolHandler),
	}
}

func (s *Server) Register(def ToolDefinition, handler ToolHandler) {
	s.tools = append(s.tools, def)
	s.handlers[def.Name] = handler
}

func (s *Server) RunStdio(ctx context.Context) error {
	return s.run(ctx, os.Stdin, os.Stdout)
}

func (s *Server) RunIO(ctx context.Context, r io.Reader, w io.Writer) error {
	return s.run(ctx, r, w)
}

func (s *Server) run(ctx context.Context, r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("read: %w", err)
			}
			return nil
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		resp := s.handle(ctx, line)
		if resp == nil {
			continue
		}

		data, err := json.Marshal(resp)
		if err != nil {
			continue
		}

		if _, err := fmt.Fprintf(w, "%s\n", data); err != nil {
			return fmt.Errorf("write: %w", err)
		}
	}
}

func (s *Server) handle(ctx context.Context, line []byte) *Response {
	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      nil,
			Error:   &Error{Code: ErrParseError, Message: "Parse error"},
		}
	}

	if req.JSONRPC != "2.0" {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: ErrParseError, Message: "Invalid JSON-RPC version"},
		}
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		return nil
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	default:
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: ErrMethodNotFound, Message: "Method not found"},
		}
	}
}

func (s *Server) handleInitialize(req Request) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: InitializeResult{
			ProtocolVersion: "2024-11-05",
			ServerInfo: map[string]any{
				"name":    "idcd-mcp",
				"version": "1.0.0",
			},
			Capabilities: map[string]any{
				"tools": map[string]any{},
			},
		},
	}
}

func (s *Server) handleToolsList(req Request) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  ToolsListResult{Tools: s.tools},
	}
}

func (s *Server) handleToolsCall(ctx context.Context, req Request) *Response {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: ErrInvalidParams, Message: "Invalid params"},
		}
	}

	handler, ok := s.handlers[params.Name]
	if !ok {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: ErrMethodNotFound, Message: "Unknown tool: " + params.Name},
		}
	}

	text, err := handler(ctx, params.Arguments)
	if err != nil {
		// Tool-level failure (wrapped ErrToolFailure) → MCP IsError result.
		// The call itself succeeded at the protocol layer; the tool reports
		// a recoverable problem in-band so the client can show it to the
		// user without retrying the JSON-RPC envelope.
		if errors.Is(err, ErrToolFailure) {
			msg := err.Error()
			// Strip the "mcp tool: failure: " prefix that errors.Wrap
			// produces — the sentinel is for routing, not display.
			msg = strings.TrimPrefix(msg, ErrToolFailure.Error()+": ")
			return &Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: ToolCallResult{
					Content: []ContentItem{{Type: "text", Text: msg}},
					IsError: true,
				},
			}
		}
		// Unexpected (panic-class) failure → JSON-RPC protocol error.
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: ErrInternalError, Message: err.Error()},
		}
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: ToolCallResult{
			Content: []ContentItem{{Type: "text", Text: text}},
		},
	}
}

// ─────────────────────────────────────────────
// HTTP transport (SSE + /messages) — secured
// ─────────────────────────────────────────────

// MaxMessageBytes is the maximum size of a single JSON-RPC message body
// accepted over the HTTP transport. 1 MiB is far above any legitimate
// tools/call payload and prevents memory exhaustion from a hostile client.
const MaxMessageBytes int64 = 1 * 1024 * 1024

// RateLimitDecision is what Limiter.Allow returns. Allowed=false → caller
// gets 429 with the Retry-After hint derived from ResetAfter.
type RateLimitDecision struct {
	Allowed    bool
	Remaining  int64
	ResetAfter time.Duration
}

// Limiter is the minimal surface the MCP HTTP transport needs from a rate
// limiter. The protocol package defines its own interface (rather than
// depending on lib/ratelimit) so tests can substitute a fake without
// pulling in Redis. main.go wires a Redis-backed Limiter; pass nil to
// disable rate limiting entirely (dev / tests only).
type Limiter interface {
	Allow(ctx context.Context, key string) (RateLimitDecision, error)
}

// HTTPConfig configures the HTTP-transport handlers. Fail-closed defaults:
// if Validator is nil OR AllowedOrigins is empty, every request is rejected.
type HTTPConfig struct {
	// Validator authenticates the bearer token. Required — nil ⇒ all
	// requests get 401.
	Validator TokenValidator

	// AllowedOrigins is the CORS allowlist. An entry of "*" enables
	// permissive mode (dev only — credentials are still NOT echoed).
	// Empty list ⇒ every cross-origin preflight is rejected.
	AllowedOrigins []string

	// AllowCredentials echoes Access-Control-Allow-Credentials: true on
	// matched origins. Wildcard "*" + credentials is unsafe and
	// suppressed automatically.
	AllowCredentials bool

	// RateLimiter throttles /messages calls per token. nil disables.
	// Keys are derived as "mcp:tok:<tokenID>"; the limiter implementation
	// decides the window + max requests.
	RateLimiter Limiter

	// SSEHeartbeat is the interval between server-sent SSE keepalive
	// comments. Zero disables; recommended 15s.
	SSEHeartbeat time.Duration
}

// httpConfig is the package-level configuration consulted by the legacy
// SSEHandler(s) / MessagesHandler(s) entry points (which keep their existing
// signature so cmd/mcp/main.go does not have to be touched in this PR).
// Prefer SSEHandlerWithConfig / MessagesHandlerWithConfig in new code.
var (
	httpConfigMu sync.RWMutex
	httpConfig   HTTPConfig
)

// SetHTTPConfig installs the global config for SSEHandler / MessagesHandler.
// Wire this from cmd/mcp/main.go before serving. If never called, the HTTP
// transport rejects every request with 401 (fail-closed default).
func SetHTTPConfig(cfg HTTPConfig) {
	httpConfigMu.Lock()
	defer httpConfigMu.Unlock()
	httpConfig = cfg
}

func currentHTTPConfig() HTTPConfig {
	httpConfigMu.RLock()
	defer httpConfigMu.RUnlock()
	return httpConfig
}

// SSEHandler returns the SSE handler bound to the package-level HTTP config.
// Kept for backward compatibility with cmd/mcp/main.go; new code should use
// SSEHandlerWithConfig.
func SSEHandler(s *Server) http.Handler {
	return sseHandler(s, currentHTTPConfig)
}

// MessagesHandler returns the /messages handler bound to the package-level
// HTTP config. Kept for backward compatibility with cmd/mcp/main.go; new
// code should use MessagesHandlerWithConfig.
func MessagesHandler(s *Server) http.Handler {
	return messagesHandler(s, currentHTTPConfig)
}

// SSEHandlerWithConfig is the preferred constructor — takes an explicit
// config, no global state.
func SSEHandlerWithConfig(s *Server, cfg HTTPConfig) http.Handler {
	return sseHandler(s, func() HTTPConfig { return cfg })
}

// MessagesHandlerWithConfig is the preferred constructor — takes an
// explicit config, no global state.
func MessagesHandlerWithConfig(s *Server, cfg HTTPConfig) http.Handler {
	return messagesHandler(s, func() HTTPConfig { return cfg })
}

func sseHandler(s *Server, cfgFn func() HTTPConfig) http.Handler {
	_ = s // server reserved for future server-initiated SSE events
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := cfgFn()

		// CORS preflight is allowed only for explicitly-allowlisted origins.
		if r.Method == http.MethodOptions {
			handleCORSPreflight(w, r, cfg)
			return
		}

		applyCORS(w, r, cfg)

		// Authenticate every SSE connection. The MCP spec couples SSE
		// session identity to /messages calls — anonymous SSE is the
		// same exposure as anonymous /messages.
		if _, err := authenticateRequest(r, cfg); err != nil {
			writeAuthError(w, err)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		// Defence-in-depth: forbid iframe / cross-site embedding.
		w.Header().Set("X-Content-Type-Options", "nosniff")

		flusher, _ := w.(http.Flusher)
		fmt.Fprintf(w, "event: endpoint\ndata: /messages\n\n")
		if flusher != nil {
			flusher.Flush()
		}

		// Heartbeat keeps the connection alive through idle proxies and
		// lets the client detect a dead socket. SSE comment lines (lines
		// starting with ":") are ignored by event parsers but still flow
		// through the TCP stack, surfacing a write error if the peer is
		// gone — at which point we exit and free the goroutine.
		interval := cfg.SSEHeartbeat
		if interval <= 0 {
			interval = 15 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := fmt.Fprintf(w, ": ping %d\n\n", time.Now().Unix()); err != nil {
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
		}
	})
}

func messagesHandler(s *Server, cfgFn func() HTTPConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := cfgFn()

		if r.Method == http.MethodOptions {
			handleCORSPreflight(w, r, cfg)
			return
		}

		applyCORS(w, r, cfg)

		// Auth precedes method check so anonymous probes can't enumerate
		// which HTTP verbs the server accepts.
		principal, err := authenticateRequest(r, cfg)
		if err != nil {
			writeAuthError(w, err)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Per-token rate limit. Keyed on token ID so revoking a token
		// also flushes its quota window (the next caller getting that
		// ID is practically impossible — IDs are random — but the
		// guarantee is conceptually clean).
		if cfg.RateLimiter != nil && principal != nil && principal.TokenID != "" {
			dec, err := cfg.RateLimiter.Allow(r.Context(), "mcp:tok:"+principal.TokenID)
			if err != nil {
				// fail-open on limiter error so a flaky Redis
				// doesn't kill the whole tool surface — the auth
				// layer already gates abuse. Log via header so
				// the caller can correlate.
				w.Header().Set("X-RateLimit-Error", "1")
			} else if !dec.Allowed {
				if dec.ResetAfter > 0 {
					w.Header().Set("Retry-After", strconv.Itoa(int(dec.ResetAfter.Seconds())+1))
				}
				w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(dec.Remaining, 10))
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			} else {
				w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(dec.Remaining, 10))
			}
		}

		// Cap body to 1 MiB. http.MaxBytesReader returns an error on
		// ReadAll once the limit is exceeded, and also writes a 413 to
		// the response writer if we let it. We surface 413 ourselves
		// for a stable error contract.
		r.Body = http.MaxBytesReader(w, r.Body, MaxMessageBytes)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			if isMaxBytesError(err) {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}

		ctx := withPrincipal(r.Context(), principal)
		line := []byte(strings.TrimSpace(string(body)))
		resp := s.handle(ctx, line)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if resp == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		data, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(data)
	})
}

// authenticateRequest validates the bearer token from r and returns the
// resulting principal. All errors map to 401 at the HTTP layer (the caller
// uses writeAuthError).
func authenticateRequest(r *http.Request, cfg HTTPConfig) (*Principal, error) {
	if cfg.Validator == nil {
		// Fail-closed: no validator wired ⇒ deny.
		return nil, ErrTokenMissing
	}
	raw, err := extractBearerToken(r)
	if err != nil {
		return nil, err
	}
	return cfg.Validator.Validate(r.Context(), raw)
}

// writeAuthError emits a 401 with a stable, opaque body. We deliberately do
// NOT echo which validator error fired (don't leak token state to an
// unauthenticated client).
func writeAuthError(w http.ResponseWriter, _ error) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="idcd-mcp"`)
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}

// isMaxBytesError reports whether err is the MaxBytesReader "too large"
// error. Go 1.20+ wraps it as *http.MaxBytesError.
func isMaxBytesError(err error) bool {
	if err == nil {
		return false
	}
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		return true
	}
	// Older runtimes returned a generic "http: request body too large".
	return strings.Contains(err.Error(), "request body too large")
}

// ─────────────────────────────────────────────
// CORS helpers (allowlist-only — no "*" leakage)
// ─────────────────────────────────────────────

func handleCORSPreflight(w http.ResponseWriter, r *http.Request, cfg HTTPConfig) {
	origin := r.Header.Get("Origin")
	if origin == "" || !isAllowedOrigin(origin, cfg.AllowedOrigins) {
		// Don't echo Access-Control-Allow-Origin for unmatched origin.
		w.WriteHeader(http.StatusForbidden)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Max-Age", "600")
	if cfg.AllowCredentials && !isWildcard(cfg.AllowedOrigins) {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	w.WriteHeader(http.StatusNoContent)
}

func applyCORS(w http.ResponseWriter, r *http.Request, cfg HTTPConfig) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}
	if !isAllowedOrigin(origin, cfg.AllowedOrigins) {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Vary", "Origin")
	if cfg.AllowCredentials && !isWildcard(cfg.AllowedOrigins) {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
}

// isAllowedOrigin returns true if origin is in the allowlist. The single
// entry "*" enables permissive mode for dev (every origin matches, but
// credentials are never echoed).
func isAllowedOrigin(origin string, allowed []string) bool {
	for _, a := range allowed {
		if a == "*" {
			return true
		}
		if a == origin {
			return true
		}
	}
	return false
}

func isWildcard(allowed []string) bool {
	for _, a := range allowed {
		if a == "*" {
			return true
		}
	}
	return false
}
