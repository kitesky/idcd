package protocol

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type ToolHandler func(ctx context.Context, args map[string]any) (string, error)

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

func SSEHandler(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		fmt.Fprintf(w, "event: endpoint\ndata: /messages\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		<-r.Context().Done()
	})
}

func MessagesHandler(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}

		line := []byte(strings.TrimSpace(string(body)))
		resp := s.handle(r.Context(), line)

		w.Header().Set("Content-Type", "application/json")
		if resp == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		data, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Write(data)
	})
}
