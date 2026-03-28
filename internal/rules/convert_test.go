package rules

import (
	"testing"

	"github.com/thgrace/training-wheels/internal/ast"
)

func TestConvertToPack_LegacyRegexSkipped(t *testing.T) {
	// Legacy v1 regex entries should be skipped (not converted to DestructivePatterns).
	entries := []RuleEntry{
		{
			Name:     "deny-rm",
			Action:   "deny",
			Kind:     "regex",
			Pattern:  `rm\s+-rf\s+/`,
			Reason:   "dangerous delete",
			Severity: "critical",
			Keywords: []string{"rm"},
		},
	}

	pack := ConvertToPack(entries, "test.rules")
	if pack.ID != "test.rules" {
		t.Errorf("pack ID = %q, want %q", pack.ID, "test.rules")
	}
	// Legacy regex entries are skipped — no structural patterns.
	if len(pack.StructuralPatterns) != 0 {
		t.Errorf("expected 0 structural patterns for legacy regex, got %d", len(pack.StructuralPatterns))
	}
}

func TestConvertToPack_IgnoresAllowEntries(t *testing.T) {
	entries := []RuleEntry{
		{Name: "allow-test", Action: "allow", Kind: "exact", Pattern: "test", Reason: "ok"},
	}
	pack := ConvertToPack(entries, "test.allow")
	if len(pack.StructuralPatterns) != 0 {
		t.Errorf("expected 0 structural patterns for allow-only, got %d", len(pack.StructuralPatterns))
	}
}

func TestConvertToAllowEntries(t *testing.T) {
	entries := []RuleEntry{
		{Name: "deny-test", Action: "deny", Kind: "exact", Pattern: "rm -rf /", Reason: "deny"},
		{Name: "allow-exact", Action: "allow", Kind: "exact", Pattern: "ls -la", Reason: "ok"},
		{Name: "allow-rule", Action: "allow", Kind: "rule", Pattern: "core.git:reset-hard", Reason: "CI"},
		{Name: "ask-test", Action: "ask", Kind: "prefix", Pattern: "rm", Reason: "ask"},
	}

	allows := ConvertToAllowEntries(entries, ast.ShellBash)
	if len(allows) != 2 {
		t.Fatalf("expected 2 allow entries, got %d", len(allows))
	}
	if allows[0].Name != "allow-exact" {
		t.Errorf("first allow name = %q, want %q", allows[0].Name, "allow-exact")
	}
	if allows[1].Kind != "rule" {
		t.Errorf("second allow kind = %q, want %q", allows[1].Kind, "rule")
	}
}

func TestAllowEntry_Matches(t *testing.T) {
	tests := []struct {
		name    string
		entry   AllowEntry
		cmd     string
		ruleID  string
		want    bool
	}{
		{"exact match", AllowEntry{Kind: "exact", Pattern: "ls -la"}, "ls -la", "", true},
		{"exact no match", AllowEntry{Kind: "exact", Pattern: "ls -la"}, "ls -l", "", false},
		{"prefix match", AllowEntry{Kind: "prefix", Pattern: "make "}, "make build", "", true},
		{"prefix no match", AllowEntry{Kind: "prefix", Pattern: "make "}, "rake build", "", false},
		{"rule match", AllowEntry{Kind: "rule", Pattern: "core.git:reset-hard"}, "", "core.git:reset-hard", true},
		{"rule no match", AllowEntry{Kind: "rule", Pattern: "core.git:reset-hard"}, "", "core.git:other", false},
	}

	for i := range tests {
		tt := &tests[i]
		t.Run(tt.name, func(t *testing.T) {
			got := tt.entry.Matches(tt.cmd, tt.ruleID)
			if got != tt.want {
				t.Errorf("Matches(%q, %q) = %v, want %v", tt.cmd, tt.ruleID, got, tt.want)
			}
		})
	}
}

func TestCheckAllow(t *testing.T) {
	allows := []AllowEntry{
		{Name: "allow-ls", Kind: "exact", Pattern: "ls -la", Reason: "ok"},
		{Name: "allow-make", Kind: "prefix", Pattern: "make ", Reason: "build"},
	}

	if got := CheckAllow("ls -la", "", allows); got == nil {
		t.Error("expected match for 'ls -la'")
	} else if got.Name != "allow-ls" {
		t.Errorf("matched %q, want %q", got.Name, "allow-ls")
	}

	if got := CheckAllow("make test", "", allows); got == nil {
		t.Error("expected match for 'make test'")
	}

	if got := CheckAllow("rm -rf /", "", allows); got != nil {
		t.Error("expected no match for 'rm -rf /'")
	}
}
