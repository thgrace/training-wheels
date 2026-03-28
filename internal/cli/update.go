package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/thgrace/training-wheels/internal/config"
	"github.com/thgrace/training-wheels/internal/exitcodes"
	"github.com/thgrace/training-wheels/internal/logger"
	"github.com/thgrace/training-wheels/internal/osutil"
)

// httpClient is used for all update HTTP requests to enforce timeouts.
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

var (
	fetchLatestReleaseFn = fetchLatestRelease
	downloadToTempFn     = downloadToTemp
	verifyChecksumFn     = verifyChecksum
	verifyCosignBundleFn = verifyCosignBundle
	executablePathFn     = os.Executable
	evalSymlinksFn       = filepath.EvalSymlinks
	removeFileFn         = os.Remove
	renameFileFn         = os.Rename
	chmodFileFn          = os.Chmod
	copyFileFn           = osutil.CopyFile
)

var (
	updateCheckOnly bool
	updateJSON      bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update tw to the latest version",
	RunE:  runUpdate,
}

func init() {
	bindJSONOutputFlags(updateCmd.Flags(), &updateJSON)
	updateCmd.Flags().BoolVar(&updateCheckOnly, "check", false, "Only check for updates, don't download")
}

type updateJSONOutput struct {
	CurrentVersion    string `json:"current_version"`
	UpdateSource      string `json:"update_source"`
	LatestVersion     string `json:"latest_version,omitempty"`
	Status            string `json:"status"`
	UpdateAvailable   bool   `json:"update_available"`
	AssetName         string `json:"asset_name,omitempty"`
	ChecksumVerified  bool   `json:"checksum_verified,omitempty"`
	SignatureVerified bool   `json:"signature_verified,omitempty"`
}

// Version is set via ldflags at build time: -ldflags "-X ...cli.Version=v1.0.0"
var Version = "dev"

// defaultUpdateURL is the default GitHub API base for releases.
const defaultUpdateURL = "https://api.github.com/repos/thgrace/training-wheels/releases/latest"

// resolveUpdateURL returns the update URL from config, env, or default.
func resolveUpdateURL() string {
	// Env var takes highest precedence.
	if v := os.Getenv("TW_UPDATE_URL"); v != "" {
		return v
	}
	// Config file.
	cfg, err := config.Load()
	if err == nil && cfg.Update.URL != "" {
		return cfg.Update.URL
	}
	return defaultUpdateURL
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func runUpdate(cmd *cobra.Command, args []string) error {
	jsonOutput := useJSONOutput(updateJSON)
	updateURL := resolveUpdateURL()
	out := updateJSONOutput{
		CurrentVersion: Version,
		UpdateSource:   updateURL,
		Status:         "checking",
	}

	if !jsonOutput {
		fmt.Fprintf(cmd.OutOrStdout(), "Current version: %s\n", Version)
		fmt.Fprintf(cmd.OutOrStdout(), "Update source:   %s\n", updateURL)
	}

	// Fetch latest release info.
	release, err := fetchLatestReleaseFn(updateURL)
	if err != nil {
		logger.Error("failed to check for updates", "error", err)
		return silentExit(exitcodes.IOError)
	}

	latest := release.TagName
	out.LatestVersion = latest

	if !jsonOutput {
		fmt.Fprintf(cmd.OutOrStdout(), "Latest version:  %s\n", latest)
	}

	if Version == latest {
		out.Status = "up_to_date"
		if jsonOutput {
			return writeJSONOutput(cmd.OutOrStdout(), out)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Already up to date.")
		return nil
	}

	out.UpdateAvailable = true
	if updateCheckOnly {
		out.Status = "update_available"
		if jsonOutput {
			return writeJSONOutput(cmd.OutOrStdout(), out)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Update available. Run 'tw update' to install.")
		return nil
	}

	// Find the right asset for this OS/arch.
	assetName := expectedAssetName()
	var downloadURL string
	var checksumURL string
	var bundleURL string
	for _, a := range release.Assets {
		if a.Name == assetName {
			downloadURL = a.BrowserDownloadURL
		}
		if a.Name == assetName+".sha256" || a.Name == "checksums.txt" {
			checksumURL = a.BrowserDownloadURL
		}
		if a.Name == assetName+".bundle" {
			bundleURL = a.BrowserDownloadURL
		}
	}

	if downloadURL == "" {
		logger.Error("no release asset found",
			"os", runtime.GOOS,
			"arch", runtime.GOARCH,
			"expected", assetName)
		return silentExit(exitcodes.IOError)
	}

	out.AssetName = assetName
	if !jsonOutput {
		fmt.Fprintf(cmd.OutOrStdout(), "Downloading %s...\n", assetName)
	}

	// Download to temp file.
	tmpFile, err := downloadToTempFn(downloadURL)
	if err != nil {
		logger.Error("download failed", "error", err)
		return silentExit(exitcodes.IOError)
	}
	defer removeFileFn(tmpFile)

	// Verify checksum if available.
	if checksumURL != "" {
		if err := verifyChecksumFn(tmpFile, checksumURL, assetName); err != nil {
			logger.Error("checksum verification failed", "error", err)
			return silentExit(exitcodes.IOError)
		}
		out.ChecksumVerified = true
		if !jsonOutput {
			fmt.Fprintln(cmd.OutOrStdout(), "Checksum verified.")
		}
	}

	// Verify Cosign signature if bundle is available.
	if bundleURL != "" {
		bundleTmp, dlErr := downloadToTempFn(bundleURL)
		if dlErr != nil {
			logger.Error("failed to download signature bundle", "error", dlErr)
			return silentExit(exitcodes.IOError)
		}
		defer removeFileFn(bundleTmp)

		if verifyErr := verifyCosignBundleFn(tmpFile, bundleTmp); verifyErr != nil {
			logger.Error("signature verification failed", "error", verifyErr)
			return silentExit(exitcodes.IOError)
		}
		out.SignatureVerified = true
		if !jsonOutput {
			fmt.Fprintln(cmd.OutOrStdout(), "Signature verified.")
		}
	} else if !jsonOutput {
		fmt.Fprintln(cmd.OutOrStdout(), "Warning: no signature bundle found; skipping signature verification.")
	}

	// Find current binary path.
	currentBinary, err := executablePathFn()
	if err != nil {
		logger.Error("cannot determine current binary path", "error", err)
		return silentExit(exitcodes.IOError)
	}
	currentBinary, _ = evalSymlinksFn(currentBinary)

	// Replace current binary with downloaded version.
	if runtime.GOOS == "windows" {
		// Windows locks running executables. Rename the running binary
		// out of the way first, then move the new one into place.
		oldBinary := currentBinary + ".old"
		removeFileFn(oldBinary) // clean up from previous update
		if err := renameFileFn(currentBinary, oldBinary); err != nil {
			logger.Error("failed to move current binary", "error", err)
			return silentExit(exitcodes.IOError)
		}
		if err := renameFileFn(tmpFile, currentBinary); err != nil {
			if err := copyFileFn(tmpFile, currentBinary); err != nil {
				logger.Error("failed to install new binary", "error", err)
				return silentExit(exitcodes.IOError)
			}
		}
	} else {
		if err := chmodFileFn(tmpFile, 0o755); err != nil {
			logger.Error("chmod failed", "error", err)
			return silentExit(exitcodes.IOError)
		}
		if err := renameFileFn(tmpFile, currentBinary); err != nil {
			// Cross-device rename — fall back to copy.
			if err := copyFileFn(tmpFile, currentBinary); err != nil {
				logger.Error("failed to replace binary", "error", err)
				return silentExit(exitcodes.IOError)
			}
		}
	}

	out.Status = "updated"
	if jsonOutput {
		return writeJSONOutput(cmd.OutOrStdout(), out)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Updated to %s\n", latest)
	return nil
}

func fetchLatestRelease(url string) (*githubRelease, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "tw-updater")

	// Support GitHub token for private repos / rate limiting.
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parsing release JSON: %w", err)
	}
	return &release, nil
}

func expectedAssetName() string {
	goos := runtime.GOOS
	arch := runtime.GOARCH
	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}
	return fmt.Sprintf("tw-%s-%s%s", goos, arch, ext)
}

func downloadToTemp(url string) (string, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "tw-update-*")
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	tmp.Close()
	return tmp.Name(), nil
}

func verifyChecksum(filePath, checksumURL, assetName string) error {
	// Download checksum file.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, checksumURL, nil)
	if err != nil {
		return fmt.Errorf("creating checksum request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading checksum: %w", err)
	}
	defer resp.Body.Close()

	const maxChecksumSize = 1 << 20 // 1 MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxChecksumSize))
	if err != nil {
		return fmt.Errorf("reading checksum: %w", err)
	}

	// Parse checksum — could be single hash or "hash  filename" format.
	expectedHash := ""
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 1 {
			expectedHash = fields[0]
			break
		}
		if len(fields) >= 2 && (fields[1] == assetName || strings.HasSuffix(fields[1], "/"+assetName)) {
			expectedHash = fields[0]
			break
		}
	}

	if expectedHash == "" {
		return fmt.Errorf("no checksum found for %s", assetName)
	}

	// Compute actual hash.
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	actualHash := hex.EncodeToString(h.Sum(nil))

	if actualHash != expectedHash {
		return fmt.Errorf("SHA-256 mismatch: expected %s, got %s", expectedHash, actualHash)
	}
	return nil
}
