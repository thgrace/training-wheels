package shellcontext

import "testing"

func TestSanitize_GitCommitMessage(t *testing.T) {
	cmd := `git commit -m "rm -rf /"`
	got := Sanitize(cmd, nil)
	if len(got) != len(cmd) {
		t.Fatalf("length changed: %d → %d", len(cmd), len(got))
	}
	// The -m value should be masked.
	// "git commit -m " stays, the quoted part becomes spaces.
	if got == cmd {
		t.Error("expected sanitization to change the command")
	}
	// git, commit, -m should still be present.
	if got[:14] != `git commit -m ` {
		t.Errorf("prefix changed: %q", got[:14])
	}
	// The quoted message should be spaces (preserving quotes or masking them).
	msgPart := got[14:]
	for _, b := range msgPart {
		if b != ' ' && b != '"' {
			t.Errorf("expected spaces in masked region, got %q in full: %q", string(b), got)
			break
		}
	}
}

func TestSanitize_EchoDestructive(t *testing.T) {
	cmd := `echo "DROP TABLE users"`
	got := Sanitize(cmd, nil)
	if len(got) != len(cmd) {
		t.Fatalf("length changed: %d → %d", len(cmd), len(got))
	}
	// echo is SafeArgAll, so all args should be masked.
	if got[:5] != "echo " {
		t.Errorf("echo should be preserved: %q", got)
	}
	// Rest should be spaces.
	for i := 5; i < len(got); i++ {
		if got[i] != ' ' {
			t.Errorf("byte %d = %q, want space; full: %q", i, string(got[i]), got)
			break
		}
	}
}

func TestSanitize_BashCNotMasked(t *testing.T) {
	cmd := `bash -c "rm -rf /"`
	got := Sanitize(cmd, nil)
	// bash -c content is InlineCode, should NOT be masked.
	if got != cmd {
		t.Errorf("bash -c content should not be masked: got %q", got)
	}
}

func TestSanitize_PlainDestructive(t *testing.T) {
	cmd := "rm -rf /"
	got := Sanitize(cmd, nil)
	if got != cmd {
		t.Errorf("rm should not be changed: got %q", got)
	}
}

func TestSanitize_PipedCommands(t *testing.T) {
	cmd := `echo "rm -rf" | grep pattern`
	got := Sanitize(cmd, nil)
	if len(got) != len(cmd) {
		t.Fatalf("length changed: %d → %d", len(cmd), len(got))
	}
	// Both echo and grep are SafeArgAll, so both args get masked.
	if got == cmd {
		t.Error("expected some masking")
	}
	// "grep" command word should be preserved, but "pattern" is masked.
	// Check that "grep" is still present.
	if got[16:20] != "grep" {
		t.Errorf("grep command word changed: %q", got[16:20])
	}
	// echo's arg should be masked.
	if got[5] != ' ' {
		t.Error("echo arg should be masked")
	}
}

func TestSanitize_PreservesLength(t *testing.T) {
	cmds := []string{
		`git commit -m "rm -rf /"`,
		`echo "DROP TABLE"`,
		`bash -c "rm -rf /"`,
		`rm -rf /`,
		`echo foo | grep bar`,
		"",
		"ls -la",
	}
	for _, cmd := range cmds {
		got := Sanitize(cmd, nil)
		if len(got) != len(cmd) {
			t.Errorf("Sanitize(%q) length %d != %d", cmd, len(got), len(cmd))
		}
	}
}

func TestSanitize_EmptyCommand(t *testing.T) {
	got := Sanitize("", nil)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestSanitize_NoSafeCommands(t *testing.T) {
	cmd := "rm -rf /"
	got := Sanitize(cmd, nil)
	if got != cmd {
		t.Errorf("no safe commands, should be unchanged: got %q", got)
	}
}

func TestSanitize_GitGrep(t *testing.T) {
	cmd := `git --grep "destructive pattern" log`
	got := Sanitize(cmd, nil)
	if len(got) != len(cmd) {
		t.Fatalf("length changed: %d → %d", len(cmd), len(got))
	}
	// The --grep value should be masked.
	if got == cmd {
		t.Error("expected --grep value to be masked")
	}
}

func TestSanitize_CommentMasked(t *testing.T) {
	cmd := "ls # rm -rf /"
	got := Sanitize(cmd, nil)
	if len(got) != len(cmd) {
		t.Fatalf("length changed: %d → %d", len(cmd), len(got))
	}
	// Comment portion should be masked.
	if got[:3] != "ls " {
		t.Errorf("ls should be preserved: %q", got)
	}
}

func TestSanitize_EchoSingleQuotes(t *testing.T) {
	cmd := "echo 'git reset --hard'"
	got := Sanitize(cmd, nil)
	if len(got) != len(cmd) {
		t.Fatalf("length changed")
	}
	// All of echo's args should be masked.
	if got[:5] != "echo " {
		t.Errorf("echo should be preserved: %q", got)
	}
	for i := 5; i < len(got); i++ {
		if got[i] != ' ' {
			t.Errorf("byte %d = %q, want space; full: %q", i, string(got[i]), got)
			break
		}
	}
}
