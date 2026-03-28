package ast

import (
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
)

// ShellType identifies the shell dialect for parsing.
type ShellType int

const (
	ShellBash ShellType = iota
	ShellPowerShell
)

// ShellDecomposer converts a tree-sitter parse tree into TW AST types.
type ShellDecomposer interface {
	Decompose(root *gotreesitter.Node, input []byte) *CompoundCommand
}

// nodeText extracts the source text for a tree-sitter node.
func nodeText(node *gotreesitter.Node, input []byte) string {
	if node == nil {
		return ""
	}
	return node.Text(input)
}

// buildRaw constructs the raw command text from a node's source range.
func buildRaw(node *gotreesitter.Node, input []byte) string {
	if node == nil {
		return ""
	}
	start := node.StartByte()
	end := node.EndByte()
	if int(end) > len(input) {
		end = uint32(len(input))
	}
	return string(input[start:end])
}

// normalizeCommandName strips path prefixes and .exe suffixes from a command name.
// /usr/bin/rm → rm, C:\tools\git.exe → git, git.exe → git
func normalizeCommandName(name string) string {
	if name == "" {
		return name
	}
	// Normalize backslashes for uniform path handling.
	normalized := strings.ReplaceAll(name, "\\", "/")
	// Strip path prefix: take everything after the last '/'.
	if idx := strings.LastIndex(normalized, "/"); idx >= 0 {
		after := normalized[idx+1:]
		if after != "" {
			name = after
		}
		// If after is empty (input was "/" or "C:\"), keep original name.
	}
	// Strip .exe suffix (case-insensitive).
	if len(name) > 4 && strings.EqualFold(name[len(name)-4:], ".exe") {
		name = name[:len(name)-4]
	}
	return name
}

// isFlag returns true if the argument looks like a command flag.
func isFlag(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}

// trimQuotes strips matching outer quotes from a string.
func trimQuotes(s string, q byte) string {
	if len(s) >= 2 && s[0] == q && s[len(s)-1] == q {
		return s[1 : len(s)-1]
	}
	return s
}
