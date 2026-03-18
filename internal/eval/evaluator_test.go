package eval_test

import (
	"context"
	"testing"
	"time"


	"github.com/thgrace/training-wheels/internal/config"
	"github.com/thgrace/training-wheels/internal/eval"
	"github.com/thgrace/training-wheels/internal/override"
	"github.com/thgrace/training-wheels/internal/packs"
)

func newTestEvaluator() *eval.Evaluator {
	return eval.NewEvaluator(config.DefaultConfig(), packs.DefaultRegistry())
}

func TestEvaluateCommand_Allow(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(context.Background(), "git status")
	if result.Decision != eval.DecisionAllow {
		t.Errorf("expected Allow, got %v", result.Decision)
	}
}

func TestEvaluateCommand_Deny(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(context.Background(), "git reset --hard")
	if result.Decision != eval.DecisionDeny {
		t.Fatalf("expected Deny, got %v", result.Decision)
	}
	if result.PatternInfo == nil {
		t.Fatal("expected PatternInfo")
	}
	if result.PatternInfo.PackID != "core.git" {
		t.Errorf("PackID = %q, want core.git", result.PatternInfo.PackID)
	}
	if result.PatternInfo.PatternName != "reset-hard" {
		t.Errorf("PatternName = %q, want reset-hard", result.PatternInfo.PatternName)
	}
	if result.PatternInfo.Severity != "critical" {
		t.Errorf("Severity = %q, want critical", result.PatternInfo.Severity)
	}
}

func TestEvaluateCommand_EmptyCommand(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(context.Background(), "")
	if result.Decision != eval.DecisionAllow {
		t.Errorf("expected Allow for empty command, got %v", result.Decision)
	}
}

func TestEvaluateCommand_OverrideDenyExact(t *testing.T) {
	e := newTestEvaluator()
	ov := &override.Overrides{}
	ov.Add(override.ActionDeny, override.SelectorExact, "evil-command --flag", "dangerous")
	e.SetOverrides(ov, nil)

	result := e.Evaluate(context.Background(), "evil-command --flag")
	if result.Decision != eval.DecisionDeny {
		t.Errorf("expected Deny from deny override, got %v", result.Decision)
	}
	if result.PatternInfo == nil || result.PatternInfo.Source != eval.SourceOverrideDeny {
		t.Error("expected SourceOverrideDeny")
	}
}

func TestEvaluateCommand_OverrideDenyPrefix(t *testing.T) {
	e := newTestEvaluator()
	ov := &override.Overrides{}
	ov.Add(override.ActionDeny, override.SelectorPrefix, "evil-command", "dangerous")
	e.SetOverrides(ov, nil)

	result := e.Evaluate(context.Background(), "evil-command --flag")
	if result.Decision != eval.DecisionDeny {
		t.Errorf("expected Deny from deny prefix override, got %v", result.Decision)
	}
}

func TestEvaluateCommand_OverrideAllowExact(t *testing.T) {
	e := newTestEvaluator()
	ov := &override.Overrides{}
	ov.Add(override.ActionAllow, override.SelectorExact, "rm -rf ./dist", "Build cleanup")
	e.SetOverrides(ov, nil)

	result := e.Evaluate(context.Background(), "rm -rf ./dist")
	if result.Decision != eval.DecisionAllow {
		t.Errorf("expected Allow for overridden command, got %v", result.Decision)
	}
	if result.OverrideEntry == nil {
		t.Error("expected OverrideEntry to be set")
	}
}

func TestEvaluateCommand_OverrideAllowRule(t *testing.T) {
	e := newTestEvaluator()
	ov := &override.Overrides{}
	ov.Add(override.ActionAllow, override.SelectorRule, "core.git:reset-hard", "Known safe")
	e.SetOverrides(ov, nil)

	result := e.Evaluate(context.Background(), "git reset --hard")
	if result.Decision != eval.DecisionAllow {
		t.Errorf("expected Allow for rule-overridden command, got %v", result.Decision)
	}
}

func TestEvaluateCommand_OverrideAllowDoesNotAffectOthers(t *testing.T) {
	e := newTestEvaluator()
	ov := &override.Overrides{}
	ov.Add(override.ActionAllow, override.SelectorExact, "rm -rf ./dist", "Build cleanup")
	e.SetOverrides(ov, nil)

	// Different destructive command should still be denied.
	result := e.Evaluate(context.Background(), "rm -rf /")
	if result.Decision != eval.DecisionDeny {
		t.Errorf("expected Deny for non-overridden command, got %v", result.Decision)
	}
}

func TestEvaluateCommand_NoKeywords(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(context.Background(), "echo hello world")
	if result.Decision != eval.DecisionAllow {
		t.Errorf("expected Allow for no-keyword command, got %v", result.Decision)
	}
}

func TestEvaluateCommand_NormalizedDeny(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(context.Background(), "sudo git reset --hard")
	if result.Decision != eval.DecisionDeny {
		t.Errorf("expected Deny for 'sudo git reset --hard', got %v", result.Decision)
	}
}

func TestEvaluateCommand_Timeout(t *testing.T) {
	e := newTestEvaluator()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond) // ensure timeout fires

	result := e.Evaluate(ctx, "git reset --hard")
	if result.Decision != eval.DecisionAllow {
		t.Errorf("expected Allow on timeout, got %v", result.Decision)
	}
	if !result.SkippedDueToBudget {
		t.Error("expected SkippedDueToBudget=true")
	}
}

func TestEvaluateCommand_RmRfDeny(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(context.Background(), "rm -rf /")
	if result.Decision != eval.DecisionDeny {
		t.Errorf("expected Deny for 'rm -rf /', got %v", result.Decision)
	}
}

func TestEvaluateCommand_RmRfTmpAllow(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(context.Background(), "rm -rf /tmp/mydir")
	if result.Decision != eval.DecisionAllow {
		t.Errorf("expected Allow for 'rm -rf /tmp/mydir', got %v", result.Decision)
	}
}

func TestEvaluateCommand_AllPacks(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Packs.Enabled = []string{"core", "database", "kubernetes", "cloud",
		"containers", "infrastructure", "storage", "remote"}
	e := eval.NewEvaluator(cfg, packs.DefaultRegistry())

	tests := []struct {
		cmd  string
		want eval.EvaluationDecision
	}{
		{"git push --force", eval.DecisionDeny},
		{"kubectl delete namespace production", eval.DecisionDeny},
		{"terraform destroy", eval.DecisionDeny},
		{"docker system prune -a --force", eval.DecisionDeny},
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			result := e.Evaluate(context.Background(), tt.cmd)
			if result.Decision != tt.want {
				t.Errorf("Evaluate(%q) = %v, want %v", tt.cmd, result.Decision, tt.want)
				if result.PatternInfo != nil {
					t.Logf("  matched: %s", result.PatternInfo.RuleID)
				}
			}
		})
	}
}

// Project-level override bypasses denial.
func TestEvaluateCommand_ProjectOverrideAllowExact(t *testing.T) {
	e := newTestEvaluator()
	project := &override.Overrides{}
	project.Add(override.ActionAllow, override.SelectorExact, "rm -rf ./dist", "Build cleanup")
	e.SetOverrides(nil, project)

	result := e.Evaluate(context.Background(), "rm -rf ./dist")
	if result.Decision != eval.DecisionAllow {
		t.Errorf("expected Allow for project-overridden command, got %v", result.Decision)
	}
	if result.OverrideEntry == nil {
		t.Error("expected OverrideEntry to be set")
	}
}

// Project override takes precedence over user.
func TestEvaluateCommand_ProjectOverrideTakesPrecedence(t *testing.T) {
	e := newTestEvaluator()
	user := &override.Overrides{}
	user.Add(override.ActionAllow, override.SelectorExact, "rm -rf ./dist", "from user")
	project := &override.Overrides{}
	project.Add(override.ActionAllow, override.SelectorExact, "rm -rf ./dist", "from project")
	e.SetOverrides(user, project)

	result := e.Evaluate(context.Background(), "rm -rf ./dist")
	if result.Decision != eval.DecisionAllow {
		t.Errorf("expected Allow, got %v", result.Decision)
	}
	if result.OverrideEntry == nil {
		t.Fatal("expected OverrideEntry")
	}
	if result.OverrideEntry.Reason != "from project" {
		t.Errorf("expected project entry to win, got reason=%q", result.OverrideEntry.Reason)
	}
}

// Prefix override entry.
func TestEvaluateCommand_OverrideAllowPrefix(t *testing.T) {
	e := newTestEvaluator()
	ov := &override.Overrides{}
	ov.Add(override.ActionAllow, override.SelectorPrefix, "rm -rf ./", "Allow relative rm")
	e.SetOverrides(ov, nil)

	result := e.Evaluate(context.Background(), "rm -rf ./dist")
	if result.Decision != eval.DecisionAllow {
		t.Errorf("expected Allow for prefix-overridden command, got %v", result.Decision)
	}
}
