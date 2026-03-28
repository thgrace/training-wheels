package rules

import (
	"github.com/thgrace/training-wheels/internal/ast"
	"github.com/thgrace/training-wheels/internal/logger"
	"github.com/thgrace/training-wheels/internal/packs"
)

// ConvertToPack builds a Pack from deny/ask rule entries.
// v2 "command" entries become structural patterns.
// Legacy v1 regex/exact/prefix entries are skipped (no longer supported).
// Self-lockout safety: tw commands are handled by the evaluator checking
// for tw command prefixes before pack evaluation.
func ConvertToPack(entries []RuleEntry, packID string) *packs.Pack {
	p := &packs.Pack{
		ID:   packID,
		Name: packID,
	}

	var keywords []string

	for _, e := range entries {
		switch e.Action {
		case "deny", "ask":
			sev, err := packs.ParseSeverity(e.Severity)
			if err != nil {
				sev = packs.SeverityMedium
			}

			var suggestions []packs.PatternSuggestion
			for _, s := range e.Suggestions {
				suggestions = append(suggestions, packs.PatternSuggestion{
					Command:     s.Command,
					Description: s.Description,
				})
			}

			action := "deny"
			if e.Action == "ask" {
				action = "ask"
			}

			if e.Kind == "command" && e.When != nil {
				// v2: structural pattern.
				sp := packs.StructuralPattern{
					Name:        e.Name,
					When:        *e.When,
					Reason:      e.Reason,
					Severity:    sev,
					Explanation: e.Explanation,
					Suggestions: suggestions,
					Action:      action,
				}
				if e.Unless != nil {
					sp.Unless = *e.Unless
				}
				p.StructuralPatterns = append(p.StructuralPatterns, sp)

				// Keywords from When.Command.
				if len(e.Keywords) > 0 {
					keywords = append(keywords, e.Keywords...)
				} else {
					keywords = append(keywords, e.When.Command...)
				}
			}
			// Legacy v1 regex/exact/prefix entries are skipped.
			if e.Kind != "command" {
				logger.Warn("legacy rule skipped (migrate to --command)", "name", e.Name, "kind", e.Kind)
			}
		}
	}

	p.Keywords = keywords
	return p
}

// AllowEntry represents an allow rule for pre-pack evaluation.
type AllowEntry struct {
	Name    string
	Kind    string // "command", "rule", or legacy: "exact", "prefix"
	Pattern string // "rule" kind: rule ID; legacy: match pattern
	Reason  string
	Shell   ast.ShellType           // shell type for AST parsing
	When    *packs.PatternCondition // v2 "command" kind
	Unless  *packs.PatternCondition // v2 "command" kind
}

// Matches checks if this allow entry matches a command or rule ID.
func (a *AllowEntry) Matches(command, ruleID string) bool {
	return a.MatchesParsed(command, ruleID, nil)
}

// MatchesParsed checks if this allow entry matches using pre-parsed commands when available.
func (a *AllowEntry) MatchesParsed(command, ruleID string, parsed []ast.SimpleCommand) bool {
	switch a.Kind {
	case "command":
		if a.When == nil {
			return false
		}
		cmds := parsed
		if cmds == nil {
			cc := ast.ParseShell([]byte(command), a.Shell)
			if cc == nil {
				return false
			}
			cc = ast.Unwrap(cc, a.Shell)
			cmds = cc.AllCommands()
			ast.EnrichCommands(cmds)
		}
		for _, sc := range cmds {
			if a.When.Match(sc, false) {
				if a.Unless != nil && !a.Unless.IsEmpty() && a.Unless.Match(sc, false) {
					continue
				}
				return true
			}
		}
		return false
	case "rule":
		return a.Pattern == ruleID
	case "exact":
		return command == a.Pattern
	case "prefix":
		return len(command) >= len(a.Pattern) && command[:len(a.Pattern)] == a.Pattern
	default:
		return false
	}
}

// ConvertToAllowEntries extracts allow rules for pre-pack and post-pack evaluation.
// The shell parameter sets the AST shell type for "command" kind matching.
func ConvertToAllowEntries(entries []RuleEntry, shell ast.ShellType) []AllowEntry {
	var allows []AllowEntry
	for _, e := range entries {
		if e.Action == "allow" {
			ae := AllowEntry{
				Name:    e.Name,
				Kind:    e.Kind,
				Pattern: e.Pattern,
				Reason:  e.Reason,
				Shell:   shell,
			}
			if e.Kind == "command" {
				ae.When = e.When
				ae.Unless = e.Unless
			}
			allows = append(allows, ae)
		}
	}
	return allows
}

// CheckAllow checks if a command or rule ID matches any allow entry.
// For pre-pack checks, pass ruleID as "".
// For post-pack checks, pass the matched ruleID.
func CheckAllow(command, ruleID string, entries []AllowEntry) *AllowEntry {
	return CheckAllowParsed(command, ruleID, nil, entries)
}

// CheckAllowParsed checks if a command or rule ID matches any allow entry using pre-parsed commands when available.
func CheckAllowParsed(command, ruleID string, parsed []ast.SimpleCommand, entries []AllowEntry) *AllowEntry {
	for i := range entries {
		if entries[i].MatchesParsed(command, ruleID, parsed) {
			return &entries[i]
		}
	}
	return nil
}

// HasCommandAllowEntries reports whether any allow entry requires parsed command matching.
func HasCommandAllowEntries(entries []AllowEntry) bool {
	for i := range entries {
		if entries[i].Kind == "command" {
			return true
		}
	}
	return false
}
