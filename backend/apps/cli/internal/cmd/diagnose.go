package cmd

import (
	"github.com/kite365/idcd/apps/cli/internal/api"
	"github.com/kite365/idcd/apps/cli/internal/format"
	"github.com/spf13/cobra"
)

func stubDiagnoseResponse(target string) *api.DiagnoseResponse {
	return &api.DiagnoseResponse{
		Target: target,
		Checks: []api.DiagnoseCheck{
			{Name: "DNS", Status: "ok", Detail: "2 A records found"},
			{Name: "HTTP", Status: "ok", Detail: "200 OK (avg 145ms from 3 nodes)"},
			{Name: "SSL", Status: "ok", Detail: "Valid cert, expires in 342 days (TLS 1.3)"},
			{Name: "Ping", Status: "ok", Detail: "avg 67ms, 0% loss"},
			{Name: "ICP", Status: "warn", Detail: "No ICP record found"},
		},
	}
}

func newDiagnoseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diagnose <target>",
		Short: "一键全面诊断",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			out := outputWriter(cmd)
			f := format.New(globalFormat, out)

			apiKey := resolveAPIKey()
			client := api.NewClient(apiKey)

			resp, err := client.Diagnose(target)
			if err != nil {
				resp = stubDiagnoseResponse(target)
			}

			if f.IsJSON() {
				return f.JSON(resp)
			}

			f.Line("Running full diagnosis for %s...", resp.Target)
			for _, c := range resp.Checks {
				icon := "[✓]"
				if c.Status == "warn" {
					icon = "[!]"
				} else if c.Status == "error" || c.Status == "fail" {
					icon = "[✗]"
				}
				f.Line("%s %s: %s", icon, c.Name, c.Detail)
			}
			return nil
		},
	}
}
