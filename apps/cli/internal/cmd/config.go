package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cfgCmd := &cobra.Command{
		Use:   "config",
		Short: "配置 idcd CLI",
	}

	cfgCmd.AddCommand(newConfigSetKeyCmd())
	cfgCmd.AddCommand(newConfigGetCmd())
	return cfgCmd
}

func newConfigSetKeyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-key <api-key>",
		Short: "保存 API key 到 ~/.idcd/config.yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiKey := args[0]
			return writeAPIKey(apiKey, cmd.OutOrStdout())
		},
	}
}

func writeAPIKey(apiKey string, out interface{ Write([]byte) (int, error) }) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".idcd")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("cannot create config directory: %w", err)
	}
	content := fmt.Sprintf("api_key: %s\n", apiKey)
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("cannot write config: %w", err)
	}
	fmt.Fprintf(out, "API key saved to %s\n", path)
	return nil
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "查看当前配置",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			key := resolveAPIKey()
			if key == "" {
				fmt.Fprintln(out, "api_key: (not set)")
			} else {
				masked := key
				if len(key) > 8 {
					masked = key[:4] + "****" + key[len(key)-4:]
				}
				fmt.Fprintf(out, "api_key: %s\n", masked)
			}
			return nil
		},
	}
}
