package tools

import (
	"context"
	"errors"
	"fmt"

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

func handleTraceroute(_ context.Context, args map[string]any) (string, error) {
	target, _ := args["target"].(string)
	if target == "" {
		return "", errors.New("target is required")
	}
	maxHops := 30
	if v, ok := args["max_hops"].(float64); ok && v > 0 {
		maxHops = int(v)
	}
	return fmt.Sprintf("TRACEROUTE %s (max %d hops): 8 hops, destination reached, total latency 48ms\n  1. 192.168.1.1 (1ms)\n  2. 10.0.0.1 (5ms)\n  3. 203.0.113.1 (12ms)\n  8. %s (48ms)", target, maxHops, target), nil
}
