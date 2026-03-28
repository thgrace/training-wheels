package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/thgrace/training-wheels/internal/eval"
	"github.com/thgrace/training-wheels/internal/exitcodes"
	"github.com/thgrace/training-wheels/internal/override"
	"github.com/thgrace/training-wheels/internal/session"
)

func TestRunTestForcedDecisionJSON(t *testing.T) {
	t.Cleanup(resetTestCommandState)

	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)

	testJSON = true
	testForce = "allow"

	err := runTest(cmd, []string{"git status"})
	if err != nil {
		t.Fatalf("runTest() error = %v", err)
	}

	var out struct {
		Decision string `json:"decision"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("stdout is not valid test json: %v\nstdout=%q", err, stdout.String())
	}
	if out.Decision != "allow" {
		t.Fatalf("decision = %q, want allow", out.Decision)
	}
}

func TestRunTestForcedDecisionPretty(t *testing.T) {
	t.Cleanup(resetTestCommandState)

	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)

	testForce = "deny"

	err := runTest(cmd, []string{"git push --force"})
	if err == nil {
		t.Fatal("runTest() error = nil, want ExitError")
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("runTest() error = %T, want *ExitError", err)
	}
	if exitErr.Code != exitcodes.Deny || !exitErr.Silent {
		t.Fatalf("exit error = %+v, want silent deny", exitErr)
	}

	got := stdout.String()
	if !strings.Contains(got, "DENY: git push --force") {
		t.Fatalf("stdout = %q, want deny summary", got)
	}
}

func TestRunTestRejectsInvalidForceAndExpect(t *testing.T) {
	tests := []struct {
		name      string
		setup     func()
		wantError string
	}{
		{
			name: "invalid force",
			setup: func() {
				testForce = "block"
			},
			wantError: `invalid --force value "block": must be allow, deny, or ask`,
		},
		{
			name: "invalid expect",
			setup: func() {
				testExpect = "block"
			},
			wantError: `invalid --expect value "block": must be allow, deny, or ask`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(resetTestCommandState)

			var stdout bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&stdout)

			tt.setup()

			err := runTest(cmd, []string{"git status"})
			if err == nil {
				t.Fatal("runTest() error = nil, want ExitError")
			}
			var exitErr *ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("runTest() error = %T, want *ExitError", err)
			}
			if exitErr.Code != exitcodes.ConfigError || !exitErr.Silent {
				t.Fatalf("exit error = %+v, want silent config error", exitErr)
			}
			if !strings.Contains(stdout.String(), tt.wantError) {
				t.Fatalf("stdout = %q, want %q", stdout.String(), tt.wantError)
			}
		})
	}
}

func TestRunExpectCheck(t *testing.T) {
	tests := []struct {
		name      string
		result    *eval.EvaluationResult
		expected  eval.EvaluationDecision
		wantError bool
		wantText  string
	}{
		{
			name: "pass",
			result: &eval.EvaluationResult{
				Decision: eval.DecisionAllow,
				PatternInfo: &eval.PatternMatch{
					RuleID: "core.git:reset-hard",
					Reason: "dangerous command",
				},
			},
			expected: eval.DecisionAllow,
			wantText: "PASS",
		},
		{
			name: "fail",
			result: &eval.EvaluationResult{
				Decision: eval.DecisionAllow,
			},
			expected:  eval.DecisionDeny,
			wantError: true,
			wantText:  "FAIL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer

			err := runExpectCheck(&stdout, tt.result, "git status", tt.expected)
			if tt.wantError {
				if err == nil {
					t.Fatal("runExpectCheck() error = nil, want ExitError")
				}
				var exitErr *ExitError
				if !errors.As(err, &exitErr) {
					t.Fatalf("runExpectCheck() error = %T, want *ExitError", err)
				}
				if exitErr.Code != 1 || !exitErr.Silent {
					t.Fatalf("exit error = %+v, want silent exit 1", exitErr)
				}
			} else if err != nil {
				t.Fatalf("runExpectCheck() error = %v, want nil", err)
			}

			if !strings.Contains(stdout.String(), tt.wantText) {
				t.Fatalf("stdout = %q, want %q", stdout.String(), tt.wantText)
			}
		})
	}
}

func TestShouldApplyDenyToAskCompatibility(t *testing.T) {
	result := &eval.EvaluationResult{
		Decision: eval.DecisionDeny,
		PatternInfo: &eval.PatternMatch{
			Source: eval.SourcePack,
		},
	}

	if !shouldApplyDenyToAskCompatibility(eval.DecisionAsk, result) {
		t.Fatal("shouldApplyDenyToAskCompatibility() = false, want true for ask expectations")
	}

	if shouldApplyDenyToAskCompatibility(eval.DecisionAllow, result) {
		t.Fatal("shouldApplyDenyToAskCompatibility() = true, want false for non-ask expectations")
	}

	if shouldApplyDenyToAskCompatibility(eval.DecisionAsk, &eval.EvaluationResult{Decision: eval.DecisionAllow}) {
		t.Fatal("shouldApplyDenyToAskCompatibility() = true, want false for allow decisions")
	}
}

func TestParseDecisionFlag(t *testing.T) {
	decision, ok := parseDecisionFlag("ASK")
	if !ok {
		t.Fatal("parseDecisionFlag(\"ASK\") = !ok, want ok")
	}
	if decision != eval.DecisionAsk {
		t.Fatalf("parseDecisionFlag(\"ASK\") = %v, want ask", decision)
	}

	if _, ok := parseDecisionFlag("block"); ok {
		t.Fatal("parseDecisionFlag(\"block\") = ok, want invalid value")
	}
}

func TestPrintTestJSON(t *testing.T) {
	var stdout bytes.Buffer
	printTestJSON(&stdout, &eval.EvaluationResult{
		Decision: eval.DecisionAsk,
		PatternInfo: &eval.PatternMatch{
			RuleID:      "core.git:reset-hard",
			PackID:      "core.git",
			PatternName: "reset-hard",
			Severity:    "critical",
			Reason:      "dangerous command",
		},
		OverrideEntry: &override.Entry{
			ID:     "ovr-1",
			Action: "allow",
			Kind:   "exact",
			Value:  "git status",
			Reason: "reviewed",
		},
		SessionEntry: &session.Entry{
			ID:     "sa-1",
			Kind:   "prefix",
			Value:  "git push",
			Reason: "temporary approval",
		},
	})

	var out struct {
		Decision      string               `json:"decision"`
		RuleID        string               `json:"rule_id"`
		PackID        string               `json:"pack_id"`
		PatternName   string               `json:"pattern_name"`
		Severity      string               `json:"severity"`
		Reason        string               `json:"reason"`
		OverrideMatch *testJSONOverrideOut `json:"override_match"`
		SessionMatch  *testJSONSessionOut  `json:"session_match"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("stdout is not valid json: %v\nstdout=%q", err, stdout.String())
	}
	if out.Decision != "ask" || out.RuleID != "core.git:reset-hard" || out.PackID != "core.git" {
		t.Fatalf("unexpected json output: %+v", out)
	}
	if out.OverrideMatch == nil || out.SessionMatch == nil {
		t.Fatalf("expected override and session matches in json output: %+v", out)
	}
}

func TestPrintTestPretty(t *testing.T) {
	tests := []struct {
		name     string
		result   *eval.EvaluationResult
		command  string
		wantText []string
	}{
		{
			name: "allow",
			result: &eval.EvaluationResult{
				Decision: eval.DecisionAllow,
				OverrideEntry: &override.Entry{
					ID:     "ovr-1",
					Action: "allow",
					Kind:   "exact",
					Value:  "git status",
					Reason: "reviewed",
				},
				SessionEntry: &session.Entry{
					ID:     "sa-1",
					Kind:   "prefix",
					Value:  "git push",
					Reason: "temporary approval",
				},
			},
			command:  "git status",
			wantText: []string{"ALLOW: git status", "Override:", "Session:"},
		},
		{
			name: "ask",
			result: &eval.EvaluationResult{
				Decision: eval.DecisionAsk,
				OverrideEntry: &override.Entry{
					ID:     "ovr-2",
					Action: "allow",
					Kind:   "rule",
					Value:  "core.git:*",
					Reason: "manual review",
				},
				PatternInfo: &eval.PatternMatch{
					RuleID:   "core.git:reset-hard",
					PackID:   "core.git",
					Severity: "critical",
					Reason:   "dangerous command",
				},
			},
			command:  "git reset --hard",
			wantText: []string{"ASK: git reset --hard", "Rule:", "Pack:", "Severity:", "Reason:"},
		},
		{
			name: "deny",
			result: &eval.EvaluationResult{
				Decision: eval.DecisionDeny,
				OverrideEntry: &override.Entry{
					ID:     "ovr-3",
					Action: "deny",
					Kind:   "prefix",
					Value:  "rm -rf",
					Reason: "blocked globally",
				},
				PatternInfo: &eval.PatternMatch{
					RuleID:   "core.filesystem:rm-rf-general",
					PackID:   "core.filesystem",
					Severity: "high",
					Reason:   "dangerous command",
				},
			},
			command:  "rm -rf build",
			wantText: []string{"DENY: rm -rf build", "Rule:", "Pack:", "Severity:", "Reason:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			printTestPretty(&stdout, tt.result, tt.command)

			got := stdout.String()
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Fatalf("stdout = %q, want %q", got, want)
				}
			}
		})
	}
}

func TestExitForDecision(t *testing.T) {
	tests := []struct {
		name      string
		decision  eval.EvaluationDecision
		wantCode  int
		wantError bool
	}{
		{name: "allow", decision: eval.DecisionAllow, wantCode: 0},
		{name: "ask", decision: eval.DecisionAsk, wantCode: exitcodes.Deny, wantError: true},
		{name: "deny", decision: eval.DecisionDeny, wantCode: exitcodes.Deny, wantError: true},
		{name: "warn", decision: eval.DecisionWarn, wantCode: exitcodes.Deny, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := exitForDecision(tt.decision)
			if tt.wantError {
				if err == nil {
					t.Fatal("exitForDecision() error = nil, want ExitError")
				}
				var exitErr *ExitError
				if !errors.As(err, &exitErr) {
					t.Fatalf("exitForDecision() error = %T, want *ExitError", err)
				}
				if exitErr.Code != tt.wantCode || !exitErr.Silent {
					t.Fatalf("exit error = %+v, want silent code %d", exitErr, tt.wantCode)
				}
				return
			}

			if err != nil {
				t.Fatalf("exitForDecision() error = %v, want nil", err)
			}
		})
	}
}

func resetTestCommandState() {
	testJSON = false
	testShell = ""
	testExpect = ""
	testForce = ""
}
