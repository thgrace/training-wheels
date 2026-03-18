package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAgentHookExists(t *testing.T) {
	for _, a := range allAgents {
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
	for _, a := range allAgents {
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
	for _, a := range allAgents {
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
	agents := []agentDef{cursorAgent, copilotAgent}
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

	agents := detectAgents(home, nil)
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
	agents := detectAgents(home, []string{"cursor", "copilot"})
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
	agents = detectAgents(home, []string{"claude"})
	names = agentNames(agents)
	if len(agents) != 1 || names[0] != "claude" {
		t.Fatalf("got agents %v, want [claude]", names)
	}
}

func TestInstallForAgentUsesPathHook(t *testing.T) {
	for _, a := range allAgents {
		t.Run(a.Name, func(t *testing.T) {
			settingsPath := filepath.Join(t.TempDir(), a.UserFile)

			var out bytes.Buffer
			installForAgent(&out, a, settingsPath, "tw")

			settings, existed := readSettings(settingsPath)
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
	for _, a := range allAgents {
		t.Run(a.Name, func(t *testing.T) {
			settingsPath := filepath.Join(t.TempDir(), a.UserFile)
			settings := make(map[string]interface{})
			a.AddHookEntry(settings, "tw")
			a.AddHookEntry(settings, "other-tool")
			writeSettings(settingsPath, settings)

			var out bytes.Buffer
			uninstallForAgent(&out, a, settingsPath, "tw")

			updated, existed := readSettings(settingsPath)
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

func agentNames(agents []agentDef) []string {
	names := make([]string, len(agents))
	for i, a := range agents {
		names[i] = a.Name
	}
	return names
}
