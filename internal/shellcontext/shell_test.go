package shellcontext

import (
	"testing"
)

func TestParseShell(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
		wantOK   bool
	}{
		{name: "posix", input: "posix", wantName: "posix", wantOK: true},
		{name: "bash alias", input: "bash", wantName: "posix", wantOK: true},
		{name: "powershell", input: "powershell", wantName: "powershell", wantOK: true},
		{name: "pwsh alias", input: "pwsh", wantName: "powershell", wantOK: true},
		{name: "cmd", input: "cmd", wantName: "cmd", wantOK: true},
		{name: "trimmed", input: "  zsh  ", wantName: "posix", wantOK: true},
		{name: "unknown", input: "nonsense", wantOK: false},
		{name: "empty", input: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseShell(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("ParseShell(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if !tt.wantOK {
				if got != nil {
					t.Fatalf("ParseShell(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("ParseShell(%q) = nil, want %q", tt.input, tt.wantName)
			}
			if got.Name() != tt.wantName {
				t.Fatalf("ParseShell(%q).Name() = %q, want %q", tt.input, got.Name(), tt.wantName)
			}
		})
	}
}

func TestFromNameFallsBackToDefaultShell(t *testing.T) {
	got := FromName("nonsense")
	if got == nil {
		t.Fatal("FromName returned nil")
	}
	if got.Name() != DefaultShell().Name() {
		t.Fatalf("FromName(%q).Name() = %q, want default shell %q", "nonsense", got.Name(), DefaultShell().Name())
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
