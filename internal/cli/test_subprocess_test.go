package cli

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thgrace/training-wheels/internal/exitcodes"
)

func TestTWTestCommandFlags(t *testing.T) {
	twBinary := buildTWBinary(t)

	tests := []struct {
		name        string
		args        []string
		wantExit    int
		checkStdout func(t *testing.T, stdout string)
		checkStderr func(t *testing.T, stderr string)
	}{
		{
			name:     "json output stays in test mode",
			args:     []string{"test", "--json", "git status"},
			wantExit: exitcodes.Allow,
			checkStdout: func(t *testing.T, stdout string) {
				t.Helper()

				var out struct {
					Decision string `json:"decision"`
				}
				if err := json.Unmarshal([]byte(stdout), &out); err != nil {
					t.Fatalf("stdout is not valid test json: %v\nstdout=%q", err, stdout)
				}
				if out.Decision != "allow" {
					t.Fatalf("decision = %q, want allow", out.Decision)
				}
				if strings.Contains(stdout, "=== TW Explain ===") {
					t.Fatalf("stdout unexpectedly contains explain output: %q", stdout)
				}
			},
			checkStderr: func(t *testing.T, stderr string) {
				t.Helper()
				if strings.TrimSpace(stderr) != "" {
					t.Fatalf("stderr = %q, want empty", stderr)
				}
			},
		},
		{
			name:     "expect deny is honored",
			args:     []string{"test", "--expect", "deny", "git push --force"},
			wantExit: exitcodes.Allow,
			checkStdout: func(t *testing.T, stdout string) {
				t.Helper()
				if !strings.Contains(stdout, "Expected: deny") {
					t.Fatalf("stdout missing expected decision: %q", stdout)
				}
				if !strings.Contains(stdout, "Got:      deny") {
					t.Fatalf("stdout missing actual decision: %q", stdout)
				}
				if !strings.Contains(stdout, "PASS") {
					t.Fatalf("stdout missing PASS marker: %q", stdout)
				}
			},
			checkStderr: func(t *testing.T, stderr string) {
				t.Helper()
				if strings.TrimSpace(stderr) != "" {
					t.Fatalf("stderr = %q, want empty", stderr)
				}
			},
		},
		{
			name:     "force deny is honored",
			args:     []string{"test", "--force", "deny", "--json", "git status"},
			wantExit: exitcodes.Deny,
			checkStdout: func(t *testing.T, stdout string) {
				t.Helper()

				var out struct {
					Decision string `json:"decision"`
				}
				if err := json.Unmarshal([]byte(stdout), &out); err != nil {
					t.Fatalf("stdout is not valid test json: %v\nstdout=%q", err, stdout)
				}
				if out.Decision != "deny" {
					t.Fatalf("decision = %q, want deny", out.Decision)
				}
			},
			checkStderr: func(t *testing.T, stderr string) {
				t.Helper()
				if strings.TrimSpace(stderr) != "" {
					t.Fatalf("stderr = %q, want empty", stderr)
				}
			},
		},
		{
			name:     "format flag is rejected",
			args:     []string{"test", "--format", "json", "git status"},
			wantExit: 1,
			checkStdout: func(t *testing.T, stdout string) {
				t.Helper()
				if strings.TrimSpace(stdout) != "" {
					t.Fatalf("stdout = %q, want empty", stdout)
				}
			},
			checkStderr: func(t *testing.T, stderr string) {
				t.Helper()
				if !strings.Contains(stderr, "unknown flag: --format") {
					t.Fatalf("stderr = %q, want unknown flag error", stderr)
				}
			},
		},
		{
			name:     "invalid shell is rejected",
			args:     []string{"test", "--shell", "nonsense", "git status"},
			wantExit: exitcodes.ConfigError,
			checkStdout: func(t *testing.T, stdout string) {
				t.Helper()
				if !strings.Contains(stdout, `invalid --shell value "nonsense": must be bash, cmd, posix, powershell, pwsh, sh, or zsh`) {
					t.Fatalf("stdout = %q, want invalid shell error", stdout)
				}
			},
			checkStderr: func(t *testing.T, stderr string) {
				t.Helper()
				if strings.TrimSpace(stderr) != "" {
					t.Fatalf("stderr = %q, want empty", stderr)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, exitCode := runTWCommand(t, twBinary, tt.args...)
			if exitCode != tt.wantExit {
				t.Fatalf("exit code = %d, want %d\nstdout=%q\nstderr=%q", exitCode, tt.wantExit, stdout, stderr)
			}
			tt.checkStdout(t, stdout)
			tt.checkStderr(t, stderr)
		})
	}
}

func TestTWExplainRejectsInvalidShellFlag(t *testing.T) {
	twBinary := buildTWBinary(t)

	stdout, stderr, exitCode := runTWCommand(t, twBinary, "explain", "--shell", "nonsense", "git status")
	if exitCode != exitcodes.ConfigError {
		t.Fatalf("exit code = %d, want %d\nstdout=%q\nstderr=%q", exitCode, exitcodes.ConfigError, stdout, stderr)
	}
	if !strings.Contains(stdout, `invalid --shell value "nonsense": must be bash, cmd, posix, powershell, pwsh, sh, or zsh`) {
		t.Fatalf("stdout = %q, want invalid shell error", stdout)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

func TestTWJSONFlagSupportAcrossCommands(t *testing.T) {
	twBinary := buildTWBinary(t)

	t.Run("override list supports json", func(t *testing.T) {
		stdout, stderr, exitCode := runTWCommand(t, twBinary, "override", "list", "--json")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, want 0\nstdout=%q\nstderr=%q", exitCode, stdout, stderr)
		}
		if strings.TrimSpace(stderr) != "" {
			t.Fatalf("stderr = %q, want empty", stderr)
		}

		var out []map[string]any
		if err := json.Unmarshal([]byte(stdout), &out); err != nil {
			t.Fatalf("stdout is not valid override json: %v\nstdout=%q", err, stdout)
		}
	})

	t.Run("version supports json", func(t *testing.T) {
		stdout, stderr, exitCode := runTWCommand(t, twBinary, "version", "--json")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, want 0\nstdout=%q\nstderr=%q", exitCode, stdout, stderr)
		}
		if strings.TrimSpace(stderr) != "" {
			t.Fatalf("stderr = %q, want empty", stderr)
		}

		var out struct {
			Version string `json:"version"`
			GOOS    string `json:"goos"`
			GOARCH  string `json:"goarch"`
		}
		if err := json.Unmarshal([]byte(stdout), &out); err != nil {
			t.Fatalf("stdout is not valid version json: %v\nstdout=%q", err, stdout)
		}
		if out.Version == "" || out.GOOS == "" || out.GOARCH == "" {
			t.Fatalf("version json missing required fields: %+v", out)
		}
	})

	t.Run("help shows json for json-capable commands", func(t *testing.T) {
		for _, command := range []string{"override", "install", "uninstall", "update", "version"} {
			stdout, stderr, exitCode := runTWCommand(t, twBinary, command, "--help")
			if exitCode != 0 {
				t.Fatalf("%s help exit code = %d, want 0\nstdout=%q\nstderr=%q", command, exitCode, stdout, stderr)
			}
			if strings.TrimSpace(stderr) != "" {
				t.Fatalf("%s help stderr = %q, want empty", command, stderr)
			}
			if !strings.Contains(stdout, "--json") {
				t.Fatalf("%s help missing --json flag:\n%s", command, stdout)
			}
		}
	})

	t.Run("root help shows completions command", func(t *testing.T) {
		stdout, stderr, exitCode := runTWCommand(t, twBinary, "--help")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, want 0\nstdout=%q\nstderr=%q", exitCode, stdout, stderr)
		}
		if strings.TrimSpace(stderr) != "" {
			t.Fatalf("stderr = %q, want empty", stderr)
		}
		if !strings.Contains(stdout, "completions") {
			t.Fatalf("root help missing completions command:\n%s", stdout)
		}
		if !strings.Contains(stdout, "Generate shell completion scripts") {
			t.Fatalf("root help missing completions description:\n%s", stdout)
		}
	})
}

func TestTWGlobalRobotFlagRejected(t *testing.T) {
	twBinary := buildTWBinary(t)

	t.Run("test rejects robot", func(t *testing.T) {
		stdout, stderr, exitCode := runTWCommand(t, twBinary, "test", "--robot", "git status")
		if exitCode != 1 {
			t.Fatalf("exit code = %d, want 1\nstdout=%q\nstderr=%q", exitCode, stdout, stderr)
		}
		if strings.TrimSpace(stdout) != "" {
			t.Fatalf("stdout = %q, want empty", stdout)
		}
		if !strings.Contains(stderr, "unknown flag: --robot") {
			t.Fatalf("stderr = %q, want unknown flag error", stderr)
		}
	})

	t.Run("config rejects robot", func(t *testing.T) {
		stdout, stderr, exitCode := runTWCommand(t, twBinary, "config", "--robot")
		if exitCode != 1 {
			t.Fatalf("exit code = %d, want 1\nstdout=%q\nstderr=%q", exitCode, stdout, stderr)
		}
		if strings.TrimSpace(stdout) != "" {
			t.Fatalf("stdout = %q, want empty", stdout)
		}
		if !strings.Contains(stderr, "unknown flag: --robot") {
			t.Fatalf("stderr = %q, want unknown flag error", stderr)
		}
	})
}

func runTWCommand(t *testing.T, twBinary string, args ...string) (stdout string, stderr string, exitCode int) {
	t.Helper()

	home := t.TempDir()
	projectDir := t.TempDir()

	return runTWCommandInDirs(t, twBinary, home, projectDir, args...)
}

func runTWCommandInDirs(t *testing.T, twBinary, home, projectDir string, args ...string) (stdout string, stderr string, exitCode int) {
	t.Helper()

	cmd := exec.Command(twBinary, args...)
	cmd.Dir = projectDir
	cmd.Env = twCLITestEnv(home)

	outBytes, errBytes, err := runCommandCapture(cmd)
	exitCode = exitcodes.Allow
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("run tw %v: %v", args, err)
		}
		exitCode = exitErr.ExitCode()
	}

	return string(outBytes), string(errBytes), exitCode
}

func runCommandCapture(cmd *exec.Cmd) ([]byte, []byte, error) {
	var stdout strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return []byte(stdout.String()), []byte(stderr.String()), err
}

func twCLITestEnv(home string) []string {
	env := make([]string, 0, len(os.Environ())+2)
	for _, entry := range os.Environ() {
		switch {
		case strings.HasPrefix(entry, "HOME="):
			continue
		case strings.HasPrefix(entry, "XDG_CONFIG_HOME="):
			continue
		case strings.HasPrefix(entry, "TW_"):
			continue
		}
		env = append(env, entry)
	}

	env = append(env,
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
	)
	return env
}
