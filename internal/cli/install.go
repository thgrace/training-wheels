package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/thgrace/training-wheels/internal/agentsettings"
	"github.com/thgrace/training-wheels/internal/config"
	"github.com/thgrace/training-wheels/internal/exitcodes"
	"github.com/thgrace/training-wheels/internal/logger"
	"github.com/thgrace/training-wheels/internal/osutil"
	"github.com/thgrace/training-wheels/internal/skills"
)

var (
	installProject     bool
	installAgentFilter string
	installJSON        bool
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install TW hooks and skills into AI agent settings",
	RunE:  runInstall,
}

func init() {
	bindJSONOutputFlags(installCmd.Flags(), &installJSON)
	installCmd.Flags().BoolVar(&installProject, "project", false, "Install to project-level settings")
	installCmd.Flags().StringVar(&installAgentFilter, "agent", "", "Comma-separated list of agents to target (claude,cursor,gemini,copilot)")
}

var (
	uninstallProject     bool
	uninstallAgentFilter string
	uninstallJSON        bool
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove TW hooks and skills from AI agent settings",
	RunE:  runUninstall,
}

func init() {
	bindJSONOutputFlags(uninstallCmd.Flags(), &uninstallJSON)
	uninstallCmd.Flags().BoolVar(&uninstallProject, "project", false, "Remove from project-level settings")
	uninstallCmd.Flags().StringVar(&uninstallAgentFilter, "agent", "", "Comma-separated list of agents to target (claude,cursor,gemini,copilot)")
}

type installAction struct {
	Kind    string `json:"kind"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
	Agent   string `json:"agent,omitempty"`
}

type installReporter struct {
	w       io.Writer
	json    bool
	actions []installAction
}

func newInstallReporter(w io.Writer, jsonOutput bool) *installReporter {
	return &installReporter{w: w, json: useJSONOutput(jsonOutput)}
}

func (r *installReporter) record(action installAction) {
	if r.json {
		r.actions = append(r.actions, action)
		return
	}
	fmt.Fprintln(r.w, action.Message)
}

func (r *installReporter) flush() error {
	if !r.json {
		return nil
	}
	return writeJSONOutput(r.w, r.actions)
}

func stableBinaryPath(home string) string {
	name := "tw"
	if osutil.IsWindows() {
		name = "tw.exe"
	}
	return filepath.Join(home, ".tw", "bin", name)
}

func runInstall(cmd *cobra.Command, args []string) error {
	reporter := newInstallReporter(cmd.OutOrStdout(), installJSON)

	home, err := os.UserHomeDir()
	if err != nil {
		logger.Error("cannot determine home directory", "error", err)
		os.Exit(exitcodes.IOError)
	}

	stablePath := stableBinaryPath(home)
	currentExec, err := os.Executable()
	if err != nil {
		logger.Error("cannot determine current binary path", "error", err)
		os.Exit(exitcodes.IOError)
	}

	if currentExec != stablePath {
		if err := os.MkdirAll(filepath.Dir(stablePath), 0o755); err != nil {
			logger.Error("failed to create stable bin directory", "error", err)
			os.Exit(exitcodes.IOError)
		}
		if err := osutil.CopyFile(currentExec, stablePath); err != nil {
			logger.Error("failed to install binary to stable path", "error", err)
			os.Exit(exitcodes.IOError)
		}
		reporter.record(installAction{
			Kind:    "binary",
			Status:  "installed",
			Message: fmt.Sprintf("Installed binary to %s", stablePath),
			Path:    stablePath,
		})
	}

	// 1. Create user-level config if it doesn't exist.
	createUserConfig(reporter)

	// 2. Install agent skill (cross-client)
	crossClientDir := filepath.Join(home, skills.SkillRelDir)
	installSkill(reporter, crossClientDir)

	// 3. Install the hook into agent settings
	filter := agentsettings.ParseFilter(installAgentFilter)
	agents := agentsettings.Detect(home, filter)
	if len(agents) == 0 {
		reporter.record(installAction{
			Kind:    "agent_detection",
			Status:  "none",
			Message: "No supported AI agent installations detected.",
		})
		reporter.record(installAction{
			Kind:    "hint",
			Status:  "info",
			Message: "Tip: Run 'tw install --agent <name>' to install for a specific agent.",
		})
		return reporter.flush()
	}

	// 3.5 Install Claude-specific skill if Claude is detected.
	for _, a := range agents {
		if a.Name == "claude" {
			claudeDir := filepath.Join(home, skills.ClaudeSkillRelDir)
			installSkill(reporter, claudeDir)
			break
		}
	}

	for _, a := range agents {
		path := agentsettings.SettingsPath(a, installProject, home)
		installForAgent(reporter, a, path, stablePath)
	}
	return reporter.flush()
}

func runUninstall(cmd *cobra.Command, args []string) error {
	reporter := newInstallReporter(cmd.OutOrStdout(), uninstallJSON)

	home, err := os.UserHomeDir()
	if err != nil {
		logger.Error("cannot determine home directory", "error", err)
		os.Exit(exitcodes.IOError)
	}

	stablePath := stableBinaryPath(home)

	filter := agentsettings.ParseFilter(uninstallAgentFilter)
	agents := agentsettings.Detect(home, filter)
	if len(agents) == 0 {
		reporter.record(installAction{
			Kind:    "agent_detection",
			Status:  "none",
			Message: "No supported AI agent installations detected.",
		})
		reporter.record(installAction{
			Kind:    "hint",
			Status:  "info",
			Message: "Tip: Run 'tw uninstall --agent <name>' to uninstall for a specific agent.",
		})
		return reporter.flush()
	}

	for _, a := range agents {
		path := agentsettings.SettingsPath(a, uninstallProject, home)
		uninstallForAgent(reporter, a, path, stablePath)
	}

	// Remove agent skills from both locations.
	uninstallSkill(reporter, filepath.Join(home, skills.SkillRelDir))
	uninstallSkill(reporter, filepath.Join(home, skills.ClaudeSkillRelDir))

	return reporter.flush()
}

func installForAgent(reporter *installReporter, a agentsettings.Agent, settingsPath, twHookRef string) {
	settings, _, err := agentsettings.ReadSettings(settingsPath)
	if err != nil {
		logger.Error("error reading settings", "path", settingsPath, "error", err)
		if isSettingsParseError(err) {
			os.Exit(exitcodes.ConfigError)
		}
		os.Exit(exitcodes.IOError)
	}
	if a.HookExists(settings, twHookRef) {
		reporter.record(installAction{
			Kind:    "hook",
			Status:  "unchanged",
			Message: fmt.Sprintf("TW hook already installed in %s  [%s]", settingsPath, a.Name),
			Path:    settingsPath,
			Agent:   a.Name,
		})
		return
	}
	a.AddHookEntry(settings, twHookRef)
	if err := agentsettings.WriteSettings(settingsPath, settings); err != nil {
		logger.Error("error writing settings", "path", settingsPath, "error", err)
		os.Exit(exitcodes.IOError)
	}
	reporter.record(installAction{
		Kind:    "hook",
		Status:  "installed",
		Message: fmt.Sprintf("TW hook installed to %s  [%s]", settingsPath, a.Name),
		Path:    settingsPath,
		Agent:   a.Name,
	})
}

func uninstallForAgent(reporter *installReporter, a agentsettings.Agent, settingsPath, twHookRef string) {
	settings, existed, err := agentsettings.ReadSettings(settingsPath)
	if err != nil {
		logger.Error("error reading settings", "path", settingsPath, "error", err)
		if isSettingsParseError(err) {
			os.Exit(exitcodes.ConfigError)
		}
		os.Exit(exitcodes.IOError)
	}
	if !existed {
		reporter.record(installAction{
			Kind:    "hook",
			Status:  "missing",
			Message: fmt.Sprintf("TW hook not found in %s  [%s]", settingsPath, a.Name),
			Path:    settingsPath,
			Agent:   a.Name,
		})
		return
	}
	if !a.RemoveHookEntry(settings, twHookRef) {
		reporter.record(installAction{
			Kind:    "hook",
			Status:  "missing",
			Message: fmt.Sprintf("TW hook not found in %s  [%s]", settingsPath, a.Name),
			Path:    settingsPath,
			Agent:   a.Name,
		})
		return
	}
	if err := agentsettings.WriteSettings(settingsPath, settings); err != nil {
		logger.Error("error writing settings", "path", settingsPath, "error", err)
		os.Exit(exitcodes.IOError)
	}
	reporter.record(installAction{
		Kind:    "hook",
		Status:  "removed",
		Message: fmt.Sprintf("TW hook removed from %s  [%s]", settingsPath, a.Name),
		Path:    settingsPath,
		Agent:   a.Name,
	})
}

func installSkill(reporter *installReporter, dir string) {
	if skills.ExistsAt(dir) {
		reporter.record(installAction{
			Kind:    "skill",
			Status:  "unchanged",
			Message: fmt.Sprintf("Agent skill already installed in %s", dir),
			Path:    dir,
		})
		return
	}
	if err := skills.InstallTo(dir); err != nil {
		logger.Warn("could not install agent skill", "dir", dir, "error", err)
		return
	}
	reporter.record(installAction{
		Kind:    "skill",
		Status:  "installed",
		Message: fmt.Sprintf("Agent skill installed to %s", dir),
		Path:    dir,
	})
}

func uninstallSkill(reporter *installReporter, dir string) {
	if err := skills.UninstallFrom(dir); err != nil {
		logger.Warn("could not remove agent skill", "dir", dir, "error", err)
		return
	}
	reporter.record(installAction{
		Kind:    "skill",
		Status:  "removed",
		Message: fmt.Sprintf("Agent skill removed from %s", dir),
		Path:    dir,
	})
}

func createUserConfig(reporter *installReporter) {
	cfgPath, err := config.UserConfigPath()
	if err != nil {
		logger.Warn("could not determine user config path", "error", err)
		return
	}
	if _, err := os.Stat(cfgPath); err == nil {
		reporter.record(installAction{
			Kind:    "config",
			Status:  "unchanged",
			Message: fmt.Sprintf("User config already exists at %s", cfgPath),
			Path:    cfgPath,
		})
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
	reporter.record(installAction{
		Kind:    "config",
		Status:  "created",
		Message: fmt.Sprintf("User config created at %s", cfgPath),
		Path:    cfgPath,
	})
}

func isSettingsParseError(err error) bool {
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return true
	}
	var typeErr *json.UnmarshalTypeError
	return errors.As(err, &typeErr)
}
