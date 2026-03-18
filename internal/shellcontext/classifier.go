// Package shellcontext classifies byte ranges in shell commands as executed code,
// arguments, data, inline code, comments, or unknown.
package shellcontext

import "strings"

// SpanKind classifies the execution context of a byte range.
type SpanKind int

const (
	SpanExecuted   SpanKind = iota // Directly executed code — MUST be scanned
	SpanArgument                   // Flag/argument — do NOT scan
	SpanData                       // Data content — do NOT scan
	SpanInlineCode                 // Code inside bash -c "..." — MUST be scanned
	SpanComment                    // Shell comment — do NOT scan
	SpanUnknown                    // Ambiguous/unclosed — scan conservatively
)

func (k SpanKind) String() string {
	switch k {
	case SpanExecuted:
		return "Executed"
	case SpanArgument:
		return "Argument"
	case SpanData:
		return "Data"
	case SpanInlineCode:
		return "InlineCode"
	case SpanComment:
		return "Comment"
	case SpanUnknown:
		return "Unknown"
	default:
		return "?"
	}
}

// ShouldScan returns true if this span kind should be scanned for destructive patterns.
func (k SpanKind) ShouldScan() bool {
	return k == SpanExecuted || k == SpanInlineCode || k == SpanUnknown
}

// Span is a classified byte range within a command string.
type Span struct {
	Start int // byte offset (inclusive)
	End   int // byte offset (exclusive)
	Kind  SpanKind
}

// Classify takes a shell command string and returns a slice of Spans.
// If shell is nil, it uses the default shell for the platform.
func Classify(cmd string, shell Shell) []Span {
	if len(cmd) == 0 {
		return nil
	}
	if shell == nil {
		shell = DefaultShell()
	}

	c := classifier{
		src:    cmd,
		pos:    0,
		cmdPos: true, // start in command position
		shell:  shell,
	}
	c.run()
	return c.spans
}

type classifier struct {
	src      string
	pos      int
	spans    []Span
	cmdPos   bool   // next token is in command position
	cmdWord  string // the command word for the current pipeline segment
	sawDashC bool   // saw -c flag for an inline-shell command
	shell    Shell
}

func (c *classifier) run() {
	for c.pos < len(c.src) {
		c.skipWhitespaceSpans()
		if c.pos >= len(c.src) {
			break
		}

		ch := c.src[c.pos]

		// Comment.
		if ch == '#' {
			c.consumeComment()
			continue
		}

		// Operators: |, ||, &&, ;
		if c.tryOperator() {
			continue
		}

		// Here-string <<< (POSIX only)
		if _, ok := c.shell.(*POSIXShell); ok {
			if c.tryHereString() {
				continue
			}
		}

		// Token (possibly quoted).
		prevPos := c.pos
		c.consumeToken()
		if c.pos == prevPos {
			c.emit(c.pos, c.pos+1, SpanUnknown)
			c.pos++
		}
	}
}

func (c *classifier) skipWhitespaceSpans() {
	start := c.pos
	for c.pos < len(c.src) && isSpace(c.src[c.pos]) {
		c.pos++
	}
	if c.pos > start {
		c.emit(start, c.pos, SpanArgument)
	}
}

func (c *classifier) consumeComment() {
	start := c.pos
	for c.pos < len(c.src) && c.src[c.pos] != '\n' {
		c.pos++
	}
	c.emit(start, c.pos, SpanComment)
}

func (c *classifier) tryOperator() bool {
	if c.pos >= len(c.src) {
		return false
	}
	ch := c.src[c.pos]

	if ch == ';' {
		c.emit(c.pos, c.pos+1, SpanArgument)
		c.pos++
		c.resetCommand()
		return true
	}

	if ch == '&' {
		if c.pos+1 < len(c.src) && c.src[c.pos+1] == '&' {
			c.emit(c.pos, c.pos+2, SpanArgument)
			c.pos += 2
			c.resetCommand()
			return true
		}
		c.emit(c.pos, c.pos+1, SpanArgument)
		c.pos++
		c.resetCommand()
		return true
	}

	if ch == '|' {
		if c.pos+1 < len(c.src) && c.src[c.pos+1] == '|' {
			c.emit(c.pos, c.pos+2, SpanArgument)
			c.pos += 2
			c.resetCommand()
			return true
		}
		c.emit(c.pos, c.pos+1, SpanArgument)
		c.pos++
		c.resetCommand()
		return true
	}

	return false
}

func (c *classifier) tryHereString() bool {
	if c.pos+3 <= len(c.src) && c.src[c.pos:c.pos+3] == "<<<" {
		c.emit(c.pos, c.pos+3, SpanArgument)
		c.pos += 3
		for c.pos < len(c.src) && isSpace(c.src[c.pos]) {
			c.pos++
		}
		kind := SpanData
		if isInlineShell(c.cmdWord) {
			kind = SpanInlineCode
		}
		if c.pos < len(c.src) {
			start := c.pos
			c.scanWord()
			c.emit(start, c.pos, kind)
		}
		return true
	}
	return false
}

func (c *classifier) consumeToken() {
	start := c.pos
	ch := c.src[c.pos]

	// POSIX command substitution $( ... ) or `...`
	if _, ok := c.shell.(*POSIXShell); ok {
		if ch == '$' && c.pos+1 < len(c.src) && c.src[c.pos+1] == '(' {
			c.consumeCommandSubstitution()
			return
		}
		if ch == '`' {
			c.consumeBacktickSubstitution()
			return
		}
	}

	c.scanWord()
	tokenText := c.src[start:c.pos]
	unquoted := unquoteSimple(tokenText, c.shell)

	var kind SpanKind
	if c.cmdPos {
		kind = SpanExecuted
		c.cmdWord = unquoted
		c.cmdPos = false
		c.sawDashC = false
	} else if c.sawDashC && isInlineShell(c.cmdWord) {
		kind = SpanInlineCode
		c.sawDashC = false
	} else {
		kind = SpanArgument
		if isInlineShell(c.cmdWord) {
			// POSIX -c, CMD /c, PWSH -Command
			if unquoted == "-c" || unquoted == "/c" || strings.EqualFold(unquoted, "-Command") || strings.EqualFold(unquoted, "-c") {
				c.sawDashC = true
			}
		}
	}

	c.emit(start, c.pos, kind)
}

func (c *classifier) consumeCommandSubstitution() {
	start := c.pos
	c.pos += 2
	depth := 1
	rules := c.shell.QuoteRules()
	for c.pos < len(c.src) && depth > 0 {
		ch := c.src[c.pos]
		switch {
		case ch == '(' && c.pos > 0 && c.src[c.pos-1] == '$':
			depth++
			c.pos++
		case ch == ')':
			depth--
			c.pos++
		case ch == '\'' && rules.SupportsSingle:
			c.skipSingleQuote()
		case ch == '"' && rules.SupportsDouble:
			c.skipDoubleQuote()
		case c.shell.IsEscape(c.src, c.pos):
			c.pos += 2
		default:
			c.pos++
		}
	}
	c.emit(start, c.pos, SpanExecuted)
}

func (c *classifier) consumeBacktickSubstitution() {
	start := c.pos
	c.pos++
	for c.pos < len(c.src) {
		if c.src[c.pos] == '`' {
			c.pos++
			break
		}
		if c.shell.IsEscape(c.src, c.pos) && c.pos+1 < len(c.src) {
			c.pos += 2
			continue
		}
		c.pos++
	}
	c.emit(start, c.pos, SpanExecuted)
}

func (c *classifier) scanWord() {
	rules := c.shell.QuoteRules()
	for c.pos < len(c.src) {
		ch := c.src[c.pos]
		if isSpace(ch) || ch == '|' || ch == '&' || ch == ';' || ch == '#' || ch == ')' {
			break
		}

		if c.shell.IsLineContinuation(c.src, c.pos) {
			c.pos++ // consume escape
			// skip whitespace until newline
			for c.pos < len(c.src) && c.src[c.pos] != '\n' {
				c.pos++
			}
			if c.pos < len(c.src) {
				c.pos++ // consume \n
			}
			continue
		}

		switch {
		case ch == '\'' && rules.SupportsSingle:
			c.skipSingleQuote()
		case ch == '"' && rules.SupportsDouble:
			c.skipDoubleQuote()
		case c.shell.IsEscape(c.src, c.pos):
			if c.pos+1 < len(c.src) {
				c.pos += 2
			} else {
				c.pos++
			}
		case ch == '$':
			if _, ok := c.shell.(*POSIXShell); ok {
				if c.pos+1 < len(c.src) && c.src[c.pos+1] == '\'' {
					c.pos += 2
					c.skipSingleQuote()
				} else if c.pos+1 < len(c.src) && c.src[c.pos+1] == '(' {
					return
				} else {
					c.pos++
				}
			} else {
				c.pos++
			}
		case ch == '`':
			if _, ok := c.shell.(*POSIXShell); ok {
				return
			}
			c.pos++
		default:
			c.pos++
		}
	}
}

func (c *classifier) skipSingleQuote() {
	c.pos++
	rules := c.shell.QuoteRules()
	for c.pos < len(c.src) {
		if c.src[c.pos] == '\'' {
			// PowerShell '' is a literal ' inside single quotes.
			if rules.DoubleQuoteEscapesDouble && c.pos+1 < len(c.src) && c.src[c.pos+1] == '\'' {
				c.pos += 2
				continue
			}
			c.pos++
			return
		}
		c.pos++
	}
}

func (c *classifier) skipDoubleQuote() {
	c.pos++
	rules := c.shell.QuoteRules()
	for c.pos < len(c.src) {
		ch := c.src[c.pos]
		if ch == '"' {
			if rules.DoubleQuoteEscapesDouble && c.pos+1 < len(c.src) && c.src[c.pos+1] == '"' {
				c.pos += 2
				continue
			}
			c.pos++
			return
		}
		if rules.EscapeInsideDouble != 0 && ch == rules.EscapeInsideDouble && c.pos+1 < len(c.src) {
			c.pos += 2
			continue
		}
		c.pos++
	}
}

func (c *classifier) emit(start, end int, kind SpanKind) {
	if start >= end {
		return
	}
	c.spans = append(c.spans, Span{Start: start, End: end, Kind: kind})
}

func (c *classifier) resetCommand() {
	c.cmdPos = true
	c.cmdWord = ""
	c.sawDashC = false
}

func unquoteSimple(s string, shell Shell) string {
	if len(s) < 2 {
		return s
	}
	rules := shell.QuoteRules()
	if rules.SupportsSingle && s[0] == '\'' && s[len(s)-1] == '\'' {
		return s[1 : len(s)-1]
	}
	if rules.SupportsDouble && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// isInlineShell reports whether cmdName is a known inline-code interpreter.
func isInlineShell(cmdName string) bool {
	if inlineShells[cmdName] {
		return true
	}
	name := cmdName
	if len(name) > 4 && strings.EqualFold(name[len(name)-4:], ".exe") {
		name = name[:len(name)-4]
		if inlineShells[name] {
			return true
		}
	}
	i := len(name) - 1
	for i >= 0 && (name[i] >= '0' && name[i] <= '9' || name[i] == '.') {
		i--
	}
	if i >= 0 && i < len(name)-1 {
		base := name[:i+1]
		if inlineShells[base] {
			return true
		}
	}
	return false
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

var inlineShells = map[string]bool{
	"bash": true, "sh": true, "zsh": true,
	"dash": true, "ksh": true, "eval": true,
	"cmd": true, "powershell": true, "pwsh": true,
	"python": true, "python3": true,
	"node": true,
	"ruby": true,
	"perl": true,
	"php": true,
	"lua": true,
}
