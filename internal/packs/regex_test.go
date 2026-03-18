package packs_test

import (
	"testing"

	"github.com/thgrace/training-wheels/internal/packs"
)

func TestValidateRegex_ValidPatterns(t *testing.T) {
	valid := []string{
		`^hello$`,
		`\bfoo\b`,
		`rm\s+-rf\s+/`,
		`(?i)drop\s+database`,
		`[a-z]+`,
		`a{1,3}`,
		`(?:group)`,
	}
	for _, pattern := range valid {
		t.Run(pattern, func(t *testing.T) {
			if err := packs.ValidateRegex(pattern); err != nil {
				t.Errorf("ValidateRegex(%q) = %v; want nil", pattern, err)
			}
		})
	}
}

func TestValidateRegex_InvalidPatterns(t *testing.T) {
	invalid := []string{
		`(`,
		`[`,
		`*`,
		`(?P<name`,
		`\`,
	}
	for _, pattern := range invalid {
		t.Run(pattern, func(t *testing.T) {
			if err := packs.ValidateRegex(pattern); err == nil {
				t.Errorf("ValidateRegex(%q) = nil; want error", pattern)
			}
		})
	}
}

func TestValidateRegex_BacktrackingPatterns(t *testing.T) {
	// Valid patterns that require the backtracking engine
	valid := []string{
		`foo(?=bar)`,
		`(?<=baz)qux`,
		`(?!bad)good`,
	}
	for _, pattern := range valid {
		t.Run(pattern, func(t *testing.T) {
			if err := packs.ValidateRegex(pattern); err != nil {
				t.Errorf("ValidateRegex(%q) = %v; want nil", pattern, err)
			}
		})
	}
}
