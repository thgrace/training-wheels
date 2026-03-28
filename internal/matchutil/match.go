// Package matchutil provides shared string and pattern matching utilities.
package matchutil

import "strings"

// MatchRule matches a rule ID pattern against a concrete rule ID.
// Supports * as a wildcard for any sequence of characters (including empty).
// Multiple wildcards are supported.
// e.g., "core.git:*" matches "core.git:reset-hard"
// e.g., "core.*:*" matches "core.git:reset-hard"
// e.g., "core.*:reset-*" matches "core.git:reset-hard"
func MatchRule(pattern, ruleID string) bool {
	if pattern == "*" {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return pattern == ruleID
	}
	// Split pattern on * to get literal segments, then verify
	// ruleID contains all segments in order.
	segments := strings.Split(pattern, "*")

	// The ruleID must start with the first segment.
	if !strings.HasPrefix(ruleID, segments[0]) {
		return false
	}
	// Walk through ruleID, matching each segment in order.
	remaining := ruleID[len(segments[0]):]
	for _, seg := range segments[1:] {
		idx := strings.Index(remaining, seg)
		if idx < 0 {
			return false
		}
		remaining = remaining[idx+len(seg):]
	}
	// If the pattern does not end with *, the ruleID must end exactly
	// at the last segment (no trailing characters allowed).
	if !strings.HasSuffix(pattern, "*") && remaining != "" {
		return false
	}
	return true
}
