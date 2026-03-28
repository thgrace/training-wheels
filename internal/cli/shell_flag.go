package cli

import (
	"fmt"

	"github.com/thgrace/training-wheels/internal/shellcontext"
)

const shellFlagUsage = "Shell context: bash, cmd, posix, powershell, pwsh, sh, or zsh"

func resolveShellFlag(value string) (shellcontext.Shell, error) {
	if value == "" {
		return nil, nil
	}

	shell, ok := shellcontext.ParseShell(value)
	if !ok {
		return nil, fmt.Errorf("invalid --shell value %q: must be bash, cmd, posix, powershell, pwsh, sh, or zsh", value)
	}

	return shell, nil
}
