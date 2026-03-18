package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thgrace/training-wheels/internal/config"
	"github.com/thgrace/training-wheels/internal/exitcodes"
	"github.com/thgrace/training-wheels/internal/logger"
)

var configFormat string

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show resolved configuration",
	RunE:  runConfig,
}

func init() {
	configCmd.Flags().StringVar(&configFormat, "format", "pretty", "Output format: pretty or json")
}

func runConfig(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		logger.Error("config error", "error", err)
		os.Exit(exitcodes.ConfigError)
	}

	externalPaths, err := cfg.ExternalPackPaths()
	if err != nil {
		logger.Error("pack path resolution error", "error", err)
		os.Exit(exitcodes.ConfigError)
	}

	switch configFormat {
	case "json":
		out, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			logger.Error("json marshal error", "error", err)
			os.Exit(exitcodes.ConfigError)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(out))
	default:
		fmt.Fprintln(cmd.OutOrStdout(), "TW Configuration (resolved)")
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "[general]")
		fmt.Fprintf(cmd.OutOrStdout(), "  hook_timeout_ms = %d\n", cfg.General.HookTimeoutMs)
		fmt.Fprintf(cmd.OutOrStdout(), "  max_command_bytes = %d\n", cfg.General.MaxCommandBytes)
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "[packs]")
		fmt.Fprintf(cmd.OutOrStdout(), "  enabled = [%s]\n", formatStringSlice(cfg.Packs.Enabled))
		fmt.Fprintf(cmd.OutOrStdout(), "  disabled = [%s]\n", formatStringSlice(cfg.Packs.Disabled))
		fmt.Fprintf(cmd.OutOrStdout(), "  paths = [%s]\n", formatStringSlice(cfg.Packs.Paths))
		fmt.Fprintf(cmd.OutOrStdout(), "  resolved_paths = [%s]\n", formatStringSlice(externalPaths))
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "[update]")
		if cfg.Update.URL != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  url = %q\n", cfg.Update.URL)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "  url = (default)")
		}
	}

	return nil
}

// formatStringSlice renders a []string as a quoted, comma-separated list.
func formatStringSlice(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	quoted := make([]string, len(ss))
	for i, s := range ss {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(quoted, ", ")
}
