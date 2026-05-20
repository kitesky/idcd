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

		var sb strings.Builder
		fmt.Fprintf(&sb, "SSL/TLS for %s:\n", host)
		// 每个字段独立守护:api 部分字段缺失时不渲染 "证书: " 这种半空行。
		// 没法一刀切用 "未知" 占位 — issuer/subject 是必有项,缺了说明上游异常,
		// 不输出比输出空更诚实。
		if result.Subject != "" || result.Issuer != "" {
			fmt.Fprintf(&sb, "证书: %s | 颁发者: %s\n", result.Subject, result.Issuer)
		}
		if expiry != "" {
			fmt.Fprintf(&sb, "有效期: %s (还有 %d 天)\n", expiry, result.DaysUntilExpiry)
		}
		if result.Protocol != "" {
			fmt.Fprintf(&sb, "协议: %s\n", result.Protocol)
		}
		return strings.TrimRight(sb.String(), "\n"), nil
	}
}
