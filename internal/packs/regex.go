package packs

import (
	"regexp"
	"strings"
	"sync"

	"github.com/dlclark/regexp2"
	"github.com/thgrace/training-wheels/internal/logger"
)

// CompiledRegex abstracts over both RE2 and backtracking regex engines.
type CompiledRegex interface {
	IsMatch(s string) bool
	FindIndex(s string) (start, end int, ok bool)
}

// linearRegex wraps Go's stdlib regexp (RE2, linear time).
type linearRegex struct {
	re *regexp.Regexp
}

func (r *linearRegex) IsMatch(s string) bool {
	return r.re.MatchString(s)
}

func (r *linearRegex) FindIndex(s string) (start, end int, ok bool) {
	loc := r.re.FindStringIndex(s)
	if loc == nil {
		return 0, 0, false
	}
	return loc[0], loc[1], true
}

// backtrackingRegex wraps regexp2 (supports lookahead/lookbehind).
type backtrackingRegex struct {
	re *regexp2.Regexp
}

func (r *backtrackingRegex) IsMatch(s string) bool {
	ok, _ := r.re.MatchString(s)
	return ok
}

func (r *backtrackingRegex) FindIndex(s string) (start, end int, ok bool) {
	m, _ := r.re.FindStringMatch(s)
	if m == nil {
		return 0, 0, false
	}
	return m.Index, m.Index + m.Length, true
}

// NeedsBacktracking returns true if the pattern uses features that require
// a backtracking engine (lookahead, lookbehind, atomic groups, possessive
// quantifiers, or backreferences).
func NeedsBacktracking(pattern string) bool {
	if strings.Contains(pattern, "(?=") ||
		strings.Contains(pattern, "(?!") ||
		strings.Contains(pattern, "(?<=") ||
		strings.Contains(pattern, "(?<!") ||
		strings.Contains(pattern, "(?>") {
		return true
	}
	// Check for possessive quantifiers: *+, ++, ?+, }+
	for i := 0; i < len(pattern)-1; i++ {
		if pattern[i+1] == '+' {
			switch pattern[i] {
			case '*', '?':
				return true
			case '+':
				// ++ is possessive
				return true
			case '}':
				return true
			}
		}
	}
	// Check for backreferences \1 through \9
	for i := 0; i < len(pattern)-1; i++ {
		if pattern[i] == '\\' && pattern[i+1] >= '1' && pattern[i+1] <= '9' {
			// Not a backreference if preceded by another backslash
			if i > 0 && pattern[i-1] == '\\' {
				continue
			}
			return true
		}
	}
	return false
}

// ValidateRegex checks that a pattern compiles without error.
// Used at load time to reject invalid regexes eagerly.
func ValidateRegex(pattern string) error {
	_, err := CompileRegex(pattern)
	return err
}

// CompileRegex compiles a pattern, auto-selecting the appropriate engine.
func CompileRegex(pattern string) (CompiledRegex, error) {
	if NeedsBacktracking(pattern) {
		re, err := regexp2.Compile(pattern, regexp2.RE2)
		if err != nil {
			// Try without RE2 flag for full .NET regex support
			re, err = regexp2.Compile(pattern, 0)
			if err != nil {
				return nil, err
			}
		}
		return &backtrackingRegex{re: re}, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &linearRegex{re: re}, nil
}

// LazyRegex compiles a regex on first use via sync.Once.
type LazyRegex struct {
	pattern  string
	once     sync.Once
	compiled CompiledRegex
	err      error
}

// NewLazyRegex creates a LazyRegex that will compile on first use.
func NewLazyRegex(pattern string) *LazyRegex {
	return &LazyRegex{pattern: pattern}
}

func (lr *LazyRegex) init() {
	lr.compiled, lr.err = CompileRegex(lr.pattern)
	if lr.err != nil {
		logger.Error("regex compile error (pattern will be skipped)",
			"pattern", lr.pattern,
			"error", lr.err)
	}
}

// IsMatch returns true if the pattern matches. Fail-open on compile error.
func (lr *LazyRegex) IsMatch(s string) bool {
	lr.once.Do(lr.init)
	if lr.err != nil {
		return false // fail-open
	}
	return lr.compiled.IsMatch(s)
}

// FindIndex returns the byte offsets of the first match. Fail-open on error.
func (lr *LazyRegex) FindIndex(s string) (start, end int, ok bool) {
	lr.once.Do(lr.init)
	if lr.err != nil {
		return 0, 0, false // fail-open
	}
	return lr.compiled.FindIndex(s)
}

// Pattern returns the raw pattern string.
func (lr *LazyRegex) Pattern() string {
	return lr.pattern
}
