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
			return "", fmt.Errorf("%w: %s", protocol.ErrToolFailure, err.Error())
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
			StatusCode int   `json:"status_code"`
			TotalMs    int64 `json:"total_ms"`
			DurationMs int64 `json:"duration_ms"`
			Success    bool  `json:"success"`
		}
		// probe/http 用 target,而且是 async — 走 polling
		httpBody := map[string]any{"target": httpURL}
		if err := client.PollProbeTask(ctx, "/v1/probe/http", httpBody, &httpResult); err != nil {
			fmt.Fprintf(&sb, "[✗] HTTP: %s\n", err.Error())
		} else {
			ms := httpResult.TotalMs
			if ms == 0 {
				ms = httpResult.DurationMs
			}
			mark := "✓"
			if !httpResult.Success {
				mark = "✗"
			}
			fmt.Fprintf(&sb, "[%s] HTTP: %d (%dms)\n", mark, httpResult.StatusCode, ms)
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
			if expiry == "" {
				fmt.Fprintf(&sb, "[?] SSL: no expiry data\n")
			} else {
				fmt.Fprintf(&sb, "[✓] SSL: valid, expires %s\n", expiry)
			}
		}

		var pingResult struct {
			AvgMs      float64 `json:"avg_ms"`
			DurationMs float64 `json:"duration_ms"`
			PacketLoss float64 `json:"packet_loss"`
			Success    bool    `json:"success"`
		}
		// count 必须放 params 子对象，与 ping.go 契约保持一致；
		// 顶层 count 会被 api 忽略，agent 拿到的 options 为空，回退默认 5 包。
		pingBody := map[string]any{"target": host, "params": map[string]any{"count": 3}}
		if err := client.PollProbeTask(ctx, "/v1/probe/ping", pingBody, &pingResult); err != nil {
			fmt.Fprintf(&sb, "[✗] Ping: %s", err.Error())
		} else {
			avg := pingResult.AvgMs
			if avg == 0 && pingResult.DurationMs > 0 {
				avg = pingResult.DurationMs
			}
			mark := "✓"
			if !pingResult.Success || pingResult.PacketLoss == 100 {
				mark = "✗"
			}
			fmt.Fprintf(&sb, "[%s] Ping: avg %.1fms, %.0f%% loss", mark, avg, pingResult.PacketLoss)
		}

		return sb.String(), nil
	}
}
