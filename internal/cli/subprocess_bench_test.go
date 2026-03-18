package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/thgrace/training-wheels/internal/exitcodes"
)

func BenchmarkTWSubprocessHook_Allow(b *testing.B) {
	twBinary := buildTWBinary(b)
	home := b.TempDir()
	projectDir := b.TempDir()
	payload := []byte(`{"tool_name":"Bash","tool_input":{"command":"git status"}}`)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.Command(twBinary, "hook")
		cmd.Dir = projectDir
		cmd.Env = benchmarkHookEnv(home)
		cmd.Stdin = bytes.NewReader(payload)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if err := cmd.Run(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTWSubprocessHook_Deny(b *testing.B) {
	twBinary := buildTWBinary(b)
	home := b.TempDir()
	projectDir := b.TempDir()
	payload := []byte(`{"tool_name":"Bash","tool_input":{"command":"git reset --hard HEAD"}}`)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.Command(twBinary, "hook")
		cmd.Dir = projectDir
		cmd.Env = benchmarkHookEnv(home)
		cmd.Stdin = bytes.NewReader(payload)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard

		err := cmd.Run()
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != exitcodes.Deny {
			b.Fatalf("expected deny exit code %d, got err=%v", exitcodes.Deny, err)
		}
	}
}

func buildTWBinary(tb testing.TB) string {
	tb.Helper()

	repoRoot := benchmarkRepoRoot(tb)
	outDir := tb.TempDir()
	binaryPath := filepath.Join(outDir, "tw")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/tw")
	cmd.Dir = repoRoot
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		tb.Fatalf("go build tw: %v", err)
	}

	return binaryPath
}

func benchmarkRepoRoot(tb testing.TB) string {
	tb.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		tb.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}

func benchmarkHookEnv(home string) []string {
	return append(os.Environ(),
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
	)
}
