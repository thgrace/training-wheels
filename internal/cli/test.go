package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thgrace/training-wheels/internal/config"
	"github.com/thgrace/training-wheels/internal/eval"
	"github.com/thgrace/training-wheels/internal/exitcodes"
	"github.com/thgrace/training-wheels/internal/logger"
	"github.com/thgrace/training-wheels/internal/override"
	"github.com/thgrace/training-wheels/internal/packs"
	"github.com/thgrace/training-wheels/internal/session"
	"github.com/thgrace/training-wheels/internal/shellcontext"
)

var testFormat string
var testExplain bool
var testShell string
var testExpect string
var testForce string

var testCmd = &cobra.Command{
	Use:   "test <command>",
	Short: "Test a command against TW rules",
	Args:  cobra.ExactArgs(1),
	RunE:  runTest,
}

func init() {
	testCmd.Flags().StringVar(&testFormat, "format", "pretty", "Output format: pretty or json")
	testCmd.Flags().BoolVar(&testExplain, "explain", false, "Show detailed explanation (same as tw explain)")
	testCmd.Flags().StringVar(&testShell, "shell", "", "Shell context: posix, powershell, cmd")
	testCmd.Flags().StringVar(&testExpect, "expect", "", "Expected decision: allow, deny, or ask. Exits 0 on match, 1 on mismatch")
	testCmd.Flags().StringVar(&testForce, "force", "", "Skip evaluation and force a decision: allow, deny, or ask")
}

// validDecisions are the accepted values for --expect and --force.
var validDecisions = map[string]eval.EvaluationDecision{
	"allow": eval.DecisionAllow,
	"deny":  eval.DecisionDeny,
	"ask":   eval.DecisionAsk,
}

func runTest(cmd *cobra.Command, args []string) error {
	if testExplain {
		return runExplain(cmd, args)
	}

	command := args[0]
	w := cmd.OutOrStdout()

	// Validate --expect value if provided.
	if testExpect != "" {
		if _, ok := validDecisions[strings.ToLower(testExpect)]; !ok {
			fmt.Fprintf(w, "invalid --expect value %q: must be allow, deny, or ask\n", testExpect)
			os.Exit(exitcodes.ConfigError)
		}
	}

	// Validate --force value if provided.
	if testForce != "" {
		if _, ok := validDecisions[strings.ToLower(testForce)]; !ok {
			fmt.Fprintf(w, "invalid --force value %q: must be allow, deny, or ask\n", testForce)
			os.Exit(exitcodes.ConfigError)
		}
	}

	// --force: skip evaluation, return the forced decision.
	if testForce != "" {
		forcedDecision := validDecisions[strings.ToLower(testForce)]
		result := &eval.EvaluationResult{
			Decision: forcedDecision,
			Command:  command,
		}
		printResult(w, result, command)
		exitForDecision(forcedDecision)
		return nil
	}

	// Normal evaluation path.
	cfg, err := config.Load()
	if err != nil {
		logger.Error("config error", "error", err)
		os.Exit(exitcodes.ConfigError)
	}

	evaluator := eval.NewEvaluator(cfg, packs.DefaultRegistry())

	if testShell != "" {
		evaluator.SetShell(shellcontext.FromName(testShell))
	}

	// Load overrides.
	user, project, ovErr := override.LoadMerged()
	if ovErr != nil {
		logger.Warn("override load error", "error", ovErr)
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

	ctx, cancel := evalContext(cfg)
	defer cancel()

	result := evaluator.Evaluate(ctx, command)

	// Apply Deny → Ask conversion when checking for ask (mirrors hook.go behavior).
	if testExpect == "ask" && result.Decision == eval.DecisionDeny {
		result.Decision = eval.DecisionAsk
	}

	// --expect: validate the result against expected decision.
	if testExpect != "" {
		expected := validDecisions[strings.ToLower(testExpect)]
		runExpectCheck(w, result, command, expected)
		return nil
	}

	// Default: print result and exit with decision code.
	printResult(w, result, command)
	exitForDecision(result.Decision)
	return nil
}

func runExpectCheck(w io.Writer, result *eval.EvaluationResult, command string, expected eval.EvaluationDecision) {
	fmt.Fprintf(w, "Command:  %s\n", command)
	fmt.Fprintf(w, "Expected: %s\n", expected)
	fmt.Fprintf(w, "Got:      %s\n", result.Decision)

	if result.PatternInfo != nil {
		fmt.Fprintf(w, "Rule:     %s\n", result.PatternInfo.RuleID)
		fmt.Fprintf(w, "Reason:   %s\n", result.PatternInfo.Reason)
	}

	if result.Decision == expected {
		fmt.Fprintf(w, "\n✓ PASS\n")
		os.Exit(exitcodes.Allow)
	}

	fmt.Fprintf(w, "\n✗ FAIL\n")
	os.Exit(1)
}

func printResult(w io.Writer, result *eval.EvaluationResult, command string) {
	switch testFormat {
	case "json":
		printTestJSON(w, result)
	default:
		printTestPretty(w, result, command)
	}
}

func exitForDecision(d eval.EvaluationDecision) {
	switch d {
	case eval.DecisionAllow, eval.DecisionAsk:
		os.Exit(exitcodes.Allow)
	default:
		os.Exit(exitcodes.Deny)
	}
}

type testJSONOutput struct {
	Decision      string               `json:"decision"`
	RuleID        string               `json:"rule_id,omitempty"`
	PackID        string               `json:"pack_id,omitempty"`
	PatternName   string               `json:"pattern_name,omitempty"`
	Severity      string               `json:"severity,omitempty"`
	Reason        string               `json:"reason,omitempty"`
	OverrideMatch *testJSONOverrideOut `json:"override_match,omitempty"`
	SessionMatch  *testJSONSessionOut  `json:"session_match,omitempty"`
}

type testJSONOverrideOut struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Kind   string `json:"kind"`
	Value  string `json:"value"`
	Reason string `json:"reason"`
}

type testJSONSessionOut struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Value  string `json:"value"`
	Reason string `json:"reason"`
}

func printTestJSON(w io.Writer, result *eval.EvaluationResult) {
	out := testJSONOutput{
		Decision: result.Decision.String(),
	}
	if result.PatternInfo != nil {
		out.RuleID = result.PatternInfo.RuleID
		out.PackID = result.PatternInfo.PackID
		out.PatternName = result.PatternInfo.PatternName
		out.Severity = result.PatternInfo.Severity
		out.Reason = result.PatternInfo.Reason
	}
	if result.OverrideEntry != nil {
		out.OverrideMatch = &testJSONOverrideOut{
			ID:     result.OverrideEntry.ID,
			Action: result.OverrideEntry.Action,
			Kind:   result.OverrideEntry.Kind,
			Value:  result.OverrideEntry.Value,
			Reason: result.OverrideEntry.Reason,
		}
	}
	if result.SessionEntry != nil {
		out.SessionMatch = &testJSONSessionOut{
			ID:     result.SessionEntry.ID,
			Kind:   result.SessionEntry.Kind,
			Value:  result.SessionEntry.Value,
			Reason: result.SessionEntry.Reason,
		}
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	fmt.Fprintln(w, string(data))
}

func printTestPretty(w io.Writer, result *eval.EvaluationResult, command string) {
	switch result.Decision {
	case eval.DecisionAllow:
		fmt.Fprintf(w, "✓ ALLOW: %s\n", command)
		if result.OverrideEntry != nil {
			fmt.Fprintf(w, "  Override: %s %s=%q (reason: %s)\n",
				result.OverrideEntry.Action, result.OverrideEntry.Kind, result.OverrideEntry.Value, result.OverrideEntry.Reason)
		}
		if result.SessionEntry != nil {
			fmt.Fprintf(w, "  Session: allow %s=%q (reason: %s)\n",
				result.SessionEntry.Kind, result.SessionEntry.Value, result.SessionEntry.Reason)
		}
	case eval.DecisionAsk:
		fmt.Fprintf(w, "? ASK: %s\n", command)
		if result.PatternInfo != nil {
			pi := result.PatternInfo
			fmt.Fprintf(w, "  Rule:     %s\n", pi.RuleID)
			fmt.Fprintf(w, "  Pack:     %s\n", pi.PackID)
			fmt.Fprintf(w, "  Severity: %s\n", pi.Severity)
			fmt.Fprintf(w, "  Reason:   %s\n", pi.Reason)
		}
	default:
		fmt.Fprintf(w, "✗ DENY: %s\n", command)
		if result.PatternInfo != nil {
			pi := result.PatternInfo
			fmt.Fprintf(w, "  Rule:     %s\n", pi.RuleID)
			fmt.Fprintf(w, "  Pack:     %s\n", pi.PackID)
			fmt.Fprintf(w, "  Severity: %s\n", pi.Severity)
			fmt.Fprintf(w, "  Reason:   %s\n", pi.Reason)
		}
	}
}
