// Package eval normalization tests.
//
// These tests verify that the evaluator correctly handles natural command
// variations that AI coding agents actually produce: absolute paths, wrapper
// prefixes, Windows-style binaries, compound commands, line continuations,
// and multi-argument forms. The goal is to catch accidental destructive
// commands, not to defend against adversarial bypass attempts.
package eval

import (
	"testing"
)

// TestNormalization_AbsolutePath tests that absolute paths are correctly
// normalized and detected.
func TestNormalization_AbsolutePath(t *testing.T) {
	e := newTestEvaluator()
	tests := []struct {
		name string
		cmd  string
		want EvaluationDecision
	}{
		{"sudo with absolute git path", "sudo /bin/git reset --hard", DecisionDeny},
		{"env with absolute git path", "env /usr/bin/git reset --hard", DecisionDeny},
		{"bare absolute git path", "/usr/bin/git reset --hard", DecisionDeny},
		{"absolute rm -rf /", "/usr/local/bin/rm -rf /", DecisionDeny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Evaluate(testCtx(t), tt.cmd)
			if result.Decision != tt.want {
				t.Errorf("Evaluate(%q) = %v, want %v (rule: %s)",
					tt.cmd, result.Decision, tt.want, ruleID(result))
			}
		})
	}
}

// TestNormalization_WrapperPrefix tests that runner/scheduler wrapper prefixes
// (nice, time, nohup, etc.) are stripped during normalization.
func TestNormalization_WrapperPrefix(t *testing.T) {
	e := newTestEvaluator()
	tests := []struct {
		name string
		cmd  string
		want EvaluationDecision
	}{
		{"nice prefix", "nice rm -rf /", DecisionDeny},
		{"time prefix", "time rm -rf /", DecisionDeny},
		{"nohup prefix", "nohup rm -rf /", DecisionDeny},
		{"watch prefix", "watch rm -rf /", DecisionDeny},
		{"timeout prefix", "timeout 10s rm -rf /", DecisionDeny},
		{"strace prefix", "strace rm -rf /", DecisionDeny},
		{"ltrace prefix", "ltrace rm -rf /", DecisionDeny},
		{"strace with flags", "strace -f -e trace=open rm -rf /", DecisionDeny},
		{"timeout with -k flag", "timeout -k 5 30 rm -rf /", DecisionDeny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Evaluate(testCtx(t), tt.cmd)
			if result.Decision != tt.want {
				t.Errorf("Evaluate(%q) = %v, want %v (rule: %s)",
					tt.cmd, result.Decision, tt.want, ruleID(result))
			}
		})
	}
}

// TestNormalization_WindowsExe tests that Windows-style binary names with .exe
// extensions are correctly normalized and detected.
func TestNormalization_WindowsExe(t *testing.T) {
	e := newTestEvaluator()
	tests := []struct {
		name string
		cmd  string
		want EvaluationDecision
	}{
		{"git.exe reset --hard", "git.exe reset --hard", DecisionDeny},
		{"rm.exe -rf /", "rm.exe -rf /", DecisionDeny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Evaluate(testCtx(t), tt.cmd)
			if result.Decision != tt.want {
				t.Errorf("Evaluate(%q) = %v, want %v (rule: %s)",
					tt.cmd, result.Decision, tt.want, ruleID(result))
			}
		})
	}
}

// TestNormalization_MultiArgRm tests that multiple arguments to rm are all
// checked, not just the first.
func TestNormalization_MultiArgRm(t *testing.T) {
	e := newTestEvaluator()
	tests := []struct {
		name string
		cmd  string
		want EvaluationDecision
	}{
		{"safe first arg destructive second arg", "rm -rf /tmp/safe /etc/passwd", DecisionDeny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Evaluate(testCtx(t), tt.cmd)
			if result.Decision != tt.want {
				t.Errorf("Evaluate(%q) = %v, want %v (rule: %s)",
					tt.cmd, result.Decision, tt.want, ruleID(result))
			}
		})
	}
}

// TestNormalization_CompoundCommand tests that compound commands (semicolons,
// pipes) are split and each segment is evaluated independently.
func TestNormalization_CompoundCommand(t *testing.T) {
	e := newTestEvaluator()
	tests := []struct {
		name string
		cmd  string
		want EvaluationDecision
	}{
		{"destructive first then safe semicolon", "rm -rf / ; git checkout -b foo", DecisionDeny},
		{"safe first then destructive semicolon", "git checkout -b foo ; rm -rf /", DecisionDeny},
		{"destructive first then safe pipe", "rm -rf / | git checkout -b foo", DecisionDeny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Evaluate(testCtx(t), tt.cmd)
			if result.Decision != tt.want {
				t.Errorf("Evaluate(%q) = %v, want %v (rule: %s)",
					tt.cmd, result.Decision, tt.want, ruleID(result))
			}
		})
	}
}

// TestNormalization_NewlineSeparator tests that newlines act as command
// separators so each line is evaluated independently.
func TestNormalization_NewlineSeparator(t *testing.T) {
	e := newTestEvaluator()
	tests := []struct {
		name string
		cmd  string
		want EvaluationDecision
	}{
		{"newline separates rm from -m argument", "git commit -m\nrm -rf /", DecisionDeny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Evaluate(testCtx(t), tt.cmd)
			if result.Decision != tt.want {
				t.Errorf("Evaluate(%q) = %v, want %v (rule: %s)",
					tt.cmd, result.Decision, tt.want, ruleID(result))
			}
		})
	}
}

// TestNormalization_LineContinuationSplit tests that backslash-newline line
// continuations are joined before evaluation.
func TestNormalization_LineContinuationSplit(t *testing.T) {
	e := newTestEvaluator()
	tests := []struct {
		name string
		cmd  string
		want EvaluationDecision
	}{
		{"backslash-newline splits reset across lines", "git re\\\nset --hard", DecisionDeny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Evaluate(testCtx(t), tt.cmd)
			if result.Decision != tt.want {
				t.Errorf("Evaluate(%q) = %v, want %v (rule: %s)",
					tt.cmd, result.Decision, tt.want, ruleID(result))
			}
		})
	}
}

// TestNormalization_RelativePath tests that relative paths (e.g., ./git) are
// correctly normalized and detected.
func TestNormalization_RelativePath(t *testing.T) {
	e := newTestEvaluator()
	tests := []struct {
		name string
		cmd  string
		want EvaluationDecision
	}{
		{"relative ./git", "./git reset --hard", DecisionDeny},
		{"relative ./rm", "./rm -rf /", DecisionDeny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Evaluate(testCtx(t), tt.cmd)
			if result.Decision != tt.want {
				t.Errorf("Evaluate(%q) = %v, want %v (rule: %s)",
					tt.cmd, result.Decision, tt.want, ruleID(result))
			}
		})
	}
}

// TestNormalization_SubstringFalsePositive tests that commands whose binary
// name contains a keyword as a substring (e.g., "digit", "farm") are not
// falsely denied.
func TestNormalization_SubstringFalsePositive(t *testing.T) {
	e := newTestEvaluator()
	tests := []struct {
		name string
		cmd  string
		want EvaluationDecision
	}{
		{"digit contains git", "digit reset --hard", DecisionAllow},
		{"farm contains rm", "farm -rf /", DecisionAllow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Evaluate(testCtx(t), tt.cmd)
			if result.Decision != tt.want {
				t.Errorf("Evaluate(%q) = %v, want %v (rule: %s)",
					tt.cmd, result.Decision, tt.want, ruleID(result))
			}
		})
	}
}

// TestNormalization_CustomBinPath tests that binaries in non-standard
// directories (e.g., /opt/custom/git) are correctly normalized and detected.
func TestNormalization_CustomBinPath(t *testing.T) {
	e := newTestEvaluator()
	tests := []struct {
		name string
		cmd  string
		want EvaluationDecision
	}{
		{"custom bin path git", "/opt/custom/git reset --hard", DecisionDeny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Evaluate(testCtx(t), tt.cmd)
			if result.Decision != tt.want {
				t.Errorf("Evaluate(%q) = %v, want %v (rule: %s)",
					tt.cmd, result.Decision, tt.want, ruleID(result))
			}
		})
	}
}

// TestNormalization_WindowsAbsolutePath tests that Windows-style absolute
// paths (e.g., C:/Git/bin/git.exe) are correctly normalized and detected.
func TestNormalization_WindowsAbsolutePath(t *testing.T) {
	e := newTestEvaluator()

	// Simple Windows path (no spaces) — works via .exe stripping + regex.
	t.Run("windows path", func(t *testing.T) {
		result := e.Evaluate(testCtx(t), "C:/Git/bin/git.exe reset --hard")
		if result.Decision != DecisionDeny {
			t.Errorf("Evaluate(%q) = %v, want deny (rule: %s)",
				"C:/Git/bin/git.exe reset --hard", result.Decision, ruleID(result))
		}
	})

	// Windows path with spaces requires path-aware tokenization that doesn't
	// exist in the Go normalization pipeline yet.
	t.Run("windows path quoted with spaces", func(t *testing.T) {
		t.Skip("Known limitation: Windows paths with spaces in quotes not yet supported")
	})
}

// TestNormalization_GitGlobalFlags tests that git global flags (e.g., -C,
// --work-tree) before the subcommand are handled during normalization.
func TestNormalization_GitGlobalFlags(t *testing.T) {
	e := newTestEvaluator()
	tests := []struct {
		name string
		cmd  string
		want EvaluationDecision
	}{
		{"git -C dir reset --hard", "git -C /tmp/repo reset --hard", DecisionDeny},
		{"git --work-tree=dir reset --hard", "git --work-tree=/tmp/repo reset --hard", DecisionDeny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Evaluate(testCtx(t), tt.cmd)
			if result.Decision != tt.want {
				t.Errorf("Evaluate(%q) = %v, want %v (rule: %s)",
					tt.cmd, result.Decision, tt.want, ruleID(result))
			}
		})
	}
}

// TestNormalization_LongFormFlag tests that long-form flags (--force, --recursive,
// --delete) are correctly matched by pack patterns.
func TestNormalization_LongFormFlag(t *testing.T) {
	e := newTestEvaluator()
	tests := []struct {
		name string
		cmd  string
		want EvaluationDecision
	}{
		{"git clean --force", "git clean --force", DecisionDeny},
		{"git branch --delete --force", "git branch --delete --force feature", DecisionDeny},
		{"rm --recursive --force /", "rm --recursive --force /important", DecisionDeny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Evaluate(testCtx(t), tt.cmd)
			if result.Decision != tt.want {
				t.Errorf("Evaluate(%q) = %v, want %v (rule: %s)",
					tt.cmd, result.Decision, tt.want, ruleID(result))
			}
		})
	}
}

// TestNormalization_InterpreterExtraction tests that destructive commands
// embedded inside interpreter -c/-e payloads are detected through the
// full evaluation pipeline.
func TestNormalization_InterpreterExtraction(t *testing.T) {
	e := newTestEvaluator()
	tests := []struct {
		name string
		cmd  string
		want EvaluationDecision
	}{
		{"ruby -e system rm", `ruby -e "system('rm -rf /')"`, DecisionDeny},
		{"node -e exec rm", `node -e "require('child_process').exec('rm -rf /')"`, DecisionDeny},
		{"perl -e system rm", `perl -e "system('rm -rf /')"`, DecisionDeny},
		{"python3 -c os.system rm", `python3 -c "import os; os.system('rm -rf /')"`, DecisionDeny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Evaluate(testCtx(t), tt.cmd)
			if result.Decision != tt.want {
				t.Errorf("Evaluate(%q) = %v, want %v (rule: %s)",
					tt.cmd, result.Decision, tt.want, ruleID(result))
			}
		})
	}
}

// TestNormalization_NestedSubstitution tests that nested command substitutions
// ($(...$(...))) are unwrapped and evaluated correctly.
func TestNormalization_NestedSubstitution(t *testing.T) {
	e := newTestEvaluator()

	// Direct nested substitution with a destructive inner command is denied.
	t.Run("nested substitution with rm", func(t *testing.T) {
		result := e.Evaluate(testCtx(t), "echo $(echo $(rm -rf /))")
		if result.Decision != DecisionDeny {
			t.Errorf("Evaluate(%q) = %v, want deny (rule: %s)",
				"echo $(echo $(rm -rf /))", result.Decision, ruleID(result))
		}
	})

	// Known limitation: when the destructive subcommand is dynamically
	// reconstructed across substitution boundaries, the evaluator cannot
	// see the full command. E.g., `git $(echo reset) --hard` produces
	// separate inner commands for `git` and `echo`, but never evaluates
	// `git reset --hard` as a whole.
	t.Run("dynamically reconstructed git reset", func(t *testing.T) {
		t.Skip("Known limitation: dynamically reconstructed commands across substitution boundaries")
	})
}

// TestNormalization_HeredocDestructive tests that destructive commands inside
// heredocs are detected through the evaluation pipeline.
func TestNormalization_HeredocDestructive(t *testing.T) {
	// Known limitation: heredoc body content is not currently extracted
	// as inner commands through the evaluation pipeline. The decomposer's
	// resolveText on heredoc_redirect nodes returns the full node text
	// including markers, which does not re-parse usefully.
	t.Skip("Known limitation: heredoc inner command extraction not implemented in eval pipeline")
}

// TestNormalization_PowerShellDestructive tests that PowerShell destructive
// commands are detected through the evaluation pipeline.
func TestNormalization_PowerShellDestructive(t *testing.T) {
	e := newTestEvaluator()

	// powershell -Command with inline code is detected via bash -c style extraction.
	t.Run("powershell -Command Remove-Item", func(t *testing.T) {
		result := e.Evaluate(testCtx(t), `powershell -Command "Remove-Item -Recurse -Force C:\Windows"`)
		if result.Decision != DecisionDeny {
			t.Errorf("expected DENY, got %v (rule: %s)", result.Decision, ruleID(result))
		}
	})

	// Known limitation: Invoke-Expression is a PowerShell-only construct
	// that is not detected when the evaluator defaults to Bash shell parsing.
	t.Run("Invoke-Expression rm", func(t *testing.T) {
		t.Skip("Known limitation: Invoke-Expression not detected under default Bash shell parsing")
	})
}
