package shellcontext

import (
	"testing"
)

func TestShell_Classify_POSIX(t *testing.T) {
	shell := &POSIXShell{}
	
	// Test line continuation.
	cmd := "rm \\\n-rf /"
	spans := Classify(cmd, shell)
	
	// Expect spans to cover the full string.
	// Bash logic treats line continuation as whitespace/separator if not inside a token.
	// But our classifier currently treats it as part of the scan if not careful.
	// With the new logic, scanWord should skip it.
	
	foundExecuted := false
	for _, s := range spans {
		if s.Kind == SpanExecuted {
			foundExecuted = true
		}
	}
	if !foundExecuted {
		t.Error("expected to find Executed span in POSIX command")
	}
}

func TestShell_Classify_PowerShell(t *testing.T) {
	shell := &PowerShell{}
	
	// Test backtick line continuation.
	cmd := "rm `\n-rf /"
	spans := Classify(cmd, shell)
	
	foundExecuted := false
	for _, s := range spans {
		if s.Kind == SpanExecuted {
			foundExecuted = true
		}
	}
	if !foundExecuted {
		t.Error("expected to find Executed span in PowerShell command")
	}
}

func TestShell_Sanitize_PowerShell(t *testing.T) {
	shell := &PowerShell{}
	
	// PowerShell uses "" for literal quote inside double quotes.
	cmd := `echo "this is a ""destructive"" test"`
	sanitized := Sanitize(cmd, shell)
	
	// The content inside quotes should be masked.
	expected := `echo                                 `
	if sanitized != expected {
		t.Errorf("expected %q, got %q", expected, sanitized)
	}
}

func TestShell_Sanitize_POSIX_DataRegion(t *testing.T) {
	shell := &POSIXShell{}
	
	// git commit -m "..." is a data region.
	cmd := `git commit -m "rm -rf /"`
	sanitized := Sanitize(cmd, shell)
	
	// "rm -rf /" should be masked.
	if sanitized == cmd {
		t.Error("expected sanitization to mask data region")
	}
}

func TestShell_Classify_CmdExe(t *testing.T) {
	shell := &CmdExe{}
	
	// cmd.exe doesn't support single quotes.
	cmd := `echo 'rm -rf /'`
	spans := Classify(cmd, shell)
	
	for _, s := range spans {
		t.Logf("Span: %s %q", s.Kind, cmd[s.Start:s.End])
	}

	// In cmd.exe, ' is just a character, so rm -rf / should be visible as Arguments.
	// We want to make sure it's NOT SpanData (which would be masked).
	foundRM := false
	for _, s := range spans {
		text := cmd[s.Start:s.End]
		if text == "'rm" {
			foundRM = true
			if s.Kind == SpanData {
				t.Error("expected 'rm' NOT to be Data in cmd.exe")
			}
		}
	}
	if !foundRM {
		t.Error("expected to find 'rm' token in cmd.exe output")
	}
}

func TestDetectShellFromCommand(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		wantName string // "" means nil (no detection)
	}{
		{"PowerShell cmdlet Remove-Item", "Remove-Item -Recurse -Force C:/Windows", "powershell"},
		{"PowerShell cmdlet Get-Process", "Get-Process -Name notepad", "powershell"},
		{"PowerShell cmdlet Stop-Service", "Stop-Service -Name wuauserv", "powershell"},
		{"drive letter backslash", `dir C:\Windows\System32`, "powershell"},
		{"drive letter forward slash", "ls C:/Users/foo", "powershell"},
		{"cmd env var", "echo %USERPROFILE%", "cmd"},
		{"cmd env var path", "cd %APPDATA%", "cmd"},
		{"POSIX command", "rm -rf /tmp/foo", ""},
		{"POSIX git", "git status", ""},
		{"empty string", "", ""},
		{"hyphenated non-cmdlet", "my-script --flag", ""},
		{"lowercase verb-noun", "get-process foo", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectShellFromCommand(tt.cmd)
			if tt.wantName == "" {
				if got != nil {
					t.Errorf("DetectShellFromCommand(%q) = %q, want nil", tt.cmd, got.Name())
				}
				return
			}
			if got == nil {
				t.Fatalf("DetectShellFromCommand(%q) = nil, want %q", tt.cmd, tt.wantName)
			}
			if got.Name() != tt.wantName {
				t.Errorf("DetectShellFromCommand(%q).Name() = %q, want %q", tt.cmd, got.Name(), tt.wantName)
			}
		})
	}
}
