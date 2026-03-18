package eval

import (
	"context"
	"testing"
	"time"

	"github.com/thgrace/training-wheels/internal/config"
	"github.com/thgrace/training-wheels/internal/packs"
)

// newTestEvaluator creates an evaluator with all packs enabled for security testing.
func newTestEvaluator() *Evaluator {
	cfg := config.DefaultConfig()
	reg := packs.DefaultRegistry()
	cfg.Packs.Enabled = reg.AllIDs()
	return NewEvaluator(cfg, reg)
}

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// --- Phase A "Done When" criteria ---

func TestSecurity_GitCommitMessageAllowed(t *testing.T) {
	// git commit -m "rm -rf /" → Allow (commit message is data, not executed)
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), `git commit -m "rm -rf /"`)
	if result.Decision != DecisionAllow {
		t.Errorf("git commit -m should be ALLOW, got %v (rule: %s)",
			result.Decision, ruleID(result))
	}
}

func TestSecurity_GrepDestructivePatternAllowed(t *testing.T) {
	// grep "git reset --hard" file.txt → Allow (search pattern is data)
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), `grep "git reset --hard" file.txt`)
	if result.Decision != DecisionAllow {
		t.Errorf("grep search pattern should be ALLOW, got %v (rule: %s)",
			result.Decision, ruleID(result))
	}
}

func TestSecurity_EchoDropTableAllowed(t *testing.T) {
	// echo "DROP TABLE users" → Allow (echo args are data)
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), `echo "DROP TABLE users"`)
	if result.Decision != DecisionAllow {
		t.Errorf("echo should be ALLOW, got %v (rule: %s)",
			result.Decision, ruleID(result))
	}
}

func TestSecurity_PsqlSelectStringLiteralAllowed(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), `psql -c "SELECT 'DROP TABLE users';"`)
	if result.Decision != DecisionAllow {
		t.Errorf("psql SELECT string literal should be ALLOW, got %v (rule: %s)",
			result.Decision, ruleID(result))
	}
}

func TestSecurity_SQLiteSelectStringLiteralAllowed(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), `sqlite3 app.db "SELECT 'DROP TABLE users';"`)
	if result.Decision != DecisionAllow {
		t.Errorf("sqlite SELECT string literal should be ALLOW, got %v (rule: %s)",
			result.Decision, ruleID(result))
	}
}

func TestSecurity_RmRfStillDenied(t *testing.T) {
	// rm -rf / → still Deny
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), "rm -rf /")
	if result.Decision != DecisionDeny {
		t.Error("rm -rf / should be DENY")
	}
}

func TestSecurity_BashCStillDenied(t *testing.T) {
	// bash -c "rm -rf /" → still Deny (inline code, not data)
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), `bash -c "rm -rf /"`)
	if result.Decision != DecisionDeny {
		t.Errorf("bash -c 'rm -rf /' should be DENY, got %v", result.Decision)
	}
}

func TestSecurity_GitResetHardStillDenied(t *testing.T) {
	// Direct git reset --hard → still Deny
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), "git reset --hard")
	if result.Decision != DecisionDeny {
		t.Error("git reset --hard should be DENY")
	}
}

func TestSecurity_GitPushForceStillDenied(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), "git push --force")
	if result.Decision != DecisionDeny {
		t.Error("git push --force should be DENY")
	}
}

func TestSecurity_TerraformPlanDestroyAllowed(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), "terraform plan -destroy")
	if result.Decision != DecisionAllow {
		t.Errorf("terraform plan -destroy should be ALLOW, got %v (rule: %s)",
			result.Decision, ruleID(result))
	}
}

func TestSecurity_AnsiblePlaybookAllowed(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), "ansible-playbook -i inventory ping.yml")
	if result.Decision != DecisionAllow {
		t.Errorf("ansible-playbook ping should be ALLOW, got %v (rule: %s)",
			result.Decision, ruleID(result))
	}
}

func TestSecurity_KubectlForceConflictsAllowed(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), "kubectl apply --server-side --force-conflicts -f deploy.yaml")
	if result.Decision != DecisionAllow {
		t.Errorf("kubectl apply --force-conflicts should be ALLOW, got %v (rule: %s)",
			result.Decision, ruleID(result))
	}
}

// --- Additional false-positive scenarios ---

func TestSecurity_GitCommitMessageSingleQuotes(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), `git commit -m 'rm -rf /'`)
	if result.Decision != DecisionAllow {
		t.Errorf("git commit -m (single quotes) should be ALLOW, got %v (rule: %s)",
			result.Decision, ruleID(result))
	}
}

func TestSecurity_EchoGitResetAllowed(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), `echo "git reset --hard"`)
	if result.Decision != DecisionAllow {
		t.Errorf("echo git reset should be ALLOW, got %v (rule: %s)",
			result.Decision, ruleID(result))
	}
}

func TestSecurity_PrintfDestructiveAllowed(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), `printf "DROP DATABASE production\n"`)
	if result.Decision != DecisionAllow {
		t.Errorf("printf should be ALLOW, got %v (rule: %s)",
			result.Decision, ruleID(result))
	}
}

func TestSecurity_GrepEFlagAllowed(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), `grep -e "rm -rf" /var/log/audit.log`)
	if result.Decision != DecisionAllow {
		t.Errorf("grep -e should be ALLOW, got %v (rule: %s)",
			result.Decision, ruleID(result))
	}
}

func TestSecurity_GitGrepAllowed(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), `git --grep "reset --hard" log`)
	if result.Decision != DecisionAllow {
		t.Errorf("git --grep should be ALLOW, got %v (rule: %s)",
			result.Decision, ruleID(result))
	}
}

func TestSecurity_CommentIgnored(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), "ls # rm -rf /")
	if result.Decision != DecisionAllow {
		t.Errorf("comment should be ALLOW, got %v (rule: %s)",
			result.Decision, ruleID(result))
	}
}

// --- Ensure real destructive commands are still caught ---

func TestSecurity_DropDatabaseDenied(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), `psql -c "DROP DATABASE production"`)
	if result.Decision != DecisionDeny {
		t.Error("DROP DATABASE should be DENY")
	}
}

func TestSecurity_KubectlDeleteNamespaceDenied(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), "kubectl delete namespace production")
	if result.Decision != DecisionDeny {
		t.Error("kubectl delete namespace should be DENY")
	}
}

func TestSecurity_DockerSystemPruneDenied(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), "docker system prune -af")
	if result.Decision != DecisionDeny {
		t.Error("docker system prune should be DENY")
	}
}

func TestSecurity_SudoRmRfDenied(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), "sudo rm -rf /")
	if result.Decision != DecisionDeny {
		t.Error("sudo rm -rf / should be DENY")
	}
}

func TestSecurity_PipedDestructiveStillDenied(t *testing.T) {
	// echo "data" | rm -rf / → the rm is still destructive
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), `echo "data" | rm -rf /`)
	if result.Decision != DecisionDeny {
		t.Error("piped rm -rf should be DENY")
	}
}

func TestSecurity_TerraformDestroyDenied(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), "terraform destroy -auto-approve")
	if result.Decision != DecisionDeny {
		t.Error("terraform destroy should be DENY")
	}
}

func ruleID(r *EvaluationResult) string {
	if r.PatternInfo != nil {
		return r.PatternInfo.RuleID
	}
	return "<none>"
}
