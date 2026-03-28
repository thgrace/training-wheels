// Package packs provides the pattern pack system for TW.
package packs

import (
	"fmt"
	"strings"

	"github.com/thgrace/training-wheels/internal/ast"
)

// Severity indicates how dangerous a destructive pattern is.
type Severity int

const (
	SeverityCritical Severity = iota
	SeverityHigh
	SeverityMedium
	SeverityLow
)

func (s Severity) String() string {
	switch s {
	case SeverityCritical:
		return "critical"
	case SeverityHigh:
		return "high"
	case SeverityMedium:
		return "medium"
	case SeverityLow:
		return "low"
	default:
		return "unknown"
	}
}

// ParseSeverity converts a JSON/string severity to the runtime enum.
func ParseSeverity(s string) (Severity, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return SeverityCritical, nil
	case "high":
		return SeverityHigh, nil
	case "medium":
		return SeverityMedium, nil
	case "low":
		return SeverityLow, nil
	default:
		return SeverityLow, fmt.Errorf("unknown severity %q", s)
	}
}

// Platform indicates which OS a suggestion applies to.
type Platform int

const (
	PlatformAll Platform = iota
	PlatformLinux
	PlatformMacOS
	PlatformWindows
	PlatformBSD
)

func (p Platform) String() string {
	switch p {
	case PlatformLinux:
		return "linux"
	case PlatformMacOS:
		return "macos"
	case PlatformWindows:
		return "windows"
	case PlatformBSD:
		return "bsd"
	default:
		return "all"
	}
}

// ParsePlatform converts a JSON/string platform to the runtime enum.
func ParsePlatform(s string) (Platform, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "all":
		return PlatformAll, nil
	case "linux":
		return PlatformLinux, nil
	case "macos":
		return PlatformMacOS, nil
	case "windows":
		return PlatformWindows, nil
	case "bsd":
		return PlatformBSD, nil
	default:
		return PlatformAll, fmt.Errorf("unknown platform %q", s)
	}
}

// PatternSuggestion is a safer alternative shown when a command is denied.
type PatternSuggestion struct {
	Command     string
	Description string
	Platform    Platform
}

// DestructiveMatch is the result of a destructive pattern match.
type DestructiveMatch struct {
	Name        string
	Reason      string
	Severity    Severity
	Explanation string
	Suggestions []PatternSuggestion
	MatchStart  int
	MatchEnd    int
	Action      string // "deny" (default) or "ask"; empty means "deny"
}

// Pack is a named collection of structural patterns.
type Pack struct {
	ID                 string
	Name               string
	Description        string
	Keywords           []string
	StructuralPatterns []StructuralPattern // v2: when/unless structural patterns
}

// CheckStructural evaluates pre-parsed SimpleCommands against v2 structural patterns.
func (p *Pack) CheckStructural(cmds []ast.SimpleCommand) *DestructiveMatch {
	for _, sc := range cmds {
		for i := range p.StructuralPatterns {
			sp := &p.StructuralPatterns[i]
			if sp.MatchCommand(sc) {
				return &DestructiveMatch{
					Name:        sp.Name,
					Reason:      sp.Reason,
					Severity:    sp.Severity,
					Explanation: sp.Explanation,
					Suggestions: sp.Suggestions,
					Action:      sp.Action,
				}
			}
		}
	}
	return nil
}

// Check parses the command through the AST and evaluates structural patterns.
// Returns nil if the command doesn't match any pattern.
func (p *Pack) Check(cmd string) *DestructiveMatch {
	if len(p.StructuralPatterns) == 0 {
		return nil
	}
	cc := ast.Parse([]byte(cmd))
	if cc == nil {
		return nil
	}
	cc = ast.Unwrap(cc, ast.ShellBash)
	cmds := cc.AllCommands()
	ast.EnrichCommands(cmds)
	return p.CheckStructural(cmds)
}
