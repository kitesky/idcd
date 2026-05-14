package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kite365/idcd/apps/mcp/internal/apiclient"
	"github.com/kite365/idcd/apps/mcp/internal/protocol"
)

func tracerouteDef() protocol.ToolDefinition {
	return protocol.ToolDefinition{
		Name:        "traceroute",
		Description: "Trace the network path to a host",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target": map[string]any{
					"type":        "string",
					"description": "Hostname or IP address",
				},
				"max_hops": map[string]any{
					"type":        "integer",
					"description": "Maximum number of hops",
					"default":     30,
				},
			},
			"required": []string{"target"},
		},
	}
}

func handleTracerouteFunc(client *apiclient.Client) protocol.ToolHandler {
	return func(ctx context.Context, args map[string]any) (string, error) {
		target, _ := args["target"].(string)
		if target == "" {
			return "", errors.New("target is required")
		}
		maxHops := 30
		if v, ok := args["max_hops"].(float64); ok && v > 0 {
			maxHops = int(v)
		}

		if !client.HasAPIKey() {
			return "⚠ 需要 API key，请设置 IDCD_API_KEY 环境变量", nil
		}

		var result struct {
			Target string `json:"target"`
			Hops   []struct {
				TTL     int     `json:"ttl"`
				IP      string  `json:"ip"`
				Latency float64 `json:"latency_ms"`
				ASN     string  `json:"asn"`
				ISP     string  `json:"isp"`
			} `json:"hops"`
		}

		body := map[string]any{"target": target, "max_hops": maxHops}
		if err := client.Post(ctx, "/v1/probe/traceroute", body, &result); err != nil {
			return fmt.Sprintf("✗ 调用失败: %s", err.Error()), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "TRACEROUTE %s\n", target)
		for _, h := range result.Hops {
			extra := ""
			if h.ASN != "" || h.ISP != "" {
				parts := []string{}
				if h.ASN != "" {
					parts = append(parts, h.ASN)
				}
				if h.ISP != "" {
					parts = append(parts, h.ISP)
				}
				extra = fmt.Sprintf(" (%s)", strings.Join(parts, ", "))
			}
			fmt.Fprintf(&sb, "%2d. %-18s %.1fms%s\n", h.TTL, h.IP, h.Latency, extra)
		}
		return strings.TrimRight(sb.String(), "\n"), nil
	}
}
