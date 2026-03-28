package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/thgrace/training-wheels/internal/exitcodes"
)

func TestRunUpdateFetchFailureReturnsSilentExit(t *testing.T) {
	restore := stubUpdateDependencies(t)
	defer restore()

	updateJSON = true
	updateCheckOnly = false

	fetchLatestReleaseFn = func(string) (*githubRelease, error) {
		return nil, errors.New("boom")
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := runUpdate(cmd, nil)
	if err == nil {
		t.Fatal("runUpdate() error = nil, want exit error")
	}
	if got := ExitCode(err); got != exitcodes.IOError {
		t.Fatalf("ExitCode(runUpdate()) = %d, want %d", got, exitcodes.IOError)
	}
	if ShouldPrintError(err) {
		t.Fatal("ShouldPrintError(runUpdate()) = true, want false")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunUpdateCosignIntegration(t *testing.T) {
	t.Run("bundle verifies and sets json flag", func(t *testing.T) {
		restore := stubUpdateDependencies(t)
		defer restore()

		updateJSON = true
		updateCheckOnly = false
		Version = "v1.0.0"

		assetName := expectedAssetName()
		assetURL := "https://example.test/" + assetName
		checksumURL := "https://example.test/" + assetName + ".sha256"
		bundleURL := "https://example.test/" + assetName + ".bundle"

		currentBinary := filepath.Join(t.TempDir(), "tw-current")
		if err := os.WriteFile(currentBinary, []byte("old binary"), 0o755); err != nil {
			t.Fatalf("WriteFile(%s): %v", currentBinary, err)
		}

		downloadedAsset := filepath.Join(t.TempDir(), "downloaded-asset")
		if err := os.WriteFile(downloadedAsset, []byte("new binary"), 0o755); err != nil {
			t.Fatalf("WriteFile(%s): %v", downloadedAsset, err)
		}

		downloadedBundle := filepath.Join(t.TempDir(), "downloaded.bundle")
		if err := os.WriteFile(downloadedBundle, []byte("bundle"), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", downloadedBundle, err)
		}

		fetchLatestReleaseFn = func(string) (*githubRelease, error) {
			return &githubRelease{
				TagName: "v2.0.0",
				Assets: []githubAsset{
					{Name: assetName, BrowserDownloadURL: assetURL},
					{Name: assetName + ".sha256", BrowserDownloadURL: checksumURL},
					{Name: assetName + ".bundle", BrowserDownloadURL: bundleURL},
				},
			}, nil
		}

		downloadToTempFn = func(url string) (string, error) {
			switch url {
			case assetURL:
				return downloadedAsset, nil
			case bundleURL:
				return downloadedBundle, nil
			default:
				t.Fatalf("downloadToTempFn url = %q, want %q or %q", url, assetURL, bundleURL)
				return "", nil
			}
		}

		var checksumCalls int
		verifyChecksumFn = func(filePath, url, gotAssetName string) error {
			checksumCalls++
			if filePath != downloadedAsset {
				t.Fatalf("verifyChecksumFn filePath = %q, want %q", filePath, downloadedAsset)
			}
			if url != checksumURL {
				t.Fatalf("verifyChecksumFn url = %q, want %q", url, checksumURL)
			}
			if gotAssetName != assetName {
				t.Fatalf("verifyChecksumFn assetName = %q, want %q", gotAssetName, assetName)
			}
			return nil
		}

		var cosignCalls int
		verifyCosignBundleFn = func(binaryPath, bundlePath string) error {
			cosignCalls++
			if binaryPath != downloadedAsset {
				t.Fatalf("verifyCosignBundleFn binaryPath = %q, want %q", binaryPath, downloadedAsset)
			}
			if bundlePath != downloadedBundle {
				t.Fatalf("verifyCosignBundleFn bundlePath = %q, want %q", bundlePath, downloadedBundle)
			}
			return nil
		}

		executablePathFn = func() (string, error) { return currentBinary, nil }
		evalSymlinksFn = func(path string) (string, error) { return path, nil }

		var stdout bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&stdout)
		cmd.SetErr(&bytes.Buffer{})

		if err := runUpdate(cmd, nil); err != nil {
			t.Fatalf("runUpdate() error = %v, want nil", err)
		}
		if checksumCalls != 1 {
			t.Fatalf("verifyChecksumFn called %d times, want 1", checksumCalls)
		}
		if cosignCalls != 1 {
			t.Fatalf("verifyCosignBundleFn called %d times, want 1", cosignCalls)
		}

		var out updateJSONOutput
		if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
			t.Fatalf("json.Unmarshal(stdout) error = %v\nstdout=%q", err, stdout.String())
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
		if out.AssetName != assetName {
			t.Fatalf("asset_name = %q, want %q", out.AssetName, assetName)
		}

		replacedBytes, err := os.ReadFile(currentBinary)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", currentBinary, err)
		}
		if string(replacedBytes) != "new binary" {
			t.Fatalf("current binary contents = %q, want %q", string(replacedBytes), "new binary")
		}
	})

	t.Run("missing bundle skips cosign and warns", func(t *testing.T) {
		restore := stubUpdateDependencies(t)
		defer restore()

		updateJSON = false
		updateCheckOnly = false
		Version = "v1.0.0"

		assetName := expectedAssetName()
		assetURL := "https://example.test/" + assetName
		checksumURL := "https://example.test/" + assetName + ".sha256"

		currentBinary := filepath.Join(t.TempDir(), "tw-current")
		if err := os.WriteFile(currentBinary, []byte("old binary"), 0o755); err != nil {
			t.Fatalf("WriteFile(%s): %v", currentBinary, err)
		}

		downloadedAsset := filepath.Join(t.TempDir(), "downloaded-asset")
		if err := os.WriteFile(downloadedAsset, []byte("new binary"), 0o755); err != nil {
			t.Fatalf("WriteFile(%s): %v", downloadedAsset, err)
		}

		fetchLatestReleaseFn = func(string) (*githubRelease, error) {
			return &githubRelease{
				TagName: "v2.0.0",
				Assets: []githubAsset{
					{Name: assetName, BrowserDownloadURL: assetURL},
					{Name: assetName + ".sha256", BrowserDownloadURL: checksumURL},
				},
			}, nil
		}

		downloadToTempFn = func(url string) (string, error) {
			if url != assetURL {
				t.Fatalf("downloadToTempFn url = %q, want %q", url, assetURL)
			}
			return downloadedAsset, nil
		}

		verifyChecksumFn = func(filePath, url, gotAssetName string) error {
			if filePath != downloadedAsset || url != checksumURL || gotAssetName != assetName {
				t.Fatalf("verifyChecksumFn got (%q, %q, %q)", filePath, url, gotAssetName)
			}
			return nil
		}

		verifyCosignBundleFn = func(string, string) error {
			t.Fatal("verifyCosignBundleFn should not be called when no bundle is advertised")
			return nil
		}

		executablePathFn = func() (string, error) { return currentBinary, nil }
		evalSymlinksFn = func(path string) (string, error) { return path, nil }

		var stdout bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&stdout)
		cmd.SetErr(&bytes.Buffer{})

		if err := runUpdate(cmd, nil); err != nil {
			t.Fatalf("runUpdate() error = %v, want nil", err)
		}

		output := stdout.String()
		if !strings.Contains(output, "Warning: no signature bundle found; skipping signature verification.") {
			t.Fatalf("stdout = %q, want missing bundle warning", output)
		}
		if !strings.Contains(output, "Updated to v2.0.0") {
			t.Fatalf("stdout = %q, want update success message", output)
		}
	})
}

func stubUpdateDependencies(t *testing.T) func() {
	t.Helper()

	origUpdateJSON := updateJSON
	origUpdateCheckOnly := updateCheckOnly
	origVersion := Version
	origFetchLatestReleaseFn := fetchLatestReleaseFn
	origDownloadToTempFn := downloadToTempFn
	origVerifyChecksumFn := verifyChecksumFn
	origVerifyCosignBundleFn := verifyCosignBundleFn
	origExecutablePathFn := executablePathFn
	origEvalSymlinksFn := evalSymlinksFn
	origRemoveFileFn := removeFileFn
	origRenameFileFn := renameFileFn
	origChmodFileFn := chmodFileFn
	origCopyFileFn := copyFileFn

	return func() {
		updateJSON = origUpdateJSON
		updateCheckOnly = origUpdateCheckOnly
		Version = origVersion
		fetchLatestReleaseFn = origFetchLatestReleaseFn
		downloadToTempFn = origDownloadToTempFn
		verifyChecksumFn = origVerifyChecksumFn
		verifyCosignBundleFn = origVerifyCosignBundleFn
		executablePathFn = origExecutablePathFn
		evalSymlinksFn = origEvalSymlinksFn
		removeFileFn = origRemoveFileFn
		renameFileFn = origRenameFileFn
		chmodFileFn = origChmodFileFn
		copyFileFn = origCopyFileFn
	}
}
