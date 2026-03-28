package matchutil

import "testing"

func TestMatchRule(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		ruleID  string
		want    bool
	}{
		{"exact match", "core.git:reset-hard", "core.git:reset-hard", true},
		{"exact no match", "core.git:reset-hard", "core.git:push-force", false},
		{"global wildcard", "*", "anything:here", true},
		{"suffix wildcard", "core.git:*", "core.git:push-force", true},
		{"suffix wildcard no match", "core.git:*", "core.filesystem:rm-rf", false},
		{"multi wildcard", "core.*:reset-*", "core.git:reset-hard", true},
		{"multi wildcard 2", "core.*:reset-*", "core.filesystem:reset-soft", true},
		{"multi wildcard no match", "core.*:reset-*", "core.git:push-force", false},
		{"middle wildcard", "core.*:reset-hard", "core.git:reset-hard", true},
		{"prefix wildcard", "*:reset-hard", "core.git:reset-hard", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchRule(tt.pattern, tt.ruleID); got != tt.want {
				t.Errorf("MatchRule(%q, %q) = %v, want %v", tt.pattern, tt.ruleID, got, tt.want)
			}
		})
	}
}
