package tools

import (
	"context"
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
		rawTarget, _ := args["target"].(string)
		target, err := validateTarget(rawTarget)
		if err != nil {
			return "", fmt.Errorf("%w: %s", protocol.ErrToolFailure, err.Error())
		}
		maxHops := 30
		if v, ok := args["max_hops"].(float64); ok {
			maxHops = validateCount(v, 30, 64)
		}
		// agent 写回的 traceroute summary,Hop 字段名实测来自 agent/probe.TracerouteHop
		var result struct {
			NodeID  string `json:"node_id"`
			Success bool   `json:"success"`
			Hops    []struct {
				Hop      int     `json:"hop"`
				IP       string  `json:"ip"`
				Hostname string  `json:"hostname"`
				RTTMs    float64 `json:"rtt_ms"`
				Timeout  bool    `json:"timeout"`
				Country  string  `json:"country"`
				City     string  `json:"city"`
			} `json:"hops"`
		}

		body := map[string]any{"target": target, "params": map[string]any{"max_hops": maxHops}}
		if err := client.PollProbeTask(ctx, "/v1/probe/traceroute", body, &result); err != nil {
			return "", fmt.Errorf("%w: %s", protocol.ErrToolFailure, err.Error())
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "TRACEROUTE %s (节点 %s)\n", target, result.NodeID)
		for _, h := range result.Hops {
			ip := h.IP
			if ip == "" || h.Timeout {
				ip = "*"
			}
			geo := ""
			if h.Country != "" || h.City != "" {
				parts := []string{}
				if h.City != "" {
					parts = append(parts, h.City)
				}
				if h.Country != "" {
					parts = append(parts, h.Country)
				}
				geo = fmt.Sprintf(" (%s)", strings.Join(parts, ", "))
			}
			if h.Timeout {
				fmt.Fprintf(&sb, "%2d. %-18s timeout%s\n", h.Hop, ip, geo)
			} else {
				fmt.Fprintf(&sb, "%2d. %-18s %.1fms%s\n", h.Hop, ip, h.RTTMs, geo)
			}
		}
		if len(result.Hops) == 0 {
			fmt.Fprintf(&sb, "无 hop 数据(agent 可能不支持 traceroute 或链路全 timeout)")
		}
		return strings.TrimRight(sb.String(), "\n"), nil
	}
}
