package tools

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/kite365/idcd/apps/mcp/internal/apiclient"
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

func handleSSLFunc(client *apiclient.Client) protocol.ToolHandler {
	return func(ctx context.Context, args map[string]any) (string, error) {
		rawHost, _ := args["host"].(string)
		host, err := validateTarget(rawHost)
		if err != nil {
			return "", fmt.Errorf("%w: %s", protocol.ErrToolFailure, err.Error())
		}

		if !client.HasAPIKey() {
			return "", fmt.Errorf("%w: 需要 API key，请设置 IDCD_API_KEY 环境变量", protocol.ErrToolFailure)
		}

		var result struct {
			Domain          string `json:"domain"`
			Issuer          string `json:"issuer"`
			Subject         string `json:"subject"`
			NotAfter        string `json:"not_after"`
			Protocol        string `json:"protocol"`
			DaysUntilExpiry int    `json:"days_until_expiry"`
		}

		params := url.Values{}
		params.Set("q", host)
		if err := client.Get(ctx, "/v1/info/ssl", params, &result); err != nil {
			return "", fmt.Errorf("%w: 调用失败: %s", protocol.ErrToolFailure, err.Error())
		}

		expiry := result.NotAfter
		if len(expiry) >= 10 {
			expiry = expiry[:10]
		}

		proto := result.Protocol
		if proto == "" {
			proto = "TLS 1.3"
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "SSL/TLS for %s:\n", host)
		fmt.Fprintf(&sb, "证书: %s | 颁发者: %s\n", result.Subject, result.Issuer)
		fmt.Fprintf(&sb, "有效期: %s (还有 %d 天)\n", expiry, result.DaysUntilExpiry)
		fmt.Fprintf(&sb, "协议: %s", proto)
		return sb.String(), nil
	}
}
