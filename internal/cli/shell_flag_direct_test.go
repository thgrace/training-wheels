package cli

import (
	"strings"
	"testing"
)

func TestResolveShellFlag(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantName  string
		wantNil   bool
		wantError string
	}{
		{
			name:    "empty",
			input:   "",
			wantNil: true,
		},
		{
			name:     "posix",
			input:    "posix",
			wantName: "posix",
		},
		{
			name:     "powershell",
			input:    "powershell",
			wantName: "powershell",
		},
		{
			name:     "cmd",
			input:    "cmd",
			wantName: "cmd",
		},
		{
			name:     "alias",
			input:    "pwsh",
			wantName: "powershell",
		},
		{
			name:     "bash",
			input:    "bash",
			wantName: "posix",
		},
		{
			name:     "zsh",
			input:    "zsh",
			wantName: "posix",
		},
		{
			name:     "sh",
			input:    "sh",
			wantName: "posix",
		},
		{
			name:      "invalid",
			input:     "nonsense",
			wantError: `invalid --shell value "nonsense": must be bash, cmd, posix, powershell, pwsh, sh, or zsh`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveShellFlag(tt.input)
			if tt.wantError != "" {
				if err == nil {
					t.Fatal("resolveShellFlag() error = nil, want error")
				}
				if !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("error = %q, want %q", err.Error(), tt.wantError)
				}
				if got != nil {
					t.Fatalf("resolveShellFlag() shell = %v, want nil", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("resolveShellFlag() error = %v", err)
			}
			if tt.wantNil {
				if got != nil {
					t.Fatalf("resolveShellFlag() shell = %T, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("resolveShellFlag() shell = nil, want shell")
			}
			if got.Name() != tt.wantName {
				t.Fatalf("resolveShellFlag() shell name = %q, want %q", got.Name(), tt.wantName)
			}
		})
	}
}
