package ast

import (
	"testing"
)

// ---------- helpers ----------

func requireUnwrapNonNil(t *testing.T, cc *CompoundCommand) {
	t.Helper()
	if cc == nil {
		t.Fatal("expected non-nil CompoundCommand from Unwrap, got nil")
	}
}

func unwrapBash(t *testing.T, input string) *CompoundCommand {
	t.Helper()
	cc := Parse([]byte(input))
	if cc == nil {
		t.Fatalf("Parse(%q) returned nil", input)
	}
	result := Unwrap(cc, ShellBash)
	requireUnwrapNonNil(t, result)
	return result
}

func requireFirstCommand(t *testing.T, cc *CompoundCommand) SimpleCommand {
	t.Helper()
	cmds := cc.AllCommands()
	if len(cmds) == 0 {
		t.Fatal("expected at least 1 command, got 0")
	}
	return cmds[0]
}

func hasCommandNamed(cmds []SimpleCommand, name string) bool {
	for _, cmd := range cmds {
		if cmd.Name == name {
			return true
		}
	}
	return false
}

// ---------- sudo stripping ----------

func TestUnwrap_SudoStripping(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantName  string
		wantFlags []string
		wantArgs  []string
	}{
		{
			name:      "basic sudo",
			input:     "sudo rm -rf /",
			wantName:  "rm",
			wantFlags: []string{"-rf"},
			wantArgs:  []string{"/"},
		},
		{
			name:      "sudo with -u flag",
			input:     "sudo -u root rm -rf /",
			wantName:  "rm",
			wantFlags: []string{"-rf"},
			wantArgs:  []string{"/"},
		},
		{
			name:      "sudo with --user flag",
			input:     "sudo --user root rm -rf /",
			wantName:  "rm",
			wantFlags: []string{"-rf"},
			wantArgs:  []string{"/"},
		},
		{
			name:      "sudo with -- separator",
			input:     "sudo -- rm -rf /",
			wantName:  "rm",
			wantFlags: []string{"-rf"},
			wantArgs:  []string{"/"},
		},
		{
			name:      "sudo with multiple flags",
			input:     "sudo -E -u www-data rm -rf /tmp/cache",
			wantName:  "rm",
			wantFlags: []string{"-rf"},
			wantArgs:  []string{"/tmp/cache"},
		},
		{
			name:      "sudo with -n flag",
			input:     "sudo -n rm -rf /",
			wantName:  "rm",
			wantFlags: []string{"-rf"},
			wantArgs:  []string{"/"},
		},
		{
			name:      "sudo with combined -nu flag",
			input:     "sudo -nu root rm -rf /",
			wantName:  "rm",
			wantFlags: []string{"-rf"},
			wantArgs:  []string{"/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := unwrapBash(t, tt.input)
			cmd := requireFirstCommand(t, cc)

			if cmd.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", cmd.Name, tt.wantName)
			}
			assertSlice(t, "Flags", cmd.Flags, tt.wantFlags)
			assertSlice(t, "Args", cmd.Args, tt.wantArgs)
		})
	}
}

// ---------- env stripping ----------

func TestUnwrap_EnvStripping(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantName  string
		wantFlags []string
		wantArgs  []string
	}{
		{
			name:      "env with VAR=val",
			input:     "env VAR=val rm -rf /",
			wantName:  "rm",
			wantFlags: []string{"-rf"},
			wantArgs:  []string{"/"},
		},
		{
			name:      "env with -i flag",
			input:     "env -i PATH=/usr/bin rm -rf /",
			wantName:  "rm",
			wantFlags: []string{"-rf"},
			wantArgs:  []string{"/"},
		},
		{
			name:      "env with multiple vars",
			input:     "env FOO=bar BAZ=qux rm -rf /",
			wantName:  "rm",
			wantFlags: []string{"-rf"},
			wantArgs:  []string{"/"},
		},
		{
			name:      "env with -- separator",
			input:     "env -- ls -la",
			wantName:  "ls",
			wantFlags: []string{"-la"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := unwrapBash(t, tt.input)
			cmd := requireFirstCommand(t, cc)

			if cmd.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", cmd.Name, tt.wantName)
			}
			assertSlice(t, "Flags", cmd.Flags, tt.wantFlags)
			if tt.wantArgs != nil {
				assertSlice(t, "Args", cmd.Args, tt.wantArgs)
			}
		})
	}
}

// ---------- command builtin stripping ----------

func TestUnwrap_CommandBuiltin(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantName  string
		wantFlags []string
		wantArgs  []string
	}{
		{
			name:      "basic command builtin",
			input:     "command rm -rf /",
			wantName:  "rm",
			wantFlags: []string{"-rf"},
			wantArgs:  []string{"/"},
		},
		{
			name:     "command with -v flag (lookup mode)",
			input:    "command -v git",
			wantName: "git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := unwrapBash(t, tt.input)
			cmd := requireFirstCommand(t, cc)

			if cmd.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", cmd.Name, tt.wantName)
			}
			if tt.wantFlags != nil {
				assertSlice(t, "Flags", cmd.Flags, tt.wantFlags)
			}
			if tt.wantArgs != nil {
				assertSlice(t, "Args", cmd.Args, tt.wantArgs)
			}
		})
	}
}

// ---------- xargs stripping ----------

func TestUnwrap_Xargs(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantName  string
		wantFlags []string
		wantArgs  []string
	}{
		{
			name:      "basic xargs command",
			input:     "xargs rm -rf /",
			wantName:  "rm",
			wantFlags: []string{"-rf"},
			wantArgs:  []string{"/"},
		},
		{
			name:      "xargs with wrapper flags",
			input:     "xargs -0 -n 1 rm -rf /",
			wantName:  "rm",
			wantFlags: []string{"-rf"},
			wantArgs:  []string{"/"},
		},
		{
			name:      "xargs with explicit separator",
			input:     "xargs -- rm -rf /",
			wantName:  "rm",
			wantFlags: []string{"-rf"},
			wantArgs:  []string{"/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := unwrapBash(t, tt.input)
			cmd := requireFirstCommand(t, cc)

			if cmd.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", cmd.Name, tt.wantName)
			}
			assertSlice(t, "Flags", cmd.Flags, tt.wantFlags)
			assertSlice(t, "Args", cmd.Args, tt.wantArgs)
		})
	}
}

// ---------- bash -c / sh -c extraction ----------

func TestUnwrap_BashC(t *testing.T) {
	cc := unwrapBash(t, `bash -c "rm -rf /"`)
	cmds := cc.AllCommands()

	// The main command is bash; inner commands should include rm.
	if cmds[0].Name != "bash" {
		t.Errorf("main command Name = %q, want %q", cmds[0].Name, "bash")
	}

	// Collect all commands (main + inner from stages).
	var allCmds []SimpleCommand
	for _, stmt := range cc.Statements {
		for _, stage := range stmt.Stages {
			allCmds = append(allCmds, stage.Command)
			allCmds = append(allCmds, stage.Inner...)
		}
	}

	if !hasCommandNamed(allCmds, "rm") {
		names := make([]string, len(allCmds))
		for i, c := range allCmds {
			names[i] = c.Name
		}
		t.Errorf("expected inner command 'rm', got commands: %v", names)
	}
}

func TestUnwrap_ShC(t *testing.T) {
	cc := unwrapBash(t, `sh -c 'ls -la'`)
	cmds := cc.AllCommands()

	if cmds[0].Name != "sh" {
		t.Errorf("main command Name = %q, want %q", cmds[0].Name, "sh")
	}

	var allCmds []SimpleCommand
	for _, stmt := range cc.Statements {
		for _, stage := range stmt.Stages {
			allCmds = append(allCmds, stage.Command)
			allCmds = append(allCmds, stage.Inner...)
		}
	}

	if !hasCommandNamed(allCmds, "ls") {
		names := make([]string, len(allCmds))
		for i, c := range allCmds {
			names[i] = c.Name
		}
		t.Errorf("expected inner command 'ls', got commands: %v", names)
	}
}

func TestUnwrap_BashC_MultipleInnerCommands(t *testing.T) {
	cc := unwrapBash(t, `bash -c "cd /tmp && rm -rf *"`)

	var allCmds []SimpleCommand
	for _, stmt := range cc.Statements {
		for _, stage := range stmt.Stages {
			allCmds = append(allCmds, stage.Command)
			allCmds = append(allCmds, stage.Inner...)
		}
	}

	if !hasCommandNamed(allCmds, "cd") {
		t.Errorf("expected inner command 'cd' from bash -c")
	}
	if !hasCommandNamed(allCmds, "rm") {
		t.Errorf("expected inner command 'rm' from bash -c")
	}
}

// ---------- nested wrappers ----------

func TestUnwrap_NestedSudoBashC(t *testing.T) {
	cc := unwrapBash(t, `sudo bash -c "rm -rf /"`)

	var allCmds []SimpleCommand
	for _, stmt := range cc.Statements {
		for _, stage := range stmt.Stages {
			allCmds = append(allCmds, stage.Command)
			allCmds = append(allCmds, stage.Inner...)
		}
	}

	// sudo should be stripped, leaving bash as the main command.
	if allCmds[0].Name != "bash" {
		t.Errorf("main command Name = %q, want %q (sudo stripped)", allCmds[0].Name, "bash")
	}

	// Inner commands should include rm from bash -c.
	if !hasCommandNamed(allCmds, "rm") {
		names := make([]string, len(allCmds))
		for i, c := range allCmds {
			names[i] = c.Name
		}
		t.Errorf("expected inner command 'rm' from sudo bash -c, got: %v", names)
	}
}

func TestUnwrap_NestedSudoEnv(t *testing.T) {
	cc := unwrapBash(t, "sudo env VAR=val rm -rf /")
	cmd := requireFirstCommand(t, cc)

	if cmd.Name != "rm" {
		t.Errorf("Name = %q, want %q (sudo+env stripped)", cmd.Name, "rm")
	}
	assertSlice(t, "Flags", cmd.Flags, []string{"-rf"})
	assertSlice(t, "Args", cmd.Args, []string{"/"})
}

func TestUnwrap_NestedEnvSudo(t *testing.T) {
	cc := unwrapBash(t, "env PATH=/usr/bin sudo rm -rf /")
	cmd := requireFirstCommand(t, cc)

	if cmd.Name != "rm" {
		t.Errorf("Name = %q, want %q (env+sudo stripped)", cmd.Name, "rm")
	}
}

// ---------- recursive inner unwrap (Bug 10) ----------

func TestUnwrap_InnerCommandsUnwrapped(t *testing.T) {
	// Commands placed in stage.Inner (from command substitutions) must themselves
	// be unwrapped. e.g., the inner sudo rm from $(sudo rm -rf /) should resolve
	// to rm, not sudo.
	cc := unwrapBash(t, "echo $(sudo rm -rf /)")

	var allCmds []SimpleCommand
	for _, stmt := range cc.Statements {
		for _, stage := range stmt.Stages {
			allCmds = append(allCmds, stage.Command)
			allCmds = append(allCmds, stage.Inner...)
		}
	}

	if !hasCommandNamed(allCmds, "rm") {
		names := make([]string, len(allCmds))
		for i, c := range allCmds {
			names[i] = c.Name
		}
		t.Errorf("expected inner command 'rm' (sudo stripped from substitution), got: %v", names)
	}
}

// ---------- no wrapper ----------

func TestUnwrap_NoWrapper(t *testing.T) {
	cc := unwrapBash(t, "rm -rf /")
	cmd := requireFirstCommand(t, cc)

	if cmd.Name != "rm" {
		t.Errorf("Name = %q, want %q", cmd.Name, "rm")
	}
	assertSlice(t, "Flags", cmd.Flags, []string{"-rf"})
	assertSlice(t, "Args", cmd.Args, []string{"/"})
}

func TestUnwrap_PlainCommand(t *testing.T) {
	cc := unwrapBash(t, "git status")
	cmd := requireFirstCommand(t, cc)

	if cmd.Name != "git" {
		t.Errorf("Name = %q, want %q", cmd.Name, "git")
	}
	assertSlice(t, "Args", cmd.Args, []string{"status"})
}

// ---------- nil input ----------

func TestUnwrap_NilInput(t *testing.T) {
	result := Unwrap(nil, ShellBash)
	if result != nil {
		t.Errorf("expected nil output for nil input, got %+v", result)
	}
}

// ---------- backslash alias bypass ----------

func TestUnwrap_BackslashAliasBypass(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
	}{
		{
			name:     "backslash rm",
			input:    `\rm -rf /`,
			wantName: "rm",
		},
		{
			name:     "backslash ls",
			input:    `\ls -la`,
			wantName: "ls",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := unwrapBash(t, tt.input)
			cmd := requireFirstCommand(t, cc)
			if cmd.Name != tt.wantName {
				t.Errorf("Name = %q, want %q (backslash stripped)", cmd.Name, tt.wantName)
			}
		})
	}
}

// ---------- pipeline through unwrap ----------

func TestUnwrap_Pipeline(t *testing.T) {
	cc := unwrapBash(t, "sudo ls -la | grep foo")

	if len(cc.Statements) == 0 {
		t.Fatal("expected at least 1 statement")
	}

	// After unwrap, the first stage should have sudo stripped.
	firstCmd := cc.Statements[0].Stages[0].Command
	if firstCmd.Name != "ls" {
		t.Errorf("first pipeline stage Name = %q, want %q (sudo stripped)", firstCmd.Name, "ls")
	}
}

// ---------- empty CompoundCommand ----------

func TestUnwrap_EmptyCompoundCommand(t *testing.T) {
	cc := &CompoundCommand{}
	result := Unwrap(cc, ShellBash)
	requireUnwrapNonNil(t, result)
	cmds := result.AllCommands()
	if len(cmds) != 0 {
		t.Errorf("expected 0 commands for empty CompoundCommand, got %d", len(cmds))
	}
}

// ---------- shellSplit tests ----------

func TestShellSplit(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple tokens",
			input: "rm -rf /",
			want:  []string{"rm", "-rf", "/"},
		},
		{
			name:  "double quoted token",
			input: `echo "hello world"`,
			want:  []string{"echo", `"hello world"`},
		},
		{
			name:  "single quoted token",
			input: "echo 'hello world'",
			want:  []string{"echo", "'hello world'"},
		},
		{
			name:  "mixed quotes",
			input: `echo "hello" 'world'`,
			want:  []string{"echo", `"hello"`, "'world'"},
		},
		{
			name:  "escaped space",
			input: `echo hello\ world`,
			want:  []string{"echo", `hello\ world`},
		},
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name:  "whitespace only",
			input: "   \t  ",
			want:  nil,
		},
		{
			name:  "tabs between tokens",
			input: "rm\t-rf\t/",
			want:  []string{"rm", "-rf", "/"},
		},
		{
			name:  "multiple spaces between tokens",
			input: "rm   -rf   /",
			want:  []string{"rm", "-rf", "/"},
		},
		{
			name:  "quoted string with embedded backslash",
			input: `echo "hello\"world"`,
			want:  []string{"echo", `"hello\"world"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shellSplit(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("shellSplit(%q): got %v (len %d), want %v (len %d)", tt.input, got, len(got), tt.want, len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("shellSplit(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------- unquoteSimple tests ----------

func TestUnquoteSimple(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "double quoted",
			input: `"hello world"`,
			want:  "hello world",
		},
		{
			name:  "single quoted",
			input: "'hello world'",
			want:  "hello world",
		},
		{
			name:  "no quotes",
			input: "hello",
			want:  "hello",
		},
		{
			name:  "mismatched quotes",
			input: `"hello'`,
			want:  `"hello'`,
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "single char",
			input: "a",
			want:  "a",
		},
		{
			name:  "empty double quotes",
			input: `""`,
			want:  "",
		},
		{
			name:  "empty single quotes",
			input: "''",
			want:  "",
		},
		{
			name:  "nested quotes preserved",
			input: `"hello 'world'"`,
			want:  "hello 'world'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unquoteSimple(tt.input)
			if got != tt.want {
				t.Errorf("unquoteSimple(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------- timeout stripping ----------

func TestUnwrap_TimeoutStripping(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantName  string
		wantFlags []string
		wantArgs  []string
	}{
		{
			name:      "timeout with duration",
			input:     "timeout 30 rm -rf /",
			wantName:  "rm",
			wantFlags: []string{"-rf"},
			wantArgs:  []string{"/"},
		},
		{
			name:      "timeout with duration suffix",
			input:     "timeout 10s rm -rf /tmp",
			wantName:  "rm",
			wantFlags: []string{"-rf"},
			wantArgs:  []string{"/tmp"},
		},
		{
			name:      "timeout with -k flag and duration",
			input:     "timeout -k 5 30 rm -rf /",
			wantName:  "rm",
			wantFlags: []string{"-rf"},
			wantArgs:  []string{"/"},
		},
		{
			name:      "timeout with --signal= and duration",
			input:     "timeout --signal=KILL 60 ls -la",
			wantName:  "ls",
			wantFlags: []string{"-la"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := unwrapBash(t, tt.input)
			cmd := requireFirstCommand(t, cc)

			if cmd.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", cmd.Name, tt.wantName)
			}
			if tt.wantFlags != nil {
				assertSlice(t, "Flags", cmd.Flags, tt.wantFlags)
			}
			if tt.wantArgs != nil {
				assertSlice(t, "Args", cmd.Args, tt.wantArgs)
			}
		})
	}
}

// ---------- strace/ltrace stripping ----------

func TestUnwrap_StraceStripping(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantName  string
		wantFlags []string
		wantArgs  []string
	}{
		{
			name:      "basic strace",
			input:     "strace rm -rf /",
			wantName:  "rm",
			wantFlags: []string{"-rf"},
			wantArgs:  []string{"/"},
		},
		{
			name:      "strace with flags",
			input:     "strace -f -e trace=open ls -la",
			wantName:  "ls",
			wantFlags: []string{"-la"},
		},
		{
			name:      "basic ltrace",
			input:     "ltrace rm -rf /",
			wantName:  "rm",
			wantFlags: []string{"-rf"},
			wantArgs:  []string{"/"},
		},
		{
			name:     "ltrace with -c flag",
			input:    "ltrace -c ls -la",
			wantName: "ls",
			wantFlags: []string{"-la"},
		},
		{
			name:      "strace with -e flag consuming argument",
			input:     "strace -e open rm -rf /",
			wantName:  "rm",
			wantFlags: []string{"-rf"},
			wantArgs:  []string{"/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := unwrapBash(t, tt.input)
			cmd := requireFirstCommand(t, cc)

			if cmd.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", cmd.Name, tt.wantName)
			}
			if tt.wantFlags != nil {
				assertSlice(t, "Flags", cmd.Flags, tt.wantFlags)
			}
			if tt.wantArgs != nil {
				assertSlice(t, "Args", cmd.Args, tt.wantArgs)
			}
		})
	}
}

// ---------- interpreter inline code extraction ----------

func TestUnwrap_InterpreterExtraction(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantMainName string
		wantInner    []string
	}{
		{
			name:         "python -c with os.system",
			input:        `python -c "import os; os.system('rm -rf /')"`,
			wantMainName: "python",
			wantInner:    []string{"rm"},
		},
		{
			name:         "python3 -c with subprocess",
			input:        `python3 -c "import subprocess; subprocess.run('git reset --hard', shell=True)"`,
			wantMainName: "python3",
			wantInner:    []string{"git"},
		},
		{
			name:         "ruby -e with system call",
			input:        `ruby -e "system('rm -rf /')"`,
			wantMainName: "ruby",
			wantInner:    []string{"rm"},
		},
		{
			name:         "node -e with exec",
			input:        `node -e "require('child_process').exec('rm -rf /')"`,
			wantMainName: "node",
			wantInner:    []string{"rm"},
		},
		{
			name:         "perl -e with system",
			input:        `perl -e "system('rm -rf /')"`,
			wantMainName: "perl",
			wantInner:    []string{"rm"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := unwrapBash(t, tt.input)
			cmd := requireFirstCommand(t, cc)

			if cmd.Name != tt.wantMainName {
				t.Errorf("Name = %q, want %q", cmd.Name, tt.wantMainName)
			}

			if len(tt.wantInner) > 0 {
				var innerCmds []SimpleCommand
				for _, stmt := range cc.Statements {
					for _, stage := range stmt.Stages {
						innerCmds = append(innerCmds, stage.Inner...)
					}
				}

				for _, wantName := range tt.wantInner {
					if !hasCommandNamed(innerCmds, wantName) {
						names := make([]string, len(innerCmds))
						for i, c := range innerCmds {
							names[i] = c.Name
						}
						t.Errorf("expected inner command %q, got inner commands: %v", wantName, names)
					}
				}
			}
		})
	}
}

// ---------- table-driven unwrap end-to-end ----------

func TestUnwrap_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantMainName  string
		wantMainFlags []string
		wantMainArgs  []string
		wantInner     []string // expected inner command names
	}{
		{
			name:          "sudo rm -rf /",
			input:         "sudo rm -rf /",
			wantMainName:  "rm",
			wantMainFlags: []string{"-rf"},
			wantMainArgs:  []string{"/"},
		},
		{
			name:          "sudo -u root rm -rf /",
			input:         "sudo -u root rm -rf /",
			wantMainName:  "rm",
			wantMainFlags: []string{"-rf"},
			wantMainArgs:  []string{"/"},
		},
		{
			name:          "sudo -- rm -rf /",
			input:         "sudo -- rm -rf /",
			wantMainName:  "rm",
			wantMainFlags: []string{"-rf"},
			wantMainArgs:  []string{"/"},
		},
		{
			name:          "env VAR=val rm -rf /",
			input:         "env VAR=val rm -rf /",
			wantMainName:  "rm",
			wantMainFlags: []string{"-rf"},
			wantMainArgs:  []string{"/"},
		},
		{
			name:          "env -i PATH=/usr/bin rm -rf /",
			input:         "env -i PATH=/usr/bin rm -rf /",
			wantMainName:  "rm",
			wantMainFlags: []string{"-rf"},
			wantMainArgs:  []string{"/"},
		},
		{
			name:          "command rm -rf /",
			input:         "command rm -rf /",
			wantMainName:  "rm",
			wantMainFlags: []string{"-rf"},
			wantMainArgs:  []string{"/"},
		},
		{
			name:         "bash -c extraction",
			input:        `bash -c "rm -rf /"`,
			wantMainName: "bash",
			wantInner:    []string{"rm"},
		},
		{
			name:         "sh -c extraction",
			input:        `sh -c 'ls -la'`,
			wantMainName: "sh",
			wantInner:    []string{"ls"},
		},
		{
			name:         "sudo bash -c nested",
			input:        `sudo bash -c "rm -rf /"`,
			wantMainName: "bash",
			wantInner:    []string{"rm"},
		},
		{
			name:          "sudo env VAR=val rm -rf /",
			input:         "sudo env VAR=val rm -rf /",
			wantMainName:  "rm",
			wantMainFlags: []string{"-rf"},
			wantMainArgs:  []string{"/"},
		},
		{
			name:          "no wrapper",
			input:         "rm -rf /",
			wantMainName:  "rm",
			wantMainFlags: []string{"-rf"},
			wantMainArgs:  []string{"/"},
		},
		{
			name:          "timeout -- rm -rf /",
			input:         "timeout -- rm -rf /",
			wantMainName:  "rm",
			wantMainFlags: []string{"-rf"},
			wantMainArgs:  []string{"/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := unwrapBash(t, tt.input)
			cmd := requireFirstCommand(t, cc)

			if cmd.Name != tt.wantMainName {
				t.Errorf("Name = %q, want %q", cmd.Name, tt.wantMainName)
			}
			if tt.wantMainFlags != nil {
				assertSlice(t, "Flags", cmd.Flags, tt.wantMainFlags)
			}
			if tt.wantMainArgs != nil {
				assertSlice(t, "Args", cmd.Args, tt.wantMainArgs)
			}

			if len(tt.wantInner) > 0 {
				// Collect all inner commands from all stages.
				var innerCmds []SimpleCommand
				for _, stmt := range cc.Statements {
					for _, stage := range stmt.Stages {
						innerCmds = append(innerCmds, stage.Inner...)
					}
				}

				for _, wantName := range tt.wantInner {
					if !hasCommandNamed(innerCmds, wantName) {
						names := make([]string, len(innerCmds))
						for i, c := range innerCmds {
							names[i] = c.Name
						}
						t.Errorf("expected inner command %q, got inner commands: %v", wantName, names)
					}
				}
			}
		})
	}
}
