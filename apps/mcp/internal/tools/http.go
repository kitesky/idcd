package tools

import (
	"context"
	"fmt"

	"github.com/kite365/idcd/apps/mcp/internal/apiclient"
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

func handleHTTPFunc(client *apiclient.Client) protocol.ToolHandler {
	return func(ctx context.Context, args map[string]any) (string, error) {
		rawURL, _ := args["url"].(string)
		targetURL, err := validateURL(rawURL)
		if err != nil {
			return "", err
		}
		method := "GET"
		if v, ok := args["method"].(string); ok && v != "" {
			// Whitelist verbs — apiclient appends to a URL but the
			// downstream service uses the method as-is; an injected
			// "GET\r\nX-Admin: 1" would smuggle a header.
			switch v {
			case "GET", "HEAD", "POST", "PUT", "DELETE", "PATCH", "OPTIONS":
				method = v
			default:
				return "", fmt.Errorf("unsupported method %q", v)
			}
		}

		if !client.HasAPIKey() {
			return "⚠ 需要 API key，请设置 IDCD_API_KEY 环境变量", nil
		}

		var result struct {
			URL         string `json:"url"`
			StatusCode  int    `json:"status_code"`
			StatusText  string `json:"status_text"`
			LatencyMs   int    `json:"latency_ms"`
			TLSVersion  string `json:"tls_version"`
			ContentType string `json:"content_type"`
		}

		body := map[string]any{"url": targetURL, "method": method}
		if err := client.Post(ctx, "/v1/probe/http", body, &result); err != nil {
			return fmt.Sprintf("✗ 调用失败: %s", err.Error()), nil
		}

		tls := ""
		if result.TLSVersion != "" {
			tls = fmt.Sprintf(" | TLS: %s", result.TLSVersion)
		}
		ct := ""
		if result.ContentType != "" {
			ct = fmt.Sprintf("\n响应头: Content-Type: %s", result.ContentType)
		}

		return fmt.Sprintf("%s %s\n状态: %d %s | 延迟: %dms%s%s",
			method, targetURL,
			result.StatusCode, result.StatusText,
			result.LatencyMs,
			tls,
			ct,
		), nil
	}
}
