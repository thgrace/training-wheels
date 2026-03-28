package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/thgrace/training-wheels/internal/agentsettings"
	"github.com/thgrace/training-wheels/internal/config"
	"github.com/thgrace/training-wheels/internal/packs"
	"github.com/thgrace/training-wheels/internal/skills"
)

var doctorJSON bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check TW installation health",
	RunE:  runDoctor,
}

func init() {
	bindJSONOutputFlags(doctorCmd.Flags(), &doctorJSON)
}

type checkResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "ok", "warn", "error"
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
}

func runDoctor(cmd *cobra.Command, args []string) error {
	checks := make([]checkResult, 0, 4)

	checks = append(checks, checkBinary())
	checks = append(checks, checkConfig())
	checks = append(checks, checkHooksInstalled()...)
	checks = append(checks, checkPacks())
	checks = append(checks, checkSkills()...)

	if useJSONOutput(doctorJSON) {
		if err := writeJSONOutput(cmd.OutOrStdout(), checks); err != nil {
			return err
		}
	} else {
		printDoctorPretty(cmd.OutOrStdout(), checks)
	}

	for _, c := range checks {
		if c.Status == "error" {
			return silentExit(1)
		}
	}
	return nil
}

func checkBinary() checkResult {
	path, err := os.Executable()
	if err != nil {
		return checkResult{
			Name:    "binary",
			Status:  "error",
			Message: fmt.Sprintf("Cannot determine current binary path: %v", err),
			Fix:     "Reinstall tw or check binary permissions",
		}
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return checkResult{
		Name:    "binary",
		Status:  "ok",
		Message: fmt.Sprintf("Binary running from: %s (%s/%s)", path, runtime.GOOS, runtime.GOARCH),
	}
}

func checkConfig() checkResult {
	_, err := config.Load()
	if err != nil {
		return checkResult{
			Name:    "config",
			Status:  "error",
			Message: fmt.Sprintf("Config error: %v", err),
			Fix:     "Check ~/.tw/config.json syntax or environment variables",
		}
	}

	return checkResult{
		Name:    "config",
		Status:  "ok",
		Message: "Config loads without errors",
	}
}

func checkHooksInstalled() []checkResult {
	home, err := os.UserHomeDir()
	if err != nil {
		return []checkResult{{
			Name:    "hook",
			Status:  "warn",
			Message: fmt.Sprintf("Cannot determine home directory: %v", err),
		}}
	}

	agents := agentsettings.Detect(home, nil)
	if len(agents) == 0 {
		return []checkResult{{
			Name:    "hook",
			Status:  "warn",
			Message: "No supported AI agent installations detected",
			Fix:     "Run 'tw install --agent <name>' to install for a specific agent",
		}}
	}

	var results []checkResult
	for _, a := range agents {
		userPath := agentsettings.SettingsPath(a, false, home)
		projectPath := agentsettings.SettingsPath(a, true, home)

		userInstalled := isAgentHookInstalled(a, userPath, "tw")
		projectInstalled := isAgentHookInstalled(a, projectPath, "tw")

		name := "hook:" + a.Name
		if userInstalled || projectInstalled {
			where := userPath
			if projectInstalled {
				where = projectPath
			}
			results = append(results, checkResult{
				Name:    name,
				Status:  "ok",
				Message: fmt.Sprintf("Hook installed in %s", where),
			})
		} else {
			results = append(results, checkResult{
				Name:    name,
				Status:  "warn",
				Message: fmt.Sprintf("Hook not detected in %s settings", a.Name),
				Fix:     fmt.Sprintf("Run 'tw install --agent %s'", a.Name),
			})
		}
	}
	return results
}

func isAgentHookInstalled(a agentsettings.Agent, path, twHookRef string) bool {
	settings, _, err := agentsettings.ReadSettings(path)
	if err != nil {
		return false
	}
	return a.HookExists(settings, twHookRef)
}

func checkPacks() checkResult {
	cfg, err := config.Load()
	if err != nil {
		return checkResult{
			Name:    "packs",
			Status:  "error",
			Message: fmt.Sprintf("Cannot load config: %v", err),
		}
	}

	reg := packs.DefaultRegistry()
	allIDs := reg.AllIDs()
	enabledIDs, _ := reg.ResolveEnabledSet(cfg.Packs.Enabled, cfg.Packs.Disabled)
	enabledCount := len(enabledIDs)

	message := fmt.Sprintf("%d packs loaded (%d enabled)", len(allIDs), enabledCount)

	return checkResult{
		Name:    "packs",
		Status:  "ok",
		Message: message,
	}
}

func checkSkills() []checkResult {
	home, err := os.UserHomeDir()
	if err != nil {
		return []checkResult{{
			Name:    "skill",
			Status:  "warn",
			Message: fmt.Sprintf("Cannot determine home directory: %v", err),
		}}
	}

	type target struct {
		name string
		dir  string
	}
	targets := []target{
		{"skill:agents", filepath.Join(home, skills.SkillRelDir)},
	}

	// Only check Claude skill location if Claude is detected.
	agents := agentsettings.Detect(home, nil)
	for _, a := range agents {
		if a.Name == "claude" {
			targets = append(targets, target{"skill:claude", filepath.Join(home, skills.ClaudeSkillRelDir)})
			break
		}
	}

	var results []checkResult
	for _, t := range targets {
		if skills.ExistsAt(t.dir) {
			results = append(results, checkResult{
				Name:    t.name,
				Status:  "ok",
				Message: fmt.Sprintf("Agent skill installed in %s", t.dir),
			})
		} else {
			results = append(results, checkResult{
				Name:    t.name,
				Status:  "warn",
				Message: fmt.Sprintf("Agent skill not found in %s", t.dir),
				Fix:     "Run 'tw install' to install agent skills",
			})
		}
	}
	return results
}

func printDoctorPretty(out io.Writer, checks []checkResult) {
	warnings := 0
	errors := 0

	for _, c := range checks {
		var prefix string
		switch c.Status {
		case "ok":
			prefix = "  OK   "
		case "warn":
			prefix = "  WARN "
			warnings++
		case "error":
			prefix = "  ERR  "
			errors++
		}
		fmt.Fprintf(out, "%s %s\n", prefix, c.Message)
		if c.Fix != "" {
			fmt.Fprintf(out, "         Fix: %s\n", c.Fix)
		}
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "%d warning(s), %d error(s)\n", warnings, errors)
}
