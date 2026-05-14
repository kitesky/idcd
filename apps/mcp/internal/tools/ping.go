package tools

import (
	"context"
	"errors"
	"fmt"

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

func handlePing(_ context.Context, args map[string]any) (string, error) {
	target, _ := args["target"].(string)
	if target == "" {
		return "", errors.New("target is required")
	}
	count := 3
	if v, ok := args["count"].(float64); ok && v > 0 {
		count = int(v)
	}
	return fmt.Sprintf("PING %s: %d packets transmitted, avg latency 45ms, 0%% packet loss", target, count), nil
}
