package override

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thgrace/training-wheels/internal/matchutil"
)

func TestAdd_AllowExact(t *testing.T) {
	o := &Overrides{path: "/dev/null"}
	e := o.Add(ActionAllow, SelectorExact, "rm -rf ./dist", "Build cleanup")
	if e.Action != "allow" {
		t.Errorf("action = %q, want allow", e.Action)
	}
	if e.Kind != "exact" {
		t.Errorf("kind = %q, want exact", e.Kind)
	}
	if e.Value != "rm -rf ./dist" {
		t.Errorf("value = %q", e.Value)
	}
	if e.ID == "" {
		t.Error("ID should not be empty")
	}
	if len(o.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(o.Entries))
	}
}

func TestAdd_DenyExact(t *testing.T) {
	o := &Overrides{path: "/dev/null"}
	e := o.Add(ActionDeny, SelectorExact, "evil-command", "Never run this")
	if e.Action != "deny" {
		t.Errorf("action = %q, want deny", e.Action)
	}
	if e.Kind != "exact" {
		t.Errorf("kind = %q, want exact", e.Kind)
	}
}

func TestAdd_AskExact(t *testing.T) {
	o := &Overrides{path: "/dev/null"}
	e := o.Add(ActionAsk, SelectorExact, "kubectl delete ns prod", "Require confirmation")
	if e.Action != "ask" {
		t.Errorf("action = %q, want ask", e.Action)
	}
	if e.Kind != "exact" {
		t.Errorf("kind = %q, want exact", e.Kind)
	}
}

func TestAdd_AllowPrefix(t *testing.T) {
	o := &Overrides{path: "/dev/null"}
	e := o.Add(ActionAllow, SelectorPrefix, "make clean", "Standard build")
	if e.Kind != "prefix" {
		t.Errorf("kind = %q, want prefix", e.Kind)
	}
}

func TestAdd_AllowRule(t *testing.T) {
	o := &Overrides{path: "/dev/null"}
	e := o.Add(ActionAllow, SelectorRule, "core.filesystem:rm-rf-relative", "Known safe")
	if e.Kind != "rule" {
		t.Errorf("kind = %q, want rule", e.Kind)
	}
}

func TestMatches_Exact(t *testing.T) {
	e := Entry{Kind: "exact", Value: "rm -rf ./dist"}
	if !e.Matches("rm -rf ./dist", "") {
		t.Error("exact match should succeed")
	}
	if e.Matches("rm -rf ./dist2", "") {
		t.Error("exact match should not match different command")
	}
}

func TestMatches_Prefix(t *testing.T) {
	e := Entry{Kind: "prefix", Value: "make clean"}
	if !e.Matches("make clean", "") {
		t.Error("prefix match should succeed for exact")
	}
	if !e.Matches("make clean all", "") {
		t.Error("prefix match should succeed for extension")
	}
	if e.Matches("make build", "") {
		t.Error("prefix match should not match different prefix")
	}
}

func TestMatches_Rule(t *testing.T) {
	e := Entry{Kind: "rule", Value: "core.git:reset-hard"}
	if !e.Matches("", "core.git:reset-hard") {
		t.Error("rule match should succeed for exact ID")
	}
	if e.Matches("", "core.git:push-force") {
		t.Error("rule match should not match different rule")
	}
}

func TestMatches_RuleWildcard(t *testing.T) {
	e := Entry{Kind: "rule", Value: "core.git:*"}
	if !e.Matches("", "core.git:reset-hard") {
		t.Error("wildcard should match core.git:reset-hard")
	}
	if !e.Matches("", "core.git:push-force") {
		t.Error("wildcard should match core.git:push-force")
	}
	if e.Matches("", "core.filesystem:rm-rf") {
		t.Error("wildcard should not match different pack")
	}
}

func TestMatches_RuleGlobalWildcard(t *testing.T) {
	e := Entry{Kind: "rule", Value: "*"}
	if !e.Matches("", "anything:here") {
		t.Error("global wildcard should match everything")
	}
}

func TestRemove(t *testing.T) {
	o := &Overrides{path: "/dev/null"}
	o.Add(ActionAllow, SelectorExact, "cmd1", "r1")
	e2 := o.Add(ActionAllow, SelectorExact, "cmd2", "r2")
	o.Add(ActionAllow, SelectorExact, "cmd3", "r3")

	if !o.Remove(e2.ID) {
		t.Error("remove should return true for existing ID")
	}
	if len(o.Entries) != 2 {
		t.Errorf("expected 2 entries after remove, got %d", len(o.Entries))
	}
	if o.Remove("nonexistent") {
		t.Error("remove should return false for unknown ID")
	}
}

func TestMatchesAllow(t *testing.T) {
	o := &Overrides{path: "/dev/null"}
	o.Add(ActionAllow, SelectorExact, "rm -rf ./dist", "cleanup")
	o.Add(ActionAllow, SelectorRule, "core.git:*", "allow all git")
	o.Add(ActionDeny, SelectorExact, "evil-cmd", "dangerous")

	if o.MatchesAllow("rm -rf ./dist", "") == nil {
		t.Error("should match exact allow")
	}
	if o.MatchesAllow("", "core.git:reset-hard") == nil {
		t.Error("should match rule wildcard allow")
	}
	if o.MatchesAllow("evil-cmd", "") != nil {
		t.Error("should not match deny entry as allow")
	}
	if o.MatchesAllow("ls -la", "safe:list") != nil {
		t.Error("should not match unrelated command")
	}
}

func TestMatchesAsk(t *testing.T) {
	o := &Overrides{path: "/dev/null"}
	o.Add(ActionAsk, SelectorExact, "kubectl delete ns prod", "Require confirmation")
	o.Add(ActionAsk, SelectorRule, "core.git:reset-hard", "Require confirmation")
	o.Add(ActionAllow, SelectorExact, "safe-cmd", "safe")

	if o.MatchesAsk("kubectl delete ns prod", "") == nil {
		t.Error("should match exact ask")
	}
	if o.MatchesAsk("", "core.git:reset-hard") == nil {
		t.Error("should match rule ask")
	}
	if o.MatchesAsk("safe-cmd", "") != nil {
		t.Error("should not match allow entry as ask")
	}
}

func TestLoadSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overrides.json")

	o := &Overrides{path: path}
	o.Add(ActionAllow, SelectorExact, "rm -rf ./dist", "Build output cleanup")
	o.Add(ActionDeny, SelectorRule, "core.git:reset-hard", "Never allow this")

	if err := o.Save(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatal("file should exist after save")
	}

	o2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(o2.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(o2.Entries))
	}
	if o2.Entries[0].Value != "rm -rf ./dist" {
		t.Errorf("entry 0 value = %q", o2.Entries[0].Value)
	}
	if o2.Entries[0].Action != "allow" {
		t.Errorf("entry 0 action = %q", o2.Entries[0].Action)
	}
	if o2.Entries[1].Kind != "rule" {
		t.Errorf("entry 1 kind = %q", o2.Entries[1].Kind)
	}
	if o2.Entries[1].Action != "deny" {
		t.Errorf("entry 1 action = %q", o2.Entries[1].Action)
	}
}

func TestLoad_NonExistent(t *testing.T) {
	o, err := Load("/nonexistent/path/overrides.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(o.Entries) != 0 {
		t.Error("non-existent file should return empty overrides")
	}
}

func TestCheckAllow(t *testing.T) {
	user := &Overrides{path: "/dev/null"}
	user.Add(ActionAllow, SelectorExact, "user-cmd", "from user")

	project := &Overrides{path: "/dev/null"}
	project.Add(ActionAllow, SelectorExact, "project-cmd", "from project")

	if e := CheckAllow("project-cmd", "", user, project); e == nil {
		t.Error("should match project allow")
	}
	if e := CheckAllow("user-cmd", "", user, project); e == nil {
		t.Error("should match user allow")
	}
	if e := CheckAllow("other-cmd", "", user, project); e != nil {
		t.Error("should not match")
	}
}

func TestCheckAsk(t *testing.T) {
	user := &Overrides{path: "/dev/null"}
	user.Add(ActionAsk, SelectorExact, "user-ask", "from user")

	project := &Overrides{path: "/dev/null"}
	project.Add(ActionAsk, SelectorExact, "project-ask", "from project")

	if e := CheckAsk("project-ask", "", user, project); e == nil {
		t.Error("should match project ask")
	}
	if e := CheckAsk("user-ask", "", user, project); e == nil {
		t.Error("should match user ask")
	}
	if e := CheckAsk("other-cmd", "", user, project); e != nil {
		t.Error("should not match")
	}
}

func TestCheckAllow_ProjectPrecedence(t *testing.T) {
	user := &Overrides{path: "/dev/null"}
	user.Add(ActionAllow, SelectorExact, "shared-cmd", "from user")

	project := &Overrides{path: "/dev/null"}
	project.Add(ActionAllow, SelectorExact, "shared-cmd", "from project")

	e := CheckAllow("shared-cmd", "", user, project)
	if e == nil {
		t.Fatal("should match")
		return
	}
	if e.Reason != "from project" {
		t.Errorf("should prefer project entry, got reason=%q", e.Reason)
	}
}

func TestLoad_FromJSONWithEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overrides.json")
	content := `{
  "entries": [
    {
      "id": "ov-test1",
      "action": "allow",
      "kind": "exact",
      "value": "rm -rf ./dist",
      "reason": "Build output cleanup",
      "added_at": "2025-01-01T00:00:00Z"
    },
    {
      "id": "ov-test2",
      "action": "allow",
      "kind": "rule",
      "value": "core.git:*",
      "reason": "Allow all git operations",
      "added_at": "2025-01-01T00:00:00Z"
    },
    {
      "id": "ov-test3",
      "action": "deny",
      "kind": "exact",
      "value": "evil-command",
      "reason": "Never allow this",
      "added_at": "2025-01-01T00:00:00Z"
    },
    {
      "id": "ov-test4",
      "action": "ask",
      "kind": "rule",
      "value": "core.filesystem:rm-rf-general",
      "reason": "Require confirmation",
      "added_at": "2025-01-01T00:00:00Z"
    }
  ]
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	o, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(o.Entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(o.Entries))
	}
	if o.Entries[0].Kind != "exact" || o.Entries[0].Value != "rm -rf ./dist" {
		t.Errorf("entry 0: kind=%q value=%q", o.Entries[0].Kind, o.Entries[0].Value)
	}
	if o.MatchesAllow("rm -rf ./dist", "") == nil {
		t.Error("loaded exact allow entry should match")
	}
	if o.MatchesAllow("", "core.git:push-force") == nil {
		t.Error("loaded rule wildcard allow entry should match")
	}
	if o.MatchesAsk("", "core.filesystem:rm-rf-general") == nil {
		t.Error("loaded ask entry should match")
	}
}

func TestMatchRule_MultiWildcard(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		ruleID  string
		want    bool
	}{
		{"core.*:* matches core.git:reset-hard", "core.*:*", "core.git:reset-hard", true},
		{"core.*:* matches core.filesystem:rm-rf", "core.*:*", "core.filesystem:rm-rf", true},
		{"*:* matches any pack:rule", "*:*", "core.git:reset-hard", true},
		{"*:* matches simple pair", "*:*", "foo:bar", true},
		{"core.*:reset-* matches core.git:reset-hard", "core.*:reset-*", "core.git:reset-hard", true},
		{"core.*:reset-* does not match core.git:push-force", "core.*:reset-*", "core.git:push-force", false},
		{"core.git:* still works (single wildcard)", "core.git:*", "core.git:reset-hard", true},
		{"core.git:* does not match other pack", "core.git:*", "core.filesystem:rm-rf", false},
		{"exact match no wildcards", "core.git:reset-hard", "core.git:reset-hard", true},
		{"exact mismatch no wildcards", "core.git:reset-hard", "core.git:push-force", false},
		{"global wildcard", "*", "anything:here", true},
		{"trailing wildcard empty match", "core.git:reset-hard*", "core.git:reset-hard", true},
		{"leading wildcard", "*:reset-hard", "core.git:reset-hard", true},
		{"leading wildcard no match", "*:reset-hard", "core.git:push-force", false},
		{"leading wildcard must not match suffix", "*:reset-hard", "core.git:reset-hard-extra", false},
		{"mid wildcard must not match suffix", "core.*:reset-hard", "core.git:reset-hard-extra", false},
		{"trailing wildcard allows suffix", "core.*:reset-*", "core.git:reset-hard-extra", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchutil.MatchRule(tt.pattern, tt.ruleID)
			if got != tt.want {
				t.Errorf("MatchRule(%q, %q) = %v, want %v", tt.pattern, tt.ruleID, got, tt.want)
			}
		})
	}
}

func TestAskMatchesIndependently(t *testing.T) {
	o := &Overrides{path: "/dev/null"}
	o.Add(ActionAsk, SelectorExact, "risky-cmd", "ask first")
	o.Add(ActionAllow, SelectorExact, "risky-cmd", "allowed")

	if o.MatchesAsk("risky-cmd", "") == nil {
		t.Error("should match ask")
	}
	if o.MatchesAllow("risky-cmd", "") == nil {
		t.Error("should match allow")
	}
}
