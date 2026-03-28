package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunOverrideAddRejectsInvalidAction(t *testing.T) {
	resetOverrideAddFlags()
	t.Cleanup(resetOverrideAddFlags)

	cmd, _, _ := newOverrideAddTestCommand()
	err := runOverrideAdd(cmd, []string{"block", "git status"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), `invalid action "block": must be allow or ask`) {
		t.Fatalf("error = %q, want invalid action error", err)
	}
}

func TestRunOverrideAddRejectsConflictingMatchModes(t *testing.T) {
	resetOverrideAddFlags()
	t.Cleanup(resetOverrideAddFlags)
	overrideAddSession = true
	overrideAddPrefix = true
	overrideAddRule = true

	cmd, _, _ := newOverrideAddTestCommand()
	err := runOverrideAdd(cmd, []string{"allow", "git status"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "at most one of --prefix or --rule may be specified") {
		t.Fatalf("error = %q, want conflicting match mode error", err)
	}
}

func TestRunOverrideAddRejectsDenyAction(t *testing.T) {
	resetOverrideAddFlags()
	t.Cleanup(resetOverrideAddFlags)
	overrideAddSession = true

	cmd, _, _ := newOverrideAddTestCommand()
	err := runOverrideAdd(cmd, []string{"deny", "git status"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), `invalid action "deny": must be allow or ask`) {
		t.Fatalf("error = %q, want deny rejection error", err)
	}
}

func TestOverrideAddHelpMatchesSupportedSurface(t *testing.T) {
	if overrideAddCmd.Use != "add <allow|ask> <command-or-rule>" {
		t.Fatalf("Use = %q", overrideAddCmd.Use)
	}
	if overrideAddCmd.Flags().Lookup("permanent") != nil {
		t.Fatal("expected --permanent to be absent from tw override add")
	}
	if overrideAddCmd.Flags().Lookup("project") != nil {
		t.Fatal("expected --project to be absent from tw override add")
	}
}

func resetOverrideAddFlags() {
	overrideJSON = false
	overrideAddSession = false
	overrideAddTime = ""
	overrideAddPrefix = false
	overrideAddRule = false
	overrideAddReason = ""
}

func newOverrideAddTestCommand() (cmd *cobra.Command, out *bytes.Buffer, errOut *bytes.Buffer) {
	var o bytes.Buffer
	var e bytes.Buffer
	cmd = &cobra.Command{}
	cmd.SetOut(&o)
	cmd.SetErr(&e)
	return cmd, &o, &e
}
