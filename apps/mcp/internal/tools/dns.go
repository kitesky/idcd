package tools

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/kite365/idcd/apps/mcp/internal/apiclient"
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

func handleDNSFunc(client *apiclient.Client) protocol.ToolHandler {
	return func(ctx context.Context, args map[string]any) (string, error) {
		rawDomain, _ := args["domain"].(string)
		domain, err := validateTarget(rawDomain)
		if err != nil {
			return "", fmt.Errorf("%w: %s", protocol.ErrToolFailure, err.Error())
		}
		recordType := "A"
		if v, ok := args["type"].(string); ok && v != "" {
			// DNS record type — short token, must be alphanumeric.
			if len(v) > 16 {
				return "", fmt.Errorf("%w: type too long", protocol.ErrToolFailure)
			}
			recordType = v
		}

		if !client.HasAPIKey() {
			return "", fmt.Errorf("%w: 需要 API key，请设置 IDCD_API_KEY 环境变量", protocol.ErrToolFailure)
		}

		var result struct {
			Domain  string `json:"domain"`
			Type    string `json:"type"`
			Records []struct {
				Value string `json:"value"`
				TTL   uint32 `json:"ttl"`
			} `json:"records"`
		}

		params := url.Values{}
		params.Set("q", domain)
		params.Set("type", recordType)
		if err := client.Get(ctx, "/v1/info/dns", params, &result); err != nil {
			return "", fmt.Errorf("%w: 调用失败: %s", protocol.ErrToolFailure, err.Error())
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "DNS %s (%s):\n", domain, recordType)
		for _, r := range result.Records {
			if r.TTL > 0 {
				fmt.Fprintf(&sb, "%s: %s (TTL: %d)\n", recordType, r.Value, r.TTL)
			} else {
				fmt.Fprintf(&sb, "%s: %s\n", recordType, r.Value)
			}
		}
		if len(result.Records) == 0 {
			fmt.Fprintf(&sb, "无记录")
		}
		return strings.TrimRight(sb.String(), "\n"), nil
	}
}
