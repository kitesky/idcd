package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kite365/idcd/apps/mcp/internal/apiclient"
	"github.com/kite365/idcd/apps/mcp/internal/protocol"
)

func pingDef() protocol.ToolDefinition {
	return protocol.ToolDefinition{
		Name:        "ping",
		Description: "Ping a host to measure round-trip latency and packet loss",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target": map[string]any{
					"type":        "string",
					"description": "Hostname or IP address to ping",
				},
				"count": map[string]any{
					"type":        "integer",
					"description": "Number of packets to send",
					"default":     3,
				},
			},
			"required": []string{"target"},
		},
	}
}

func handlePingFunc(client *apiclient.Client) protocol.ToolHandler {
	return func(ctx context.Context, args map[string]any) (string, error) {
		target, _ := args["target"].(string)
		if target == "" {
			return "", errors.New("target is required")
		}
		count := 3
		if v, ok := args["count"].(float64); ok && v > 0 {
			count = int(v)
		}

		if !client.HasAPIKey() {
			return "⚠ 需要 API key，请设置 IDCD_API_KEY 环境变量", nil
		}

		var result struct {
			Target  string `json:"target"`
			Results []struct {
				Node    string  `json:"node"`
				Country string  `json:"country"`
				AvgMs   float64 `json:"avg_ms"`
				Loss    float64 `json:"loss_pct"`
				Success bool    `json:"success"`
			} `json:"results"`
			AvgMs   float64 `json:"avg_ms"`
			LossPct float64 `json:"loss_pct"`
		}

		body := map[string]any{"target": target, "count": count}
		if err := client.Post(ctx, "/v1/probe/ping", body, &result); err != nil {
			return fmt.Sprintf("✗ 调用失败: %s", err.Error()), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "PING %s\n", target)
		for _, r := range result.Results {
			loc := r.Node
			if r.Country != "" {
				loc = fmt.Sprintf("%s %s", r.Node, r.Country)
			}
			status := "✓"
			if !r.Success {
				status = "✗"
			}
			fmt.Fprintf(&sb, "节点: %s — %.0fms %s\n", loc, r.AvgMs, status)
		}
		fmt.Fprintf(&sb, "平均: %.0fms | 丢包: %.0f%%", result.AvgMs, result.LossPct)
		return sb.String(), nil
	}
}
