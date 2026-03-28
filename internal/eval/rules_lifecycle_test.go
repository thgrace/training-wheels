package eval

import (
	"context"
	"testing"
	"time"

	"github.com/thgrace/training-wheels/internal/override"
	"github.com/thgrace/training-wheels/internal/packs"
	"github.com/thgrace/training-wheels/internal/rules"
	"github.com/thgrace/training-wheels/internal/session"
)

func TestSetRules_ReplacesAndClearsSyntheticPack(t *testing.T) {
	e := newTestEvaluator()
	basePackCount := len(e.EnabledPackIDs())

	firstRules := &rules.RulesFile{
		Rules: []rules.RuleEntry{
			{
				Name:     "deny-status",
				Action:   "deny",
				Kind:     "command",
				When:     &packs.PatternCondition{Command: []string{"git"}, Subcommand: []string{"status"}},
				Severity: "critical",
				Keywords: []string{"git"},
				Reason:   "block git status",
			},
		},
	}
	e.SetRules(firstRules, nil)

	result := e.Evaluate(context.Background(), "git status")
	if result.Decision != DecisionDeny {
		t.Fatalf("first rules should deny git status, got %v", result.Decision)
	}
	if result.PatternInfo == nil || result.PatternInfo.RuleID != "rules.custom:deny-status" {
		t.Fatalf("unexpected rule match after first SetRules: %+v", result.PatternInfo)
	}
	assertSingleSyntheticPack(t, e, basePackCount+1)

	secondRules := &rules.RulesFile{
		Rules: []rules.RuleEntry{
			{
				Name:     "deny-echo",
				Action:   "deny",
				Kind:     "command",
				When:     &packs.PatternCondition{Command: []string{"echo"}, ArgContains: []string{"hello"}},
				Severity: "critical",
				Keywords: []string{"echo"},
				Reason:   "block echo hello",
			},
		},
	}
	e.SetRules(secondRules, nil)

	if result := e.Evaluate(context.Background(), "git status"); result.Decision != DecisionAllow {
		t.Fatalf("second SetRules should replace old synthetic rules, got %v", result.Decision)
	}
	result = e.Evaluate(context.Background(), "echo hello")
	if result.Decision != DecisionDeny {
		t.Fatalf("second rules should deny echo hello, got %v", result.Decision)
	}
	if result.PatternInfo == nil || result.PatternInfo.RuleID != "rules.custom:deny-echo" {
		t.Fatalf("unexpected rule match after second SetRules: %+v", result.PatternInfo)
	}
	assertSingleSyntheticPack(t, e, basePackCount+1)

	e.SetRules(nil, nil)

	if result := e.Evaluate(context.Background(), "echo hello"); result.Decision != DecisionAllow {
		t.Fatalf("clearing rules should remove synthetic deny, got %v", result.Decision)
	}
	assertNoSyntheticPack(t, e, basePackCount)
}

func TestSetRules_CustomAllowByRule(t *testing.T) {
	e := newTestEvaluator()
	rf := &rules.RulesFile{
		Rules: []rules.RuleEntry{
			{
				Name:     "deny-status",
				Action:   "deny",
				Kind:     "command",
				When:     &packs.PatternCondition{Command: []string{"git"}, Subcommand: []string{"status"}},
				Severity: "critical",
				Keywords: []string{"git"},
				Reason:   "block git status",
			},
			{
				Name:    "allow-status",
				Action:  "allow",
				Kind:    "rule",
				Pattern: "rules.custom:deny-status",
				Reason:  "approved git status",
			},
		},
	}
	e.SetRules(rf, nil)

	result := e.Evaluate(context.Background(), "git status")
	if result.Decision != DecisionAllow {
		t.Fatalf("rule allow should override synthetic deny, got %v", result.Decision)
	}
	if result.RuleEntry == nil || result.RuleEntry.Name != "allow-status" {
		t.Fatalf("expected matching allow rule entry, got %+v", result.RuleEntry)
	}
}

func TestPostPackChecks_UsesCleanedFallbacks(t *testing.T) {
	raw := `C:\Git\bin\git.exe reset --hard`
	cleaned := `C:/Git/bin/git.exe reset --hard`
	ruleID := "core.git:reset-hard"
	match := &PatternMatch{
		PackID:      "core.git",
		PatternName: "reset-hard",
		RuleID:      ruleID,
		Severity:    packs.SeverityCritical.String(),
		Source:      SourcePack,
	}
	dm := &packs.DestructiveMatch{
		Name:     "reset-hard",
		Severity: packs.SeverityCritical,
	}

	t.Run("rule allow", func(t *testing.T) {
		e := newTestEvaluator()
		e.SetRules(&rules.RulesFile{
			Rules: []rules.RuleEntry{
				{
					Name:    "allow-cleaned-exact",
					Action:  "allow",
					Kind:    "exact",
					Pattern: cleaned,
					Reason:  "allow cleaned exact command",
				},
			},
		}, nil)

		result := e.postPackChecks(context.Background(), raw, cleaned, ruleID, nil, match, dm, time.Now())
		if result == nil || result.Decision != DecisionAllow {
			t.Fatalf("cleaned rule allow should override, got %+v", result)
		}
		if result.RuleEntry == nil || result.RuleEntry.Name != "allow-cleaned-exact" {
			t.Fatalf("expected cleaned allow rule entry, got %+v", result.RuleEntry)
		}
	})

	t.Run("override ask", func(t *testing.T) {
		e := newTestEvaluator()
		ov := &override.Overrides{}
		ov.Add(override.ActionAsk, override.SelectorExact, cleaned, "ask on cleaned exact")
		e.SetOverrides(ov, nil)

		result := e.postPackChecks(context.Background(), raw, cleaned, ruleID, nil, match, dm, time.Now())
		if result == nil || result.Decision != DecisionAsk {
			t.Fatalf("cleaned override ask should win, got %+v", result)
		}
		if result.OverrideEntry == nil || result.OverrideEntry.Action != "ask" {
			t.Fatalf("expected cleaned ask override entry, got %+v", result.OverrideEntry)
		}
	})

	t.Run("override allow", func(t *testing.T) {
		e := newTestEvaluator()
		ov := &override.Overrides{}
		ov.Add(override.ActionAllow, override.SelectorExact, cleaned, "allow on cleaned exact")
		e.SetOverrides(ov, nil)

		result := e.postPackChecks(context.Background(), raw, cleaned, ruleID, nil, match, dm, time.Now())
		if result == nil || result.Decision != DecisionAllow {
			t.Fatalf("cleaned override allow should win, got %+v", result)
		}
		if result.OverrideEntry == nil || result.OverrideEntry.Action != "allow" {
			t.Fatalf("expected cleaned allow override entry, got %+v", result.OverrideEntry)
		}
	})

	t.Run("session allow", func(t *testing.T) {
		e := newTestEvaluator()
		sa := &session.Allowlist{}
		sa.Add([]byte("secret"), "allow", "exact", cleaned, "allow cleaned exact", time.Time{})
		e.SetSessionAllows(sa)

		result := e.postPackChecks(context.Background(), raw, cleaned, ruleID, nil, match, dm, time.Now())
		if result == nil || result.Decision != DecisionAllow {
			t.Fatalf("cleaned session allow should win, got %+v", result)
		}
		if result.SessionEntry == nil || result.SessionEntry.Kind != "exact" {
			t.Fatalf("expected cleaned session allow entry, got %+v", result.SessionEntry)
		}
	})
}

func assertSingleSyntheticPack(t *testing.T, e *Evaluator, wantCount int) {
	t.Helper()

	got := e.EnabledPackIDs()
	if len(got) != wantCount {
		t.Fatalf("EnabledPackIDs count = %d, want %d (%v)", len(got), wantCount, got)
	}
	count := 0
	for _, id := range got {
		if id == "rules.custom" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("rules.custom count = %d, want 1 (%v)", count, got)
	}
}

func assertNoSyntheticPack(t *testing.T, e *Evaluator, wantCount int) {
	t.Helper()

	got := e.EnabledPackIDs()
	if len(got) != wantCount {
		t.Fatalf("EnabledPackIDs count = %d, want %d (%v)", len(got), wantCount, got)
	}
	for _, id := range got {
		if id == "rules.custom" {
			t.Fatalf("unexpected synthetic pack after clearing rules: %v", got)
		}
	}
}
