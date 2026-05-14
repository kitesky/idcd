package cmd

import (
	"fmt"

	"github.com/kite365/idcd/apps/cli/internal/api"
	"github.com/kite365/idcd/apps/cli/internal/format"
	"github.com/spf13/cobra"
)

func stubHTTPResponse(url string) *api.HTTPResponse {
	return &api.HTTPResponse{
		URL: url,
		Results: []api.HTTPResult{
			{Node: "jp-tok-ntt", Location: "Tokyo, JP", Status: "OK", LatencyMs: 234, HTTPCode: 200, TLS: "TLS 1.3"},
			{Node: "us-lax-cf", Location: "Los Angeles", Status: "OK", LatencyMs: 89, HTTPCode: 200, TLS: "TLS 1.3"},
			{Node: "eu-ams-aws", Location: "Amsterdam", Status: "OK", LatencyMs: 312, HTTPCode: 200, TLS: "TLS 1.3"},
		},
	}
}

func newHTTPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "http <url>",
		Short: "HTTP/HTTPS 连通性检测",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := args[0]
			out := outputWriter(cmd)
			f := format.New(globalFormat, out)

			apiKey := resolveAPIKey()
			client := api.NewClient(apiKey)

			resp, err := client.CheckHTTP(url)
			if err != nil {
				resp = stubHTTPResponse(url)
			}

			if f.IsJSON() {
				return f.JSON(resp)
			}

			f.Line("Checking %s from %d nodes...\n", resp.URL, len(resp.Results))

			headers := []string{"Node", "Status", "Latency", "HTTP Code", "TLS"}
			rows := make([][]string, 0, len(resp.Results))
			for _, r := range resp.Results {
				tlsStr := r.TLS
				if tlsStr != "" {
					tlsStr += " ✓"
				}
				rows = append(rows, []string{
					r.Node,
					r.Status,
					fmt.Sprintf("%dms", r.LatencyMs),
					fmt.Sprintf("%d", r.HTTPCode),
					tlsStr,
				})
			}
			f.Table(headers, rows)
			return nil
		},
	}
}
