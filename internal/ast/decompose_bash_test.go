package ast

import (
	"fmt"
	"testing"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func parseBash(t *testing.T, input string) (*gotreesitter.Tree, *gotreesitter.Node) {
	t.Helper()
	lang := grammars.BashLanguage()
	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return tree, tree.RootNode()
}

func assertCommands(t *testing.T, got []SimpleCommand, want []SimpleCommand) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("command count: got %d, want %d\ngot:  %+v\nwant: %+v", len(got), len(want), got, want)
	}
	for i := range want {
		assertCommand(t, i, got[i], want[i])
	}
}

func assertCommand(t *testing.T, idx int, got, want SimpleCommand) {
	t.Helper()
	if got.Name != want.Name {
		t.Errorf("cmd[%d].Name = %q, want %q", idx, got.Name, want.Name)
	}
	assertSlice(t, fmt.Sprintf("cmd[%d].Flags", idx), got.Flags, want.Flags)
	assertSlice(t, fmt.Sprintf("cmd[%d].Args", idx), got.Args, want.Args)
}

func assertSlice(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: got %v (len %d), want %v (len %d)", label, got, len(got), want, len(want))
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s[%d] = %q, want %q", label, i, got[i], want[i])
		}
	}
}

func TestBashDecomposer_SimpleCommand(t *testing.T) {
	decomposer := NewBashDecomposer()

	tests := []struct {
		name     string
		input    string
		wantCmds []SimpleCommand
	}{
		{
			name:  "basic command with flags and args",
			input: "rm -rf /tmp",
			wantCmds: []SimpleCommand{
				{Name: "rm", Flags: []string{"-rf"}, Args: []string{"/tmp"}},
			},
		},
		{
			name:  "command no flags",
			input: "ls /home",
			wantCmds: []SimpleCommand{
				{Name: "ls", Args: []string{"/home"}},
			},
		},
		{
			name:  "command no args",
			input: "pwd",
			wantCmds: []SimpleCommand{
				{Name: "pwd"}},
		},
		{
			name:  "long flags",
			input: "git push --force --no-verify",
			wantCmds: []SimpleCommand{
				{Name: "git", Flags: []string{"--force", "--no-verify"}, Args: []string{"push"}},
			},
		},
		{
			name:  "mixed short and long flags",
			input: "rm -r --no-preserve-root /",
			wantCmds: []SimpleCommand{
				{Name: "rm", Flags: []string{"-r", "--no-preserve-root"}, Args: []string{"/"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, root := parseBash(t, tt.input)
			cc := decomposer.Decompose(root, []byte(tt.input))
			got := cc.AllCommands()
			assertCommands(t, got, tt.wantCmds)
		})
	}
}

func TestBashDecomposer_Pipeline(t *testing.T) {
	decomposer := NewBashDecomposer()

	_, root := parseBash(t, "ls -la | grep foo | wc -l")
	cc := decomposer.Decompose(root, []byte("ls -la | grep foo | wc -l"))

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

func TestBashDecomposer_AndOrChain(t *testing.T) {
	decomposer := NewBashDecomposer()

	_, root := parseBash(t, "cd /tmp && rm -rf * || echo failed")
	cc := decomposer.Decompose(root, []byte("cd /tmp && rm -rf * || echo failed"))

	cmds := cc.AllCommands()
	assertCommands(t, cmds, []SimpleCommand{
		{Name: "cd", Args: []string{"/tmp"}},
		{Name: "rm", Flags: []string{"-rf"}, Args: []string{"*"}},
		{Name: "echo", Args: []string{"failed"}},
	})

	// Verify operators.
	if len(cc.Statements) < 2 {
		t.Fatalf("expected at least 2 statements, got %d", len(cc.Statements))
	}
	if cc.Statements[0].Operator != "&&" {
		t.Errorf("stmt[0].Operator = %q, want %q", cc.Statements[0].Operator, "&&")
	}
	if cc.Statements[1].Operator != "||" {
		t.Errorf("stmt[1].Operator = %q, want %q", cc.Statements[1].Operator, "||")
	}
}

func TestBashDecomposer_Subshell(t *testing.T) {
	decomposer := NewBashDecomposer()

	_, root := parseBash(t, "(cd /tmp && rm -rf *)")
	cc := decomposer.Decompose(root, []byte("(cd /tmp && rm -rf *)"))

	cmds := cc.AllCommands()
	assertCommands(t, cmds, []SimpleCommand{
		{Name: "cd", Args: []string{"/tmp"}},
		{Name: "rm", Flags: []string{"-rf"}, Args: []string{"*"}},
	})
}

func TestBashDecomposer_IfStatement(t *testing.T) {
	decomposer := NewBashDecomposer()

	input := "if test -f file; then rm file; fi"
	_, root := parseBash(t, input)
	cc := decomposer.Decompose(root, []byte(input))

	cmds := cc.AllCommands()
	assertCommands(t, cmds, []SimpleCommand{
		{Name: "test", Flags: []string{"-f"}, Args: []string{"file"}},
		{Name: "rm", Args: []string{"file"}},
	})
}

func TestBashDecomposer_WhileLoop(t *testing.T) {
	decomposer := NewBashDecomposer()

	input := "while true; do echo hello; done"
	_, root := parseBash(t, input)
	cc := decomposer.Decompose(root, []byte(input))

	cmds := cc.AllCommands()
	assertCommands(t, cmds, []SimpleCommand{
		{Name: "true"},
		{Name: "echo", Args: []string{"hello"}},
	})
}

func TestBashDecomposer_ForLoop(t *testing.T) {
	decomposer := NewBashDecomposer()

	input := `for f in *.txt; do rm "$f"; done`
	_, root := parseBash(t, input)
	cc := decomposer.Decompose(root, []byte(input))

	cmds := cc.AllCommands()
	if len(cmds) < 1 {
		t.Fatalf("expected at least 1 command, got %d", len(cmds))
	}
	if cmds[len(cmds)-1].Name != "rm" {
		t.Errorf("expected rm command in for body, got %q", cmds[len(cmds)-1].Name)
	}
}

func TestBashDecomposer_CaseStatement(t *testing.T) {
	decomposer := NewBashDecomposer()

	input := `case $1 in start) echo starting;; stop) echo stopping;; esac`
	_, root := parseBash(t, input)
	cc := decomposer.Decompose(root, []byte(input))

	cmds := cc.AllCommands()
	names := make([]string, len(cmds))
	for i, cmd := range cmds {
		names[i] = cmd.Name
	}
	// Should find at least the echo commands from case branches.
	found := 0
	for _, name := range names {
		if name == "echo" {
			found++
		}
	}
	if found < 2 {
		t.Errorf("expected at least 2 echo commands in case branches, got %d; all commands: %v", found, names)
	}
}

func TestBashDecomposer_CommandSubstitution(t *testing.T) {
	decomposer := NewBashDecomposer()

	input := "echo $(rm -rf /)"
	_, root := parseBash(t, input)
	cc := decomposer.Decompose(root, []byte(input))

	cmds := cc.AllCommands()
	if len(cmds) < 2 {
		t.Fatalf("expected at least 2 commands (echo + inner rm), got %d: %+v", len(cmds), cmds)
	}

	// Main command is echo.
	if cmds[0].Name != "echo" {
		t.Errorf("cmd[0].Name = %q, want %q", cmds[0].Name, "echo")
	}

	// Inner command is rm.
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

func TestBashDecomposer_FunctionDefinition(t *testing.T) {
	decomposer := NewBashDecomposer()

	input := "foo() { echo bar; }"
	_, root := parseBash(t, input)
	cc := decomposer.Decompose(root, []byte(input))

	cmds := cc.AllCommands()
	if len(cmds) != 0 {
		t.Errorf("expected 0 commands (function def skipped), got %d: %+v", len(cmds), cmds)
	}
}

func TestBashDecomposer_Declaration(t *testing.T) {
	decomposer := NewBashDecomposer()

	tests := []struct {
		name     string
		input    string
		wantName string
	}{
		{"export", "export PATH=/usr/bin", "export"},
		{"local", "local foo=bar", "local"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, root := parseBash(t, tt.input)
			cc := decomposer.Decompose(root, []byte(tt.input))
			cmds := cc.AllCommands()
			if len(cmds) != 1 {
				t.Fatalf("expected 1 command, got %d: %+v", len(cmds), cmds)
			}
			if cmds[0].Name != tt.wantName {
				t.Errorf("Name = %q, want %q", cmds[0].Name, tt.wantName)
			}
		})
	}
}

func TestBashDecomposer_PathNormalization(t *testing.T) {
	decomposer := NewBashDecomposer()

	tests := []struct {
		name     string
		input    string
		wantName string
	}{
		{"unix path", "/usr/bin/rm -rf /", "rm"},
		{"deep path", "/usr/local/bin/git status", "git"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, root := parseBash(t, tt.input)
			cc := decomposer.Decompose(root, []byte(tt.input))
			cmds := cc.AllCommands()
			if len(cmds) == 0 {
				t.Fatal("expected at least 1 command")
			}
			if cmds[0].Name != tt.wantName {
				t.Errorf("Name = %q, want %q", cmds[0].Name, tt.wantName)
			}
		})
	}
}

func TestBashDecomposer_ExeStripping(t *testing.T) {
	decomposer := NewBashDecomposer()

	_, root := parseBash(t, "git.exe status")
	cc := decomposer.Decompose(root, []byte("git.exe status"))

	cmds := cc.AllCommands()
	if len(cmds) == 0 {
		t.Fatal("expected at least 1 command")
	}
	if cmds[0].Name != "git" {
		t.Errorf("Name = %q, want %q", cmds[0].Name, "git")
	}
}

func TestBashDecomposer_StdinRedirect(t *testing.T) {
	decomposer := NewBashDecomposer()

	input := "sqlite3 db < evil.sql"
	_, root := parseBash(t, input)
	cc := decomposer.Decompose(root, []byte(input))

	cmds := cc.AllCommands()
	if len(cmds) == 0 {
		t.Fatal("expected at least 1 command")
	}

	cmd := cmds[0]
	if cmd.Name != "sqlite3" {
		t.Errorf("Name = %q, want %q", cmd.Name, "sqlite3")
	}

	// stdin redirect should add "<" flag and the file as arg.
	assertSlice(t, "flags", cmd.Flags, []string{"<"})
	assertSlice(t, "args", cmd.Args, []string{"db", "evil.sql"})
}

func TestBashDecomposer_OutputRedirectTrackedSeparately(t *testing.T) {
	decomposer := NewBashDecomposer()

	input := "echo hello > /dev/null"
	_, root := parseBash(t, input)
	cc := decomposer.Decompose(root, []byte(input))

	cmds := cc.AllCommands()
	if len(cmds) == 0 {
		t.Fatal("expected at least 1 command")
	}

	cmd := cmds[0]
	if cmd.Name != "echo" {
		t.Errorf("Name = %q, want %q", cmd.Name, "echo")
	}
	// Output redirect targets should stay out of Args, but remain available
	// separately for write-sensitive safety rules.
	for _, f := range cmd.Flags {
		if f == ">" || f == "<" {
			t.Errorf("unexpected redirect flag: %q", f)
		}
	}
	for _, a := range cmd.Args {
		if a == "/dev/null" {
			t.Errorf("output redirect target should not appear in args")
		}
	}
	found := false
	for _, a := range cmd.OutputRedirects {
		if a == "/dev/null" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("output redirect target should appear in OutputRedirects: got %v", cmd.OutputRedirects)
	}
}

func TestBashDecomposer_QuotedArguments(t *testing.T) {
	decomposer := NewBashDecomposer()

	input := `rm -rf "/tmp/my dir"`
	_, root := parseBash(t, input)
	cc := decomposer.Decompose(root, []byte(input))

	cmds := cc.AllCommands()
	if len(cmds) == 0 {
		t.Fatal("expected at least 1 command")
	}
	assertCommand(t, 0, cmds[0], SimpleCommand{
		Name:  "rm",
		Flags: []string{"-rf"},
		Args:  []string{"/tmp/my dir"},
	})
}

func TestBashDecomposer_SingleQuotedArg(t *testing.T) {
	decomposer := NewBashDecomposer()

	input := `echo 'hello world'`
	_, root := parseBash(t, input)
	cc := decomposer.Decompose(root, []byte(input))

	cmds := cc.AllCommands()
	if len(cmds) == 0 {
		t.Fatal("expected at least 1 command")
	}
	assertCommand(t, 0, cmds[0], SimpleCommand{
		Name: "echo",
		Args: []string{"hello world"},
	})
}

func TestBashDecomposer_Semicolons(t *testing.T) {
	decomposer := NewBashDecomposer()

	input := "echo a; echo b; echo c"
	_, root := parseBash(t, input)
	cc := decomposer.Decompose(root, []byte(input))

	cmds := cc.AllCommands()
	assertCommands(t, cmds, []SimpleCommand{
		{Name: "echo", Args: []string{"a"}},
		{Name: "echo", Args: []string{"b"}},
		{Name: "echo", Args: []string{"c"}},
	})
}

func TestNormalizeCommandName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/usr/bin/rm", "rm"},
		{"/usr/local/bin/git", "git"},
		{"rm", "rm"},
		{"git.exe", "git"},
		{"git.EXE", "git"},
		{"C:\\tools\\git.exe", "git"},
		{"/usr/bin/git.exe", "git"},
		{"", ""},
		{"./script.sh", "script.sh"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeCommandName(tt.input)
			if got != tt.want {
				t.Errorf("normalizeCommandName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBashDecomposer_Background(t *testing.T) {
	decomposer := NewBashDecomposer()

	input := "sleep 10 &"
	_, root := parseBash(t, input)
	cc := decomposer.Decompose(root, []byte(input))

	cmds := cc.AllCommands()
	if len(cmds) == 0 {
		t.Fatal("expected at least 1 command")
	}
	if cmds[0].Name != "sleep" {
		t.Errorf("Name = %q, want %q", cmds[0].Name, "sleep")
	}
}

func TestBashDecomposer_EmptyInput(t *testing.T) {
	decomposer := NewBashDecomposer()

	_, root := parseBash(t, "")
	cc := decomposer.Decompose(root, []byte(""))

	cmds := cc.AllCommands()
	if len(cmds) != 0 {
		t.Errorf("expected 0 commands for empty input, got %d: %+v", len(cmds), cmds)
	}
}

func TestBashDecomposer_CommentOnly(t *testing.T) {
	decomposer := NewBashDecomposer()

	input := "# this is a comment"
	_, root := parseBash(t, input)
	cc := decomposer.Decompose(root, []byte(input))

	cmds := cc.AllCommands()
	if len(cmds) != 0 {
		t.Errorf("expected 0 commands for comment-only input, got %d: %+v", len(cmds), cmds)
	}
}

func TestBashDecomposer_LastStatementOperatorEmpty(t *testing.T) {
	decomposer := NewBashDecomposer()

	tests := []struct {
		name       string
		input      string
		wantStmts  int
		wantOps    []string // operators on each statement except last
	}{
		{
			name:      "and-or chain",
			input:     "cd /tmp && rm -rf * || echo failed",
			wantStmts: 3,
			wantOps:   []string{"&&", "||"},
		},
		{
			name:      "single and",
			input:     "mkdir dir && cd dir",
			wantStmts: 2,
			wantOps:   []string{"&&"},
		},
		{
			name:      "single command",
			input:     "ls -la",
			wantStmts: 1,
			wantOps:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, root := parseBash(t, tt.input)
			cc := decomposer.Decompose(root, []byte(tt.input))

			if len(cc.Statements) != tt.wantStmts {
				t.Fatalf("expected %d statements, got %d", tt.wantStmts, len(cc.Statements))
			}

			// Verify operators on non-last statements.
			for i, wantOp := range tt.wantOps {
				if cc.Statements[i].Operator != wantOp {
					t.Errorf("stmt[%d].Operator = %q, want %q", i, cc.Statements[i].Operator, wantOp)
				}
			}

			// The invariant: last statement must have empty operator.
			last := cc.Statements[len(cc.Statements)-1]
			if last.Operator != "" {
				t.Errorf("last statement Operator = %q, want empty string", last.Operator)
			}
		})
	}
}

func TestBashDecomposer_NestedCommandSubstitution(t *testing.T) {
	decomposer := NewBashDecomposer()

	input := "echo $(ls $(pwd))"
	_, root := parseBash(t, input)
	cc := decomposer.Decompose(root, []byte(input))

	cmds := cc.AllCommands()
	if len(cmds) < 3 {
		t.Fatalf("expected at least 3 commands (echo, ls, pwd), got %d: %+v", len(cmds), cmds)
	}

	// Verify all three commands are found.
	foundEcho := false
	foundLS := false
	foundPwd := false
	for _, cmd := range cmds {
		switch cmd.Name {
		case "echo":
			foundEcho = true
		case "ls":
			foundLS = true
		case "pwd":
			foundPwd = true
		}
	}
	if !foundEcho {
		t.Error("expected echo command")
	}
	if !foundLS {
		t.Error("expected ls command from outer substitution")
	}
	if !foundPwd {
		t.Error("expected pwd command from nested substitution")
	}
}

func TestBashDecomposer_HeredocRedirect(t *testing.T) {
	decomposer := NewBashDecomposer()

	// Heredoc redirects are processed via decomposeRedirectedStatement.
	// The main command should still be extracted correctly.
	input := "cat <<EOF\nrm -rf /\nEOF"
	_, root := parseBash(t, input)
	cc := decomposer.Decompose(root, []byte(input))

	cmds := cc.AllCommands()
	if len(cmds) == 0 {
		t.Fatal("expected at least 1 command")
	}

	// The main command should be cat.
	if cmds[0].Name != "cat" {
		t.Errorf("cmd[0].Name = %q, want %q", cmds[0].Name, "cat")
	}

	// If inner commands are extracted from the heredoc body, they appear
	// as "INNER:..." in Args. This is a best-effort extraction — heredoc
	// body content extraction depends on resolveText correctly isolating
	// the body text from the heredoc markers.
	// Currently, resolveText returns the full heredoc_redirect node text
	// (including <<EOF markers), so inner command extraction may not work
	// for all heredoc forms. Verify the main command is at least preserved.
}

func TestBashDecomposer_HereString(t *testing.T) {
	decomposer := NewBashDecomposer()

	// Here-strings (<<<) should preserve the main command.
	input := "cat <<<hello"
	_, root := parseBash(t, input)
	cc := decomposer.Decompose(root, []byte(input))

	cmds := cc.AllCommands()
	if len(cmds) == 0 {
		t.Fatal("expected at least 1 command")
	}
	if cmds[0].Name != "cat" {
		t.Errorf("cmd[0].Name = %q, want %q", cmds[0].Name, "cat")
	}
}
