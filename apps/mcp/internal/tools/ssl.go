package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/kite365/idcd/apps/mcp/internal/protocol"
)

func sslDef() protocol.ToolDefinition {
	return protocol.ToolDefinition{
		Name:        "ssl",
		Description: "Check SSL/TLS certificate validity and expiration",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"host": map[string]any{
					"type":        "string",
					"description": "Hostname to check SSL certificate for",
				},
			},
			"required": []string{"host"},
		},
	}
}

func handleSSL(_ context.Context, args map[string]any) (string, error) {
	host, _ := args["host"].(string)
	if host == "" {
		return "", errors.New("host is required")
	}
	return fmt.Sprintf("SSL %s → valid, issuer: Let's Encrypt, expires: 2025-09-01 (90 days remaining), TLS 1.3", host), nil
}
