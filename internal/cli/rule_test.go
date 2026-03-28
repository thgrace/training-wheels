package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thgrace/training-wheels/internal/rules"
)

// setTempHome sets HOME to a temp dir so rules files are isolated.
// Returns a cleanup function to restore the original HOME.
func setTempHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	orig := os.Getenv("HOME")
	t.Setenv("HOME", dir)
	t.Cleanup(func() {
		os.Setenv("HOME", orig)
	})
}

func resetRuleAddFlags() {
	ruleJSON = false
	ruleAddName = ""
	ruleAddRuleID = ""
	ruleAddReason = ""
	ruleAddSeverity = ""
	ruleAddExplanation = ""
	ruleAddSuggest = nil
	ruleAddKeyword = nil
	ruleAddProject = false
	ruleAddDryRun = false
	ruleAddCommand = nil
	ruleAddSubcommand = nil
	ruleAddFlag = nil
	ruleAddAllFlags = nil
	ruleAddArgExact = nil
	ruleAddArgPrefix = nil
	ruleAddArgContains = nil
	ruleAddUnlessFlag = nil
	ruleAddUnlessArg = nil
}

func resetRuleRemoveFlags() {
	ruleJSON = false
	ruleRemoveProject = false
	ruleRemoveYes = false
}

func newRuleTestCommand() (out *bytes.Buffer, errOut *bytes.Buffer) {
	var o bytes.Buffer
	var e bytes.Buffer
	rootCmd.SetOut(&o)
	rootCmd.SetErr(&e)
	return &o, &e
}

func TestRuleAddDenyAllFlags(t *testing.T) {
	setTempHome(t)
	resetRuleAddFlags()
	t.Cleanup(resetRuleAddFlags)

	out, _ := newRuleTestCommand()
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	rootCmd.SetArgs([]string{
		"rule", "add", "deny",
		"--name", "no-rm-rf",
		"--command", "rm",
		"--flag", "-rf",
		"--arg-prefix", "/",
		"--reason", "dangerous recursive delete",
		"--severity", "critical",
		"--explanation", "This deletes from root",
		"--suggest", "rm -rf ./build||Delete build dir only",
		"--keyword", "rm",
		"--keyword", "-rf",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Added user rule") {
		t.Errorf("output = %q, want 'Added user rule'", output)
	}
	if !strings.Contains(output, "no-rm-rf") {
		t.Errorf("output = %q, want rule name", output)
	}

	// Verify it was saved.
	home := os.Getenv("HOME")
	rulesPath := filepath.Join(home, ".tw", "rules.json")
	rf, err := rules.LoadOrCreate(rulesPath)
	if err != nil {
		t.Fatalf("loading rules: %v", err)
	}
	if len(rf.List()) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rf.List()))
	}
	r := rf.List()[0]
	if r.Name != "no-rm-rf" {
		t.Errorf("name = %q", r.Name)
	}
	if r.Action != "deny" {
		t.Errorf("action = %q", r.Action)
	}
	if r.Kind != "command" {
		t.Errorf("kind = %q, want command", r.Kind)
	}
	if r.Severity != "critical" {
		t.Errorf("severity = %q", r.Severity)
	}
	if r.Explanation != "This deletes from root" {
		t.Errorf("explanation = %q", r.Explanation)
	}
	if r.When == nil {
		t.Fatal("expected When to be set")
	}
	if len(r.When.Command) != 1 || r.When.Command[0] != "rm" {
		t.Errorf("when.command = %v, want [rm]", r.When.Command)
	}
	if len(r.When.Flag) != 1 || r.When.Flag[0] != "-rf" {
		t.Errorf("when.flag = %v, want [-rf]", r.When.Flag)
	}
	if len(r.When.ArgPrefix) != 1 || r.When.ArgPrefix[0] != "/" {
		t.Errorf("when.arg_prefix = %v, want [/]", r.When.ArgPrefix)
	}
	if len(r.Keywords) != 2 || r.Keywords[0] != "rm" || r.Keywords[1] != "-rf" {
		t.Errorf("keywords = %v", r.Keywords)
	}
	if len(r.Suggestions) != 1 || r.Suggestions[0].Command != "rm -rf ./build" {
		t.Errorf("suggestions = %v", r.Suggestions)
	}
}

func TestRuleAddAllowWithRule(t *testing.T) {
	setTempHome(t)
	resetRuleAddFlags()
	t.Cleanup(resetRuleAddFlags)

	out, _ := newRuleTestCommand()
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	rootCmd.SetArgs([]string{
		"rule", "add", "allow",
		"--name", "allow-reset",
		"--rule", "core.git:reset-hard",
		"--reason", "reviewed and approved",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Added user rule") {
		t.Errorf("output = %q, want 'Added user rule'", output)
	}

	// Verify it was saved.
	home := os.Getenv("HOME")
	rulesPath := filepath.Join(home, ".tw", "rules.json")
	rf, err := rules.LoadOrCreate(rulesPath)
	if err != nil {
		t.Fatalf("loading rules: %v", err)
	}
	r := rf.List()[0]
	if r.Kind != "rule" {
		t.Errorf("kind = %q, want rule", r.Kind)
	}
	if r.Pattern != "core.git:reset-hard" {
		t.Errorf("pattern = %q", r.Pattern)
	}
	_ = out
}

func TestRuleAddMissingName(t *testing.T) {
	setTempHome(t)
	resetRuleAddFlags()
	t.Cleanup(resetRuleAddFlags)

	out, errOut := newRuleTestCommand()
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	rootCmd.SetArgs([]string{
		"rule", "add", "deny",
		"--command", "rm",
		"--reason", "bad",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --name")
	}
	if !strings.Contains(err.Error(), "--name is required") {
		t.Errorf("error = %q, want --name required", err.Error())
	}
	_ = out
	_ = errOut
}

func TestRuleAddInvalidAction(t *testing.T) {
	setTempHome(t)
	resetRuleAddFlags()
	t.Cleanup(resetRuleAddFlags)

	out, _ := newRuleTestCommand()
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	rootCmd.SetArgs([]string{
		"rule", "add", "block",
		"--name", "test",
		"--command", "cmd",
		"--reason", "bad",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid action")
	}
	if !strings.Contains(err.Error(), "invalid action") {
		t.Errorf("error = %q, want invalid action", err.Error())
	}
	_ = out
}

func TestRuleAddDenyWithRuleFlag(t *testing.T) {
	setTempHome(t)
	resetRuleAddFlags()
	t.Cleanup(resetRuleAddFlags)

	out, _ := newRuleTestCommand()
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	rootCmd.SetArgs([]string{
		"rule", "add", "deny",
		"--name", "bad-combo",
		"--rule", "some.rule:id",
		"--reason", "test",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error: --rule only valid with allow")
	}
	if !strings.Contains(err.Error(), "--rule is only valid with allow action") {
		t.Errorf("error = %q, want --rule allow-only error", err.Error())
	}
	_ = out
}

func TestRuleListEmpty(t *testing.T) {
	setTempHome(t)
	resetRuleAddFlags()
	t.Cleanup(resetRuleAddFlags)

	out, _ := newRuleTestCommand()
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	rootCmd.SetArgs([]string{"rule", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "No rules.") {
		t.Errorf("output = %q, want 'No rules.'", output)
	}
}

func TestRuleListAfterAdd(t *testing.T) {
	setTempHome(t)
	resetRuleAddFlags()
	t.Cleanup(resetRuleAddFlags)

	out, _ := newRuleTestCommand()
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	// Add a rule first.
	rootCmd.SetArgs([]string{
		"rule", "add", "deny",
		"--name", "no-drop",
		"--command", "mysql",
		"--arg-contains", "DROP DATABASE",
		"--reason", "no drops",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("add error: %v", err)
	}
	out.Reset()

	// Now list.
	rootCmd.SetArgs([]string{"rule", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("list error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "no-drop") {
		t.Errorf("output = %q, want rule name 'no-drop'", output)
	}
	if !strings.Contains(output, "deny") {
		t.Errorf("output = %q, want action 'deny'", output)
	}
	if !strings.Contains(output, "User") {
		t.Errorf("output = %q, want source 'User'", output)
	}
}

func TestRuleRemoveExisting(t *testing.T) {
	setTempHome(t)
	resetRuleAddFlags()
	t.Cleanup(resetRuleAddFlags)

	out, _ := newRuleTestCommand()
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	// Add a rule.
	rootCmd.SetArgs([]string{
		"rule", "add", "deny",
		"--name", "to-remove",
		"--command", "bad",
		"--reason", "test",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("add error: %v", err)
	}
	out.Reset()

	// Remove it.
	resetRuleRemoveFlags()
	ruleRemoveYes = true
	rootCmd.SetArgs([]string{"rule", "remove", "to-remove", "--yes"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("remove error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Removed user rule: to-remove") {
		t.Errorf("output = %q, want removed message", output)
	}

	// Verify it's gone.
	home := os.Getenv("HOME")
	rulesPath := filepath.Join(home, ".tw", "rules.json")
	rf, err := rules.LoadOrCreate(rulesPath)
	if err != nil {
		t.Fatalf("loading rules: %v", err)
	}
	if len(rf.List()) != 0 {
		t.Errorf("expected 0 rules after remove, got %d", len(rf.List()))
	}
}

func TestRuleRemoveNonExistent(t *testing.T) {
	setTempHome(t)
	resetRuleRemoveFlags()
	t.Cleanup(resetRuleRemoveFlags)

	out, _ := newRuleTestCommand()
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	ruleRemoveYes = true
	rootCmd.SetArgs([]string{"rule", "remove", "ghost", "--yes"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent rule")
	}
	if !strings.Contains(err.Error(), `rule "ghost" not found`) {
		t.Errorf("error = %q, want not found", err.Error())
	}
	_ = out
}

func TestRuleAddDryRun(t *testing.T) {
	setTempHome(t)
	resetRuleAddFlags()
	t.Cleanup(resetRuleAddFlags)

	out, _ := newRuleTestCommand()
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	rootCmd.SetArgs([]string{
		"rule", "add", "deny",
		"--name", "dry-test",
		"--command", "bad",
		"--reason", "just testing",
		"--dry-run",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()

	// Should be valid JSON.
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("dry-run output is not valid JSON: %v\noutput: %s", err, output)
	}
	if parsed["name"] != "dry-test" {
		t.Errorf("name = %v, want dry-test", parsed["name"])
	}
	if parsed["action"] != "deny" {
		t.Errorf("action = %v, want deny", parsed["action"])
	}

	// Should NOT be saved to disk.
	home := os.Getenv("HOME")
	rulesPath := filepath.Join(home, ".tw", "rules.json")
	rf, err := rules.LoadOrCreate(rulesPath)
	if err != nil {
		t.Fatalf("loading rules: %v", err)
	}
	if len(rf.List()) != 0 {
		t.Errorf("dry-run should not save, got %d rules", len(rf.List()))
	}
}

func TestRuleAddDefaultSeverityAndExplanation(t *testing.T) {
	setTempHome(t)
	resetRuleAddFlags()
	t.Cleanup(resetRuleAddFlags)

	out, _ := newRuleTestCommand()
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	rootCmd.SetArgs([]string{
		"rule", "add", "deny",
		"--name", "default-meta",
		"--command", "dangerous",
		"--reason", "too risky",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	home := os.Getenv("HOME")
	rulesPath := filepath.Join(home, ".tw", "rules.json")
	rf, err := rules.LoadOrCreate(rulesPath)
	if err != nil {
		t.Fatalf("loading rules: %v", err)
	}
	r := rf.List()[0]
	if r.Severity != "medium" {
		t.Errorf("severity = %q, want medium (default)", r.Severity)
	}
	if r.Explanation != "too risky" {
		t.Errorf("explanation = %q, want reason as default", r.Explanation)
	}
	_ = out
}

func TestRuleAddMissingReason(t *testing.T) {
	setTempHome(t)
	resetRuleAddFlags()
	t.Cleanup(resetRuleAddFlags)

	_, _ = newRuleTestCommand()
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	rootCmd.SetArgs([]string{
		"rule", "add", "deny",
		"--name", "no-reason",
		"--command", "cmd",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --reason")
	}
	if !strings.Contains(err.Error(), "--reason is required") {
		t.Errorf("error = %q, want --reason required", err.Error())
	}
}

func TestRuleAddNoMatchKind(t *testing.T) {
	setTempHome(t)
	resetRuleAddFlags()
	t.Cleanup(resetRuleAddFlags)

	_, _ = newRuleTestCommand()
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	rootCmd.SetArgs([]string{
		"rule", "add", "deny",
		"--name", "no-kind",
		"--reason", "test",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for no match kind")
	}
	if !strings.Contains(err.Error(), "exactly one of --command or --rule") {
		t.Errorf("error = %q, want match kind error", err.Error())
	}
}

func TestRuleRemoveRequiresYes(t *testing.T) {
	setTempHome(t)
	resetRuleRemoveFlags()
	t.Cleanup(resetRuleRemoveFlags)

	_, _ = newRuleTestCommand()
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	rootCmd.SetArgs([]string{"rule", "remove", "some-rule"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error without --yes")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Errorf("error = %q, want --yes required", err.Error())
	}
}

func TestRuleListJSON(t *testing.T) {
	setTempHome(t)
	resetRuleAddFlags()
	t.Cleanup(resetRuleAddFlags)

	out, _ := newRuleTestCommand()
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	// Add a rule.
	rootCmd.SetArgs([]string{
		"rule", "add", "deny",
		"--name", "json-list-test",
		"--command", "bad",
		"--reason", "test",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("add error: %v", err)
	}
	out.Reset()

	// List with --json.
	rootCmd.SetArgs([]string{"rule", "list", "--json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("list error: %v", err)
	}

	var entries []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0]["source"] != "user" {
		t.Errorf("source = %v, want user", entries[0]["source"])
	}
	if entries[0]["name"] != "json-list-test" {
		t.Errorf("name = %v, want json-list-test", entries[0]["name"])
	}
}

func TestRuleAddAutoKeywords(t *testing.T) {
	setTempHome(t)
	resetRuleAddFlags()
	t.Cleanup(resetRuleAddFlags)

	out, _ := newRuleTestCommand()
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	rootCmd.SetArgs([]string{
		"rule", "add", "deny",
		"--name", "auto-kw",
		"--command", "rm",
		"--flag", "-rf",
		"--reason", "test auto keywords",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	home := os.Getenv("HOME")
	rulesPath := filepath.Join(home, ".tw", "rules.json")
	rf, err := rules.LoadOrCreate(rulesPath)
	if err != nil {
		t.Fatalf("loading rules: %v", err)
	}
	r := rf.List()[0]
	// Auto-keywords from When.Command should yield ["rm"]
	if len(r.Keywords) == 0 {
		t.Error("expected auto-extracted keywords, got none")
	}
	if len(r.Keywords) > 0 && r.Keywords[0] != "rm" {
		t.Errorf("keywords[0] = %q, want rm", r.Keywords[0])
	}
	_ = out
}

func TestRuleAddWithUnlessFlags(t *testing.T) {
	setTempHome(t)
	resetRuleAddFlags()
	t.Cleanup(resetRuleAddFlags)

	out, _ := newRuleTestCommand()
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	rootCmd.SetArgs([]string{
		"rule", "add", "deny",
		"--name", "no-force-push",
		"--command", "git",
		"--subcommand", "push",
		"--flag", "--force",
		"--unless-flag", "--force-with-lease",
		"--reason", "use lease instead",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	home := os.Getenv("HOME")
	rulesPath := filepath.Join(home, ".tw", "rules.json")
	rf, err := rules.LoadOrCreate(rulesPath)
	if err != nil {
		t.Fatalf("loading rules: %v", err)
	}
	r := rf.List()[0]
	if r.Kind != "command" {
		t.Errorf("kind = %q, want command", r.Kind)
	}
	if r.When == nil {
		t.Fatal("expected When to be set")
	}
	if len(r.When.Command) != 1 || r.When.Command[0] != "git" {
		t.Errorf("when.command = %v, want [git]", r.When.Command)
	}
	if len(r.When.Subcommand) != 1 || r.When.Subcommand[0] != "push" {
		t.Errorf("when.subcommand = %v, want [push]", r.When.Subcommand)
	}
	if r.Unless == nil {
		t.Fatal("expected Unless to be set")
	}
	if len(r.Unless.Flag) != 1 || r.Unless.Flag[0] != "--force-with-lease" {
		t.Errorf("unless.flag = %v, want [--force-with-lease]", r.Unless.Flag)
	}
	_ = out
}

func TestRuleAddAllowWithCommand(t *testing.T) {
	setTempHome(t)
	resetRuleAddFlags()
	t.Cleanup(resetRuleAddFlags)

	out, _ := newRuleTestCommand()
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	rootCmd.SetArgs([]string{
		"rule", "add", "allow",
		"--name", "allow-git-status",
		"--command", "git",
		"--subcommand", "status",
		"--reason", "safe read-only command",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	home := os.Getenv("HOME")
	rulesPath := filepath.Join(home, ".tw", "rules.json")
	rf, err := rules.LoadOrCreate(rulesPath)
	if err != nil {
		t.Fatalf("loading rules: %v", err)
	}
	r := rf.List()[0]
	if r.Kind != "command" {
		t.Errorf("kind = %q, want command", r.Kind)
	}
	if r.Action != "allow" {
		t.Errorf("action = %q, want allow", r.Action)
	}
	if r.When == nil || len(r.When.Command) != 1 || r.When.Command[0] != "git" {
		t.Errorf("when.command = %v, want [git]", r.When.Command)
	}
	_ = out
}
