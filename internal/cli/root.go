// Package cli provides the CLI commands for tw.
package cli

import (
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/thgrace/training-wheels/internal/app"
	"github.com/thgrace/training-wheels/internal/logger"
)

var (
	verbose bool
	quiet   bool
)

var rootCmd = &cobra.Command{
	Use:   "tw",
	Short: "Training Wheels — pre-execution safety for AI coding agents",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Configure logging based on global flags.
		level := slog.LevelInfo
		if verbose {
			level = slog.LevelDebug
		} else if quiet {
			level = slog.LevelWarn
		}
		logger.Configure(cmd.ErrOrStderr(), level, logger.FormatText)

		app.EnsureDefaultRegistry()
		return nil
	},
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress informational output")

	rootCmd.AddCommand(hookCmd)
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(explainCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(packsCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(overrideCmd)
	rootCmd.AddCommand(ruleCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(completionsCmd)
	rootCmd.AddCommand(versionCmd)
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
