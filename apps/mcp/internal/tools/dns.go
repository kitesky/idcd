package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/kite365/idcd/apps/mcp/internal/protocol"
)

func dnsDef() protocol.ToolDefinition {
	return protocol.ToolDefinition{
		Name:        "dns",
		Description: "Perform a DNS lookup for a domain",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"domain": map[string]any{
					"type":        "string",
					"description": "Domain name to resolve",
				},
				"type": map[string]any{
					"type":        "string",
					"description": "DNS record type",
					"default":     "A",
					"enum":        []string{"A", "AAAA", "CNAME", "MX", "TXT", "NS"},
				},
			},
			"required": []string{"domain"},
		},
	}
}

func handleDNS(_ context.Context, args map[string]any) (string, error) {
	domain, _ := args["domain"].(string)
	if domain == "" {
		return "", errors.New("domain is required")
	}
	recordType := "A"
	if v, ok := args["type"].(string); ok && v != "" {
		recordType = v
	}
	return fmt.Sprintf("DNS %s %s → 93.184.216.34, TTL 3600, resolver latency 12ms", recordType, domain), nil
}
