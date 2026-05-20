package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const version = "0.1.0"

var (
	globalAPIKey string
	globalFormat string
	globalOutput string
)

func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".idcd", "config.yaml")
}

func loadConfig() string {
	path := configPath()
	if path == "" {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "api_key:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "api_key:"))
			return strings.Trim(val, `"'`)
		}
	}
	return ""
}

func resolveAPIKey() string {
	if globalAPIKey != "" {
		return globalAPIKey
	}
	if v := os.Getenv("IDCD_API_KEY"); v != "" {
		return v
	}
	return loadConfig()
}

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "idcd",
		Short: "idcd — 多节点拨测 CLI",
		Long:  "idcd CLI 让你从终端直接运行拨测命令，无需打开网页。",
	}

	root.PersistentFlags().StringVar(&globalAPIKey, "api-key", "", "API key（优先级高于配置文件）")
	root.PersistentFlags().StringVar(&globalFormat, "format", "table", "输出格式（table/json）")
	root.PersistentFlags().StringVar(&globalOutput, "output", "", "输出文件路径（默认 stdout）")

	root.Version = version
	root.SetVersionTemplate(fmt.Sprintf("idcd %s\n", version))

	root.AddCommand(
		newPingCmd(),
		newHTTPCmd(),
		newDNSCmd(),
		newDiagnoseCmd(),
		newMonitorCmd(),
		newConfigCmd(),
	)

	return root
}

func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
