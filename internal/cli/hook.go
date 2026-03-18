package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thgrace/training-wheels/internal/config"
	"github.com/thgrace/training-wheels/internal/eval"
	"github.com/thgrace/training-wheels/internal/exitcodes"
	"github.com/thgrace/training-wheels/internal/hook"
	"github.com/thgrace/training-wheels/internal/logger"
	"github.com/thgrace/training-wheels/internal/override"
	"github.com/thgrace/training-wheels/internal/packs"
	"github.com/thgrace/training-wheels/internal/session"
	"github.com/thgrace/training-wheels/internal/shellcontext"
)

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Run as a pre-execution hook (reads JSON from stdin)",
	RunE:  runHook,
}

func runHook(cmd *cobra.Command, args []string) error {
	// Load config — fail-open on error.
	cfg, err := config.Load()
	if err != nil {
		logger.Error("config error, allowing command", "error", err)
		os.Exit(exitcodes.Allow)
	}

	// Read hook input.
	input, err := hook.ReadHookInput(cmd.InOrStdin(), cfg.General.MaxCommandBytes)
	if err != nil {
		logger.Error("parse error, allowing command", "error", err)
		os.Exit(exitcodes.Allow)
	}

	// Extract command.
	command, protocol, err := hook.ExtractCommand(input)
	if err != nil {
		logger.Error("extract error, allowing command", "error", err)
		os.Exit(exitcodes.Allow)
	}
	if command == "" {
		os.Exit(exitcodes.Allow)
	}

	// Build evaluator.
	evaluator := eval.NewEvaluator(cfg, packs.DefaultRegistry())

	// Detect shell context from hook input or environment.
	var shell shellcontext.Shell
	if input.ToolName != nil {
		shell = shellcontext.FromName(*input.ToolName)
	} else if detected := shellcontext.DetectShellFromCommand(command); detected != nil {
		shell = detected
	} else {
		shell = shellcontext.DefaultShell()
	}
	evaluator.SetShell(shell)

	// Load overrides (fail-open on error).
	user, project, ovErr := override.LoadMerged()
	if ovErr != nil {
		logger.Warn("override load error, continuing without overrides", "error", ovErr)
	} else {
		evaluator.SetOverrides(user, project)
	}

	// Load session allowlist (fail-open).
	if token, _ := session.ReadToken(); token != "" {
		if secret, err := session.LoadOrCreateSecret(session.SecretPath()); err == nil {
			if sa, err := session.Load(token, secret); err == nil {
				evaluator.SetSessionAllows(sa)
			}
		}
	}

	// Set up timeout.
	ctx, cancel := evalContext(cfg)
	defer cancel()

	// Evaluate.
	result := evaluator.Evaluate(ctx, command)

	if result.SkippedDueToBudget {
		logger.Warn("evaluation timed out, allowing command (fail-open)",
			"timeout_ms", cfg.General.HookTimeoutMs)
		os.Exit(exitcodes.Allow)
	}

	if result.OverrideEntry != nil && result.Decision == eval.DecisionAllow {
		// Command was allowed by override — log it for visibility.
		logger.Info("allowed by override",
			"id", result.OverrideEntry.ID,
			"action", result.OverrideEntry.Action,
			"kind", result.OverrideEntry.Kind,
			"value", result.OverrideEntry.Value)
		os.Exit(exitcodes.Allow)
	}

	if result.SessionEntry != nil && result.Decision == eval.DecisionAllow {
		logger.Info("allowed by session entry",
			"id", result.SessionEntry.ID,
			"kind", result.SessionEntry.Kind,
			"value", result.SessionEntry.Value)
		os.Exit(exitcodes.Allow)
	}

	// Apply DefaultAction if it was a denial.
	if result.Decision == eval.DecisionDeny {
		if strings.ToLower(cfg.Packs.DefaultAction) == "ask" {
			result.Decision = eval.DecisionAsk
		}
	}

	if result.Decision == eval.DecisionDeny && result.PatternInfo != nil {
		pi := result.PatternInfo
		// Write denial JSON to stdout.
		if err := hook.OutputDenial(cmd.OutOrStdout(), protocol,
			pi.Reason, pi.RuleID, pi.PackID, pi.Severity); err != nil {
			logger.Error("output error", "error", err)
			os.Exit(exitcodes.IOError)
		}
		// Write warning to stderr.
		logger.Warn("command denied",
			"reason", pi.Reason,
			"rule_id", pi.RuleID,
			"pack_id", pi.PackID,
			"severity", pi.Severity)
		os.Exit(exitcodes.Deny)
	}

	if result.Decision == eval.DecisionAsk && result.PatternInfo != nil {
		pi := result.PatternInfo
		// Write "ask" JSON to stdout.
		if err := hook.OutputAsk(cmd.OutOrStdout(), protocol,
			pi.Reason, pi.RuleID, pi.PackID, pi.Severity); err != nil {
			logger.Error("output error", "error", err)
			os.Exit(exitcodes.IOError)
		}
		// Write warning to stderr (labeled as verify).
		logger.Warn("command verification required",
			"reason", pi.Reason,
			"rule_id", pi.RuleID,
			"pack_id", pi.PackID,
			"severity", pi.Severity)
		os.Exit(exitcodes.Deny)
	}

	os.Exit(exitcodes.Allow)
	return nil
}
