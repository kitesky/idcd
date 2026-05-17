package tools

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/kite365/idcd/apps/mcp/internal/apiclient"
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

func handleWhoisFunc(client *apiclient.Client) protocol.ToolHandler {
	return func(ctx context.Context, args map[string]any) (string, error) {
		rawQuery, _ := args["query"].(string)
		query, err := validateTarget(rawQuery)
		if err != nil {
			return "", err
		}

		if !client.HasAPIKey() {
			return "⚠ 需要 API key，请设置 IDCD_API_KEY 环境变量", nil
		}

		var result struct {
			Domain       string   `json:"domain"`
			Registrar    string   `json:"registrar"`
			CreationDate string   `json:"creation_date"`
			ExpiryDate   string   `json:"expiry_date"`
			NameServers  []string `json:"name_servers"`
		}

		params := url.Values{}
		params.Set("q", query)
		if err := client.Get(ctx, "/v1/info/whois", params, &result); err != nil {
			return fmt.Sprintf("✗ 调用失败: %s", err.Error()), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "WHOIS: %s\n", query)
		if result.Registrar != "" {
			fmt.Fprintf(&sb, "注册商: %s\n", result.Registrar)
		}
		if result.CreationDate != "" {
			fmt.Fprintf(&sb, "注册日期: %s\n", result.CreationDate)
		}
		if result.ExpiryDate != "" {
			fmt.Fprintf(&sb, "到期日期: %s", result.ExpiryDate)
		}
		return strings.TrimRight(sb.String(), "\n"), nil
	}
}
