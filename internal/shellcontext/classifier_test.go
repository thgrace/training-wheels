package shellcontext

import (
	"testing"
)

func findSpanContaining(spans []Span, offset int) *Span {
	for i := range spans {
		if spans[i].Start <= offset && offset < spans[i].End {
			return &spans[i]
		}
	}
	return nil
}

func assertSpanKindAt(t *testing.T, spans []Span, cmd string, substr string, want SpanKind) {
	t.Helper()
	idx := -1
	for i := 0; i+len(substr) <= len(cmd); i++ {
		if cmd[i:i+len(substr)] == substr {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("substring %q not found in %q", substr, cmd)
	}
	sp := findSpanContaining(spans, idx)
	if sp == nil {
		t.Fatalf("no span at offset %d for %q in %q", idx, substr, cmd)
	}
	if sp.Kind != want {
		t.Errorf("span at %q (offset %d) = %v, want %v; span=[%d,%d)", substr, idx, sp.Kind, want, sp.Start, sp.End)
	}
}

func TestClassify_EmptyString(t *testing.T) {
	spans := Classify("", nil)
	if len(spans) != 0 {
		t.Errorf("expected 0 spans, got %d", len(spans))
	}
}

func TestClassify_SimpleCommand(t *testing.T) {
	cmd := "ls -la"
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "ls", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, "-la", SpanArgument)
}

func TestClassify_PipedCommand(t *testing.T) {
	cmd := "echo foo | grep bar"
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "echo", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, "foo", SpanArgument)
	assertSpanKindAt(t, spans, cmd, "grep", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, "bar", SpanArgument)
}

func TestClassify_InlineCode(t *testing.T) {
	cmd := "bash -c 'rm -rf /'"
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "bash", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, "-c", SpanArgument)
	assertSpanKindAt(t, spans, cmd, "'rm -rf /'", SpanInlineCode)
}

func TestClassify_InlineCodeDoubleQuotes(t *testing.T) {
	cmd := `sh -c "rm -rf /"`
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "sh", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, "-c", SpanArgument)
	assertSpanKindAt(t, spans, cmd, `"rm -rf /"`, SpanInlineCode)
}

func TestClassify_SingleQuotes(t *testing.T) {
	cmd := "git commit -m 'some message'"
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "git", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, "commit", SpanArgument)
	assertSpanKindAt(t, spans, cmd, "-m", SpanArgument)
	assertSpanKindAt(t, spans, cmd, "'some message'", SpanArgument)
}

func TestClassify_DoubleQuotes(t *testing.T) {
	cmd := `echo "hello world"`
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "echo", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, `"hello world"`, SpanArgument)
}

func TestClassify_Comment(t *testing.T) {
	cmd := "ls # comment here"
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "ls", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, "# comment here", SpanComment)
}

func TestClassify_CommentOnly(t *testing.T) {
	cmd := "# just a comment"
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "# just a comment", SpanComment)
}

func TestClassify_CommandSubstitution(t *testing.T) {
	cmd := "echo $(rm -rf /)"
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "echo", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, "$(rm -rf /)", SpanExecuted)
}

func TestClassify_ChainedCommands(t *testing.T) {
	cmd := "ls && rm -rf /"
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "ls", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, "rm", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, "-rf", SpanArgument)
}

func TestClassify_Semicolons(t *testing.T) {
	cmd := "echo a; rm b"
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "echo", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, "rm", SpanExecuted)
}

func TestClassify_BacktickSubstitution(t *testing.T) {
	cmd := "echo `whoami`"
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "echo", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, "`whoami`", SpanExecuted)
}

func TestClassify_CoverageComplete(t *testing.T) {
	// Verify spans cover every byte.
	cmds := []string{
		"ls -la",
		"echo foo | grep bar",
		"bash -c 'rm -rf /'",
		"git commit -m 'msg'",
		"ls # comment",
		"echo $(date); rm /tmp/x",
	}
	for _, cmd := range cmds {
		spans := Classify(cmd, nil)
		covered := make([]bool, len(cmd))
		for _, sp := range spans {
			for i := sp.Start; i < sp.End; i++ {
				covered[i] = true
			}
		}
		for i, c := range covered {
			if !c {
				t.Errorf("byte %d not covered in %q", i, cmd)
			}
		}
	}
}

func TestClassify_EvalInlineCode(t *testing.T) {
	cmd := "eval 'rm -rf /'"
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "eval", SpanExecuted)
	// eval without -c: the next argument is NOT marked as InlineCode by the classifier.
	// eval treats all args as code but the classifier only knows about -c.
	// This is intentional — eval's args are still SpanArgument from the classifier's perspective.
	// The safe registry does NOT list eval, so the sanitizer won't mask it either.
	// The patterns will still fire on the raw command.
}

func TestClassify_HereString(t *testing.T) {
	cmd := "cat <<<hello"
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "cat", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, "hello", SpanData)
}

func TestClassify_CmdExeInlineCode(t *testing.T) {
	cmd := `cmd /c "rmdir /s /q C:\temp"`
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "cmd", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, "/c", SpanArgument)
	assertSpanKindAt(t, spans, cmd, `"rmdir /s /q C:\temp"`, SpanInlineCode)
}

func TestClassify_PowerShellInlineCode(t *testing.T) {
	cmd := `powershell -Command "Remove-Item -Recurse -Force C:\temp"`
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "powershell", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, "-Command", SpanArgument)
	assertSpanKindAt(t, spans, cmd, `"Remove-Item -Recurse -Force C:\temp"`, SpanInlineCode)
}

func TestClassify_PwshInlineCode(t *testing.T) {
	cmd := `pwsh -c "Get-Process"`
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "pwsh", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, "-c", SpanArgument)
	assertSpanKindAt(t, spans, cmd, `"Get-Process"`, SpanInlineCode)
}

func TestClassify_PythonInlineCode(t *testing.T) {
	cmd := `python -c "import os; os.system('rm -rf /')"`
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "python", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, "-c", SpanArgument)
	assertSpanKindAt(t, spans, cmd, `"import os; os.system('rm -rf /')"`, SpanInlineCode)
}

func TestClassify_Python3InlineCode(t *testing.T) {
	cmd := `python3 -c "import shutil; shutil.rmtree('/')"`
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "python3", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, `"import shutil; shutil.rmtree('/')"`, SpanInlineCode)
}

func TestClassify_Python3VersionedInlineCode(t *testing.T) {
	cmd := `python3.11 -c "import os; os.remove('/etc/passwd')"`
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "python3.11", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, `"import os; os.remove('/etc/passwd')"`, SpanInlineCode)
}

func TestClassify_NodeInlineCode(t *testing.T) {
	cmd := `node -c "require('fs').rmSync('/', {recursive: true})"`
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "node", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, `"require('fs').rmSync('/', {recursive: true})"`, SpanInlineCode)
}

func TestClassify_RubyInlineCode(t *testing.T) {
	cmd := `ruby -c "system('rm -rf /')"`
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "ruby", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, `"system('rm -rf /')"`, SpanInlineCode)
}

func TestClassify_PerlInlineCode(t *testing.T) {
	cmd := `perl -c "system('rm -rf /')"`
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "perl", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, `"system('rm -rf /')"`, SpanInlineCode)
}

func TestClassify_NodeVersionedInlineCode(t *testing.T) {
	cmd := `node20 -c "require('child_process').execSync('rm -rf /')"`
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "node20", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, `"require('child_process').execSync('rm -rf /')"`, SpanInlineCode)
}

func TestClassify_PythonExeInlineCode(t *testing.T) {
	cmd := `python.exe -c "import os; os.system('rm -rf /')"`
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "python.exe", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, `"import os; os.system('rm -rf /')"`, SpanInlineCode)
}

func TestClassify_InterpreterWithoutDashC(t *testing.T) {
	// python without -c: arguments should NOT be InlineCode.
	cmd := `python script.py --verbose`
	spans := Classify(cmd, nil)
	assertSpanKindAt(t, spans, cmd, "python", SpanExecuted)
	assertSpanKindAt(t, spans, cmd, "script.py", SpanArgument)
	assertSpanKindAt(t, spans, cmd, "--verbose", SpanArgument)
}
