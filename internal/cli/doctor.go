package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/thgrace/training-wheels/internal/config"
	"github.com/thgrace/training-wheels/internal/packs"
	"github.com/thgrace/training-wheels/internal/skills"
	"github.com/spf13/cobra"
)

var doctorFormat string

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check TW installation health",
	RunE:  runDoctor,
}

func init() {
	doctorCmd.Flags().StringVar(&doctorFormat, "format", "pretty", "Output format: pretty or json")
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

	if doctorFormat == "json" {
		data, _ := json.MarshalIndent(checks, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		printDoctorPretty(cmd.OutOrStdout(), checks)
	}

	for _, c := range checks {
		if c.Status == "error" {
			os.Exit(1)
		}
	}
	return nil
}

func checkBinary() checkResult {
	path, err := exec.LookPath("tw")
	if err != nil {
		return checkResult{
			Name:    "binary",
			Status:  "error",
			Message: "tw not found in PATH",
			Fix:     "Add tw to your PATH or install it",
		}
	}
	return checkResult{
		Name:    "binary",
		Status:  "ok",
		Message: fmt.Sprintf("Binary found: %s (%s/%s)", path, runtime.GOOS, runtime.GOARCH),
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

	agents := detectAgents(home, nil)
	if len(agents) == 0 {
		return []checkResult{{
			Name:    "hook",
			Status:  "warn",
			Message: "No supported AI agent installations detected",
			Fix:     "Run 'tw install --agent <name>' to install for a specific agent",
		}}
	}

	twExe := "tw"
	if runtime.GOOS == "windows" {
		twExe = "tw.exe"
	}
	twPath := filepath.Join(home, ".tw", "bin", twExe)

	var results []checkResult
	for _, a := range agents {
		userPath := agentSettingsPath(a, false, home)
		projectPath := agentSettingsPath(a, true, home)

		userInstalled := isAgentHookInstalled(a, userPath, twPath)
		projectInstalled := isAgentHookInstalled(a, projectPath, twPath)

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

func isAgentHookInstalled(a agentDef, path, twPath string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	settings := make(map[string]interface{})
	if err := json.Unmarshal(data, &settings); err != nil {
		return false
	}
	return a.HookExists(settings, twPath)
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

	return checkResult{
		Name:    "packs",
		Status:  "ok",
		Message: fmt.Sprintf("%d packs loaded (%d enabled)", len(allIDs), enabledCount),
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
	agents := detectAgents(home, nil)
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
