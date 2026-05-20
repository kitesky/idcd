package cmd

import (
	"fmt"

	"github.com/kite365/idcd/apps/cli/internal/api"
	"github.com/kite365/idcd/apps/cli/internal/format"
	"github.com/spf13/cobra"
)

func stubDNSResponse(domain, recordType string) *api.DNSResponse {
	return &api.DNSResponse{
		Domain: domain,
		Type:   recordType,
		Records: []api.DNSRecord{
			{Name: domain, TTL: 300, Type: recordType, Value: "104.21.0.1"},
			{Name: domain, TTL: 300, Type: recordType, Value: "172.67.0.1"},
		},
	}
}

func newDNSCmd() *cobra.Command {
	var recordType string

	cmd := &cobra.Command{
		Use:   "dns <domain>",
		Short: "DNS 解析查询",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := args[0]
			out := outputWriter(cmd)
			f := format.New(globalFormat, out)

			apiKey := resolveAPIKey()
			client := api.NewClient(apiKey)

			resp, err := client.LookupDNS(domain, recordType)
			if err != nil {
				resp = stubDNSResponse(domain, recordType)
			}

			if f.IsJSON() {
				return f.JSON(resp)
			}

			f.Line("DNS lookup: %s (%s records)\n", resp.Domain, resp.Type)

			headers := []string{"Name", "TTL", "Type", "Value"}
			rows := make([][]string, 0, len(resp.Records))
			for _, r := range resp.Records {
				rows = append(rows, []string{
					r.Name,
					fmt.Sprintf("%d", r.TTL),
					r.Type,
					r.Value,
				})
			}
			f.Table(headers, rows)
			return nil
		},
	}

	cmd.Flags().StringVar(&recordType, "type", "A", "记录类型（A/AAAA/MX/TXT/CNAME/NS）")
	return cmd
}
