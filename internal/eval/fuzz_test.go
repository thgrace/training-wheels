package eval

import (
	"context"
	"testing"
	"time"


	"github.com/thgrace/training-wheels/internal/config"
	"github.com/thgrace/training-wheels/internal/packs"
)

func FuzzEvaluateCommand(f *testing.F) {
	seeds := []string{
		"git status",
		"rm -rf /",
		"git reset --hard",
		"sudo rm -rf /",
		"echo hello",
		"",
		`git commit -m "rm -rf /"`,
		`bash -c "rm -rf /"`,
		"kubectl delete namespace production",
		"docker system prune -af",
		"git push --force",
		"echo rm -rf / # comment",
		"ls -la # rm -rf /",
		`echo "$(echo " ) " )"; rm -rf /`,
		"git re\\\nset --hard",
		"nice rm -rf /",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	// Build evaluator once, reuse across fuzz iterations
	cfg := config.DefaultConfig()
	reg := packs.DefaultRegistry()
	cfg.Packs.Enabled = reg.AllIDs()
	e := NewEvaluator(cfg, reg)

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 10_000 {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Must not panic
		result := e.Evaluate(ctx, input)

		// Decision must be a valid value
		switch result.Decision {
		case DecisionAllow, DecisionDeny, DecisionWarn:
			// ok
		default:
			t.Errorf("invalid decision: %v", result.Decision)
		}

		// If denied, PatternInfo should be non-nil
		if result.Decision == DecisionDeny && result.PatternInfo == nil {
			t.Error("decision is Deny but PatternInfo is nil")
		}
	})
}
