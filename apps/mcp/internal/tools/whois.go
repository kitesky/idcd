package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/kite365/idcd/apps/mcp/internal/protocol"
)

func whoisDef() protocol.ToolDefinition {
	return protocol.ToolDefinition{
		Name:        "whois",
		Description: "Query WHOIS registration information for a domain or IP",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Domain name or IP address to query",
				},
			},
			"required": []string{"query"},
		},
	}
}

func handleWhois(_ context.Context, args map[string]any) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "", errors.New("query is required")
	}
	return fmt.Sprintf("WHOIS %s → registrar: GoDaddy, created: 2010-01-01, expires: 2030-01-01, status: active, nameservers: ns1.example.com ns2.example.com", query), nil
}
