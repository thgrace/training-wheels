package ast

import (
	"sync"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const maxInputSize = 64 * 1024 // 64KB

// parseTimeoutMicros caps tree-sitter GLR parse time per command. Commands
// that exceed this limit fail-open (nil → Allow). The limit exists because
// certain input shapes (many repeated flag pairs) trigger O(n²)+ GLR work.
// See docs/tree-sitter/parse-timeout.md for analysis and the regex-fallback plan.
const parseTimeoutMicros uint64 = 450_000 // 450ms — budget is 500ms; 50ms headroom for pool overhead

var (
	bashPoolOnce sync.Once
	bashPool     *gotreesitter.ParserPool
	psPoolOnce   sync.Once
	psPool       *gotreesitter.ParserPool
)

func getBashPool() *gotreesitter.ParserPool {
	bashPoolOnce.Do(func() {
		bashPool = gotreesitter.NewParserPool(
			grammars.BashLanguage(),
			gotreesitter.WithParserPoolTimeoutMicros(parseTimeoutMicros),
		)
	})
	return bashPool
}

func getPSPool() *gotreesitter.ParserPool {
	psPoolOnce.Do(func() {
		psPool = gotreesitter.NewParserPool(
			grammars.PowershellLanguage(),
			gotreesitter.WithParserPoolTimeoutMicros(parseTimeoutMicros),
		)
	})
	return psPool
}

// Parse parses a command string as Bash and returns a decomposed CompoundCommand.
// Convenience wrapper around ParseShell with ShellBash.
func Parse(input []byte) *CompoundCommand {
	return ParseShell(input, ShellBash)
}

// ParseShell parses a command string using the specified shell grammar
// and returns a decomposed CompoundCommand.
// Returns nil on error, timeout, or unsupported shell (fail-open: caller treats nil as Allow).
func ParseShell(input []byte, shell ShellType) *CompoundCommand {
	if len(input) == 0 {
		return &CompoundCommand{}
	}
	if len(input) > maxInputSize {
		return nil
	}

	var pool *gotreesitter.ParserPool
	var decomposer ShellDecomposer

	switch shell {
	case ShellBash:
		pool = getBashPool()
		decomposer = NewBashDecomposer()
	case ShellPowerShell:
		pool = getPSPool()
		decomposer = NewPowerShellDecomposer()
	default:
		return nil
	}

	tree, err := pool.Parse(input)
	if err != nil {
		return nil
	}

	return decomposer.Decompose(tree.RootNode(), input)
}
