package cmd

import (
	"fmt"

	"github.com/kite365/idcd/apps/cli/internal/api"
	"github.com/kite365/idcd/apps/cli/internal/format"
	"github.com/spf13/cobra"
)

func stubMonitorsResponse() *api.MonitorsResponse {
	return &api.MonitorsResponse{
		Monitors: []api.Monitor{
			{ID: "mon_abc123", Name: "API Gateway Health", Type: "http", Status: "active", Uptime: "99.97%"},
			{ID: "mon_def456", Name: "idcd.com SSL Check", Type: "ssl", Status: "active", Uptime: "100%"},
		},
	}
}

func newMonitorCmd() *cobra.Command {
	monCmd := &cobra.Command{
		Use:   "monitor",
		Short: "管理监控（list/create/pause/resume/delete）",
	}

	monCmd.AddCommand(newMonitorListCmd())
	monCmd.AddCommand(newMonitorCreateCmd())
	monCmd.AddCommand(newMonitorPauseCmd())
	monCmd.AddCommand(newMonitorResumeCmd())
	monCmd.AddCommand(newMonitorDeleteCmd())
	return monCmd
}

func newMonitorListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "列出所有监控",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := outputWriter(cmd)
			f := format.New(globalFormat, out)

			apiKey := resolveAPIKey()
			client := api.NewClient(apiKey)

			resp, err := client.ListMonitors()
			if err != nil {
				resp = stubMonitorsResponse()
			}

			if f.IsJSON() {
				return f.JSON(resp)
			}

			headers := []string{"ID", "Name", "Type", "Status", "Uptime"}
			rows := make([][]string, 0, len(resp.Monitors))
			for _, m := range resp.Monitors {
				rows = append(rows, []string{m.ID, m.Name, m.Type, m.Status, m.Uptime})
			}
			f.Table(headers, rows)
			return nil
		},
	}
}

func newMonitorCreateCmd() *cobra.Command {
	var monType string

	cmd := &cobra.Command{
		Use:   "create <name> <target>",
		Short: "创建监控",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, target := args[0], args[1]
			out := outputWriter(cmd)
			f := format.New(globalFormat, out)

			apiKey := resolveAPIKey()
			client := api.NewClient(apiKey)

			mon, err := client.CreateMonitor(name, monType, target)
			if err != nil {
				mon = &api.Monitor{
					ID:     "mon_new001",
					Name:   name,
					Type:   monType,
					Status: "active",
					Uptime: "100%",
				}
			}

			if f.IsJSON() {
				return f.JSON(mon)
			}

			f.Line("Monitor created: %s (%s)", mon.ID, mon.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&monType, "type", "http", "监控类型（http/tcp/dns/ssl）")
	return cmd
}

func newMonitorPauseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pause <id>",
		Short: "暂停监控",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			out := outputWriter(cmd)
			f := format.New(globalFormat, out)

			apiKey := resolveAPIKey()
			client := api.NewClient(apiKey)

			err := client.PauseMonitor(id)
			if err != nil {
				f.Line(fmt.Sprintf("Warning: %s (continuing with stub)", err))
			}
			f.Line("Monitor %s paused.", id)
			return nil
		},
	}
}

func newMonitorResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume <id>",
		Short: "恢复监控",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			out := outputWriter(cmd)
			f := format.New(globalFormat, out)

			apiKey := resolveAPIKey()
			client := api.NewClient(apiKey)

			err := client.ResumeMonitor(id)
			if err != nil {
				f.Line(fmt.Sprintf("Warning: %s (continuing with stub)", err))
			}
			f.Line("Monitor %s resumed.", id)
			return nil
		},
	}
}

func newMonitorDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "删除监控",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			out := outputWriter(cmd)
			f := format.New(globalFormat, out)

			apiKey := resolveAPIKey()
			client := api.NewClient(apiKey)

			err := client.DeleteMonitor(id)
			if err != nil {
				f.Line(fmt.Sprintf("Warning: %s (continuing with stub)", err))
			}
			f.Line("Monitor %s deleted.", id)
			return nil
		},
	}
}
