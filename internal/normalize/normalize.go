// Package normalize provides pre-parse cleanup for shell commands.
package normalize

import (
	"strings"
)

// Token represents a shell token from the input command.
type Token struct {
	Raw      string // exact bytes from input
	Unquoted string // logical content after unquoting
	Start    int    // byte offset in original string
	End      int    // byte offset (exclusive)
}

// tokenize splits cmd into tokens respecting POSIX quoting rules.
func tokenize(cmd string) []Token {
	var tokens []Token
	i := 0
	n := len(cmd)
	for i < n {
		// Skip whitespace.
		for i < n && (cmd[i] == ' ' || cmd[i] == '\t') {
			i++
		}
		if i >= n {
			break
		}

		start := i
		var raw strings.Builder
		var unquoted strings.Builder

		for i < n && cmd[i] != ' ' && cmd[i] != '\t' {
			switch cmd[i] {
			case '\'':
				raw.WriteByte('\'')
				i++
				for i < n && cmd[i] != '\'' {
					raw.WriteByte(cmd[i])
					unquoted.WriteByte(cmd[i])
					i++
				}
				if i < n {
					raw.WriteByte('\'')
					i++ // skip closing '
				}
			case '"':
				raw.WriteByte('"')
				i++
				for i < n && cmd[i] != '"' {
					if cmd[i] == '\\' && i+1 < n {
						next := cmd[i+1]
						if next == '"' || next == '\\' || next == '$' || next == '`' {
							raw.WriteByte('\\')
							raw.WriteByte(next)
							unquoted.WriteByte(next)
							i += 2
							continue
						}
					}
					raw.WriteByte(cmd[i])
					unquoted.WriteByte(cmd[i])
					i++
				}
				if i < n {
					raw.WriteByte('"')
					i++ // skip closing "
				}
			case '\\':
				raw.WriteByte('\\')
				i++
				if i < n {
					raw.WriteByte(cmd[i])
					unquoted.WriteByte(cmd[i])
					i++
				}
			default:
				// Check for ANSI-C quoting: $'...'
				if cmd[i] == '$' && i+1 < n && cmd[i+1] == '\'' {
					raw.WriteByte('$')
					raw.WriteByte('\'')
					i += 2 // skip $'
					for i < n && cmd[i] != '\'' {
						if cmd[i] == '\\' && i+1 < n {
							raw.WriteByte('\\')
							raw.WriteByte(cmd[i+1])
							ch := interpretAnsiCEscape(cmd, &i)
							unquoted.WriteRune(ch)
						} else {
							raw.WriteByte(cmd[i])
							unquoted.WriteByte(cmd[i])
							i++
						}
					}
					if i < n {
						raw.WriteByte('\'')
						i++ // skip closing '
					}
				} else {
					raw.WriteByte(cmd[i])
					unquoted.WriteByte(cmd[i])
					i++
				}
			}
		}

		tokens = append(tokens, Token{
			Raw:      raw.String(),
			Unquoted: unquoted.String(),
			Start:    start,
			End:      i,
		})
	}
	return tokens
}

// interpretAnsiCEscape interprets a backslash escape sequence in an ANSI-C
// quoted string ($'...'). It advances *pos past the escape sequence and
// returns the interpreted rune. On entry, cmd[*pos] == '\\'.
func interpretAnsiCEscape(cmd string, pos *int) rune {
	i := *pos
	i++ // skip backslash
	if i >= len(cmd) {
		*pos = i
		return '\\'
	}
	ch := cmd[i]
	i++ // skip the escape character
	switch ch {
	case 'a':
		*pos = i
		return '\a'
	case 'b':
		*pos = i
		return '\b'
	case 'e', 'E':
		*pos = i
		return 0x1B // ESC
	case 'f':
		*pos = i
		return '\f'
	case 'n':
		*pos = i
		return '\n'
	case 'r':
		*pos = i
		return '\r'
	case 't':
		*pos = i
		return '\t'
	case 'v':
		*pos = i
		return '\v'
	case '\\':
		*pos = i
		return '\\'
	case '\'':
		*pos = i
		return '\''
	case '"':
		*pos = i
		return '"'
	case 'x':
		// \xHH — one or two hex digits
		val := 0
		count := 0
		for count < 2 && i < len(cmd) {
			d := hexVal(cmd[i])
			if d < 0 {
				break
			}
			val = val*16 + d
			i++
			count++
		}
		if count > 0 {
			*pos = i
			return rune(val)
		}
		*pos = i
		return 'x' // bare \x with no digits
	case '0', '1', '2', '3', '4', '5', '6', '7':
		// \NNN — one to three octal digits
		val := int(ch - '0')
		count := 1
		for count < 3 && i < len(cmd) && cmd[i] >= '0' && cmd[i] <= '7' {
			val = val*8 + int(cmd[i]-'0')
			i++
			count++
		}
		*pos = i
		return rune(val)
	default:
		// Unknown escape — return the character after backslash literally
		*pos = i
		return rune(ch)
	}
}

// hexVal returns the numeric value of a hex digit, or -1 if not a hex digit.
func hexVal(b byte) int {
	switch {
	case b >= '0' && b <= '9':
		return int(b - '0')
	case b >= 'a' && b <= 'f':
		return int(b - 'a' + 10)
	case b >= 'A' && b <= 'F':
		return int(b - 'A' + 10)
	default:
		return -1
	}
}

// PreParseCleanup performs minimal cleanup before AST parsing:
// CR stripping, line continuation collapsing, and Windows backslash normalization.
func PreParseCleanup(cmd string) string {
	out := stripCarriageReturns(cmd)
	out = collapseLineContinuations(out)
	out = normalizeWindowsBackslashes(out)
	return out
}

// collapseLineContinuations joins backslash-newline sequences (\<LF>),
// which in POSIX shells continue a logical line across physical lines.
// For example, "git re\\\nset --hard" becomes "git reset --hard".
func collapseLineContinuations(cmd string) string {
	if !strings.Contains(cmd, "\\\n") {
		return cmd
	}
	return strings.ReplaceAll(cmd, "\\\n", "")
}

// stripCarriageReturns normalizes Windows-style line endings before any other
// processing. Converts \r\n → \n and bare \r → \n so that line-continuation
// collapsing, heredoc handling, and newline-to-semicolon conversion all work
// correctly regardless of the input's line ending style.
func stripCarriageReturns(cmd string) string {
	if !strings.ContainsRune(cmd, '\r') {
		return cmd
	}
	// First pass: \r\n → \n (Windows line endings)
	cmd = strings.ReplaceAll(cmd, "\r\n", "\n")
	// Second pass: bare \r → \n (old Mac line endings)
	cmd = strings.ReplaceAll(cmd, "\r", "\n")
	return cmd
}

// normalizeWindowsBackslashes converts backslashes to forward slashes inside
// tokens that look like Windows absolute paths (e.g., C:\foo\bar → C:/foo/bar).
// This must run before the AST parser, which would otherwise treat
// backslashes as escape characters and mangle the path.
func normalizeWindowsBackslashes(cmd string) string {
	if !strings.ContainsRune(cmd, '\\') {
		return cmd
	}

	var out strings.Builder
	out.Grow(len(cmd))
	i := 0
	for i < len(cmd) {
		// Detect a Windows drive prefix: letter + : + backslash
		if i+2 < len(cmd) {
			ch := cmd[i]
			if ((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')) && cmd[i+1] == ':' && cmd[i+2] == '\\' {
				// Found a Windows path. Convert backslashes to forward slashes
				// until we hit whitespace, a quote, or end of string.
				out.WriteByte(ch)
				out.WriteByte(':')
				i += 2
				for i < len(cmd) && cmd[i] != ' ' && cmd[i] != '\t' && cmd[i] != '"' && cmd[i] != '\'' && cmd[i] != ';' && cmd[i] != '|' && cmd[i] != '&' {
					if cmd[i] == '\\' {
						out.WriteByte('/')
					} else {
						out.WriteByte(cmd[i])
					}
					i++
				}
				continue
			}
		}
		out.WriteByte(cmd[i])
		i++
	}
	return out.String()
}
