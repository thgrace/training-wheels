package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/thgrace/training-wheels/internal/agentsettings"
)

func TestAgentHookExists(t *testing.T) {
	for _, a := range agentsettings.AllAgents {
		t.Run(a.Name, func(t *testing.T) {
			settings := make(map[string]interface{})
			if a.HookExists(settings, "tw") {
				t.Fatal("HookExists returned true on empty map")
			}
			a.AddHookEntry(settings, "tw")
			if !a.HookExists(settings, "tw") {
				t.Fatal("HookExists returned false after AddHookEntry")
			}
		})
	}
}

func TestAgentRemoveHookEntry(t *testing.T) {
	for _, a := range agentsettings.AllAgents {
		t.Run(a.Name+"_not_present", func(t *testing.T) {
			settings := make(map[string]interface{})
			if a.RemoveHookEntry(settings, "tw") {
				t.Fatal("RemoveHookEntry returned true on empty map")
			}
		})
		t.Run(a.Name+"_after_add", func(t *testing.T) {
			settings := make(map[string]interface{})
			a.AddHookEntry(settings, "tw")
			if !a.RemoveHookEntry(settings, "tw") {
				t.Fatal("RemoveHookEntry returned false after AddHookEntry")
			}
			if a.HookExists(settings, "tw") {
				t.Fatal("HookExists returned true after RemoveHookEntry")
			}
		})
	}
}

func TestAgentAddIdempotent(t *testing.T) {
	for _, a := range agentsettings.AllAgents {
		t.Run(a.Name, func(t *testing.T) {
			settings := make(map[string]interface{})
			a.AddHookEntry(settings, "tw")
			a.AddHookEntry(settings, "tw")
			if !a.HookExists(settings, "tw") {
				t.Fatal("HookExists returned false after double AddHookEntry")
			}
			// Remove once should succeed.
			if !a.RemoveHookEntry(settings, "tw") {
				t.Fatal("RemoveHookEntry returned false after double add")
			}
		})
	}
}

func TestCursorCopilotVersionField(t *testing.T) {
	agents := []agentsettings.Agent{agentsettings.Cursor, agentsettings.Copilot}
	for _, a := range agents {
		t.Run(a.Name, func(t *testing.T) {
			settings := make(map[string]interface{})
			a.AddHookEntry(settings, "tw")
			v, ok := settings["version"]
			if !ok {
				t.Fatal("version field not set after AddHookEntry")
			}
			vf, ok := v.(float64)
			if !ok || vf != 1 {
				t.Fatalf("version = %v, want 1", v)
			}
		})
	}
}

func TestDetectAgents(t *testing.T) {
	home := t.TempDir()

	// Create only .claude and .gemini directories.
	os.Mkdir(filepath.Join(home, ".claude"), 0o755)
	os.Mkdir(filepath.Join(home, ".gemini"), 0o755)

	agents := agentsettings.Detect(home, nil)
	names := agentNames(agents)

	if len(agents) != 2 {
		t.Fatalf("got %d agents %v, want 2 (claude, gemini)", len(agents), names)
	}
	if names[0] != "claude" || names[1] != "gemini" {
		t.Fatalf("got agents %v, want [claude gemini]", names)
	}
}

func TestDetectAgentsWithFilter(t *testing.T) {
	home := t.TempDir()

	// No agent directories exist.

	// Filter limits results; explicit filter includes agents even without dirs.
	agents := agentsettings.Detect(home, []string{"cursor", "copilot"})
	names := agentNames(agents)
	if len(agents) != 2 {
		t.Fatalf("got %d agents %v, want 2 (cursor, copilot)", len(agents), names)
	}
	if names[0] != "cursor" || names[1] != "copilot" {
		t.Fatalf("got agents %v, want [cursor copilot]", names)
	}

	// Filter with existing dirs should still only return filtered agents.
	os.Mkdir(filepath.Join(home, ".claude"), 0o755)
	os.Mkdir(filepath.Join(home, ".cursor"), 0o755)
	agents = agentsettings.Detect(home, []string{"claude"})
	names = agentNames(agents)
	if len(agents) != 1 || names[0] != "claude" {
		t.Fatalf("got agents %v, want [claude]", names)
	}
}

func TestInstallForAgentUsesPathHook(t *testing.T) {
	for _, a := range agentsettings.AllAgents {
		t.Run(a.Name, func(t *testing.T) {
			settingsPath := filepath.Join(t.TempDir(), a.UserFile)

			var out bytes.Buffer
			reporter := newInstallReporter(&out, false)
			installForAgent(reporter, a, settingsPath, "tw")
			if err := reporter.flush(); err != nil {
				t.Fatalf("flush: %v", err)
			}

			settings, existed, err := agentsettings.ReadSettings(settingsPath)
			if err != nil {
				t.Fatalf("read settings: %v", err)
			}
			if !existed {
				t.Fatal("settings file was not created")
			}
			if !a.HookExists(settings, "tw") {
				t.Fatal("expected tw hook to be installed")
			}
		})
	}
}

func TestUninstallForAgentRemovesPathHook(t *testing.T) {
	for _, a := range agentsettings.AllAgents {
		t.Run(a.Name, func(t *testing.T) {
			settingsPath := filepath.Join(t.TempDir(), a.UserFile)
			settings := make(map[string]interface{})
			a.AddHookEntry(settings, "tw")
			a.AddHookEntry(settings, "other-tool")
			if err := agentsettings.WriteSettings(settingsPath, settings); err != nil {
				t.Fatalf("write settings: %v", err)
			}

			var out bytes.Buffer
			reporter := newInstallReporter(&out, false)
			uninstallForAgent(reporter, a, settingsPath, "tw")
			if err := reporter.flush(); err != nil {
				t.Fatalf("flush: %v", err)
			}

			updated, existed, err := agentsettings.ReadSettings(settingsPath)
			if err != nil {
				t.Fatalf("read settings: %v", err)
			}
			if !existed {
				t.Fatal("settings file disappeared")
			}
			if a.HookExists(updated, "tw") {
				t.Fatal("expected tw hook to be removed")
			}
			if !a.HookExists(updated, "other-tool") {
				t.Fatal("non-TW hook was removed")
			}
		})
	}
}

func TestInstallReporterJSON(t *testing.T) {
	var out bytes.Buffer
	reporter := newInstallReporter(&out, true)

	reporter.record(installAction{
		Kind:    "hook",
		Status:  "installed",
		Message: "ignored in json mode",
		Path:    "/tmp/settings.json",
		Agent:   "claude",
	})

	if err := reporter.flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	var got []installAction
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("reporter output is not valid json: %v\noutput=%q", err, out.String())
	}
	if len(got) != 1 {
		t.Fatalf("got %d actions, want 1", len(got))
	}
	if got[0].Kind != "hook" || got[0].Status != "installed" || got[0].Agent != "claude" {
		t.Fatalf("unexpected action: %+v", got[0])
	}
}

func agentNames(agents []agentsettings.Agent) []string {
	names := make([]string, len(agents))
	for i, a := range agents {
		names[i] = a.Name
	}
	return names
}
