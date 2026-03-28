package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thgrace/training-wheels/internal/app"
	"github.com/thgrace/training-wheels/internal/eval"
	"github.com/thgrace/training-wheels/internal/exitcodes"
	"github.com/thgrace/training-wheels/internal/hook"
	"github.com/thgrace/training-wheels/internal/logger"
	"github.com/thgrace/training-wheels/internal/shellcontext"
)

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Run as a pre-execution hook (reads JSON from stdin)",
	RunE:  runHook,
}

func runHook(cmd *cobra.Command, args []string) error {
	// Load config — fail-open on error.
	cfg, err := app.LoadConfig()
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
	// Detect shell context from hook input or environment.
	var shell shellcontext.Shell
	if input.ToolName != nil {
		shell = shellcontext.FromName(*input.ToolName)
	} else if detected := shellcontext.DetectShellFromCommand(command); detected != nil {
		shell = detected
	} else {
		shell = shellcontext.DefaultShell()
	}
	evaluator := app.NewEvaluator(cfg, app.EvalOptions{
		Shell:         shell,
		LoadOverrides: true,
		LoadRules:     true,
		LoadSession:   true,
	})

	// Set up timeout.
	ctx, cancel := app.EvalContext(cfg)
	defer cancel()

	// Evaluate.
	result := evaluator.Evaluate(ctx, command)

	if result.SkippedDueToBudget {
		logger.Warn("evaluation timed out, allowing command (fail-open)",
			"timeout_ms", cfg.General.HookTimeoutMs)
		os.Exit(exitcodes.Allow)
	}

	if result.OverrideEntry != nil {
		switch result.Decision {
		case eval.DecisionAllow:
			logger.Info("allowed by override",
				"id", result.OverrideEntry.ID,
				"action", result.OverrideEntry.Action,
				"kind", result.OverrideEntry.Kind,
				"value", result.OverrideEntry.Value)
			os.Exit(exitcodes.Allow)
		case eval.DecisionDeny:
			logger.Info("denied by override",
				"id", result.OverrideEntry.ID,
				"action", result.OverrideEntry.Action,
				"kind", result.OverrideEntry.Kind,
				"value", result.OverrideEntry.Value)
		case eval.DecisionAsk:
			logger.Info("verification required by override",
				"id", result.OverrideEntry.ID,
				"action", result.OverrideEntry.Action,
				"kind", result.OverrideEntry.Kind,
				"value", result.OverrideEntry.Value)
		}
	}

	if result.RuleEntry != nil && result.Decision == eval.DecisionAllow {
		logger.Info("allowed by custom rule",
			"name", result.RuleEntry.Name,
			"kind", result.RuleEntry.Kind,
			"pattern", result.RuleEntry.Pattern)
		os.Exit(exitcodes.Allow)
	}

	if result.SessionEntry != nil && result.Decision == eval.DecisionAllow {
		logger.Info("allowed by session entry",
			"id", result.SessionEntry.ID,
			"kind", result.SessionEntry.Kind,
			"value", result.SessionEntry.Value)
		os.Exit(exitcodes.Allow)
	}

	// Apply DefaultAction only to pack denials.
	if result.Decision == eval.DecisionDeny && result.PatternInfo != nil && result.PatternInfo.Source == eval.SourcePack {
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
		// Gemini reads the JSON decision field to block; exit 0 avoids a spurious
		// "hook failed" warning. Claude/Copilot use exit 1 to signal non-allow.
		if protocol == hook.ProtocolGemini {
			os.Exit(exitcodes.Allow)
		}
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
		// Gemini reads the JSON decision field; exit 0 avoids a spurious "hook failed" warning.
		// Claude/Copilot use exit 1 so the agent knows to inspect the JSON payload.
		if protocol == hook.ProtocolGemini {
			os.Exit(exitcodes.Allow)
		}
		os.Exit(exitcodes.Deny)
	}

	return silentExit(exitcodes.Allow)
}
