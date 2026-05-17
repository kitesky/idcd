package tools

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/kite365/idcd/apps/mcp/internal/apiclient"
	"github.com/kite365/idcd/apps/mcp/internal/protocol"
)

func diagnoseDef() protocol.ToolDefinition {
	return protocol.ToolDefinition{
		Name:        "diagnose",
		Description: "Run a comprehensive network diagnostic combining ping, DNS, HTTP, and SSL checks",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target": map[string]any{
					"type":        "string",
					"description": "Hostname or URL to diagnose",
				},
			},
			"required": []string{"target"},
		},
	}
}

func handleDiagnoseFunc(client *apiclient.Client) protocol.ToolHandler {
	return func(ctx context.Context, args map[string]any) (string, error) {
		rawTarget, _ := args["target"].(string)
		target, err := validateTarget(rawTarget)
		if err != nil {
			return "", err
		}

		if !client.HasAPIKey() {
			return "⚠ 需要 API key，请设置 IDCD_API_KEY 环境变量", nil
		}

		host := target
		host = strings.TrimPrefix(host, "https://")
		host = strings.TrimPrefix(host, "http://")
		if idx := strings.Index(host, "/"); idx != -1 {
			host = host[:idx]
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "DIAGNOSE %s\n", target)

		var dnsResult struct {
			Records []struct {
				Value string `json:"value"`
			} `json:"records"`
		}
		dnsParams := url.Values{}
		dnsParams.Set("q", host)
		dnsParams.Set("type", "A")
		if err := client.Get(ctx, "/v1/info/dns", dnsParams, &dnsResult); err != nil {
			fmt.Fprintf(&sb, "[✗] DNS: %s\n", err.Error())
		} else {
			fmt.Fprintf(&sb, "[✓] DNS: %d A records\n", len(dnsResult.Records))
		}

		httpURL := target
		if !strings.HasPrefix(httpURL, "http://") && !strings.HasPrefix(httpURL, "https://") {
			httpURL = "https://" + httpURL
		}
		var httpResult struct {
			StatusCode int `json:"status_code"`
			LatencyMs  int `json:"latency_ms"`
		}
		httpBody := map[string]any{"url": httpURL}
		if err := client.Post(ctx, "/v1/probe/http", httpBody, &httpResult); err != nil {
			fmt.Fprintf(&sb, "[✗] HTTP: %s\n", err.Error())
		} else {
			fmt.Fprintf(&sb, "[✓] HTTP: %d OK (%dms)\n", httpResult.StatusCode, httpResult.LatencyMs)
		}

		var sslResult struct {
			NotAfter        string `json:"not_after"`
			DaysUntilExpiry int    `json:"days_until_expiry"`
		}
		sslParams := url.Values{}
		sslParams.Set("q", host)
		if err := client.Get(ctx, "/v1/info/ssl", sslParams, &sslResult); err != nil {
			fmt.Fprintf(&sb, "[✗] SSL: %s\n", err.Error())
		} else {
			expiry := sslResult.NotAfter
			if len(expiry) >= 10 {
				expiry = expiry[:10]
			}
			fmt.Fprintf(&sb, "[✓] SSL: valid, expires %s\n", expiry)
		}

		var pingResult struct {
			AvgMs   float64 `json:"avg_ms"`
			LossPct float64 `json:"loss_pct"`
		}
		pingBody := map[string]any{"target": host, "count": 3}
		if err := client.Post(ctx, "/v1/probe/ping", pingBody, &pingResult); err != nil {
			fmt.Fprintf(&sb, "[✗] Ping: %s", err.Error())
		} else {
			fmt.Fprintf(&sb, "[✓] Ping: avg %.0fms, %.0f%% loss", pingResult.AvgMs, pingResult.LossPct)
		}

		return sb.String(), nil
	}
}
