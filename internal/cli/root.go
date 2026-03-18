// Package cli provides the CLI commands for tw.
package cli

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/thgrace/training-wheels/internal/config"
	"github.com/thgrace/training-wheels/internal/logger"
	"github.com/thgrace/training-wheels/internal/packs"
	"github.com/thgrace/training-wheels/internal/packs/allpacks"
)

var (
	verbose bool
	quiet   bool
	robot   bool
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

		format := logger.FormatText
		if robot {
			format = logger.FormatJSON
		}

		logger.Configure(cmd.ErrOrStderr(), level, format)

		initializeDefaultRegistry()
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
	rootCmd.PersistentFlags().BoolVar(&robot, "robot", false, "JSON output and silent stderr (logs only)")

	rootCmd.AddCommand(hookCmd)
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(explainCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(packsCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(allowCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(completionsCmd)
	rootCmd.AddCommand(versionCmd)
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

type packDirLoader interface {
	LoadFromDir(path string) error
}

type packFileLoader interface {
	LoadFile(path string) error
}

func initializeDefaultRegistry() {
	cfg, err := config.Load()
	if err != nil {
		logger.Warn("external pack path resolution skipped", "error", err)
		return
	}

	initializeRegistry(packs.DefaultRegistry(), cfg)
}

func initializeRegistry(reg *packs.PackRegistry, cfg *config.Config) {
	allpacks.RegisterAll(reg)
	loadPackSourcesFromConfig(reg, cfg)
}

func loadPackSourcesFromConfig(reg *packs.PackRegistry, cfg *config.Config) {
	paths, err := cfg.ExternalPackPaths()
	if err != nil {
		logger.Warn("external pack path resolution skipped", "error", err)
		return
	}

	dirLoader, hasDirLoader := any(reg).(packDirLoader)
	fileLoader, hasFileLoader := any(reg).(packFileLoader)
	if !hasDirLoader && !hasFileLoader {
		return
	}

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			logger.Warn("external pack source skipped", "path", path, "error", err)
			continue
		}

		if info.IsDir() {
			if hasDirLoader {
				if err := dirLoader.LoadFromDir(path); err != nil {
					logger.Warn("external pack directory load completed with errors", "path", path, "error", err)
				}
			}
			continue
		}

		if hasFileLoader {
			if err := fileLoader.LoadFile(path); err != nil {
				logger.Warn("external pack file load failed", "path", path, "error", err)
			}
		}
	}
}

// evalContext returns a context with the configured hook timeout.
func evalContext(cfg *config.Config) (context.Context, context.CancelFunc) {
	timeout := time.Duration(cfg.General.HookTimeoutMs) * time.Millisecond
	return context.WithTimeout(context.Background(), timeout)
}
