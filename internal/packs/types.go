// Package packs provides the pattern pack system for TW.
package packs

import (
	"fmt"
	"strings"
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

// SafePattern is a whitelist regex. If matched, the command is allowed.
type SafePattern struct {
	Name  string
	Regex *LazyRegex
}

// DestructivePattern is a blacklist regex. If matched, the command is denied.
type DestructivePattern struct {
	Name        string
	Regex       *LazyRegex
	Reason      string
	Severity    Severity
	Explanation string
	Suggestions []PatternSuggestion
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
}

// Pack is a named collection of safe and destructive patterns.
type Pack struct {
	ID                  string
	Name                string
	Description         string
	Keywords            []string
	SafePatterns        []SafePattern
	DestructivePatterns []DestructivePattern
}

// MatchesSafe returns true if any safe pattern matches the command.
func (p *Pack) MatchesSafe(cmd string) bool {
	for i := range p.SafePatterns {
		if p.SafePatterns[i].Regex.IsMatch(cmd) {
			return true
		}
	}
	return false
}

// MatchesDestructive returns the first destructive pattern match, or nil.
func (p *Pack) MatchesDestructive(cmd string) *DestructiveMatch {
	for i := range p.DestructivePatterns {
		dp := &p.DestructivePatterns[i]
		start, end, ok := dp.Regex.FindIndex(cmd)
		if ok {
			return &DestructiveMatch{
				Name:        dp.Name,
				Reason:      dp.Reason,
				Severity:    dp.Severity,
				Explanation: dp.Explanation,
				Suggestions: dp.Suggestions,
				MatchStart:  start,
				MatchEnd:    end,
			}
		}
	}
	return nil
}

// Check runs safe patterns then destructive patterns.
// Returns nil if the command is safe or doesn't match any pattern.
func (p *Pack) Check(cmd string) *DestructiveMatch {
	if p.MatchesSafe(cmd) {
		return nil
	}
	return p.MatchesDestructive(cmd)
}
