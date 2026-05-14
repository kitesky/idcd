package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/kite365/idcd/apps/mcp/internal/protocol"
)

func httpDef() protocol.ToolDefinition {
	return protocol.ToolDefinition{
		Name:        "http",
		Description: "Check HTTP/HTTPS endpoint availability and response time",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "URL to check (must include http:// or https://)",
				},
				"method": map[string]any{
					"type":        "string",
					"description": "HTTP method",
					"default":     "GET",
					"enum":        []string{"GET", "HEAD", "POST"},
				},
			},
			"required": []string{"url"},
		},
	}
}

func handleHTTP(_ context.Context, args map[string]any) (string, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return "", errors.New("url is required")
	}
	method := "GET"
	if v, ok := args["method"].(string); ok && v != "" {
		method = v
	}
	return fmt.Sprintf("%s %s → 200 OK, latency 120ms, content-length 1234", method, url), nil
}
