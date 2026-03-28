package packs

import (
	"strings"

	"github.com/thgrace/training-wheels/internal/ast"
)

// PatternCondition defines structural match conditions for a command.
// Items within a list are OR (any match triggers).
// Keys within a condition are AND (all conditions must hold).
type PatternCondition struct {
	Command                []string // command names: ["rm", "rmdir"] — any match (OR)
	Subcommand             []string // git-style: ["push", "reset"] — any match (OR)
	Flag                   []string // any flag present triggers (OR): ["-r", "-R", "--recursive"]
	AllFlags               []string // all flags must be present (AND): ["--no-preserve-root", "-r"]
	ArgExact               []string // any arg equals value (OR): ["~", "$HOME"]
	ArgPrefix              []string // any arg starts with value (OR): ["/", "~/"]
	AllArgPrefix           []string // ALL args must start with one of these prefixes (AND over args, OR over prefixes)
	ArgContains            []string // any arg contains value (OR, always case-insensitive): ["DROP DATABASE"]
	OutputRedirectContains []string // any output redirect target contains value (OR, always case-insensitive)
}

// IsEmpty returns true if no conditions are set.
func (pc *PatternCondition) IsEmpty() bool {
	return len(pc.Command) == 0 && len(pc.Subcommand) == 0 &&
		len(pc.Flag) == 0 && len(pc.AllFlags) == 0 &&
		len(pc.ArgExact) == 0 && len(pc.ArgPrefix) == 0 &&
		len(pc.AllArgPrefix) == 0 && len(pc.ArgContains) == 0 &&
		len(pc.OutputRedirectContains) == 0
}

// Match checks whether a SimpleCommand satisfies all conditions.
// All present (non-empty) fields must match (AND across fields).
// Empty/nil fields are skipped.
// When caseSensitive is false (default), flag and arg comparisons are case-insensitive.
// Command, Subcommand, and ArgContains are always case-insensitive regardless.
func (pc *PatternCondition) Match(cmd ast.SimpleCommand, caseSensitive bool) bool {
	// 1. Command — cmd.Name must appear in the list (always case-insensitive).
	if len(pc.Command) > 0 {
		if !containsCI(pc.Command, cmd.Name) {
			return false
		}
	}

	// 2. Subcommand — prefer cmd.Subcommand (set by enricher), fall back to Args[0].
	if len(pc.Subcommand) > 0 {
		sub := cmd.Subcommand
		if sub == "" && len(cmd.Args) > 0 {
			sub = cmd.Args[0] // fallback for unenriched commands
		}
		if sub == "" || !containsCI(pc.Subcommand, sub) {
			return false
		}
	}

	// 3. Flag — at least one listed flag must be present in cmd.Flags (OR).
	if len(pc.Flag) > 0 {
		if !hasAnyFlag(cmd.Flags, pc.Flag, caseSensitive) {
			return false
		}
	}

	// 4. AllFlags — ALL listed flags must be present in cmd.Flags (AND).
	if len(pc.AllFlags) > 0 {
		if !hasAllFlags(cmd.Flags, pc.AllFlags, caseSensitive) {
			return false
		}
	}

	// 5. ArgExact — at least one cmd.Arg must equal a listed value (OR).
	if len(pc.ArgExact) > 0 {
		if !hasArgExact(cmd.Args, pc.ArgExact, caseSensitive) {
			return false
		}
	}

	// 6. ArgPrefix — at least one cmd.Arg must start with a listed prefix (OR).
	if len(pc.ArgPrefix) > 0 {
		if !hasArgPrefix(cmd.Args, pc.ArgPrefix, caseSensitive) {
			return false
		}
	}

	// 6b. AllArgPrefix — ALL cmd.Args must start with one of the listed prefixes.
	if len(pc.AllArgPrefix) > 0 {
		if !hasAllArgPrefix(cmd.Args, pc.AllArgPrefix, caseSensitive) {
			return false
		}
	}

	// 7. ArgContains — at least one cmd.Arg must contain a listed substring (OR, always case-insensitive).
	if len(pc.ArgContains) > 0 {
		if !hasArgContains(cmd.Args, pc.ArgContains) {
			return false
		}
	}

	// 8. OutputRedirectContains — at least one output redirect target must
	// contain a listed substring (OR, always case-insensitive).
	if len(pc.OutputRedirectContains) > 0 {
		if !hasArgContains(cmd.OutputRedirects, pc.OutputRedirectContains) {
			return false
		}
	}

	return true
}

// StructuralPattern is a destructive pattern using when/unless structural matching.
type StructuralPattern struct {
	Name          string
	When          PatternCondition
	Unless        PatternCondition
	CaseSensitive bool // false (default): flag/arg matching is case-insensitive
	Reason        string
	Severity      Severity
	Action        string // "deny" or "ask"
	Explanation   string
	Suggestions   []PatternSuggestion
}

// MatchCommand checks a SimpleCommand against the when/unless pattern.
// Returns true if the command matches (should be denied/asked).
func (sp *StructuralPattern) MatchCommand(cmd ast.SimpleCommand) bool {
	// a. When must match.
	if !sp.When.Match(cmd, sp.CaseSensitive) {
		return false
	}
	// b. If Unless matches, exempt (no match).
	if !sp.Unless.IsEmpty() && sp.Unless.Match(cmd, sp.CaseSensitive) {
		return false
	}
	// c. Match — pattern triggers.
	return true
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

// containsCI checks if any element in the slice equals s (case-insensitive).
func containsCI(slice []string, s string) bool {
	for _, v := range slice {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

// strEq compares two strings, optionally case-insensitive.
func strEq(a, b string, caseSensitive bool) bool {
	if caseSensitive {
		return a == b
	}
	return strings.EqualFold(a, b)
}

// flagMatches checks whether a command flag (cmdFlag) matches a pattern flag
// (patternFlag). It handles:
//   - Exact match (case-sensitive or case-insensitive based on cs)
//   - Combined short flags: cmdFlag "-rf" contains single-char pattern "-r"
//   - Equals-sign flags: cmdFlag "--force-with-lease=origin" matches pattern
//     "--force-with-lease"
func flagMatches(cmdFlag, patternFlag string, cs bool) bool {
	// Exact match.
	if strEq(cmdFlag, patternFlag, cs) {
		return true
	}

	// Combined short flags: pattern is a single short flag like "-r",
	// and cmdFlag is a combined short flag like "-rf". Check if the
	// pattern's character appears in the combined flag.
	if len(patternFlag) == 2 && patternFlag[0] == '-' && patternFlag[1] != '-' &&
		len(cmdFlag) > 2 && cmdFlag[0] == '-' && cmdFlag[1] != '-' {
		pChar := patternFlag[1]
		flagChars := cmdFlag[1:]
		if cs {
			if strings.ContainsRune(flagChars, rune(pChar)) {
				return true
			}
		} else {
			if strings.ContainsRune(strings.ToLower(flagChars), rune(toLowerByte(pChar))) {
				return true
			}
		}
	}

	// Equals-sign handling: cmdFlag "--force-with-lease=origin/feature"
	// should match pattern "--force-with-lease".
	if idx := strings.IndexByte(cmdFlag, '='); idx >= 0 {
		cmdBase := cmdFlag[:idx]
		if strEq(cmdBase, patternFlag, cs) {
			return true
		}
	}

	return false
}

func toLowerByte(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

// hasAnyFlag returns true if at least one pattern flag matches any cmd flag (OR).
func hasAnyFlag(cmdFlags, patternFlags []string, cs bool) bool {
	for _, pf := range patternFlags {
		for _, cf := range cmdFlags {
			if flagMatches(cf, pf, cs) {
				return true
			}
		}
	}
	return false
}

// hasAllFlags returns true if every pattern flag matches some cmd flag (AND).
func hasAllFlags(cmdFlags, patternFlags []string, cs bool) bool {
	for _, pf := range patternFlags {
		found := false
		for _, cf := range cmdFlags {
			if flagMatches(cf, pf, cs) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// hasArgExact returns true if any cmd arg exactly equals one of the listed values.
func hasArgExact(cmdArgs, values []string, cs bool) bool {
	for _, arg := range cmdArgs {
		for _, v := range values {
			if strEq(arg, v, cs) {
				return true
			}
		}
	}
	return false
}

// hasArgPrefix returns true if any cmd arg starts with one of the listed prefixes.
func hasArgPrefix(cmdArgs, prefixes []string, cs bool) bool {
	for _, arg := range cmdArgs {
		for _, p := range prefixes {
			if cs {
				if strings.HasPrefix(arg, p) {
					return true
				}
			} else {
				if len(arg) >= len(p) && strings.EqualFold(arg[:len(p)], p) {
					return true
				}
			}
		}
	}
	return false
}

// hasAllArgPrefix returns true if ALL cmd args start with at least one of the listed prefixes.
// Returns false if there are no args (prevents vacuous truth for security rules).
func hasAllArgPrefix(cmdArgs, prefixes []string, cs bool) bool {
	if len(cmdArgs) == 0 {
		return false
	}
	for _, arg := range cmdArgs {
		matched := false
		for _, p := range prefixes {
			if cs {
				if strings.HasPrefix(arg, p) {
					matched = true
					break
				}
			} else {
				if len(arg) >= len(p) && strings.EqualFold(arg[:len(p)], p) {
					matched = true
					break
				}
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// hasArgContains returns true if any cmd arg contains one of the listed
// substrings (always case-insensitive regardless of caseSensitive setting).
func hasArgContains(cmdArgs, substrings []string) bool {
	for _, arg := range cmdArgs {
		for _, sub := range substrings {
			if containsFold(arg, sub) {
				return true
			}
		}
	}
	return false
}

// containsFold checks if s contains sub (case-insensitive).
func containsFold(s, sub string) bool {
	if sub == "" {
		return true
	}
	if len(sub) > len(s) {
		return false
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}
