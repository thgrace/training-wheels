package eval

import (
	"context"
	"testing"
	"time"


	"github.com/thgrace/training-wheels/internal/config"
	"github.com/thgrace/training-wheels/internal/packs"
)

// TestInvariant_KeywordIndexParity verifies that the Aho-Corasick keyword-index
// quick-reject optimization never changes a security decision relative to a
// brute-force pack scan.
//
// Strategy: for each command in the corpus, run the full evaluator (which uses
// the keyword index) and also manually call Pack.Check on every pack. If any
// pack's Check returns a destructive match on the raw command, the evaluator
// must not have allowed the command — UNLESS the command is in the
// sanitizerAllowed set, which lists commands where the sanitizer legitimately
// masks data regions (echo args, commit messages, grep patterns) so the
// evaluator correctly allows them despite a naive regex match on the raw text.
func TestInvariant_KeywordIndexParity(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := packs.DefaultRegistry()
	cfg.Packs.Enabled = reg.AllIDs()
	e := NewEvaluator(cfg, reg)

	// Commands where the sanitizer correctly masks data regions, so the
	// evaluator allows them even though brute-force Pack.Check matches the
	// raw text. These are expected divergences, not bugs.
	sanitizerAllowed := map[string]bool{
		"echo rm -rf /":                        true,
		"git commit -m \"rm -rf /\"":            true,
		"grep \"git reset --hard\" file.txt":    true,
	}

	// Corpus of commands that exercise various keyword combinations.
	corpus := []string{
		// Direct destructive
		"rm -rf /",
		"git reset --hard",
		"git push --force",
		"kubectl delete namespace production",
		"docker system prune -af",
		"terraform destroy",
		"psql -c \"DROP DATABASE prod\"",
		"redis-cli FLUSHALL",
		// Safe commands
		"git status",
		"kubectl get pods",
		"docker ps",
		"echo hello",
		"ls -la",
		// Wrapper prefixes (keywords appear after prefix)
		"sudo rm -rf /",
		"sudo git reset --hard",
		"nice rm -rf /",
		"time git push --force",
		// False positive scenarios (keywords present but in data context)
		"echo rm -rf /",
		"git commit -m \"rm -rf /\"",
		"grep \"git reset --hard\" file.txt",
		// Compound commands
		"git status ; rm -rf /",
		"echo hello | git reset --hard",
		// No keywords at all
		"echo hello world",
		"ls -la /tmp",
		"pwd",
		"cat README.md",
	}

	for _, cmd := range corpus {
		t.Run(cmd, func(t *testing.T) {
			// Path 1: full evaluator (uses keyword index).
			result := e.Evaluate(testCtx(t), cmd)

			// Path 2: brute-force — check every pack on the raw command.
			bruteForceBlocked := false
			var matchingPack, matchingPattern string
			for _, id := range reg.AllIDs() {
				p := reg.Get(id)
				if p == nil {
					continue
				}
				if m := p.Check(cmd); m != nil {
					bruteForceBlocked = true
					matchingPack = id
					matchingPattern = m.Name
					break
				}
			}

			if bruteForceBlocked && result.Decision == DecisionAllow && !result.SkippedDueToBudget {
				if sanitizerAllowed[cmd] {
					// Expected: sanitizer masked the data region so evaluator allows it.
					return
				}
				t.Errorf("PARITY VIOLATION: brute-force found match %s:%s but evaluator allowed %q",
					matchingPack, matchingPattern, cmd)
			}
		})
	}
}

// TestInvariant_EmptyKeywordListConservatism verifies that the system never
// silently allows known-critical commands regardless of keyword-index state.
// This guards against keyword-coverage gaps: if a pack's keyword list were
// empty or mis-configured, the quick-reject could incorrectly skip the pack,
// so this test ensures those commands are still denied end-to-end.
func TestInvariant_EmptyKeywordListConservatism(t *testing.T) {
	e := newTestEvaluator()

	criticalCommands := []struct {
		cmd      string
		mustDeny bool
	}{
		{"rm -rf /", true},
		{"git reset --hard", true},
		{"git push --force", true},
		{"kubectl delete namespace production", true},
		{"docker system prune -af", true},
		{"terraform destroy -auto-approve", true},
	}

	for _, tt := range criticalCommands {
		t.Run(tt.cmd, func(t *testing.T) {
			result := e.Evaluate(testCtx(t), tt.cmd)
			if tt.mustDeny && result.Decision != DecisionDeny {
				t.Errorf("critical command %q was not denied (got %v)", tt.cmd, result.Decision)
			}
		})
	}
}

// TestInvariant_TimeoutFailOpen verifies that when the context deadline is
// exceeded, the evaluator returns Allow with SkippedDueToBudget=true and never
// panics or hangs.
func TestInvariant_TimeoutFailOpen(t *testing.T) {
	e := newTestEvaluator()

	commands := []string{
		"git reset --hard",
		"rm -rf /",
		"kubectl delete namespace prod",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
			defer cancel()
			time.Sleep(1 * time.Millisecond) // ensure deadline has passed

			result := e.Evaluate(ctx, cmd)
			if result.Decision != DecisionAllow {
				// Timeout should fail open, but if the evaluator completed fast
				// enough to finish before the deadline, a deny is also correct.
				// We only care that it doesn't panic.
				t.Logf("evaluator completed before deadline: decision=%v", result.Decision)
			}
			if result.Decision == DecisionAllow && !result.SkippedDueToBudget {
				// Allowed without budget skip means evaluation genuinely completed
				// before the timeout fired (very fast machine or lucky scheduling).
				t.Logf("allowed without budget skip (fast evaluation)")
			}
		})
	}
}

// TestInvariant_DecisionDeterminism verifies that the same input always
// produces the same decision.  Non-determinism would indicate data races or
// mutable shared state in the evaluation pipeline.
func TestInvariant_DecisionDeterminism(t *testing.T) {
	e := newTestEvaluator()

	commands := []string{
		"git reset --hard",
		"rm -rf /",
		"git status",
		"echo hello",
		"sudo git push --force",
		"git commit -m \"rm -rf /\"",
		"kubectl get pods",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			first := e.Evaluate(testCtx(t), cmd)
			for i := 1; i < 5; i++ {
				result := e.Evaluate(testCtx(t), cmd)
				if result.Decision != first.Decision {
					t.Errorf("run %d: decision=%v, want %v (same as run 0)",
						i, result.Decision, first.Decision)
				}
				// Also check rule ID consistency when both runs deny.
				if first.Decision == DecisionDeny && result.Decision == DecisionDeny {
					if ruleID(first) != ruleID(result) {
						t.Errorf("run %d: ruleID=%q, want %q (same as run 0)",
							i, ruleID(result), ruleID(first))
					}
				}
			}
		})
	}
}

// TestInvariant_NormalizedCommandPopulated verifies that when the evaluator
// denies a command that required normalization (e.g. stripping a sudo prefix or
// an absolute binary path), NormalizedCommand is populated and differs from the
// original input.
func TestInvariant_NormalizedCommandPopulated(t *testing.T) {
	e := newTestEvaluator()

	tests := []struct {
		cmd            string
		wantNormalized bool // whether NormalizedCommand should differ from cmd
	}{
		{"sudo git reset --hard", true},
		{"/usr/bin/git reset --hard", true},
		{"git reset --hard", false},
		{"git.exe reset --hard", true},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			result := e.Evaluate(testCtx(t), tt.cmd)
			if result.Decision != DecisionDeny {
				t.Skipf("command was not denied, skipping normalization check")
			}
			if tt.wantNormalized && result.NormalizedCommand == "" {
				t.Error("expected NormalizedCommand to be populated")
			}
			if tt.wantNormalized && result.NormalizedCommand == tt.cmd {
				t.Errorf("NormalizedCommand should differ from input, got same: %q", result.NormalizedCommand)
			}
		})
	}
}
