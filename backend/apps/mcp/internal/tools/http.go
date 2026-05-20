package tools

import (
	"context"
	"fmt"
	"strings"

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
			return "", fmt.Errorf("%w: %s", protocol.ErrToolFailure, err.Error())
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
				return "", fmt.Errorf("%w: unsupported method %q", protocol.ErrToolFailure, v)
			}
		}
		// agent 写回的 http result summary(实测 schema)
		var result struct {
			NodeID         string `json:"node_id"`
			Success        bool   `json:"success"`
			StatusCode     int    `json:"status_code"`
			TLSVersion     string `json:"tls_version"`
			ContentLength  int64  `json:"content_length"`
			DurationMs     int64  `json:"duration_ms"`
			DNSMs          int64  `json:"dns_ms"`
			ConnectMs      int64  `json:"connect_ms"`
			SSLMs          int64  `json:"ssl_ms"`
			TTFBMs         int64  `json:"ttfb_ms"`
			ServerMs       int64  `json:"server_ms"`
			DownloadMs     int64  `json:"download_ms"`
			TotalMs        int64  `json:"total_ms"`
		}

		// api 用 `target` 接收 URL,不是 `url`(参数名跟 ping/traceroute 统一)
		body := map[string]any{"target": targetURL, "params": map[string]any{"method": method}}
		if err := client.PollProbeTask(ctx, "/v1/probe/http", body, &result); err != nil {
			return "", fmt.Errorf("%w: %s", protocol.ErrToolFailure, err.Error())
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "%s %s\n", method, targetURL)
		fmt.Fprintf(&sb, "状态: %d", result.StatusCode)
		if !result.Success {
			fmt.Fprintf(&sb, " ✗")
		}
		total := result.TotalMs
		if total == 0 {
			total = result.DurationMs
		}
		if total > 0 {
			fmt.Fprintf(&sb, " | 总耗时: %dms", total)
		}
		fmt.Fprintf(&sb, "\n")
		// 阶段分解,信号丰富的时候列出来,缺数据时跳过
		stages := []struct {
			label string
			ms    int64
		}{
			{"DNS", result.DNSMs},
			{"Connect", result.ConnectMs},
			{"SSL", result.SSLMs},
			{"TTFB", result.TTFBMs},
			{"Server", result.ServerMs},
			{"Download", result.DownloadMs},
		}
		var stageParts []string
		for _, s := range stages {
			if s.ms > 0 {
				stageParts = append(stageParts, fmt.Sprintf("%s %dms", s.label, s.ms))
			}
		}
		if len(stageParts) > 0 {
			fmt.Fprintf(&sb, "阶段: %s\n", strings.Join(stageParts, " · "))
		}
		if result.TLSVersion != "" {
			fmt.Fprintf(&sb, "TLS: %s\n", result.TLSVersion)
		}
		if result.ContentLength >= 0 {
			fmt.Fprintf(&sb, "Content-Length: %d", result.ContentLength)
		}
		return strings.TrimRight(sb.String(), "\n"), nil
	}
}
