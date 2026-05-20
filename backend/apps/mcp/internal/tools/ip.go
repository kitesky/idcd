package tools

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/kite365/idcd/apps/mcp/internal/apiclient"
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

func handleIPFunc(client *apiclient.Client) protocol.ToolHandler {
	return func(ctx context.Context, args map[string]any) (string, error) {
		rawAddress, _ := args["address"].(string)
		address, err := validateTarget(rawAddress)
		if err != nil {
			return "", fmt.Errorf("%w: %s", protocol.ErrToolFailure, err.Error())
		}

		if !client.HasAPIKey() {
			return "", fmt.Errorf("%w: 需要 API key，请设置 IDCD_API_KEY 环境变量", protocol.ErrToolFailure)
		}

		var result struct {
			IP      string `json:"ip"`
			Country string `json:"country"`
			City    string `json:"city"`
			ASN     string `json:"asn"`
			ISP     string `json:"isp"`
		}

		params := url.Values{}
		params.Set("q", address)
		if err := client.Get(ctx, "/v1/info/ip", params, &result); err != nil {
			return "", fmt.Errorf("%w: 调用失败: %s", protocol.ErrToolFailure, err.Error())
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "IP: %s\n", result.IP)
		if result.ASN != "" {
			fmt.Fprintf(&sb, "ASN: %s\n", result.ASN)
		}
		location := result.City
		if result.Country != "" {
			if location != "" {
				location = location + ", " + result.Country
			} else {
				location = result.Country
			}
		}
		if location != "" {
			fmt.Fprintf(&sb, "位置: %s\n", location)
		}
		if result.ISP != "" {
			fmt.Fprintf(&sb, "ISP: %s", result.ISP)
		}
		return strings.TrimRight(sb.String(), "\n"), nil
	}
}
