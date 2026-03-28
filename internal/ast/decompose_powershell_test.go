package ast

import (
	"testing"
)

func TestPSDecomposer_SimpleCmdlet(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCmds []SimpleCommand
	}{
		{
			name:  "cmdlet with flags and arg",
			input: "Remove-Item -Recurse -Force /tmp",
			wantCmds: []SimpleCommand{
				{Name: "remove-item", Flags: []string{"-recurse", "-force"}, Args: []string{"/tmp"}},
			},
		},
		{
			name:  "cmdlet no flags",
			input: "Get-Process",
			wantCmds: []SimpleCommand{
				{Name: "get-process"},
			},
		},
		{
			name:  "cmdlet with named parameter",
			input: "Stop-Service -Name svc1",
			wantCmds: []SimpleCommand{
				{Name: "stop-service", Flags: []string{"-name"}, Args: []string{"svc1"}},
			},
		},
		{
			name:  "cmdlet with string argument",
			input: `Write-Host "hello world"`,
			wantCmds: []SimpleCommand{
				{Name: "write-host", Args: []string{"hello world"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := ParseShell([]byte(tt.input), ShellPowerShell)
			if cc == nil {
				t.Fatal("ParseShell returned nil")
			}
			got := cc.AllCommands()
			assertCommands(t, got, tt.wantCmds)
		})
	}
}

func TestPSDecomposer_Pipeline(t *testing.T) {
	cc := ParseShell([]byte("Get-Process | Stop-Process -Force"), ShellPowerShell)
	if cc == nil {
		t.Fatal("ParseShell returned nil")
		return
	}

	if len(cc.Statements) != 1 {
		t.Fatalf("expected 1 statement (pipeline), got %d", len(cc.Statements))
	}
	stmt := cc.Statements[0]
	if len(stmt.Stages) != 2 {
		t.Fatalf("expected 2 pipeline stages, got %d", len(stmt.Stages))
	}

	assertCommand(t, 0, stmt.Stages[0].Command, SimpleCommand{Name: "get-process"})
	assertCommand(t, 1, stmt.Stages[1].Command, SimpleCommand{Name: "stop-process", Flags: []string{"-force"}})
}

func TestPSDecomposer_Semicolons(t *testing.T) {
	cc := ParseShell([]byte("Get-Process; Stop-Service -Name svc1"), ShellPowerShell)
	if cc == nil {
		t.Fatal("ParseShell returned nil")
	}

	cmds := cc.AllCommands()
	assertCommands(t, cmds, []SimpleCommand{
		{Name: "get-process"},
		{Name: "stop-service", Flags: []string{"-name"}, Args: []string{"svc1"}},
	})
}

func TestPSDecomposer_IfStatement(t *testing.T) {
	input := `if ($true) { Write-Host "hello" }`
	cc := ParseShell([]byte(input), ShellPowerShell)
	if cc == nil {
		t.Fatal("ParseShell returned nil")
	}

	cmds := cc.AllCommands()
	found := false
	for _, cmd := range cmds {
		if cmd.Name == "write-host" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected write-host command in if body, got: %+v", cmds)
	}
}

func TestPSDecomposer_ForeachStatement(t *testing.T) {
	input := "foreach ($f in Get-ChildItem) { Remove-Item $f }"
	cc := ParseShell([]byte(input), ShellPowerShell)
	if cc == nil {
		t.Fatal("ParseShell returned nil")
	}

	cmds := cc.AllCommands()
	foundRemove := false
	foundGet := false
	for _, cmd := range cmds {
		if cmd.Name == "remove-item" {
			foundRemove = true
		}
		if cmd.Name == "get-childitem" {
			foundGet = true
		}
	}
	if !foundRemove {
		t.Errorf("expected remove-item in foreach body, got: %+v", cmds)
	}
	if !foundGet {
		t.Errorf("expected get-childitem in foreach iteration, got: %+v", cmds)
	}
}

func TestPSDecomposer_CaseNormalization(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
	}{
		{"Remove-Item /tmp", "remove-item"},
		{"REMOVE-ITEM /tmp", "remove-item"},
		{"remove-item /tmp", "remove-item"},
		{"Get-Process", "get-process"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cc := ParseShell([]byte(tt.input), ShellPowerShell)
			if cc == nil {
				t.Fatal("ParseShell returned nil")
			}
			cmds := cc.AllCommands()
			if len(cmds) == 0 {
				t.Fatal("no commands")
			}
			if cmds[0].Name != tt.wantName {
				t.Errorf("Name = %q, want %q", cmds[0].Name, tt.wantName)
			}
		})
	}
}

func TestPSDecomposer_FlagNormalization(t *testing.T) {
	cc := ParseShell([]byte("Remove-Item -Recurse -Force /tmp"), ShellPowerShell)
	if cc == nil {
		t.Fatal("ParseShell returned nil")
	}
	cmds := cc.AllCommands()
	if len(cmds) == 0 {
		t.Fatal("no commands")
	}
	// Flags should be lowercased.
	for _, f := range cmds[0].Flags {
		if f != "-recurse" && f != "-force" {
			t.Errorf("unexpected flag %q (expected lowercase)", f)
		}
	}
}

func TestPSDecomposer_FlagAbbreviationExpansion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantFlag string
	}{
		{
			name:     "Rec expands to recurse",
			input:    "Remove-Item -Rec /tmp",
			wantFlag: "-recurse",
		},
		{
			name:     "Fo expands to force",
			input:    "Remove-Item -Fo /tmp",
			wantFlag: "-force",
		},
		{
			name:     "full flag stays same",
			input:    "Remove-Item -Recurse /tmp",
			wantFlag: "-recurse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := ParseShell([]byte(tt.input), ShellPowerShell)
			if cc == nil {
				t.Fatal("ParseShell returned nil")
			}
			cmds := cc.AllCommands()
			if len(cmds) == 0 {
				t.Fatal("no commands")
			}
			found := false
			for _, f := range cmds[0].Flags {
				if f == tt.wantFlag {
					found = true
				}
			}
			if !found {
				t.Errorf("expected flag %q, got flags: %v", tt.wantFlag, cmds[0].Flags)
			}
		})
	}
}

func TestPSDecomposer_FunctionDefinition(t *testing.T) {
	// Functions should be skipped (not yet executed).
	input := "function Foo { Remove-Item /tmp }"
	cc := ParseShell([]byte(input), ShellPowerShell)
	if cc == nil {
		t.Fatal("ParseShell returned nil")
	}
	cmds := cc.AllCommands()
	if len(cmds) != 0 {
		t.Errorf("expected 0 commands (function def skipped), got %d: %+v", len(cmds), cmds)
	}
}

func TestPSDecomposer_Assignment(t *testing.T) {
	input := "$x = Get-Process"
	cc := ParseShell([]byte(input), ShellPowerShell)
	if cc == nil {
		t.Fatal("ParseShell returned nil")
	}
	cmds := cc.AllCommands()
	found := false
	for _, cmd := range cmds {
		if cmd.Name == "get-process" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected get-process in assignment RHS, got: %+v", cmds)
	}
}

func TestPSDecomposer_EmptyInput(t *testing.T) {
	cc := ParseShell([]byte(""), ShellPowerShell)
	if cc == nil {
		t.Fatal("expected empty CompoundCommand, got nil")
	}
	if len(cc.AllCommands()) != 0 {
		t.Errorf("expected 0 commands, got %d", len(cc.AllCommands()))
	}
}

// ---------- PowerShell unwrap tests ----------

func TestPSUnwrap_PowershellCommand(t *testing.T) {
	input := `powershell -Command "Remove-Item -Recurse /tmp"`
	cc := ParseShell([]byte(input), ShellPowerShell)
	if cc == nil {
		t.Fatal("ParseShell returned nil")
	}
	unwrapped := Unwrap(cc, ShellPowerShell)
	cmds := unwrapped.AllCommands()

	// Main command is powershell; inner should have remove-item.
	foundPS := false
	foundRemove := false
	for _, cmd := range cmds {
		if cmd.Name == "powershell" {
			foundPS = true
		}
		if cmd.Name == "remove-item" {
			foundRemove = true
		}
	}
	if !foundPS {
		t.Error("expected powershell in commands")
	}
	if !foundRemove {
		t.Errorf("expected remove-item in inner commands, got: %+v", cmds)
	}
}

func TestPSUnwrap_InvokeExpression(t *testing.T) {
	input := `Invoke-Expression "rm -rf /"`
	cc := ParseShell([]byte(input), ShellPowerShell)
	if cc == nil {
		t.Fatal("ParseShell returned nil")
	}
	unwrapped := Unwrap(cc, ShellPowerShell)
	cmds := unwrapped.AllCommands()

	foundRM := false
	for _, cmd := range cmds {
		if cmd.Name == "rm" {
			foundRM = true
		}
	}
	if !foundRM {
		t.Errorf("expected rm in inner commands from Invoke-Expression, got: %+v", cmds)
	}
}

func TestPSDecomposer_SubexpressionInArg(t *testing.T) {
	// PowerShell $() subexpressions in argument position are currently
	// resolved to their text content as plain Args, not extracted as
	// inner commands. This test documents the current behavior.
	input := `Write-Host $(Get-Location)`
	cc := ParseShell([]byte(input), ShellPowerShell)
	if cc == nil {
		t.Fatal("ParseShell returned nil")
	}

	cmds := cc.AllCommands()
	if len(cmds) == 0 {
		t.Fatal("expected at least 1 command")
	}
	if cmds[0].Name != "write-host" {
		t.Errorf("expected write-host command, got %q", cmds[0].Name)
	}
	// The subexpression content appears as a plain arg.
	foundArg := false
	for _, arg := range cmds[0].Args {
		if arg == "Get-Location" {
			foundArg = true
		}
	}
	if !foundArg {
		t.Errorf("expected Get-Location in Args from $() subexpression, got: %v", cmds[0].Args)
	}
}

func TestPSDecomposer_PipelineSubexpression(t *testing.T) {
	// PowerShell pipeline with destructive inner commands through Unwrap.
	input := `Get-ChildItem | Remove-Item -Recurse -Force`
	cc := ParseShell([]byte(input), ShellPowerShell)
	if cc == nil {
		t.Fatal("ParseShell returned nil")
	}

	cmds := cc.AllCommands()
	foundGetChild := false
	foundRemoveItem := false
	for _, cmd := range cmds {
		if cmd.Name == "get-childitem" {
			foundGetChild = true
		}
		if cmd.Name == "remove-item" {
			foundRemoveItem = true
		}
	}
	if !foundGetChild {
		t.Errorf("expected get-childitem in pipeline, got: %+v", cmds)
	}
	if !foundRemoveItem {
		t.Errorf("expected remove-item in pipeline, got: %+v", cmds)
	}
}

func TestExpandPSFlag(t *testing.T) {
	tests := []struct {
		cmdName string
		flag    string
		want    string
	}{
		{"remove-item", "-rec", "-recurse"},
		{"remove-item", "-fo", "-force"},
		{"remove-item", "-recurse", "-recurse"},
		{"remove-item", "-force", "-force"},
		{"stop-process", "-fo", "-force"},
		{"stop-process", "-na", "-name"},
		{"unknown-cmdlet", "-fo", "-fo"}, // No expansion for unknown cmdlets.
		{"remove-item", "-f", "-f"},       // Ambiguous: -force, -filter → no expansion.
	}

	for _, tt := range tests {
		t.Run(tt.cmdName+"/"+tt.flag, func(t *testing.T) {
			got := expandPSFlag(tt.cmdName, tt.flag)
			if got != tt.want {
				t.Errorf("expandPSFlag(%q, %q) = %q, want %q", tt.cmdName, tt.flag, got, tt.want)
			}
		})
	}
}
