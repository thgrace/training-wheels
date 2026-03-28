package agentsettings

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/thgrace/training-wheels/internal/osutil"
)

// Agent describes how to install and uninstall the TW hook for a specific AI agent.
type Agent struct {
	Name        string
	BinaryNames []string
	UserDir     string
	UserFile    string
	ProjectDir  string
	ProjectFile string

	HookExists      func(settings map[string]interface{}, twPath string) bool
	AddHookEntry    func(settings map[string]interface{}, twPath string)
	RemoveHookEntry func(settings map[string]interface{}, twPath string) bool
}

// AllAgents is the list of all supported agents.
var AllAgents = []Agent{Claude, Cursor, Gemini, Copilot}

// Claude defines the Claude Code agent.
var Claude = Agent{
	Name:        "claude",
	BinaryNames: []string{"claude"},
	UserDir:     ".claude",
	UserFile:    "settings.json",
	ProjectDir:  ".claude",
	ProjectFile: "settings.json",

	HookExists: func(settings map[string]interface{}, twPath string) bool {
		return nestedHookExists(settings, "PreToolUse", "command", twPath+" hook")
	},
	AddHookEntry: func(settings map[string]interface{}, twPath string) {
		entry := map[string]interface{}{
			"matcher": "Bash",
			"hooks": []interface{}{
				map[string]interface{}{
					"type":    "command",
					"command": twPath + " hook",
				},
			},
		}
		addNestedHookEntry(settings, "PreToolUse", entry)
	},
	RemoveHookEntry: func(settings map[string]interface{}, twPath string) bool {
		return removeNestedHookEntry(settings, "PreToolUse", "command", twPath+" hook")
	},
}

// Cursor defines the Cursor agent.
var Cursor = Agent{
	Name:        "cursor",
	UserDir:     ".cursor",
	UserFile:    "hooks.json",
	ProjectDir:  ".cursor",
	ProjectFile: "hooks.json",

	HookExists: func(settings map[string]interface{}, twPath string) bool {
		return flatHookExists(settings, "preToolUse", "command", twPath+" hook")
	},
	AddHookEntry: func(settings map[string]interface{}, twPath string) {
		settings["version"] = float64(1)
		entry := map[string]interface{}{
			"command": twPath + " hook",
			"type":    "command",
			"matcher": "Shell",
		}
		addFlatHookEntry(settings, "preToolUse", entry)
	},
	RemoveHookEntry: func(settings map[string]interface{}, twPath string) bool {
		return removeFlatHookEntry(settings, "preToolUse", "command", twPath+" hook")
	},
}

// Gemini defines the Gemini CLI agent.
var Gemini = Agent{
	Name:        "gemini",
	BinaryNames: []string{"gemini"},
	UserDir:     ".gemini",
	UserFile:    "settings.json",
	ProjectDir:  ".gemini",
	ProjectFile: "settings.json",

	HookExists: func(settings map[string]interface{}, twPath string) bool {
		return nestedHookExists(settings, "BeforeTool", "command", twPath+" hook")
	},
	AddHookEntry: func(settings map[string]interface{}, twPath string) {
		entry := map[string]interface{}{
			"matcher": "shell",
			"hooks": []interface{}{
				map[string]interface{}{
					"type":    "command",
					"command": twPath + " hook",
				},
			},
		}
		addNestedHookEntry(settings, "BeforeTool", entry)
	},
	RemoveHookEntry: func(settings map[string]interface{}, twPath string) bool {
		return removeNestedHookEntry(settings, "BeforeTool", "command", twPath+" hook")
	},
}

// Copilot defines the Copilot CLI agent.
var Copilot = Agent{
	Name:        "copilot",
	UserDir:     ".copilot",
	UserFile:    "config.json",
	ProjectDir:  ".github/hooks",
	ProjectFile: "hooks.json",

	HookExists: func(settings map[string]interface{}, twPath string) bool {
		return flatHookExists(settings, "preToolUse", "bash", twPath+" hook")
	},
	AddHookEntry: func(settings map[string]interface{}, twPath string) {
		settings["version"] = float64(1)
		entry := map[string]interface{}{
			"type":       "command",
			"bash":       twPath + " hook",
			"powershell": twPath + " hook",
		}
		addFlatHookEntry(settings, "preToolUse", entry)
	},
	RemoveHookEntry: func(settings map[string]interface{}, twPath string) bool {
		return removeFlatHookEntry(settings, "preToolUse", "bash", twPath+" hook")
	},
}

var lookPathFn = exec.LookPath

// SettingsPath returns the path to the agent settings file.
func SettingsPath(a Agent, project bool, home string) string {
	if project {
		return filepath.Join(a.ProjectDir, a.ProjectFile)
	}
	return filepath.Join(home, a.UserDir, a.UserFile)
}

// Detect returns the installed agents. A non-empty filter is matched case-insensitively.
func Detect(home string, filter []string) []Agent {
	if len(filter) > 0 {
		filterSet := make(map[string]bool, len(filter))
		for _, name := range filter {
			filterSet[strings.ToLower(strings.TrimSpace(name))] = true
		}
		var result []Agent
		for _, a := range AllAgents {
			if filterSet[a.Name] {
				result = append(result, a)
			}
		}
		return result
	}

	var result []Agent
	for _, a := range AllAgents {
		dir := filepath.Join(home, a.UserDir)
		if _, err := os.Stat(dir); err == nil || binaryExists(a) {
			result = append(result, a)
		}
	}
	return result
}

func binaryExists(a Agent) bool {
	for _, name := range a.BinaryNames {
		if name == "" {
			continue
		}
		if _, err := lookPathFn(name); err == nil {
			return true
		}
	}
	return false
}

// ParseFilter parses a comma-separated list of agent names.
func ParseFilter(raw string) []string {
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

// ReadSettings reads and JSON-unmarshals an agent settings file.
// It returns the parsed settings and whether the file existed.
func ReadSettings(path string) (map[string]interface{}, bool, error) {
	settings := make(map[string]interface{})
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return settings, false, nil
		}
		return nil, false, err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return nil, true, err
		}
	}
	return settings, true, nil
}

// WriteSettings JSON-marshals settings and atomically writes them to path.
func WriteSettings(path string, settings map[string]interface{}) error {
	output, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, append(output, '\n'), 0o644); err != nil {
		return err
	}
	return osutil.AtomicRename(tmpPath, path)
}

func nestedHookExists(settings map[string]interface{}, eventKey, cmdField, cmdValue string) bool {
	hooks, ok := settings["hooks"]
	if !ok {
		return false
	}
	hooksMap, ok := hooks.(map[string]interface{})
	if !ok {
		return false
	}
	event, ok := hooksMap[eventKey]
	if !ok {
		return false
	}
	arr, ok := event.([]interface{})
	if !ok {
		return false
	}
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		innerHooks, ok := m["hooks"]
		if !ok {
			continue
		}
		innerArr, ok := innerHooks.([]interface{})
		if !ok {
			continue
		}
		for _, h := range innerArr {
			hm, ok := h.(map[string]interface{})
			if !ok {
				continue
			}
			if cmd, ok := hm[cmdField]; ok {
				if cmdStr, ok := cmd.(string); ok && cmdStr == cmdValue {
					return true
				}
			}
		}
	}
	return false
}

func addNestedHookEntry(settings map[string]interface{}, eventKey string, entry map[string]interface{}) {
	hooks, ok := settings["hooks"]
	if !ok {
		settings["hooks"] = map[string]interface{}{
			eventKey: []interface{}{entry},
		}
		return
	}
	hooksMap, ok := hooks.(map[string]interface{})
	if !ok {
		settings["hooks"] = map[string]interface{}{
			eventKey: []interface{}{entry},
		}
		return
	}
	event, ok := hooksMap[eventKey]
	if !ok {
		hooksMap[eventKey] = []interface{}{entry}
		return
	}
	arr, ok := event.([]interface{})
	if !ok {
		hooksMap[eventKey] = []interface{}{entry}
		return
	}
	hooksMap[eventKey] = append(arr, entry)
}

func removeNestedHookEntry(settings map[string]interface{}, eventKey, cmdField, cmdValue string) bool {
	hooks, ok := settings["hooks"]
	if !ok {
		return false
	}
	hooksMap, ok := hooks.(map[string]interface{})
	if !ok {
		return false
	}
	event, ok := hooksMap[eventKey]
	if !ok {
		return false
	}
	arr, ok := event.([]interface{})
	if !ok {
		return false
	}

	removed := false
	filtered := make([]interface{}, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			filtered = append(filtered, item)
			continue
		}
		innerHooks, ok := m["hooks"]
		if !ok {
			filtered = append(filtered, item)
			continue
		}
		innerArr, ok := innerHooks.([]interface{})
		if !ok {
			filtered = append(filtered, item)
			continue
		}
		hasTW := false
		for _, h := range innerArr {
			hm, ok := h.(map[string]interface{})
			if !ok {
				continue
			}
			if cmd, ok := hm[cmdField]; ok {
				if cmdStr, ok := cmd.(string); ok && cmdStr == cmdValue {
					hasTW = true
					break
				}
			}
		}
		if hasTW {
			removed = true
		} else {
			filtered = append(filtered, item)
		}
	}

	if removed {
		hooksMap[eventKey] = filtered
	}
	return removed
}

func flatHookExists(settings map[string]interface{}, eventKey, cmdField, cmdValue string) bool {
	hooks, ok := settings["hooks"]
	if !ok {
		return false
	}
	hooksMap, ok := hooks.(map[string]interface{})
	if !ok {
		return false
	}
	event, ok := hooksMap[eventKey]
	if !ok {
		return false
	}
	arr, ok := event.([]interface{})
	if !ok {
		return false
	}
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if cmd, ok := m[cmdField]; ok {
			if cmdStr, ok := cmd.(string); ok && cmdStr == cmdValue {
				return true
			}
		}
	}
	return false
}

func addFlatHookEntry(settings map[string]interface{}, eventKey string, entry map[string]interface{}) {
	hooks, ok := settings["hooks"]
	if !ok {
		settings["hooks"] = map[string]interface{}{
			eventKey: []interface{}{entry},
		}
		return
	}
	hooksMap, ok := hooks.(map[string]interface{})
	if !ok {
		settings["hooks"] = map[string]interface{}{
			eventKey: []interface{}{entry},
		}
		return
	}
	event, ok := hooksMap[eventKey]
	if !ok {
		hooksMap[eventKey] = []interface{}{entry}
		return
	}
	arr, ok := event.([]interface{})
	if !ok {
		hooksMap[eventKey] = []interface{}{entry}
		return
	}
	hooksMap[eventKey] = append(arr, entry)
}

func removeFlatHookEntry(settings map[string]interface{}, eventKey, cmdField, cmdValue string) bool {
	hooks, ok := settings["hooks"]
	if !ok {
		return false
	}
	hooksMap, ok := hooks.(map[string]interface{})
	if !ok {
		return false
	}
	event, ok := hooksMap[eventKey]
	if !ok {
		return false
	}
	arr, ok := event.([]interface{})
	if !ok {
		return false
	}

	removed := false
	filtered := make([]interface{}, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			filtered = append(filtered, item)
			continue
		}
		if cmd, ok := m[cmdField]; ok {
			if cmdStr, ok := cmd.(string); ok && cmdStr == cmdValue {
				removed = true
				continue
			}
		}
		filtered = append(filtered, item)
	}

	if removed {
		hooksMap[eventKey] = filtered
	}
	return removed
}
