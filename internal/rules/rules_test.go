package rules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAdd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.json")
	rf, err := LoadOrCreate(path)
	if err != nil {
		t.Fatal(err)
	}

	// Valid entry should succeed.
	err = rf.Add(RuleEntry{
		Name:    "no-rm-rf",
		Action:  "deny",
		Kind:    "exact",
		Pattern: "rm -rf /",
		Reason:  "Dangerous recursive delete",
	})
	if err != nil {
		t.Fatalf("Add valid entry: %v", err)
	}
	if len(rf.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rf.Rules))
	}
	if rf.Rules[0].AddedAt.IsZero() {
		t.Error("AddedAt should be set automatically")
	}

	// Duplicate name should fail.
	err = rf.Add(RuleEntry{
		Name:    "no-rm-rf",
		Action:  "deny",
		Kind:    "exact",
		Pattern: "rm -rf /",
		Reason:  "duplicate",
	})
	if err == nil {
		t.Error("expected error for duplicate name")
	}

	// Invalid name format should fail.
	err = rf.Add(RuleEntry{
		Name:    "UPPERCASE",
		Action:  "deny",
		Kind:    "exact",
		Pattern: "test",
		Reason:  "bad name",
	})
	if err == nil {
		t.Error("expected error for invalid name format")
	}

	// Name starting with digit should fail.
	err = rf.Add(RuleEntry{
		Name:    "1bad",
		Action:  "deny",
		Kind:    "exact",
		Pattern: "test",
		Reason:  "bad name",
	})
	if err == nil {
		t.Error("expected error for name starting with digit")
	}

	// Invalid action should fail.
	err = rf.Add(RuleEntry{
		Name:    "test-action",
		Action:  "block",
		Kind:    "exact",
		Pattern: "test",
		Reason:  "bad action",
	})
	if err == nil {
		t.Error("expected error for invalid action")
	}

	// Invalid kind should fail.
	err = rf.Add(RuleEntry{
		Name:    "test-kind",
		Action:  "deny",
		Kind:    "glob",
		Pattern: "test",
		Reason:  "bad kind",
	})
	if err == nil {
		t.Error("expected error for invalid kind")
	}

	// "regex" kind is no longer valid.
	err = rf.Add(RuleEntry{
		Name:    "bad-regex-kind",
		Action:  "deny",
		Kind:    "regex",
		Pattern: `rm\s+-rf`,
		Reason:  "regex kind removed",
	})
	if err == nil {
		t.Error("expected error for removed regex kind")
	}

	// Valid exact entry should succeed.
	err = rf.Add(RuleEntry{
		Name:    "no-drop-db",
		Action:  "deny",
		Kind:    "exact",
		Pattern: "DROP DATABASE",
		Reason:  "No dropping databases",
	})
	if err != nil {
		t.Fatalf("Add second valid entry: %v", err)
	}
	if len(rf.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rf.Rules))
	}

	// Valid prefix entry with allow action.
	err = rf.Add(RuleEntry{
		Name:    "allow-go-test",
		Action:  "allow",
		Kind:    "prefix",
		Pattern: "go test",
		Reason:  "Go tests are safe",
	})
	if err != nil {
		t.Fatalf("Add prefix entry: %v", err)
	}

	// Valid ask action.
	err = rf.Add(RuleEntry{
		Name:    "ask-kubectl-delete",
		Action:  "ask",
		Kind:    "prefix",
		Pattern: "kubectl delete",
		Reason:  "Confirm k8s deletes",
	})
	if err != nil {
		t.Fatalf("Add ask entry: %v", err)
	}

	if len(rf.Rules) != 4 {
		t.Fatalf("expected 4 rules, got %d", len(rf.Rules))
	}
}

func TestAdd_PreservesExplicitAddedAt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.json")
	rf, err := LoadOrCreate(path)
	if err != nil {
		t.Fatal(err)
	}

	explicit := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	err = rf.Add(RuleEntry{
		Name:    "with-timestamp",
		Action:  "deny",
		Kind:    "exact",
		Pattern: "test",
		Reason:  "has explicit time",
		AddedAt: explicit,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !rf.Rules[0].AddedAt.Equal(explicit) {
		t.Errorf("AddedAt = %v, want %v", rf.Rules[0].AddedAt, explicit)
	}
}

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.json")
	rf, err := LoadOrCreate(path)
	if err != nil {
		t.Fatal(err)
	}

	_ = rf.Add(RuleEntry{
		Name:    "rule-a",
		Action:  "deny",
		Kind:    "exact",
		Pattern: "cmd-a",
		Reason:  "reason a",
	})
	_ = rf.Add(RuleEntry{
		Name:    "rule-b",
		Action:  "deny",
		Kind:    "exact",
		Pattern: "cmd-b",
		Reason:  "reason b",
	})
	_ = rf.Add(RuleEntry{
		Name:    "rule-c",
		Action:  "deny",
		Kind:    "exact",
		Pattern: "cmd-c",
		Reason:  "reason c",
	})

	if len(rf.Rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rf.Rules))
	}

	// Remove middle entry.
	found, err := rf.Remove("rule-b")
	if !found {
		t.Error("Remove should return true for existing name")
	}
	if err != nil {
		t.Errorf("Remove save error: %v", err)
	}
	if len(rf.Rules) != 2 {
		t.Fatalf("expected 2 rules after remove, got %d", len(rf.Rules))
	}

	// Verify remaining entries.
	if rf.Rules[0].Name != "rule-a" {
		t.Errorf("first rule name = %q, want rule-a", rf.Rules[0].Name)
	}
	if rf.Rules[1].Name != "rule-c" {
		t.Errorf("second rule name = %q, want rule-c", rf.Rules[1].Name)
	}

	// Remove non-existent should return false.
	found, err = rf.Remove("nonexistent")
	if found {
		t.Error("Remove should return false for unknown name")
	}
	if err != nil {
		t.Errorf("Remove should not error for unknown name: %v", err)
	}

	// Remove should also return false for already-removed entry.
	found, _ = rf.Remove("rule-b")
	if found {
		t.Error("Remove should return false for already-removed name")
	}
}

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.json")
	rf, err := LoadOrCreate(path)
	if err != nil {
		t.Fatal(err)
	}

	// Add entries with various fields.
	err = rf.Add(RuleEntry{
		Name:        "no-rm-rf",
		Action:      "deny",
		Kind:        "exact",
		Pattern:     "rm -rf /",
		Reason:      "Dangerous recursive delete",
		Severity:    "critical",
		Keywords:    []string{"rm", "-rf"},
		Explanation: "This command recursively deletes from root",
		Suggestions: []Suggestion{
			{Command: "rm -rf ./build", Description: "Delete build directory only"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = rf.Add(RuleEntry{
		Name:    "no-force-push",
		Action:  "ask",
		Kind:    "prefix",
		Pattern: "git push --force",
		Reason:  "Force push requires confirmation",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify file exists on disk.
	if _, err := os.Stat(path); err != nil {
		t.Fatal("rules file should exist after Add")
	}

	// Load from disk.
	rf2, err := LoadOrCreate(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(rf2.Rules) != 2 {
		t.Fatalf("expected 2 rules after reload, got %d", len(rf2.Rules))
	}

	// Check first entry.
	r0 := rf2.Rules[0]
	if r0.Name != "no-rm-rf" {
		t.Errorf("rule 0 name = %q", r0.Name)
	}
	if r0.Action != "deny" {
		t.Errorf("rule 0 action = %q", r0.Action)
	}
	if r0.Kind != "exact" {
		t.Errorf("rule 0 kind = %q", r0.Kind)
	}
	if r0.Pattern != "rm -rf /" {
		t.Errorf("rule 0 pattern = %q", r0.Pattern)
	}
	if r0.Severity != "critical" {
		t.Errorf("rule 0 severity = %q", r0.Severity)
	}
	if len(r0.Keywords) != 2 || r0.Keywords[0] != "rm" || r0.Keywords[1] != "-rf" {
		t.Errorf("rule 0 keywords = %v", r0.Keywords)
	}
	if r0.Explanation != "This command recursively deletes from root" {
		t.Errorf("rule 0 explanation = %q", r0.Explanation)
	}
	if len(r0.Suggestions) != 1 || r0.Suggestions[0].Command != "rm -rf ./build" {
		t.Errorf("rule 0 suggestions = %v", r0.Suggestions)
	}
	if r0.AddedAt.IsZero() {
		t.Error("rule 0 added_at should not be zero")
	}

	// Check second entry.
	r1 := rf2.Rules[1]
	if r1.Name != "no-force-push" {
		t.Errorf("rule 1 name = %q", r1.Name)
	}
	if r1.Action != "ask" {
		t.Errorf("rule 1 action = %q", r1.Action)
	}
	if r1.Kind != "prefix" {
		t.Errorf("rule 1 kind = %q", r1.Kind)
	}

	// Verify path is preserved.
	if rf2.Path() != path {
		t.Errorf("Path() = %q, want %q", rf2.Path(), path)
	}
}

func TestLoadOrCreate(t *testing.T) {
	t.Run("non-existent path returns empty", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "nonexistent", "rules.json")
		rf, err := LoadOrCreate(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(rf.Rules) != 0 {
			t.Errorf("expected 0 rules, got %d", len(rf.Rules))
		}
		if rf.Path() != path {
			t.Errorf("Path() = %q, want %q", rf.Path(), path)
		}
	})

	t.Run("existing file loads correctly", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "rules.json")
		content := `{
  "rules": [
    {
      "name": "test-rule",
      "action": "deny",
      "kind": "exact",
      "pattern": "dangerous-command",
      "reason": "Too dangerous",
      "added_at": "2025-01-01T00:00:00Z"
    }
  ]
}`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		rf, err := LoadOrCreate(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(rf.Rules) != 1 {
			t.Fatalf("expected 1 rule, got %d", len(rf.Rules))
		}
		if rf.Rules[0].Name != "test-rule" {
			t.Errorf("name = %q, want test-rule", rf.Rules[0].Name)
		}
		if rf.Rules[0].Action != "deny" {
			t.Errorf("action = %q, want deny", rf.Rules[0].Action)
		}
		if rf.Rules[0].Pattern != "dangerous-command" {
			t.Errorf("pattern = %q", rf.Rules[0].Pattern)
		}
	})

	t.Run("empty file returns empty", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "rules.json")
		if err := os.WriteFile(path, []byte("   \n  "), 0o644); err != nil {
			t.Fatal(err)
		}

		rf, err := LoadOrCreate(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(rf.Rules) != 0 {
			t.Errorf("expected 0 rules for empty file, got %d", len(rf.Rules))
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "rules.json")
		if err := os.WriteFile(path, []byte("{bad json}"), 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := LoadOrCreate(path)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.json")
	rf, err := LoadOrCreate(path)
	if err != nil {
		t.Fatal(err)
	}

	// Empty list.
	if rules := rf.List(); len(rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(rules))
	}

	_ = rf.Add(RuleEntry{
		Name:    "rule-one",
		Action:  "deny",
		Kind:    "exact",
		Pattern: "cmd1",
		Reason:  "r1",
	})
	_ = rf.Add(RuleEntry{
		Name:    "rule-two",
		Action:  "allow",
		Kind:    "prefix",
		Pattern: "cmd2",
		Reason:  "r2",
	})

	rules := rf.List()
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	if rules[0].Name != "rule-one" {
		t.Errorf("first rule name = %q", rules[0].Name)
	}
	if rules[1].Name != "rule-two" {
		t.Errorf("second rule name = %q", rules[1].Name)
	}
}

func TestSave_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "rules.json")
	rf := &RulesFile{path: path}
	rf.Rules = []RuleEntry{
		{
			Name:    "test",
			Action:  "deny",
			Kind:    "exact",
			Pattern: "test",
			Reason:  "test",
			AddedAt: time.Now().UTC().Truncate(time.Second),
		},
	}

	if err := rf.Save(); err != nil {
		t.Fatalf("Save should create parent dirs: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatal("file should exist after Save")
	}
}

func TestSave_NoPath(t *testing.T) {
	rf := &RulesFile{}
	if err := rf.Save(); err == nil {
		t.Error("Save with empty path should return error")
	}
}

func TestSave_JSONFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.json")
	rf := &RulesFile{path: path}
	rf.Rules = []RuleEntry{
		{
			Name:    "test",
			Action:  "deny",
			Kind:    "exact",
			Pattern: "test-cmd",
			Reason:  "testing",
			AddedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	if err := rf.Save(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Verify it's valid JSON with 2-space indent.
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}

	// Verify trailing newline.
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Error("saved file should end with newline")
	}
}

func TestUserRulesPath(t *testing.T) {
	p, err := UserRulesPath()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(p) {
		t.Errorf("expected absolute path, got %q", p)
	}
	if filepath.Base(p) != "rules.json" {
		t.Errorf("expected rules.json, got %q", filepath.Base(p))
	}
	if filepath.Base(filepath.Dir(p)) != ".tw" {
		t.Errorf("expected .tw parent dir, got %q", filepath.Base(filepath.Dir(p)))
	}
}

func TestProjectRulesPath(t *testing.T) {
	p := ProjectRulesPath()
	if p != filepath.Join(".tw", "rules.json") {
		t.Errorf("expected .tw/rules.json, got %q", p)
	}
}
