package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/thgrace/training-wheels/internal/exitcodes"
)

func TestTWUpdateWithCosignSubprocess(t *testing.T) {
	const currentVersion = "v1.0.0"
	const latestVersion = "v2.0.0"

	assetName := expectedAssetName()
	newBinary := buildTWBinaryWithVersion(t, latestVersion)
	newBinaryBytes, err := os.ReadFile(newBinary)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", newBinary, err)
	}
	bundleBytes := []byte(`{"bundle":"placeholder"}`)

	home := t.TempDir()
	projectDir := t.TempDir()

	t.Run("successful update verifies checksum and cosign", func(t *testing.T) {
		twBinary := buildTWBinaryWithVersion(t, currentVersion)
		server := newUpdateTestServer(t, latestVersion, assetName, newBinaryBytes, bundleBytes)
		defer server.Close()

		stdout, stderr, exitCode := runTWCommandWithEnvInDirs(
			t,
			twBinary,
			home,
			projectDir,
			[]string{
				"TW_UPDATE_URL=" + server.URL + "/latest",
				testCosignVerifyResultEnv + "=ok",
			},
			"update",
			"--json",
		)
		if exitCode != 0 {
			t.Fatalf("exit code = %d, want 0\nstdout=%q\nstderr=%q", exitCode, stdout, stderr)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q, want empty", stderr)
		}

		var out updateJSONOutput
		if err := json.Unmarshal([]byte(stdout), &out); err != nil {
			t.Fatalf("json.Unmarshal(stdout) error = %v\nstdout=%q", err, stdout)
		}
		if out.Status != "updated" {
			t.Fatalf("status = %q, want updated", out.Status)
		}
		if !out.ChecksumVerified {
			t.Fatal("checksum_verified = false, want true")
		}
		if !out.SignatureVerified {
			t.Fatal("signature_verified = false, want true")
		}
		if out.LatestVersion != latestVersion {
			t.Fatalf("latest_version = %q, want %q", out.LatestVersion, latestVersion)
		}

		versionStdout, versionStderr, versionExitCode := runTWCommandInDirs(t, twBinary, home, projectDir, "version", "--json")
		if versionExitCode != 0 {
			t.Fatalf("updated binary exit code = %d, want 0\nstdout=%q\nstderr=%q", versionExitCode, versionStdout, versionStderr)
		}
		if versionStderr != "" {
			t.Fatalf("updated binary stderr = %q, want empty", versionStderr)
		}

		var versionOut struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal([]byte(versionStdout), &versionOut); err != nil {
			t.Fatalf("json.Unmarshal(version stdout) error = %v\nstdout=%q", err, versionStdout)
		}
		if versionOut.Version != latestVersion {
			t.Fatalf("updated binary version = %q, want %q", versionOut.Version, latestVersion)
		}
	})

	t.Run("invalid cosign bundle fails update", func(t *testing.T) {
		twBinary := buildTWBinaryWithVersion(t, currentVersion)
		server := newUpdateTestServer(t, latestVersion, assetName, newBinaryBytes, bundleBytes)
		defer server.Close()

		stdout, stderr, exitCode := runTWCommandWithEnvInDirs(
			t,
			twBinary,
			home,
			projectDir,
			[]string{
				"TW_UPDATE_URL=" + server.URL + "/latest",
				testCosignVerifyResultEnv + "=test cosign verification failure",
			},
			"update",
			"--json",
		)
		if exitCode != exitcodes.IOError {
			t.Fatalf("exit code = %d, want %d\nstdout=%q\nstderr=%q", exitCode, exitcodes.IOError, stdout, stderr)
		}
		if stdout != "" {
			t.Fatalf("stdout = %q, want empty on failure", stdout)
		}
		if !containsAll(stderr, "signature verification failed", "error=") {
			t.Fatalf("stderr = %q, want signature verification failure", stderr)
		}

		versionStdout, versionStderr, versionExitCode := runTWCommandInDirs(t, twBinary, home, projectDir, "version", "--json")
		if versionExitCode != 0 {
			t.Fatalf("post-failure version exit code = %d, want 0\nstdout=%q\nstderr=%q", versionExitCode, versionStdout, versionStderr)
		}
		if versionStderr != "" {
			t.Fatalf("post-failure version stderr = %q, want empty", versionStderr)
		}

		var versionOut struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal([]byte(versionStdout), &versionOut); err != nil {
			t.Fatalf("json.Unmarshal(version stdout) error = %v\nstdout=%q", err, versionStdout)
		}
		if versionOut.Version != currentVersion {
			t.Fatalf("binary version after failed update = %q, want %q", versionOut.Version, currentVersion)
		}
	})
}

func newUpdateTestServer(t *testing.T, latestVersion, assetName string, assetBytes, bundleBytes []byte) *httptest.Server {
	t.Helper()

	checksum := sha256.Sum256(assetBytes)
	checksumBody := hex.EncodeToString(checksum[:]) + "  " + assetName + "\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(githubRelease{
				TagName: latestVersion,
				Assets: []githubAsset{
					{Name: assetName, BrowserDownloadURL: serverURL(r, "/assets/"+assetName)},
					{Name: assetName + ".sha256", BrowserDownloadURL: serverURL(r, "/assets/"+assetName+".sha256")},
					{Name: assetName + ".bundle", BrowserDownloadURL: serverURL(r, "/assets/"+assetName+".bundle")},
				},
			})
		case "/assets/" + assetName:
			_, _ = w.Write(assetBytes)
		case "/assets/" + assetName + ".sha256":
			_, _ = io.WriteString(w, checksumBody)
		case "/assets/" + assetName + ".bundle":
			_, _ = w.Write(bundleBytes)
		default:
			http.NotFound(w, r)
		}
	}))

	return server
}

func buildTWBinaryWithVersion(tb testing.TB, version string) string {
	tb.Helper()

	repoRoot := benchmarkRepoRoot(tb)
	outDir := tb.TempDir()
	binaryPath := filepath.Join(outDir, "tw")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	cmd := exec.Command(
		"go",
		"build",
		"-ldflags",
		"-X github.com/thgrace/training-wheels/internal/cli.Version="+version,
		"-o",
		binaryPath,
		"./cmd/tw",
	)
	cmd.Dir = repoRoot
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		tb.Fatalf("go build tw version=%s: %v", version, err)
	}

	return binaryPath
}

func runTWCommandWithEnvInDirs(t *testing.T, twBinary, home, projectDir string, extraEnv []string, args ...string) (stdout string, stderr string, exitCode int) {
	t.Helper()

	cmd := exec.Command(twBinary, args...)
	cmd.Dir = projectDir
	cmd.Env = append(twCLITestEnv(home), extraEnv...)

	outBytes, errBytes, err := runCommandCapture(cmd)
	exitCode = 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("run tw %v: %v", args, err)
		}
		exitCode = exitErr.ExitCode()
	}

	return string(outBytes), string(errBytes), exitCode
}

func serverURL(r *http.Request, path string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host + path
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
