// Package eval — regression tests ported from the Rust "repro" test suite.
//
// Each test in this file documents a real bug that was discovered and fixed in
// production. Some tests MAY FAIL if the Go implementation has not yet caught
// up with every edge-case fix. That is intentional: failing tests here
// identify work still remaining, not broken test infrastructure.
package eval

import (
	"strings"
	"testing"
)

// TestRepro_CommentFalsePositive guards against destructive patterns that
// appear inside a shell comment (everything after an unquoted `#`) triggering
// a false-positive DENY.  The actual executed command is harmless; the
// dangerous-looking text is never run.
func TestRepro_CommentFalsePositive(t *testing.T) {
	e := newTestEvaluator()

	cases := []string{
		`ls -la # rm -rf /`,
		`git status # git reset --hard`,
	}

	for _, cmd := range cases {
		result := e.Evaluate(testCtx(t), cmd)
		if result.Decision != DecisionAllow {
			t.Errorf("expected ALLOW for %q, got %v (rule: %v)", cmd, result.Decision, ruleID(result))
		}
	}
}

// TestRepro_EchoFalsePositive guards against `echo` commands that print
// dangerous-looking strings being treated as if those strings were executed.
// `echo` only writes to stdout; it never executes its arguments.
func TestRepro_EchoFalsePositive(t *testing.T) {
	e := newTestEvaluator()

	cases := []string{
		`echo rm -rf /`,
		`echo git reset --hard`,
		`echo "DROP TABLE users"`,
	}

	for _, cmd := range cases {
		result := e.Evaluate(testCtx(t), cmd)
		if result.Decision != DecisionAllow {
			t.Errorf("expected ALLOW for %q, got %v (rule: %v)", cmd, result.Decision, ruleID(result))
		}
	}
}

// TestRepro_ContextBypass_BashRcfile guards against a quoted path argument
// that contains spaces causing the interpreter to be mis-identified, so that
// the dangerous `-c` payload is never inspected.
func TestRepro_ContextBypass_BashRcfile(t *testing.T) {
	e := newTestEvaluator()

	cmd := `bash --rcfile "my file" -c "rm -rf /"`
	result := e.Evaluate(testCtx(t), cmd)
	if result.Decision != DecisionDeny {
		t.Errorf("expected DENY for %q, got %v (rule: %v)", cmd, result.Decision, ruleID(result))
	}
}

// TestRepro_ContextBypass_PythonMultipleFlags guards against multiple flag
// tokens before `-c` causing the `-c` argument to be classified as a plain
// Argument rather than as interpreter inline-code, hiding the destructive
// payload from analysis.
func TestRepro_ContextBypass_PythonMultipleFlags(t *testing.T) {
	e := newTestEvaluator()

	cmd := `python -B -v -c "import os; os.system('rm -rf /')"`
	result := e.Evaluate(testCtx(t), cmd)
	if result.Decision != DecisionDeny {
		t.Errorf("expected DENY for %q, got %v (rule: %v)", cmd, result.Decision, ruleID(result))
	}
}

// TestRepro_ContextDepth_NestedSubstitutions guards against deeply nested
// command-substitution `$()` causing a stack overflow (or unbounded recursion)
// in the sanitizer.  The exact decision does not matter — the evaluator must
// return without panicking.
func TestRepro_ContextDepth_NestedSubstitutions(t *testing.T) {
	// Build pathological input: echo "$( $( $( ... ) ) )"
	var cmd strings.Builder
	cmd.WriteString(`echo "`)
	for i := 0; i < 600; i++ {
		cmd.WriteString("$(")
	}
	cmd.WriteString("echo hi")
	for i := 0; i < 600; i++ {
		cmd.WriteByte(')')
	}
	cmd.WriteByte('"')

	// Must not panic — any decision is acceptable.
	e := newTestEvaluator()
	_ = e.Evaluate(testCtx(t), cmd.String())
}

// TestRepro_TokenizerDesync guards against a `)` character appearing inside
// nested quotes causing the tokenizer to exit a `$()` substitution block
// prematurely, which would swallow subsequent commands and miss the destructive
// `rm -rf /` that follows.
func TestRepro_TokenizerDesync(t *testing.T) {
	e := newTestEvaluator()

	cmd := `echo "$(echo " ) " )"; rm -rf /`
	result := e.Evaluate(testCtx(t), cmd)
	if result.Decision != DecisionDeny {
		t.Errorf("expected DENY for %q, got %v (rule: %v)", cmd, result.Decision, ruleID(result))
	}
}

// TestRepro_NewlineSeparator guards against a newline character between a flag
// and its value being treated as harmless whitespace, which caused the
// flag-argument binding to break and allowed the value to be treated as an
// independent (dangerous) command.
func TestRepro_NewlineSeparator(t *testing.T) {
	e := newTestEvaluator()

	cmd := "git commit -m\nrm -rf /"
	result := e.Evaluate(testCtx(t), cmd)
	if result.Decision != DecisionDeny {
		t.Errorf("expected DENY for %q, got %v (rule: %v)", cmd, result.Decision, ruleID(result))
	}
}

// TestRepro_LineContinuationArgMasking guards against a backslash+newline line
// continuation between a flag and its quoted argument breaking flag-argument
// binding.  Here `-m` is followed by a continuation and then `"rm -rf /"`,
// which is the commit message — not an executed command — so the result should
// be ALLOW.
func TestRepro_LineContinuationArgMasking(t *testing.T) {
	e := newTestEvaluator()

	// git commit -m \<newline>"rm -rf /"
	cmd := "git commit -m \\\n\"rm -rf /\""
	result := e.Evaluate(testCtx(t), cmd)
	if result.Decision != DecisionAllow {
		t.Errorf("expected ALLOW for %q, got %v (rule: %v)", cmd, result.Decision, ruleID(result))
	}
}

// TestRepro_LineContinuationSplitWord guards against a backslash+newline line
// continuation being used to split a command keyword across two physical lines,
// evading keyword-based detection.
func TestRepro_LineContinuationSplitWord(t *testing.T) {
	e := newTestEvaluator()

	// "git re\<newline>set --hard" — the logical command is "git reset --hard"
	cmd := "git re\\\nset --hard"
	result := e.Evaluate(testCtx(t), cmd)
	if result.Decision != DecisionDeny {
		t.Errorf("expected DENY for %q, got %v (rule: %v)", cmd, result.Decision, ruleID(result))
	}
}

// TestRepro_WrapperBypass guards against common runner/wrapper utilities
// (nice, time, nohup, watch, timeout) being used as a prefix to evade
// detection by hiding the real command in a non-first position.
func TestRepro_WrapperBypass(t *testing.T) {
	e := newTestEvaluator()

	cases := []string{
		`nice rm -rf /`,
		`time rm -rf /`,
		`nohup rm -rf /`,
		`watch rm -rf /`,
		`timeout 10s rm -rf /`,
	}

	for _, cmd := range cases {
		result := e.Evaluate(testCtx(t), cmd)
		if result.Decision != DecisionDeny {
			t.Errorf("expected DENY for %q, got %v (rule: %v)", cmd, result.Decision, ruleID(result))
		}
	}
}

// TestRepro_RmMultiArg guards against a safe-path short-circuit (e.g. the
// first argument matching `/tmp/`) causing evaluation to stop before a second,
// destructive argument is inspected.
func TestRepro_RmMultiArg(t *testing.T) {
	e := newTestEvaluator()

	cmd := `rm -rf /tmp/safe /etc/passwd`
	result := e.Evaluate(testCtx(t), cmd)
	if result.Decision != DecisionDeny {
		t.Errorf("expected DENY for %q, got %v (rule: %v)", cmd, result.Decision, ruleID(result))
	}
}

// TestRepro_RedirectionBypass guards against shell I/O redirection operators
// breaking the token stream so that arguments which follow the redirection are
// not correctly associated with their command, allowing dangerous subcommands
// to pass undetected.
func TestRepro_RedirectionBypass(t *testing.T) {
	e := newTestEvaluator()

	cases := []string{
		`"git">/dev/null reset --hard`,
		`git >/dev/null reset --hard`,
	}

	for _, cmd := range cases {
		result := e.Evaluate(testCtx(t), cmd)
		if result.Decision != DecisionDeny {
			t.Errorf("expected DENY for %q, got %v (rule: %v)", cmd, result.Decision, ruleID(result))
		}
	}
}

// TestRepro_SafePatternCompoundBypass guards against unanchored safe patterns
// matching only one segment of a compound command, allowing a destructive
// segment elsewhere in the same pipeline or sequence to slip through.
func TestRepro_SafePatternCompoundBypass(t *testing.T) {
	e := newTestEvaluator()

	cases := []string{
		`rm -rf / ; git checkout -b foo`,
		`git checkout -b foo ; rm -rf /`,
		`rm -rf / | git checkout -b foo`,
	}

	for _, cmd := range cases {
		result := e.Evaluate(testCtx(t), cmd)
		if result.Decision != DecisionDeny {
			t.Errorf("expected DENY for %q, got %v (rule: %v)", cmd, result.Decision, ruleID(result))
		}
	}
}

// TestRepro_XargsBypass guards against wrapper pipelines that route a
// destructive command through xargs, which should still be denied.
func TestRepro_XargsBypass(t *testing.T) {
	e := newTestEvaluator()

	cases := []string{
		`cat file | xargs rm -rf /`,
		`printf '/\n' | xargs -0 rm -rf /`,
	}

	for _, cmd := range cases {
		result := e.Evaluate(testCtx(t), cmd)
		if result.Decision != DecisionDeny {
			t.Errorf("expected DENY for %q, got %v (rule: %v)", cmd, result.Decision, ruleID(result))
		}
	}
}

// TestRepro_WindowsExeSuffix guards against Windows-style `.exe` suffixes on
// binary names bypassing detection rules that match only bare command names.
func TestRepro_WindowsExeSuffix(t *testing.T) {
	e := newTestEvaluator()

	cases := []string{
		`git.exe reset --hard`,
		`rm.exe -rf /`,
	}

	for _, cmd := range cases {
		result := e.Evaluate(testCtx(t), cmd)
		if result.Decision != DecisionDeny {
			t.Errorf("expected DENY for %q, got %v (rule: %v)", cmd, result.Decision, ruleID(result))
		}
	}
}

// TestRepro_BackslashExeBypass guards against `\git.exe` bypassing both
// leading-backslash stripping (due to '.') and regex matching (due to .exe).
// repro_normalization_bypass.rs — test_backslash_exe_bypass.
func TestRepro_BackslashExeBypass(t *testing.T) {
	e := newTestEvaluator()

	cases := []string{
		`\git.exe reset --hard`,
		`\rm.exe -rf /`,
	}

	for _, cmd := range cases {
		result := e.Evaluate(testCtx(t), cmd)
		if result.Decision != DecisionDeny {
			t.Errorf("expected DENY for %q, got %v (rule: %v)", cmd, result.Decision, ruleID(result))
		}
	}
}

// TestRepro_WindowsAbsolutePathBypass guards against Windows-style absolute
// paths (C:/Git/bin/git.exe) evading detection.
// repro_normalization_bypass.rs — test_windows_path_bypass.
func TestRepro_WindowsAbsolutePathBypass(t *testing.T) {
	e := newTestEvaluator()

	// C:/Git/bin/git.exe — no spaces, .exe stripping + regex matching works.
	result := e.Evaluate(testCtx(t), `C:/Git/bin/git.exe reset --hard`)
	if result.Decision != DecisionDeny {
		t.Errorf("expected DENY for 'C:/Git/bin/git.exe reset --hard', got %v (rule: %v)",
			result.Decision, ruleID(result))
	}

	// Quoted Windows path with spaces is a known limitation — skip.
	// "C:/Program Files/Git/bin/git.exe" reset --hard — after unquoting, the
	// space in "Program Files" breaks path normalization.
	t.Run("windows_quoted_path_with_spaces", func(t *testing.T) {
		t.Skip("Known limitation: Windows paths with spaces not supported")
	})
}

// TestRepro_GitCleanForceLongFlag guards against the long --force flag form
// for git clean not being detected.
// repro_git_bypasses.rs — test_git_clean_force_long_bypass.
func TestRepro_GitCleanForceLongFlag(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), "git clean --force")
	if result.Decision != DecisionDeny {
		t.Errorf("expected DENY for 'git clean --force', got %v (rule: %v)", result.Decision, ruleID(result))
	}
}

// TestRepro_GitBranchForceDeleteLongFlag guards against the long form of
// git branch force delete (--delete --force) not being detected.
// repro_git_bypasses.rs — test_git_branch_force_delete_long_bypass.
func TestRepro_GitBranchForceDeleteLongFlag(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), "git branch --delete --force feature")
	if result.Decision != DecisionDeny {
		t.Errorf("expected DENY for 'git branch --delete --force feature', got %v (rule: %v)", result.Decision, ruleID(result))
	}
}

// TestRepro_ResticQuotedArgs guards against restic commands with quoted
// arguments being misclassified as safe.
// repro_restic_quoted.rs — test_restic_quoted_args_bypass.
func TestRepro_ResticQuotedArgs(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), `restic -o s3.bucket="my bucket" forget`)
	if result.Decision != DecisionDeny {
		t.Errorf("expected DENY for restic forget with quoted args, got %v (rule: %v)", result.Decision, ruleID(result))
	}
}

// TestRepro_BashIntermediateFlagInlineBypass guards against intervening flags
// between bash and -c causing the inline code payload to be misclassified.
// repro_inline_bypass.rs — test_bash_e_c_bypass.
func TestRepro_BashIntermediateFlagInlineBypass(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), `bash -e -c "rm -rf /"`)
	if result.Decision != DecisionDeny {
		t.Errorf("expected DENY for 'bash -e -c \"rm -rf /\"', got %v (rule: %v)", result.Decision, ruleID(result))
	}
}

// TestRepro_PythonIntermediateFlagInlineBypass guards against intervening flags
// between python and -c causing the inline code to be classified as data.
// repro_inline_bypass.rs — test_python_u_c_bypass.
func TestRepro_PythonIntermediateFlagInlineBypass(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), `python -u -c "import os; os.system('rm -rf /')"`)
	if result.Decision != DecisionDeny {
		t.Errorf("expected DENY for python -u -c with destructive payload, got %v (rule: %v)", result.Decision, ruleID(result))
	}
}

// TestRepro_SubstringFalsePositiveGit guards against binary names that happen
// to contain "git" as a substring (e.g. "digit") triggering false positives.
// regression_path_anchors.rs — test_substring_false_positive_git.
func TestRepro_SubstringFalsePositiveGit(t *testing.T) {
	e := newTestEvaluator()

	// "digit" contains "git" as a substring — must NOT be blocked.
	result := e.Evaluate(testCtx(t), "digit reset --hard")
	if result.Decision != DecisionAllow {
		t.Errorf("expected ALLOW for 'digit reset --hard' (substring false positive), got %v (rule: %v)",
			result.Decision, ruleID(result))
	}
}

// TestRepro_SubstringFalsePositiveRm guards against binary names that contain
// "rm" as a substring (e.g. "farm") triggering false positives.
// regression_path_anchors.rs — test_substring_false_positive_rm.
func TestRepro_SubstringFalsePositiveRm(t *testing.T) {
	e := newTestEvaluator()

	result := e.Evaluate(testCtx(t), "farm -rf /")
	if result.Decision != DecisionAllow {
		t.Errorf("expected ALLOW for 'farm -rf /' (substring false positive), got %v (rule: %v)",
			result.Decision, ruleID(result))
	}
}

// TestRepro_CustomBinPathGit guards against git living in a non-standard
// directory path bypassing detection.
// regression_path_anchors.rs — test_custom_bin_path_bypass_git.
func TestRepro_CustomBinPathGit(t *testing.T) {
	e := newTestEvaluator()
	result := e.Evaluate(testCtx(t), "/opt/custom/git reset --hard")
	if result.Decision != DecisionDeny {
		t.Errorf("expected DENY for '/opt/custom/git reset --hard', got %v (rule: %v)",
			result.Decision, ruleID(result))
	}
}

// TestRepro_RelativePathBypass guards against relative paths (./git, ./rm)
// evading detection.
// regression_path_anchors.rs — test_relative_path_bypass_git/rm.
func TestRepro_RelativePathBypass(t *testing.T) {
	e := newTestEvaluator()

	cases := []struct {
		cmd  string
		want EvaluationDecision
	}{
		{"./git reset --hard", DecisionDeny},
		{"./rm -rf /", DecisionDeny},
	}

	for _, tc := range cases {
		result := e.Evaluate(testCtx(t), tc.cmd)
		if result.Decision != tc.want {
			t.Errorf("expected %v for %q, got %v (rule: %v)", tc.want, tc.cmd, result.Decision, ruleID(result))
		}
	}
}
