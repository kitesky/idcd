package tools

import (
	"context"
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
		rawTarget, _ := args["target"].(string)
		target, err := validateTarget(rawTarget)
		if err != nil {
			return "", fmt.Errorf("%w: %s", protocol.ErrToolFailure, err.Error())
		}
		count := 3
		if v, ok := args["count"].(float64); ok {
			count = validateCount(v, 3, 50)
		}
		// api /v1/probe/* 是 async:POST 拿 task_id → 轮询 GET /v1/probe/tasks/{id}
		// 直到 status=completed。result 是 agent 写回的 summary,字段随 probe 类型变化。
		var result struct {
			NodeID          string  `json:"node_id"`
			Success         bool    `json:"success"`
			DurationMs      float64 `json:"duration_ms"`
			PacketLoss      float64 `json:"packet_loss"`
			PacketsSent     int     `json:"packets_sent"`
			PacketsReceived int     `json:"packets_received"`
			AvgMs           float64 `json:"avg_ms"`
			MinMs           float64 `json:"min_ms"`
			MaxMs           float64 `json:"max_ms"`
		}

		// api 期望 ProbeRequest{target, nodeID, nodes, params} — count 必须塞 params,
		// 不然 agent 拿到的 options 为空,默认 count=5。
		body := map[string]any{"target": target, "params": map[string]any{"count": count}}
		if err := client.PollProbeTask(ctx, "/v1/probe/ping", body, &result); err != nil {
			return "", fmt.Errorf("%w: %s", protocol.ErrToolFailure, err.Error())
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "PING %s (count=%d)\n", target, count)
		if result.NodeID != "" {
			fmt.Fprintf(&sb, "节点: %s\n", result.NodeID)
		}
		if result.PacketsSent > 0 {
			fmt.Fprintf(&sb, "包: 发送 %d / 收到 %d\n", result.PacketsSent, result.PacketsReceived)
		}
		// avg_ms 是新版 summary;duration_ms 是老 summary 的兜底,避免 schema drift
		avg := result.AvgMs
		if avg == 0 && result.DurationMs > 0 {
			avg = result.DurationMs
		}
		if avg > 0 {
			fmt.Fprintf(&sb, "平均: %.1fms", avg)
			if result.MinMs > 0 || result.MaxMs > 0 {
				fmt.Fprintf(&sb, " (min %.1f / max %.1f)", result.MinMs, result.MaxMs)
			}
			fmt.Fprintf(&sb, "\n")
		}
		fmt.Fprintf(&sb, "丢包: %.0f%%", result.PacketLoss)
		if !result.Success {
			fmt.Fprintf(&sb, " ✗")
		}
		return sb.String(), nil
	}
}
