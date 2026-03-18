package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/thgrace/training-wheels/internal/config"
	"github.com/thgrace/training-wheels/internal/exitcodes"
	"github.com/thgrace/training-wheels/internal/logger"
	"github.com/thgrace/training-wheels/internal/osutil"
	"github.com/thgrace/training-wheels/internal/skills"
	"github.com/spf13/cobra"
)

var (
	installProject     bool
	installAgentFilter string
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install TW hook into AI agent settings",
	RunE:  runInstall,
}

func init() {
	installCmd.Flags().BoolVar(&installProject, "project", false, "Install to project-level settings")
	installCmd.Flags().StringVar(&installAgentFilter, "agent", "", "Comma-separated list of agents to target (claude,cursor,gemini,copilot)")
}

var (
	uninstallProject     bool
	uninstallAgentFilter string
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove TW hook from AI agent settings",
	RunE:  runUninstall,
}

func init() {
	uninstallCmd.Flags().BoolVar(&uninstallProject, "project", false, "Remove from project-level settings")
	uninstallCmd.Flags().StringVar(&uninstallAgentFilter, "agent", "", "Comma-separated list of agents to target (claude,cursor,gemini,copilot)")
}

func runInstall(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		logger.Error("cannot determine home directory", "error", err)
		os.Exit(exitcodes.IOError)
	}

	// 1. Install the binary to ~/.tw/bin/tw
	twBinDir := filepath.Join(home, ".tw", "bin")
	if err := os.MkdirAll(twBinDir, 0o755); err != nil {
		logger.Error("error creating directory", "dir", twBinDir, "error", err)
		os.Exit(exitcodes.IOError)
	}

	self, err := os.Executable()
	if err != nil {
		logger.Error("cannot determine current executable path", "error", err)
		os.Exit(exitcodes.IOError)
	}

	twExe := "tw"
	if filepath.Ext(self) == ".exe" || os.PathSeparator == '\\' {
		twExe = "tw.exe"
	}

	twPath := filepath.Join(twBinDir, twExe)
	if err := copyBinary(self, twPath); err != nil {
		logger.Error("error installing binary", "to", twPath, "error", err)
		os.Exit(exitcodes.IOError)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "TW binary installed to %s\n", twPath)

	// 1.1 Create user-level config if it doesn't exist.
	createUserConfig(cmd.OutOrStdout(), home)

	// 1.5 Add ~/.tw/bin to the user's PATH.
	updatedPath, err := osutil.AddToPath(twBinDir)
	if err != nil {
		logger.Warn("could not automatically add TW to your PATH", "error", err)
		fmt.Fprintf(cmd.OutOrStdout(), "Tip: Manually add %s to your PATH to use 'tw' from any terminal.\n", twBinDir)
	} else if updatedPath {
		fmt.Fprintf(cmd.OutOrStdout(), "Added %s to your PATH. Please restart your terminal for changes to take effect.\n", twBinDir)
	}

	// 2. Install agent skill (cross-client)
	crossClientDir := filepath.Join(home, skills.SkillRelDir)
	installSkill(cmd.OutOrStdout(), crossClientDir)

	// 3. Install the hook into agent settings
	filter := parseAgentFilter(installAgentFilter)
	agents := detectAgents(home, filter)
	if len(agents) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No supported AI agent installations detected.")
		fmt.Fprintln(cmd.OutOrStdout(), "Tip: Run 'tw install --agent <name>' to install for a specific agent.")
		return nil
	}

	// 3.5 Install Claude-specific skill if Claude is detected.
	for _, a := range agents {
		if a.Name == "claude" {
			claudeDir := filepath.Join(home, skills.ClaudeSkillRelDir)
			installSkill(cmd.OutOrStdout(), claudeDir)
			break
		}
	}

	for _, a := range agents {
		path := agentSettingsPath(a, installProject, home)
		installForAgent(cmd.OutOrStdout(), a, path, twPath)
	}
	return nil
}

func copyBinary(src, dst string) error {
	// If src and dst are the same, skip copy.
	if s, err := filepath.Abs(src); err == nil {
		if d, err := filepath.Abs(dst); err == nil && s == d {
			return nil
		}
	}

	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	// Atomic write.
	tmpDst := dst + ".tmp"
	if err := os.WriteFile(tmpDst, input, 0o755); err != nil {
		return err
	}
	return osutil.AtomicRename(tmpDst, dst)
}

func runUninstall(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		logger.Error("cannot determine home directory", "error", err)
		os.Exit(exitcodes.IOError)
	}

	twExe := "tw"
	if os.PathSeparator == '\\' {
		twExe = "tw.exe"
	}
	twPath := filepath.Join(home, ".tw", "bin", twExe)

	filter := parseAgentFilter(uninstallAgentFilter)
	agents := detectAgents(home, filter)
	if len(agents) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No supported AI agent installations detected.")
		fmt.Fprintln(cmd.OutOrStdout(), "Tip: Run 'tw uninstall --agent <name>' to uninstall for a specific agent.")
		return nil
	}

	for _, a := range agents {
		path := agentSettingsPath(a, uninstallProject, home)
		uninstallForAgent(cmd.OutOrStdout(), a, path, twPath)
	}

	// Remove agent skills from both locations.
	uninstallSkill(cmd.OutOrStdout(), filepath.Join(home, skills.SkillRelDir))
	uninstallSkill(cmd.OutOrStdout(), filepath.Join(home, skills.ClaudeSkillRelDir))

	return nil
}

func installForAgent(out io.Writer, a agentDef, settingsPath, twPath string) {
	settings, _ := readSettings(settingsPath)
	if a.HookExists(settings, twPath) {
		fmt.Fprintf(out, "TW hook already installed in %s  [%s]\n", settingsPath, a.Name)
		return
	}
	a.AddHookEntry(settings, twPath)
	writeSettings(settingsPath, settings)
	fmt.Fprintf(out, "TW hook installed to %s  [%s]\n", settingsPath, a.Name)
}

func uninstallForAgent(out io.Writer, a agentDef, settingsPath, twPath string) {
	settings, existed := readSettings(settingsPath)
	if !existed {
		fmt.Fprintf(out, "TW hook not found in %s  [%s]\n", settingsPath, a.Name)
		return
	}
	if !a.RemoveHookEntry(settings, twPath) {
		fmt.Fprintf(out, "TW hook not found in %s  [%s]\n", settingsPath, a.Name)
		return
	}
	writeSettings(settingsPath, settings)
	fmt.Fprintf(out, "TW hook removed from %s  [%s]\n", settingsPath, a.Name)
}

// readSettings reads and JSON-unmarshals an agent settings file.
// Returns the settings map and whether the file existed.
// Exits on read errors (other than not-found) or parse errors.
func readSettings(path string) (settings map[string]interface{}, existed bool) {
	settings = make(map[string]interface{})
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return settings, false
		}
		logger.Error("error reading settings", "path", path, "error", err)
		os.Exit(exitcodes.IOError)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			logger.Error("error parsing settings", "path", path, "error", err)
			os.Exit(exitcodes.ConfigError)
		}
	}
	return settings, true
}

// writeSettings JSON-marshals settings and atomically writes them to path.
// Creates parent directories as needed. Exits on errors.
func writeSettings(path string, settings map[string]interface{}) {
	output, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		logger.Error("error marshaling settings", "error", err)
		os.Exit(exitcodes.IOError)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logger.Error("error creating directory", "dir", dir, "error", err)
		os.Exit(exitcodes.IOError)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, append(output, '\n'), 0o644); err != nil {
		logger.Error("error writing settings", "path", tmpPath, "error", err)
		os.Exit(exitcodes.IOError)
	}
	if err := osutil.AtomicRename(tmpPath, path); err != nil {
		logger.Error("error renaming settings", "from", tmpPath, "to", path, "error", err)
		os.Exit(exitcodes.IOError)
	}
}

func installSkill(out io.Writer, dir string) {
	if skills.ExistsAt(dir) {
		fmt.Fprintf(out, "Agent skill already installed in %s\n", dir)
		return
	}
	if err := skills.InstallTo(dir); err != nil {
		logger.Warn("could not install agent skill", "dir", dir, "error", err)
		return
	}
	fmt.Fprintf(out, "Agent skill installed to %s\n", dir)
}

func uninstallSkill(out io.Writer, dir string) {
	if err := skills.UninstallFrom(dir); err != nil {
		logger.Warn("could not remove agent skill", "dir", dir, "error", err)
		return
	}
	fmt.Fprintf(out, "Agent skill removed from %s\n", dir)
}

func createUserConfig(out io.Writer, home string) {
	cfgPath, err := config.UserConfigPath()
	if err != nil {
		logger.Warn("could not determine user config path", "error", err)
		return
	}
	if _, err := os.Stat(cfgPath); err == nil {
		fmt.Fprintf(out, "User config already exists at %s\n", cfgPath)
		return
	}
	dir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		logger.Warn("could not create config directory", "dir", dir, "error", err)
		return
	}
	starter := config.DefaultConfig()
	data, err := json.MarshalIndent(starter, "", "  ")
	if err != nil {
		logger.Warn("could not marshal default config", "error", err)
		return
	}
	if err := os.WriteFile(cfgPath, append(data, '\n'), 0o644); err != nil {
		logger.Warn("could not write user config", "path", cfgPath, "error", err)
		return
	}
	fmt.Fprintf(out, "User config created at %s\n", cfgPath)
}

func agentSettingsPath(a agentDef, project bool, home string) string {
	if project {
		return filepath.Join(a.ProjectDir, a.ProjectFile)
	}
	return filepath.Join(home, a.UserDir, a.UserFile)
}

func detectAgents(home string, filter []string) []agentDef {
	if len(filter) > 0 {
		filterSet := make(map[string]bool, len(filter))
		for _, name := range filter {
			filterSet[strings.ToLower(strings.TrimSpace(name))] = true
		}
		var result []agentDef
		for _, a := range allAgents {
			if filterSet[a.Name] {
				result = append(result, a)
			}
		}
		return result
	}

	var result []agentDef
	for _, a := range allAgents {
		dir := filepath.Join(home, a.UserDir)
		if _, err := os.Stat(dir); err == nil {
			result = append(result, a)
		}
	}
	return result
}

func parseAgentFilter(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
