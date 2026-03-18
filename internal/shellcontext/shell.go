package shellcontext

import (
	"runtime"
	"strings"
)

// Shell defines the syntactic rules for a specific shell environment.
type Shell interface {
	Name() string
	// IsEscape returns true if the character at index i is an escape character.
	IsEscape(cmd string, i int) bool
	// IsLineContinuation returns true if the character at index i is a line continuation.
	IsLineContinuation(cmd string, i int) bool
	// QuoteRules returns the quoting behavior for this shell.
	QuoteRules() QuoteRules
}

// QuoteRules describes how a shell handles single and double quotes.
type QuoteRules struct {
	SupportsSingle bool
	SupportsDouble bool
	// EscapeInsideDouble is the character used to escape inside double quotes (e.g. \ in bash).
	// 0 means no escape character.
	EscapeInsideDouble byte
	// DoubleQuoteEscapesDouble means "" results in a literal " inside double quotes.
	DoubleQuoteEscapesDouble bool
}

// POSIXShell implements standard Bash/Zsh behavior.
type POSIXShell struct{}

func (s *POSIXShell) Name() string { return "posix" }

func (s *POSIXShell) IsEscape(cmd string, i int) bool {
	return cmd[i] == '\\'
}

func (s *POSIXShell) IsLineContinuation(cmd string, i int) bool {
	if cmd[i] != '\\' {
		return false
	}
	// Check if the next non-whitespace character is a newline.
	for j := i + 1; j < len(cmd); j++ {
		if cmd[j] == '\n' {
			return true
		}
		if cmd[j] != ' ' && cmd[j] != '\t' && cmd[j] != '\r' {
			return false
		}
	}
	return false
}

func (s *POSIXShell) QuoteRules() QuoteRules {
	return QuoteRules{
		SupportsSingle:     true,
		SupportsDouble:     true,
		EscapeInsideDouble: '\\',
	}
}

// PowerShell implements Windows PowerShell / pwsh behavior.
type PowerShell struct{}

func (s *PowerShell) Name() string { return "powershell" }

func (s *PowerShell) IsEscape(cmd string, i int) bool {
	return cmd[i] == '`'
}

func (s *PowerShell) IsLineContinuation(cmd string, i int) bool {
	return cmd[i] == '`' // Simplified: pwsh usually expects newline immediately after `
}

func (s *PowerShell) QuoteRules() QuoteRules {
	return QuoteRules{
		SupportsSingle:           true,
		SupportsDouble:           true,
		EscapeInsideDouble:       '`',
		DoubleQuoteEscapesDouble: true, // "" is a literal " in double quotes
	}
}

// CmdExe implements Windows cmd.exe behavior.
type CmdExe struct{}

func (s *CmdExe) Name() string { return "cmd" }

func (s *CmdExe) IsEscape(cmd string, i int) bool {
	return cmd[i] == '^'
}

func (s *CmdExe) IsLineContinuation(cmd string, i int) bool {
	return cmd[i] == '^'
}

func (s *CmdExe) QuoteRules() QuoteRules {
	return QuoteRules{
		SupportsSingle:     false, // cmd.exe doesn't treat ' as a quote
		SupportsDouble:     true,
		EscapeInsideDouble: 0, // cmd.exe is... weird. quotes mostly just toggle state.
	}
}

// DefaultShell returns the default shell for the current platform.
func DefaultShell() Shell {
	if runtime.GOOS == "windows" {
		return &PowerShell{}
	}
	return &POSIXShell{}
}

// FromName returns a Shell implementation by name.
func FromName(name string) Shell {
	switch strings.ToLower(name) {
	case "powershell", "pwsh":
		return &PowerShell{}
	case "cmd":
		return &CmdExe{}
	case "bash", "zsh", "sh", "posix":
		return &POSIXShell{}
	default:
		return DefaultShell()
	}
}

// DetectShellFromCommand attempts to detect the shell type from command content.
// It checks for Windows-specific indicators like PowerShell cmdlet patterns
// (Verb-Noun), drive letter paths (C:\...), and cmd.exe environment variable
// syntax (%VAR%). Returns nil if no Windows indicators are found, allowing the
// caller to fall back to DefaultShell().
func DetectShellFromCommand(cmd string) Shell {
	// Check for PowerShell cmdlet pattern: Verb-Noun (e.g., Get-Process, Remove-Item).
	// PowerShell cmdlets use approved verbs followed by a hyphen and a noun.
	if hasPowerShellCmdlet(cmd) {
		return &PowerShell{}
	}

	// Check for Windows drive letter path: C:\ or D:/
	if hasDriveLetterPath(cmd) {
		return &PowerShell{}
	}

	// Check for cmd.exe-style environment variable: %VARIABLE%
	if hasCmdEnvVar(cmd) {
		return &CmdExe{}
	}

	return nil
}

// hasPowerShellCmdlet checks for PowerShell Verb-Noun cmdlet patterns.
// Matches patterns like Get-Process, Remove-Item, Stop-Service, etc.
func hasPowerShellCmdlet(cmd string) bool {
	// Look for word boundaries around Verb-Noun patterns.
	// Common PowerShell verbs that indicate cmdlet usage.
	n := len(cmd)
	for i := 0; i < n; i++ {
		// Must be at a word boundary (start of string or after non-word char).
		if i > 0 && isWordChar(cmd[i-1]) {
			continue
		}
		// Look for an uppercase letter starting a verb.
		if cmd[i] < 'A' || cmd[i] > 'Z' {
			continue
		}
		// Scan forward for a hyphen within the token.
		j := i + 1
		for j < n && cmd[j] != ' ' && cmd[j] != '\t' && cmd[j] != ';' && cmd[j] != '|' && cmd[j] != '\n' {
			if cmd[j] == '-' && j > i+1 && j+1 < n && isAlpha(cmd[j+1]) {
				// Found Verb-Noun pattern: uppercase start, hyphen, then more letters.
				return true
			}
			j++
		}
	}
	return false
}

// hasDriveLetterPath checks for Windows drive letter paths like C:\ or D:/
func hasDriveLetterPath(cmd string) bool {
	for i := 0; i < len(cmd)-2; i++ {
		ch := cmd[i]
		if ((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')) &&
			cmd[i+1] == ':' && (cmd[i+2] == '\\' || cmd[i+2] == '/') {
			// Must be at word boundary (start of string, after space, or after quote).
			if i == 0 || cmd[i-1] == ' ' || cmd[i-1] == '\t' || cmd[i-1] == '"' || cmd[i-1] == '\'' {
				return true
			}
		}
	}
	return false
}

// hasCmdEnvVar checks for cmd.exe-style %VARIABLE% syntax.
func hasCmdEnvVar(cmd string) bool {
	for i := 0; i < len(cmd)-2; i++ {
		if cmd[i] == '%' && isAlpha(cmd[i+1]) {
			// Look for closing %
			for j := i + 2; j < len(cmd); j++ {
				if cmd[j] == '%' {
					return true
				}
				if !isWordChar(cmd[j]) {
					break
				}
			}
		}
	}
	return false
}

func isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isWordChar(b byte) bool {
	return isAlpha(b) || (b >= '0' && b <= '9') || b == '_'
}
