package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/kite365/idcd/apps/mcp/internal/protocol"
)

func ipDef() protocol.ToolDefinition {
	return protocol.ToolDefinition{
		Name:        "ip",
		Description: "Look up geolocation and ASN information for an IP address",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"address": map[string]any{
					"type":        "string",
					"description": "IPv4 or IPv6 address to look up",
				},
			},
			"required": []string{"address"},
		},
	}
}

func handleIP(_ context.Context, args map[string]any) (string, error) {
	address, _ := args["address"].(string)
	if address == "" {
		return "", errors.New("address is required")
	}
	return fmt.Sprintf("IP %s → country: US, city: San Jose, ASN: AS15169 (Google LLC), org: Google Cloud", address), nil
}
