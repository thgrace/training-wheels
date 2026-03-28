package eval

import "testing"

func TestRepro_TWOverrideInCommitMessage(t *testing.T) {
	e := newTestEvaluator()

	tests := []struct {
		name string
		cmd  string
		want EvaluationDecision
	}{
		{"quoted double", `git commit -m "tw override stuff"`, DecisionAllow},
		{"quoted single", `git commit -m 'Refactor tw override handling'`, DecisionAllow},
		{"long message", `git commit -m "Add tw override support for rules"`, DecisionAllow},
		{"--message flag", `git commit --message "tw override changes"`, DecisionAllow},
		{"unquoted", `git commit -m tw override stuff`, DecisionAllow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Evaluate(testCtx(t), tt.cmd)
			if result.Decision != tt.want {
				rID := ""
				if result.PatternInfo != nil {
					rID = result.PatternInfo.RuleID
				}
				t.Errorf("cmd=%q: got %v (rule: %s), want %v", tt.cmd, result.Decision, rID, tt.want)
			}
		})
	}
}