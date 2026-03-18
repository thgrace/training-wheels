// Package normalize strips wrapper prefixes and normalizes paths in shell commands.
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

// StrippedWrapper records one layer of stripping.
type StrippedWrapper struct {
	WrapperType  string // "sudo", "env", "command", "backslash"
	StrippedText string // exact prefix text removed
}

// NormalizedCommand is the result of normalization.
type NormalizedCommand struct {
	Original         string
	Normalized       string
	StrippedWrappers []StrippedWrapper
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

// sudoNoArgFlags are single-char sudo flags that don't take an argument.
var sudoNoArgFlags = map[byte]bool{
	'E': true, 'H': true, 'n': true, 'k': true,
	'K': true, 'S': true, 'b': true,
}

// sudoArgFlags are single-char sudo flags that consume the next token.
var sudoArgFlags = map[byte]bool{
	'u': true, 'g': true, 'p': true, 'r': true, 't': true, 'c': true,
}

// sudoLongArgFlags are long-form sudo flags that consume the next token as
// their argument (e.g. --user root). All other long flags are treated as
// boolean (no argument).
var sudoLongArgFlags = map[string]bool{
	"--user":            true,
	"--group":           true,
	"--prompt":          true,
	"--role":            true,
	"--type":            true,
	"--command-timeout": true,
	"--close-from":      true,
	"--other-user":      true,
	"--host":            true,
}

// StripWrapperPrefixes iteratively removes known wrapper prefixes.
func StripWrapperPrefixes(cmd string) NormalizedCommand {
	result := NormalizedCommand{Original: cmd, Normalized: cmd}

	for iter := 0; iter < 32; iter++ {
		trimmed := strings.TrimSpace(result.Normalized)
		if trimmed == "" {
			break
		}

		stripped := false

		// Try backslash first (simplest).
		if len(trimmed) > 1 && trimmed[0] == '\\' && trimmed[1] != ' ' && trimmed[1] != '\t' {
			result.StrippedWrappers = append(result.StrippedWrappers, StrippedWrapper{
				WrapperType:  "backslash",
				StrippedText: `\`,
			})
			result.Normalized = trimmed[1:]
			stripped = true
			continue
		}

		tokens := tokenize(trimmed)
		if len(tokens) == 0 {
			break
		}

		// Try sudo.
		if tokens[0].Unquoted == "sudo" {
			newCmd, sw := stripSudo(tokens, trimmed)
			if sw != nil {
				result.StrippedWrappers = append(result.StrippedWrappers, *sw)
				result.Normalized = newCmd
				stripped = true
			}
		}

		if stripped {
			continue
		}

		// Try env.
		if tokens[0].Unquoted == "env" {
			newCmd, sw := stripEnv(tokens, trimmed)
			if sw != nil {
				result.StrippedWrappers = append(result.StrippedWrappers, *sw)
				result.Normalized = newCmd
				stripped = true
			}
		}

		if stripped {
			continue
		}

		// Try command.
		if tokens[0].Unquoted == "command" {
			newCmd, sw := stripCommand(tokens, trimmed)
			if sw != nil {
				result.StrippedWrappers = append(result.StrippedWrappers, *sw)
				result.Normalized = newCmd
				stripped = true
			}
		}

		if !stripped {
			break
		}
	}

	result.Normalized = strings.TrimSpace(result.Normalized)
	return result
}

func stripSudo(tokens []Token, full string) (string, *StrippedWrapper) {
	i := 1 // skip "sudo"
	for i < len(tokens) {
		tok := tokens[i].Unquoted
		if tok == "--" {
			i++
			break
		}
		if len(tok) > 1 && tok[0] == '-' && tok[1] != '-' {
			// Short flags: could be combined like -EHu
			j := 1
			skipNext := false
			for j < len(tok) {
				ch := tok[j]
				if sudoNoArgFlags[ch] {
					j++
					continue
				}
				if sudoArgFlags[ch] {
					// Rest of token is the argument, or next token is.
					if j+1 < len(tok) {
						// Argument is rest of this token.
						skipNext = false
					} else {
						skipNext = true
					}
					break
				}
				// Unknown flag char — treat as end of flags.
				break
			}
			i++
			if skipNext && i < len(tokens) {
				i++
			}
			continue
		}
		if strings.HasPrefix(tok, "--") {
			// Long flags: --user=foo or --user foo or --login (boolean)
			if strings.Contains(tok, "=") {
				i++
				continue
			}
			i++
			// Only consume the next token if this flag takes an argument.
			if sudoLongArgFlags[tok] && i < len(tokens) {
				i++
			}
			continue
		}
		break // First non-flag token is the command.
	}

	if i >= len(tokens) {
		return full, nil // No command after sudo flags.
	}

	stripped := full[:tokens[i].Start]
	remaining := full[tokens[i].Start:]
	return remaining, &StrippedWrapper{WrapperType: "sudo", StrippedText: stripped}
}

// tokenValueHasInlineCode reports whether a KEY=VALUE assignment value
// contains shell command substitution syntax. If it does, the assignment
// cannot be safely skipped: the shell will execute the embedded code.
func tokenValueHasInlineCode(value string) bool {
	return strings.Contains(value, "$(") || strings.Contains(value, "`")
}

func stripEnv(tokens []Token, full string) (string, *StrippedWrapper) {
	i := 1 // skip "env"
	for i < len(tokens) {
		tok := tokens[i].Unquoted
		if tok == "-i" || tok == "--ignore-environment" {
			i++
			continue
		}
		if tok == "-u" || tok == "--unset" {
			i++
			if i < len(tokens) {
				i++ // skip the variable name
			}
			continue
		}
		// -S / --split-string: the next token (or the rest of the combined
		// flag token after 'S') is a command string the shell will execute.
		// Extract it and return it as the remaining command so the
		// StripWrapperPrefixes loop can continue normalizing the embedded
		// command (e.g. strip a nested sudo).
		if tok == "--split-string" {
			i++
			if i >= len(tokens) {
				return full, nil
			}
			embedded := tokens[i].Unquoted
			stripped := full[:tokens[i-1].Start]
			return embedded, &StrippedWrapper{WrapperType: "env", StrippedText: stripped}
		}
		if strings.HasPrefix(tok, "--split-string=") {
			embedded := tok[len("--split-string="):]
			stripped := full[:tokens[i].Start]
			return embedded, &StrippedWrapper{WrapperType: "env", StrippedText: stripped}
		}
		// Short flags: handle -S alone or combined (e.g. -iS, -Si).
		if len(tok) >= 2 && tok[0] == '-' && tok[1] != '-' {
			sIdx := strings.IndexByte(tok, 'S')
			if sIdx >= 1 {
				// Characters after 'S' in the same token are the embedded
				// command string (unusual but possible, e.g. -Srm).
				// If nothing follows, the next token holds the command string.
				afterS := tok[sIdx+1:]
				if afterS != "" {
					stripped := full[:tokens[i].Start]
					return afterS, &StrippedWrapper{WrapperType: "env", StrippedText: stripped}
				}
				i++
				if i >= len(tokens) {
					return full, nil
				}
				embedded := tokens[i].Unquoted
				stripped := full[:tokens[i-1].Start]
				return embedded, &StrippedWrapper{WrapperType: "env", StrippedText: stripped}
			}
			// Other short flags (-i already handled above); skip unknown ones.
			i++
			continue
		}
		// NAME=VALUE assignments.
		if strings.Contains(tok, "=") && !strings.HasPrefix(tok, "-") {
			key, value, _ := strings.Cut(tok, "=")
			if key != "" && !strings.ContainsAny(key, " \t") {
				if tokenValueHasInlineCode(value) {
					return full, nil // Abort stripping — dangerous inline code in assignment value.
				}
				i++
				continue
			}
		}
		break // First non-env token is the command.
	}

	if i >= len(tokens) {
		return full, nil
	}

	stripped := full[:tokens[i].Start]
	remaining := full[tokens[i].Start:]
	return remaining, &StrippedWrapper{WrapperType: "env", StrippedText: stripped}
}

func stripCommand(tokens []Token, full string) (string, *StrippedWrapper) {
	i := 1 // skip "command"
	for i < len(tokens) {
		tok := tokens[i].Unquoted
		if tok == "-p" {
			i++
			continue
		}
		if tok == "--" {
			i++
			break
		}
		// -v and -V are query modes — don't strip.
		if tok == "-v" || tok == "-V" {
			return full, nil
		}
		break
	}

	if i >= len(tokens) {
		return full, nil
	}

	stripped := full[:tokens[i].Start]
	remaining := full[tokens[i].Start:]
	return remaining, &StrippedWrapper{WrapperType: "command", StrippedText: stripped}
}

// PreNormalize applies lightweight syntactic normalization that should run
// before context classification / sanitization. It collapses line
// continuations, converts newlines to semicolons, strips redirections, and
// unquotes tokens — all operations that resolve shell syntax without changing
// command semantics.
func PreNormalize(cmd string) string {
	// Step -1: Strip carriage returns so \r\n and bare \r are normalized
	// to \n before any other processing.
	out := stripCarriageReturns(cmd)

	// Step 0: Convert Windows backslash paths to forward slashes so the
	// POSIX tokenizer doesn't treat backslashes as escape characters.
	// Must run before any unquoting or line-continuation collapsing.
	out = normalizeWindowsBackslashes(out)

	// Step 1: Collapse line continuations (backslash+newline → nothing).
	out = collapseLineContinuations(out)

	// Step 2: Strip heredoc bodies so their content is not mistaken for commands.
	out = stripHeredocs(out)

	// Step 3: Treat bare newlines as semicolons (command separator).
	out = newlinesToSemicolons(out)

	// Step 4: Strip redirections that break token flow.
	out = stripRedirections(out)

	// Step 5: Unquote all tokens (handles "git", 'git', g'i't, g\it).
	out = unquoteTokens(out)

	return out
}

// NormalizeCommand strips wrapper prefixes, path prefixes, and .exe suffixes.
// The caller is responsible for running PreNormalize first if the input is a
// raw shell command.
func NormalizeCommand(cmd string) string {
	// Strip wrapper prefixes (sudo, env, command, leading backslash).
	nc := StripWrapperPrefixes(cmd)
	normalized := nc.Normalized

	// Path normalization: strip directory prefix from first token.
	normalized = normalizePath(normalized)

	// .exe stripping: remove trailing .exe from first token.
	normalized = stripExe(normalized)

	return normalized
}

// normalizePath replaces absolute path prefixes on the first token with just the binary name.
// Handles both Unix (/usr/bin/git) and Windows (C:\Program Files\Git\bin\git) paths.
func normalizePath(cmd string) string {
	if len(cmd) == 0 {
		return cmd
	}

	// Check if the first token is an absolute path (Unix or Windows).
	isAbsPath := cmd[0] == '/'
	if !isAbsPath && len(cmd) >= 3 {
		// Windows: C:\ or C:/
		ch := cmd[0]
		if ((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')) && cmd[1] == ':' && (cmd[2] == '\\' || cmd[2] == '/') {
			isAbsPath = true
		}
	}

	if !isAbsPath {
		return cmd
	}

	// Find the end of the first token.
	end := strings.IndexAny(cmd, " \t")
	if end == -1 {
		end = len(cmd)
	}
	firstToken := cmd[:end]
	// Find the last path separator (/ or \).
	lastSep := strings.LastIndexAny(firstToken, "/\\")
	if lastSep < 0 || lastSep == len(firstToken)-1 {
		return cmd
	}
	basename := firstToken[lastSep+1:]
	if basename == "" {
		return cmd
	}
	return basename + cmd[end:]
}

// stripExe removes trailing ".exe" from the first token.
func stripExe(cmd string) string {
	end := strings.IndexAny(cmd, " \t")
	if end == -1 {
		end = len(cmd)
	}
	firstToken := cmd[:end]
	if strings.HasSuffix(strings.ToLower(firstToken), ".exe") {
		firstToken = firstToken[:len(firstToken)-4]
		return firstToken + cmd[end:]
	}
	return cmd
}

// normalizeWindowsBackslashes converts backslashes to forward slashes inside
// tokens that look like Windows absolute paths (e.g., C:\foo\bar → C:/foo/bar).
// This must run before the POSIX tokenizer, which would otherwise treat
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

// stripHeredocs detects heredoc operators (<<DELIM or <<-DELIM) and replaces
// the heredoc body with spaces so that data content is not mistaken for
// executable commands by downstream pipeline stages.
func stripHeredocs(cmd string) string {
	if !strings.Contains(cmd, "<<") {
		return cmd
	}

	out := []byte(cmd)
	i := 0
	for i < len(out)-1 {
		// Skip single-quoted regions.
		if out[i] == '\'' {
			i++
			for i < len(out) && out[i] != '\'' {
				i++
			}
			if i < len(out) {
				i++
			}
			continue
		}
		// Skip double-quoted regions.
		if out[i] == '"' {
			i++
			for i < len(out) && out[i] != '"' {
				if out[i] == '\\' && i+1 < len(out) {
					i += 2
					continue
				}
				i++
			}
			if i < len(out) {
				i++
			}
			continue
		}

		// Look for <<
		if out[i] != '<' || out[i+1] != '<' {
			i++
			continue
		}

		// Skip here-strings (<<<)
		if i+2 < len(out) && out[i+2] == '<' {
			i += 3
			continue
		}

		pos := i + 2 // past <<

		// Handle <<- (tab-stripped heredocs)
		tabStripped := false
		if pos < len(out) && out[pos] == '-' {
			tabStripped = true
			pos++
		}

		// Skip optional whitespace between << and delimiter
		for pos < len(out) && (out[pos] == ' ' || out[pos] == '\t') {
			pos++
		}

		// Extract the delimiter word, stripping quotes if present
		delim, delimEnd := extractHeredocDelimiter(out, pos)
		if delim == "" {
			i = delimEnd
			continue
		}

		// Find the start of the heredoc body (after next newline)
		bodyStart := delimEnd
		for bodyStart < len(out) && out[bodyStart] != '\n' {
			bodyStart++
		}
		if bodyStart >= len(out) {
			// No newline after delimiter — no body to strip
			i = bodyStart
			continue
		}
		bodyStart++ // skip the newline

		// Find the closing delimiter line
		bodyEnd := findClosingDelimiter(out, bodyStart, delim, tabStripped)

		// Replace body bytes with spaces
		for j := bodyStart; j < bodyEnd; j++ {
			if out[j] != '\n' {
				out[j] = ' '
			}
		}

		i = bodyEnd
	}
	return string(out)
}

// extractHeredocDelimiter extracts the delimiter word starting at pos.
// Returns the unquoted delimiter and the position after it.
func extractHeredocDelimiter(cmd []byte, pos int) (string, int) {
	if pos >= len(cmd) {
		return "", pos
	}

	// Check for quoted delimiters: 'DELIM', "DELIM", or \-escaped chars
	switch cmd[pos] {
	case '\'':
		pos++ // skip opening quote
		start := pos
		for pos < len(cmd) && cmd[pos] != '\'' && cmd[pos] != '\n' {
			pos++
		}
		delim := string(cmd[start:pos])
		if pos < len(cmd) && cmd[pos] == '\'' {
			pos++
		}
		return delim, pos
	case '"':
		pos++ // skip opening quote
		start := pos
		for pos < len(cmd) && cmd[pos] != '"' && cmd[pos] != '\n' {
			pos++
		}
		delim := string(cmd[start:pos])
		if pos < len(cmd) && cmd[pos] == '"' {
			pos++
		}
		return delim, pos
	}

	// Unquoted delimiter: read until whitespace, newline, or shell metachar.
	// Strip any backslash escapes.
	start := pos
	var delim strings.Builder
	for pos < len(cmd) && cmd[pos] != ' ' && cmd[pos] != '\t' && cmd[pos] != '\n' &&
		cmd[pos] != ';' && cmd[pos] != '|' && cmd[pos] != '&' && cmd[pos] != ')' {
		if cmd[pos] == '\\' && pos+1 < len(cmd) {
			pos++
			delim.WriteByte(cmd[pos])
			pos++
			continue
		}
		delim.WriteByte(cmd[pos])
		pos++
	}
	if pos == start {
		return "", pos
	}
	return delim.String(), pos
}

// findClosingDelimiter searches for the closing delimiter line in a heredoc body.
// For <<- (tabStripped), the delimiter line may be indented with tabs.
// Returns the byte position of the start of the closing delimiter line so that
// only the body content (before the delimiter) gets masked.
func findClosingDelimiter(cmd []byte, bodyStart int, delim string, tabStripped bool) int {
	pos := bodyStart
	for pos < len(cmd) {
		// Find the start and end of this line
		lineStart := pos
		lineEnd := pos
		for lineEnd < len(cmd) && cmd[lineEnd] != '\n' {
			lineEnd++
		}

		// Check if this line is the closing delimiter
		checkStart := lineStart
		if tabStripped {
			for checkStart < lineEnd && cmd[checkStart] == '\t' {
				checkStart++
			}
		}

		lineContent := string(cmd[checkStart:lineEnd])
		if lineContent == delim {
			// Return the start of the delimiter line so the delimiter is preserved
			return lineStart
		}

		// Move to next line
		if lineEnd < len(cmd) {
			pos = lineEnd + 1
		} else {
			pos = lineEnd
			break
		}
	}
	// No closing delimiter found — return end (fail-open: treat as all body)
	return pos
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

// collapseLineContinuations removes backslash+newline sequences, joining
// split lines into a single logical line. This matches POSIX shell behavior
// where \<newline> is a line continuation that is removed before tokenization.
func collapseLineContinuations(cmd string) string {
	if !strings.Contains(cmd, "\\\n") {
		return cmd
	}
	return strings.ReplaceAll(cmd, "\\\n", "")
}

// newlinesToSemicolons replaces bare newline characters with semicolons so
// that the rest of the pipeline treats them as command separators.
func newlinesToSemicolons(cmd string) string {
	if !strings.ContainsRune(cmd, '\n') {
		return cmd
	}
	return strings.ReplaceAll(cmd, "\n", " ; ")
}

// stripRedirections removes I/O redirection operators and their targets from
// the command string so they don't break token association. Handles patterns
// like >/dev/null, 2>/dev/null, 2>&1, >>/file, <file.
func stripRedirections(cmd string) string {
	var out strings.Builder
	out.Grow(len(cmd))
	i := 0
	for i < len(cmd) {
		// Check for redirection: optional fd digit, then > or <
		redirStart := i

		// Optional file descriptor digit (0-9).
		fdDigit := byte(0)
		if i < len(cmd) && cmd[i] >= '0' && cmd[i] <= '9' {
			next := i + 1
			if next < len(cmd) && (cmd[next] == '>' || cmd[next] == '<') {
				fdDigit = cmd[i]
				i = next
			}
		}

		if i < len(cmd) && (cmd[i] == '>' || cmd[i] == '<') {
			op := cmd[i]
			ri := i
			ri++ // skip > or <
			// Handle >> or <<
			doubled := false
			if ri < len(cmd) && cmd[ri] == op {
				doubled = true
				ri++
			}
			// Handle >&
			hasAmpersand := false
			if ri < len(cmd) && cmd[ri] == '&' {
				hasAmpersand = true
				ri++
			}

			// Preserve stdin redirects: bare < (not << heredoc, not <& fd dup).
			// Stdin input redirects (< file) change command semantics in a
			// security-relevant way (e.g. sqlite3 db < evil.sql) and must
			// remain visible to pattern matching.
			if op == '<' && !doubled && !hasAmpersand && (fdDigit == 0 || fdDigit == '0') {
				// Not a heredoc (<<) or fd dup (<&) — preserve as-is.
				i = redirStart
				out.WriteByte(cmd[i])
				i++
				continue
			}

			// Skip whitespace after operator.
			for ri < len(cmd) && (cmd[ri] == ' ' || cmd[ri] == '\t') {
				ri++
			}
			// Skip the redirection target (a word).
			ri = skipRedirTarget(cmd, ri)

			// Replace the redirection with a space.
			out.WriteByte(' ')
			i = ri
			continue
		}

		// Not a redirection — reset if we advanced past a digit.
		i = redirStart

		// Handle quotes so we don't strip redirections inside quoted strings.
		switch cmd[i] {
		case '\'':
			out.WriteByte(cmd[i])
			i++
			for i < len(cmd) && cmd[i] != '\'' {
				out.WriteByte(cmd[i])
				i++
			}
			if i < len(cmd) {
				out.WriteByte(cmd[i])
				i++
			}
		case '"':
			out.WriteByte(cmd[i])
			i++
			for i < len(cmd) && cmd[i] != '"' {
				if cmd[i] == '\\' && i+1 < len(cmd) {
					out.WriteByte(cmd[i])
					i++
					out.WriteByte(cmd[i])
					i++
					continue
				}
				out.WriteByte(cmd[i])
				i++
			}
			if i < len(cmd) {
				out.WriteByte(cmd[i])
				i++
			}
		default:
			out.WriteByte(cmd[i])
			i++
		}
	}
	return out.String()
}

// skipRedirTarget advances past a redirection target word.
func skipRedirTarget(cmd string, i int) int {
	for i < len(cmd) {
		ch := cmd[i]
		if ch == ' ' || ch == '\t' || ch == ';' || ch == '|' || ch == '&' || ch == '\n' {
			break
		}
		switch ch {
		case '\'':
			i++
			for i < len(cmd) && cmd[i] != '\'' {
				i++
			}
			if i < len(cmd) {
				i++
			}
		case '"':
			i++
			for i < len(cmd) && cmd[i] != '"' {
				if cmd[i] == '\\' && i+1 < len(cmd) {
					i += 2
					continue
				}
				i++
			}
			if i < len(cmd) {
				i++
			}
		case '\\':
			if i+1 < len(cmd) {
				next := cmd[i+1]
				// Only treat backslash as escape for shell metacharacters,
				// not for characters that follow \ in Windows paths.
				if next == ' ' || next == '\t' || next == '\'' || next == '"' ||
					next == '\\' || next == '>' || next == '<' || next == '|' ||
					next == '&' || next == ';' || next == '#' || next == '\n' {
					i += 2
				} else {
					i++
				}
			} else {
				i++
			}
		default:
			i++
		}
	}
	return i
}

// unquoteTokens rebuilds the command string with all shell quoting removed
// from each token. This normalizes "git", 'git', g'i't, and g\it all to git.
// Tokens that are entirely within quotes get their quotes stripped; tokens with
// mixed quoting (g'i't) are reassembled from their logical characters.
func unquoteTokens(cmd string) string {
	tokens := tokenize(cmd)
	if len(tokens) == 0 {
		return cmd
	}

	// Check if any token actually has quoting to remove.
	anyQuoted := false
	for _, t := range tokens {
		if t.Raw != t.Unquoted {
			anyQuoted = true
			break
		}
	}
	if !anyQuoted {
		return cmd
	}

	var out strings.Builder
	out.Grow(len(cmd))
	prev := 0
	for _, t := range tokens {
		// Preserve whitespace between tokens.
		if t.Start > prev {
			out.WriteString(cmd[prev:t.Start])
		}
		out.WriteString(t.Unquoted)
		prev = t.End
	}
	// Preserve trailing content.
	if prev < len(cmd) {
		out.WriteString(cmd[prev:])
	}
	return out.String()
}
