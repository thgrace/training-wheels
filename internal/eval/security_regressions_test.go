// Package eval security regression tests.
//
// These tests port bypass scenarios from the Rust project's
// tests/security_regressions_v2.rs and tests/security_regressions_v3.rs.
//
// IMPORTANT: Some of these tests are EXPECTED TO FAIL. The Go implementation
// may not yet handle all of the bypass vectors documented here. That is
// intentional — these are regression tests that document what SHOULD be
// caught, and failures indicate gaps in the evaluator that need to be fixed.
package eval

import (
	"testing"
)

// TestSecurityRegression_QuotedBinaryBypass tests that quoting the binary name
// does not evade detection.
func TestSecurityRegression_QuotedBinaryBypass(t *testing.T) {
	e := newTestEvaluator()
	tests := []struct {
		name string
		cmd  string
		want EvaluationDecision
	}{
		{"double-quoted git binary", `"git" reset --hard`, DecisionDeny},
		{"single-quoted git binary", `'git' reset --hard`, DecisionDeny},
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

// TestSecurityRegression_QuotedSubcommandBypass tests that quoting the
// subcommand does not evade detection.
func TestSecurityRegression_QuotedSubcommandBypass(t *testing.T) {
	e := newTestEvaluator()
	tests := []struct {
		name string
		cmd  string
		want EvaluationDecision
	}{
		{"double-quoted reset subcommand", `git "reset" --hard`, DecisionDeny},
		{"single-quoted reset subcommand", `git 'reset' --hard`, DecisionDeny},
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

// TestSecurityRegression_AbsolutePathBypass tests that absolute paths do not
// evade detection.
func TestSecurityRegression_AbsolutePathBypass(t *testing.T) {
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

// TestSecurityRegression_ComplexQuotingBypass tests combinations of absolute
// paths and quoted arguments.
func TestSecurityRegression_ComplexQuotingBypass(t *testing.T) {
	e := newTestEvaluator()
	tests := []struct {
		name string
		cmd  string
		want EvaluationDecision
	}{
		{"sudo quoted absolute path and quoted subcommand", `sudo "/usr/bin/git" "reset" --hard`, DecisionDeny},
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

// TestSecurityRegression_WrapperPrefixBypass tests runner/scheduler wrapper
// prefixes that are NOT stripped by sudo/env/command normalization.
func TestSecurityRegression_WrapperPrefixBypass(t *testing.T) {
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

// TestSecurityRegression_WindowsExeBypass tests Windows-style binary names
// with .exe extensions.
func TestSecurityRegression_WindowsExeBypass(t *testing.T) {
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

// TestSecurityRegression_RedirectionBypass tests redirections injected to
// break token parsing.
func TestSecurityRegression_RedirectionBypass(t *testing.T) {
	e := newTestEvaluator()
	tests := []struct {
		name string
		cmd  string
		want EvaluationDecision
	}{
		{"redirection between binary and subcommand", "git >/dev/null reset --hard", DecisionDeny},
		{"quoted binary with adjacent redirection", `"git">/dev/null reset --hard`, DecisionDeny},
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

// TestSecurityRegression_MultiArgRm tests that multiple arguments to rm are
// all checked, not just the first.
func TestSecurityRegression_MultiArgRm(t *testing.T) {
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

// TestSecurityRegression_CompoundCommandBypass tests that a safe pattern in a
// compound command does not cause the whole command to be allowed.
func TestSecurityRegression_CompoundCommandBypass(t *testing.T) {
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

// TestSecurityRegression_NewlineSeparator tests that newlines act as command
// separators so that rm is treated as a new command, not an argument.
func TestSecurityRegression_NewlineSeparator(t *testing.T) {
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

// TestSecurityRegression_LineContinuationSplit tests that splitting a command
// word with backslash+newline is still caught.
func TestSecurityRegression_LineContinuationSplit(t *testing.T) {
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

// TestSecurityRegression_InternalEscapeBypass tests backslash inside binary
// name (bash treats `g\it` as `git`).
func TestSecurityRegression_InternalEscapeBypass(t *testing.T) {
	e := newTestEvaluator()
	tests := []struct {
		name string
		cmd  string
		want EvaluationDecision
	}{
		{"backslash inside binary name", `g\it reset --hard`, DecisionDeny},
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

// TestSecurityRegression_MixedQuotingBypass tests quotes embedded in binary
// name (bash treats `g'i't` as `git`).
func TestSecurityRegression_MixedQuotingBypass(t *testing.T) {
	e := newTestEvaluator()
	tests := []struct {
		name string
		cmd  string
		want EvaluationDecision
	}{
		{"single quotes embedded in binary name", `g'i't reset --hard`, DecisionDeny},
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

// TestSecurityRegression_RelativePathBypass tests that relative paths do not
// evade detection.
// regression_path_anchors.rs — test_relative_path_bypass_git/rm.
func TestSecurityRegression_RelativePathBypass(t *testing.T) {
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

// TestSecurityRegression_SubstringFalsePositive tests that commands whose
// binary name contains a keyword as a substring are NOT blocked.
// regression_path_anchors.rs — test_substring_false_positive_git/rm.
func TestSecurityRegression_SubstringFalsePositive(t *testing.T) {
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

// TestSecurityRegression_CustomBinPathBypass tests that git in a non-standard
// directory (/opt/custom/git) is still detected.
// regression_path_anchors.rs — test_custom_bin_path_bypass_git.
func TestSecurityRegression_CustomBinPathBypass(t *testing.T) {
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

// TestSecurityRegression_BackslashExeBypass tests that \git.exe evading
// backslash stripping (due to '.') and regex (due to .exe) is still caught.
// repro_normalization_bypass.rs — test_backslash_exe_bypass.
func TestSecurityRegression_BackslashExeBypass(t *testing.T) {
	e := newTestEvaluator()
	tests := []struct {
		name string
		cmd  string
		want EvaluationDecision
	}{
		{"backslash git.exe", `\git.exe reset --hard`, DecisionDeny},
		{"backslash rm.exe", `\rm.exe -rf /`, DecisionDeny},
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

// TestSecurityRegression_WindowsAbsolutePathBypass tests that Windows-style
// absolute paths are still caught.
// repro_normalization_bypass.rs — test_windows_path_bypass.
func TestSecurityRegression_WindowsAbsolutePathBypass(t *testing.T) {
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

// TestSecurityRegression_GitGlobalFlags tests that git global flags like -C
// before the subcommand don't evade detection.
func TestSecurityRegression_GitGlobalFlags(t *testing.T) {
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

// TestSecurityRegression_LongFormFlagBypass tests that long-form flags
// (--force instead of -f, --recursive instead of -R) are caught.
func TestSecurityRegression_LongFormFlagBypass(t *testing.T) {
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
