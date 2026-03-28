package cli

import (
	"strings"
	"testing"
)

func TestTWDoctorDoesNotDependOnPATH(t *testing.T) {
	twBinary := buildTWBinary(t)
	home := t.TempDir()
	projectDir := t.TempDir()

	stdout, stderr, exitCode := runTWCommandWithEnvInDirs(
		t,
		twBinary,
		home,
		projectDir,
		[]string{"PATH=/usr/bin:/bin"},
		"doctor",
	)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0\nstdout=%q\nstderr=%q", exitCode, stdout, stderr)
	}
	if strings.Contains(stdout, "tw not found in PATH") {
		t.Fatalf("stdout = %q, want binary check to use current executable", stdout)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}
