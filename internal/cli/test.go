package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thgrace/training-wheels/internal/app"
	"github.com/thgrace/training-wheels/internal/eval"
	"github.com/thgrace/training-wheels/internal/exitcodes"
)

var (
	testJSON   bool
	testShell  string
	testExpect string
	testForce  string
)

var testCmd = &cobra.Command{
	Use:   "test <command>",
	Short: "Test a command against TW rules",
	Args:  cobra.ExactArgs(1),
	RunE:  runTest,
}

func init() {
	bindJSONOutputFlags(testCmd.Flags(), &testJSON)
	testCmd.Flags().StringVar(&testShell, "shell", "", shellFlagUsage)
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
	command := args[0]
	w := cmd.OutOrStdout()

	// Validate --expect value if provided.
	if testExpect != "" {
		if _, ok := parseDecisionFlag(testExpect); !ok {
			fmt.Fprintf(w, "invalid --expect value %q: must be allow, deny, or ask\n", testExpect)
			return silentExit(exitcodes.ConfigError)
		}
	}

	// Validate --force value if provided.
	if testForce != "" {
		if _, ok := parseDecisionFlag(testForce); !ok {
			fmt.Fprintf(w, "invalid --force value %q: must be allow, deny, or ask\n", testForce)
			return silentExit(exitcodes.ConfigError)
		}
	}

	// --force: skip evaluation, return the forced decision.
	if testForce != "" {
		forcedDecision, _ := parseDecisionFlag(testForce)
		result := &eval.EvaluationResult{
			Decision: forcedDecision,
			Command:  command,
		}
		printResult(w, result, command, useJSONOutput(testJSON))
		return exitForDecision(forcedDecision)
	}

	// Normal evaluation path.
	cfg, err := app.LoadConfig()
	if err != nil {
		return exitErrorf(exitcodes.ConfigError, "config error: %w", err)
	}

	shell, err := resolveShellFlag(testShell)
	if err != nil {
		fmt.Fprintln(w, err)
		return silentExit(exitcodes.ConfigError)
	}

	evaluator := app.NewEvaluator(cfg, app.EvalOptions{
		Shell:         shell,
		LoadOverrides: true,
		LoadRules:     true,
		LoadSession:   true,
	})

	ctx, cancel := app.EvalContext(cfg)
	defer cancel()

	result := evaluator.Evaluate(ctx, command)

	expectedDecision, _ := parseDecisionFlag(testExpect)

	// Apply Deny → Ask conversion when checking for ask (mirrors hook.go behavior).
	if shouldApplyDenyToAskCompatibility(expectedDecision, result) {
		result.Decision = eval.DecisionAsk
	}

	// --expect: validate the result against expected decision.
	if testExpect != "" {
		return runExpectCheck(w, result, command, expectedDecision)
	}

	// Default: print result and exit with decision code.
	printResult(w, result, command, useJSONOutput(testJSON))
	return exitForDecision(result.Decision)
}

func parseDecisionFlag(value string) (eval.EvaluationDecision, bool) {
	decision, ok := validDecisions[strings.ToLower(value)]
	return decision, ok
}

func shouldApplyDenyToAskCompatibility(expected eval.EvaluationDecision, result *eval.EvaluationResult) bool {
	return expected == eval.DecisionAsk &&
		result.Decision == eval.DecisionDeny &&
		result.PatternInfo != nil &&
		result.PatternInfo.Source == eval.SourcePack
}

func runExpectCheck(w io.Writer, result *eval.EvaluationResult, command string, expected eval.EvaluationDecision) error {
	fmt.Fprintf(w, "Command:  %s\n", command)
	fmt.Fprintf(w, "Expected: %s\n", expected)
	fmt.Fprintf(w, "Got:      %s\n", result.Decision)

	if result.PatternInfo != nil {
		fmt.Fprintf(w, "Rule:     %s\n", result.PatternInfo.RuleID)
		fmt.Fprintf(w, "Reason:   %s\n", result.PatternInfo.Reason)
	}

	if result.Decision == expected {
		fmt.Fprintf(w, "\n✓ PASS\n")
		return nil
	}

	fmt.Fprintf(w, "\n✗ FAIL\n")
	return silentExit(1)
}

func printResult(w io.Writer, result *eval.EvaluationResult, command string, jsonOutput bool) {
	switch {
	case jsonOutput:
		printTestJSON(w, result)
	default:
		printTestPretty(w, result, command)
	}
}

func exitForDecision(d eval.EvaluationDecision) error {
	switch d {
	case eval.DecisionAllow:
		return nil
	case eval.DecisionAsk:
		// Ask exits non-zero so test scripts can detect it.
		// Agents distinguish ask from deny via the JSON payload.
		return silentExit(exitcodes.Deny)
	default:
		return silentExit(exitcodes.Deny)
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
	_ = writeJSONOutput(w, out)
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
		if result.OverrideEntry != nil {
			fmt.Fprintf(w, "  Override: %s %s=%q (reason: %s)\n",
				result.OverrideEntry.Action, result.OverrideEntry.Kind, result.OverrideEntry.Value, result.OverrideEntry.Reason)
		}
		if result.PatternInfo != nil {
			pi := result.PatternInfo
			fmt.Fprintf(w, "  Rule:     %s\n", pi.RuleID)
			fmt.Fprintf(w, "  Pack:     %s\n", pi.PackID)
			fmt.Fprintf(w, "  Severity: %s\n", pi.Severity)
			fmt.Fprintf(w, "  Reason:   %s\n", pi.Reason)
		}
	default:
		fmt.Fprintf(w, "✗ DENY: %s\n", command)
		if result.OverrideEntry != nil {
			fmt.Fprintf(w, "  Override: %s %s=%q (reason: %s)\n",
				result.OverrideEntry.Action, result.OverrideEntry.Kind, result.OverrideEntry.Value, result.OverrideEntry.Reason)
		}
		if result.PatternInfo != nil {
			pi := result.PatternInfo
			fmt.Fprintf(w, "  Rule:     %s\n", pi.RuleID)
			fmt.Fprintf(w, "  Pack:     %s\n", pi.PackID)
			fmt.Fprintf(w, "  Severity: %s\n", pi.Severity)
			fmt.Fprintf(w, "  Reason:   %s\n", pi.Reason)
		}
	}
}
