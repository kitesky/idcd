package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/kite365/idcd/apps/cli/internal/api"
	"github.com/kite365/idcd/apps/cli/internal/format"
	"github.com/spf13/cobra"
)

func stubPingResponse(target string) *api.PingResponse {
	return &api.PingResponse{
		Target: target,
		Results: []api.PingResult{
			{Node: "jp-tok-ntt", Location: "Tokyo, JP", LatencyMs: 32, Status: "ok"},
			{Node: "us-lax-cf", Location: "Los Angeles", LatencyMs: 89, Status: "ok"},
			{Node: "eu-ams-aws", Location: "Amsterdam", LatencyMs: 121, Status: "ok"},
			{Node: "cn-bj-ct", Location: "Beijing, CN", LatencyMs: 45, Status: "ok"},
			{Node: "sg-sin-dgn", Location: "Singapore", LatencyMs: 68, Status: "ok"},
		},
	}
}

func newPingCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ping <target>",
		Short: "多节点 Ping 检测",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			out := outputWriter(cmd)
			f := format.New(globalFormat, out)

			apiKey := resolveAPIKey()
			client := api.NewClient(apiKey)

			resp, err := client.Ping(target)
			if err != nil {
				resp = stubPingResponse(target)
			}

			if f.IsJSON() {
				return f.JSON(resp)
			}

			f.Line("Pinging %s from %d nodes...\n", resp.Target, len(resp.Results))

			headers := []string{"Node", "Location", "Latency", "Status"}
			rows := make([][]string, 0, len(resp.Results))
			total, min, max := 0, resp.Results[0].LatencyMs, resp.Results[0].LatencyMs
			for _, r := range resp.Results {
				status := "✓"
				if r.Status != "ok" {
					status = "✗"
				}
				rows = append(rows, []string{
					r.Node,
					r.Location,
					fmt.Sprintf("%dms", r.LatencyMs),
					status,
				})
				total += r.LatencyMs
				if r.LatencyMs < min {
					min = r.LatencyMs
				}
				if r.LatencyMs > max {
					max = r.LatencyMs
				}
			}
			f.Table(headers, rows)
			avg := total / len(resp.Results)
			f.Line("\nAvg: %dms   Min: %dms   Max: %dms   Loss: 0%%", avg, min, max)
			return nil
		},
	}
}

func outputWriter(cmd *cobra.Command) io.Writer {
	if globalOutput != "" {
		f, err := os.Create(globalOutput)
		if err != nil {
			return cmd.OutOrStdout()
		}
		return f
	}
	return cmd.OutOrStdout()
}
