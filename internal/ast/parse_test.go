package ast

import (
	"strings"
	"testing"
)

// ---------- helpers ----------

func requireNonNil(t *testing.T, cc *CompoundCommand) {
	t.Helper()
	if cc == nil {
		t.Fatal("expected non-nil CompoundCommand, got nil")
	}
}

func requireNil(t *testing.T, cc *CompoundCommand) {
	t.Helper()
	if cc != nil {
		t.Fatalf("expected nil CompoundCommand, got %+v", cc)
	}
}

func requireCommandCount(t *testing.T, cc *CompoundCommand, want int) []SimpleCommand {
	t.Helper()
	cmds := cc.AllCommands()
	if len(cmds) != want {
		t.Fatalf("expected %d commands, got %d: %+v", want, len(cmds), cmds)
	}
	return cmds
}

func requireMinCommandCount(t *testing.T, cc *CompoundCommand, min int) []SimpleCommand {
	t.Helper()
	cmds := cc.AllCommands()
	if len(cmds) < min {
		t.Fatalf("expected at least %d commands, got %d: %+v", min, len(cmds), cmds)
	}
	return cmds
}

// ---------- Parse tests ----------

func TestParse_SimpleCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCmds []SimpleCommand
	}{
		{
			name:  "basic command with flags and args",
			input: "rm -rf /",
			wantCmds: []SimpleCommand{
				{Name: "rm", Flags: []string{"-rf"}, Args: []string{"/"}},
			},
		},
		{
			name:  "command with no flags",
			input: "ls /home",
			wantCmds: []SimpleCommand{
				{Name: "ls", Args: []string{"/home"}},
			},
		},
		{
			name:  "command with no args",
			input: "pwd",
			wantCmds: []SimpleCommand{
				{Name: "pwd"}},
		},
		{
			name:  "git push with long flags",
			input: "git push --force origin main",
			wantCmds: []SimpleCommand{
				{Name: "git", Flags: []string{"--force"}, Args: []string{"push", "origin", "main"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := Parse([]byte(tt.input))
			requireNonNil(t, cc)
			cmds := cc.AllCommands()
			assertCommands(t, cmds, tt.wantCmds)
		})
	}
}

func TestParse_EmptyInput(t *testing.T) {
	cc := Parse([]byte(""))
	requireNonNil(t, cc)
	cmds := cc.AllCommands()
	if len(cmds) != 0 {
		t.Errorf("expected 0 commands for empty input, got %d: %+v", len(cmds), cmds)
	}
}

func TestParse_NilInput(t *testing.T) {
	cc := Parse(nil)
	requireNonNil(t, cc)
	cmds := cc.AllCommands()
	if len(cmds) != 0 {
		t.Errorf("expected 0 commands for nil input, got %d: %+v", len(cmds), cmds)
	}
}

func TestParse_OversizedInput(t *testing.T) {
	// Create input just over the 64KB limit.
	big := []byte(strings.Repeat("a", 64*1024+1))
	cc := Parse(big)
	requireNil(t, cc)
}

func TestParse_ExactlyAtSizeLimit(t *testing.T) {
	// 64KB exactly should still be parsed (limit is >64KB).
	input := []byte("echo " + strings.Repeat("a", 64*1024-5))
	if len(input) != 64*1024 {
		t.Fatalf("precondition failed: expected input length %d, got %d", 64*1024, len(input))
	}
	cc := Parse(input)
	requireNonNil(t, cc)
}

func TestParse_Pipeline(t *testing.T) {
	cc := Parse([]byte("ls -la | grep foo | wc -l"))
	requireNonNil(t, cc)

	if len(cc.Statements) != 1 {
		t.Fatalf("expected 1 statement (pipeline), got %d", len(cc.Statements))
	}
	stmt := cc.Statements[0]
	if len(stmt.Stages) != 3 {
		t.Fatalf("expected 3 pipeline stages, got %d", len(stmt.Stages))
	}

	assertCommand(t, 0, stmt.Stages[0].Command, SimpleCommand{Name: "ls", Flags: []string{"-la"}})
	assertCommand(t, 1, stmt.Stages[1].Command, SimpleCommand{Name: "grep", Args: []string{"foo"}})
	assertCommand(t, 2, stmt.Stages[2].Command, SimpleCommand{Name: "wc", Flags: []string{"-l"}})
}

func TestParse_CompoundAndOr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCmds []SimpleCommand
		wantOps  []string // operators on each statement except last
	}{
		{
			name:  "and chain",
			input: "cd /tmp && rm -rf *",
			wantCmds: []SimpleCommand{
				{Name: "cd", Args: []string{"/tmp"}},
				{Name: "rm", Flags: []string{"-rf"}, Args: []string{"*"}},
			},
			wantOps: []string{"&&"},
		},
		{
			name:  "or chain",
			input: "make build || echo failed",
			wantCmds: []SimpleCommand{
				{Name: "make", Args: []string{"build"}},
				{Name: "echo", Args: []string{"failed"}},
			},
			wantOps: []string{"||"},
		},
		{
			name:  "mixed and-or",
			input: "cd /tmp && rm -rf * || echo failed",
			wantCmds: []SimpleCommand{
				{Name: "cd", Args: []string{"/tmp"}},
				{Name: "rm", Flags: []string{"-rf"}, Args: []string{"*"}},
				{Name: "echo", Args: []string{"failed"}},
			},
			wantOps: []string{"&&", "||"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := Parse([]byte(tt.input))
			requireNonNil(t, cc)
			cmds := cc.AllCommands()
			assertCommands(t, cmds, tt.wantCmds)

			// Verify operators.
			for i, wantOp := range tt.wantOps {
				if i >= len(cc.Statements) {
					t.Fatalf("expected statement[%d] with operator %q, but only %d statements exist", i, wantOp, len(cc.Statements))
				}
				if cc.Statements[i].Operator != wantOp {
					t.Errorf("stmt[%d].Operator = %q, want %q", i, cc.Statements[i].Operator, wantOp)
				}
			}
		})
	}
}

func TestParse_Semicolons(t *testing.T) {
	cc := Parse([]byte("echo a; echo b; echo c"))
	requireNonNil(t, cc)
	cmds := cc.AllCommands()
	assertCommands(t, cmds, []SimpleCommand{
		{Name: "echo", Args: []string{"a"}},
		{Name: "echo", Args: []string{"b"}},
		{Name: "echo", Args: []string{"c"}},
	})
}

// ---------- ParseShell tests ----------

func TestParseShell_Bash(t *testing.T) {
	cc := ParseShell([]byte("rm -rf /"), ShellBash)
	requireNonNil(t, cc)
	cmds := requireCommandCount(t, cc, 1)
	assertCommand(t, 0, cmds[0], SimpleCommand{
		Name:  "rm",
		Flags: []string{"-rf"},
		Args:  []string{"/"},
	})
}

func TestParseShell_PowerShell_Works(t *testing.T) {
	cc := ParseShell([]byte("Remove-Item -Recurse -Force /"), ShellPowerShell)
	requireNonNil(t, cc)
	cmds := cc.AllCommands()
	if len(cmds) == 0 {
		t.Fatal("expected at least 1 command")
	}
	if cmds[0].Name != "remove-item" {
		t.Errorf("Name = %q, want %q", cmds[0].Name, "remove-item")
	}
}

func TestParseShell_InvalidShellType_ReturnsNil(t *testing.T) {
	// Use a ShellType value that is not defined.
	cc := ParseShell([]byte("rm -rf /"), ShellType(99))
	requireNil(t, cc)
}

func TestParseShell_EmptyInput(t *testing.T) {
	cc := ParseShell([]byte(""), ShellBash)
	requireNonNil(t, cc)
	cmds := cc.AllCommands()
	if len(cmds) != 0 {
		t.Errorf("expected 0 commands for empty input, got %d", len(cmds))
	}
}

func TestParseShell_OversizedInput(t *testing.T) {
	big := []byte(strings.Repeat("x", 64*1024+1))
	cc := ParseShell(big, ShellBash)
	requireNil(t, cc)
}

// ---------- Parse with complex structures ----------

func TestParse_Subshell(t *testing.T) {
	cc := Parse([]byte("(cd /tmp && rm -rf *)"))
	requireNonNil(t, cc)
	cmds := cc.AllCommands()
	assertCommands(t, cmds, []SimpleCommand{
		{Name: "cd", Args: []string{"/tmp"}},
		{Name: "rm", Flags: []string{"-rf"}, Args: []string{"*"}},
	})
}

func TestParse_CommandSubstitution(t *testing.T) {
	cc := Parse([]byte("echo $(rm -rf /)"))
	requireNonNil(t, cc)
	cmds := requireMinCommandCount(t, cc, 2)

	if cmds[0].Name != "echo" {
		t.Errorf("cmd[0].Name = %q, want %q", cmds[0].Name, "echo")
	}

	foundRM := false
	for _, cmd := range cmds[1:] {
		if cmd.Name == "rm" {
			foundRM = true
			assertSlice(t, "inner rm flags", cmd.Flags, []string{"-rf"})
			assertSlice(t, "inner rm args", cmd.Args, []string{"/"})
		}
	}
	if !foundRM {
		t.Errorf("expected inner rm command, got: %+v", cmds)
	}
}

func TestParse_QuotedArgs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCmds []SimpleCommand
	}{
		{
			name:  "double quoted arg",
			input: `rm -rf "/tmp/my dir"`,
			wantCmds: []SimpleCommand{
				{Name: "rm", Flags: []string{"-rf"}, Args: []string{"/tmp/my dir"}},
			},
		},
		{
			name:  "single quoted arg",
			input: `echo 'hello world'`,
			wantCmds: []SimpleCommand{
				{Name: "echo", Args: []string{"hello world"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := Parse([]byte(tt.input))
			requireNonNil(t, cc)
			cmds := cc.AllCommands()
			assertCommands(t, cmds, tt.wantCmds)
		})
	}
}

func TestParse_PathNormalization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
	}{
		{"unix path", "/usr/bin/rm -rf /", "rm"},
		{"deep path", "/usr/local/bin/git status", "git"},
		{"exe suffix", "git.exe status", "git"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := Parse([]byte(tt.input))
			requireNonNil(t, cc)
			cmds := requireMinCommandCount(t, cc, 1)
			if cmds[0].Name != tt.wantName {
				t.Errorf("Name = %q, want %q", cmds[0].Name, tt.wantName)
			}
		})
	}
}

func TestParse_CommentOnly(t *testing.T) {
	cc := Parse([]byte("# this is a comment"))
	requireNonNil(t, cc)
	cmds := cc.AllCommands()
	if len(cmds) != 0 {
		t.Errorf("expected 0 commands for comment-only input, got %d: %+v", len(cmds), cmds)
	}
}
