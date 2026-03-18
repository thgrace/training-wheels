package cli

// agentDef describes how to install/uninstall the TW hook for a specific AI agent.
type agentDef struct {
	Name        string
	UserDir     string // e.g. ".claude"
	UserFile    string // e.g. "settings.json"
	ProjectDir  string // e.g. ".claude"
	ProjectFile string // e.g. "settings.json"

	HookExists      func(settings map[string]interface{}, twPath string) bool
	AddHookEntry    func(settings map[string]interface{}, twPath string)
	RemoveHookEntry func(settings map[string]interface{}, twPath string) bool
}

var allAgents = []agentDef{claudeAgent, cursorAgent, geminiAgent, copilotAgent}

// ---------------------------------------------------------------------------
// Claude Code
// ---------------------------------------------------------------------------

var claudeAgent = agentDef{
	Name:        "claude",
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

// ---------------------------------------------------------------------------
// Cursor
// ---------------------------------------------------------------------------

var cursorAgent = agentDef{
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

// ---------------------------------------------------------------------------
// Gemini CLI
// ---------------------------------------------------------------------------

var geminiAgent = agentDef{
	Name:        "gemini",
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

// ---------------------------------------------------------------------------
// Copilot CLI
// ---------------------------------------------------------------------------

var copilotAgent = agentDef{
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

// ---------------------------------------------------------------------------
// Shared helpers: nested hooks (Claude, Gemini)
//
// Shape: hooks[eventKey][] = {matcher, hooks:[{type, command}]}
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Shared helpers: flat hooks (Cursor, Copilot)
//
// Shape: hooks[eventKey][] = {command|bash, type, ...}
// ---------------------------------------------------------------------------

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
