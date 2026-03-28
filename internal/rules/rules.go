// Package rules provides persistent custom rule management for TW.
// Rules are stored in JSON files at user (~/.tw/rules.json) and
// project (.tw/rules.json) levels.
// Each rule entry has an action, match kind, pattern, and metadata.
package rules

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/thgrace/training-wheels/internal/osutil"
	"github.com/thgrace/training-wheels/internal/packs"
)

// nameRE validates rule names: lowercase letter followed by lowercase letters, digits, hyphens, or underscores.
var nameRE = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// validActions is the set of allowed action values.
var validActions = map[string]bool{
	"deny":  true,
	"ask":   true,
	"allow": true,
}

// validKinds is the set of allowed kind values.
var validKinds = map[string]bool{
	"command": true, // v2: structural when/unless matching
	"rule":    true, // allow by rule ID
	// Legacy v1 kinds (still loadable for backward compat):
	"exact":  true,
	"prefix": true,
}

// Suggestion represents a safer alternative command.
type Suggestion struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// RuleEntry is a single custom rule entry.
type RuleEntry struct {
	Name        string                  `json:"name"`                   // unique within scope, must match ^[a-z][a-z0-9_-]*$
	Action      string                  `json:"action"`                 // "deny", "ask", or "allow"
	Kind        string                  `json:"kind"`                   // "command", "rule", or legacy: "exact", "prefix"
	When        *packs.PatternCondition `json:"when,omitempty"`         // v2: structural trigger conditions (kind "command")
	Unless      *packs.PatternCondition `json:"unless,omitempty"`       // v2: structural exemption conditions (kind "command")
	Pattern     string                  `json:"pattern,omitempty"`      // kind "rule" (rule ID) or legacy v1 pattern
	SafePattern string                  `json:"safe_pattern,omitempty"` // legacy v1 regex exception
	Reason      string                  `json:"reason"`                 // human-readable
	Severity    string                  `json:"severity,omitempty"`     // "critical"/"high"/"medium"/"low"
	Keywords    []string                `json:"keywords,omitempty"`     // for quick-reject
	Explanation string                  `json:"explanation,omitempty"`  // detailed explanation
	Suggestions []Suggestion            `json:"suggestions,omitempty"`  // safer alternatives
	AddedAt     time.Time               `json:"added_at"`
}

// RulesFile is the in-memory representation of a rules JSON file.
type RulesFile struct {
	Rules []RuleEntry `json:"rules"`
	path  string
}

// LoadOrCreate reads rules from the given JSON file path.
// Returns an empty RulesFile if the file doesn't exist (does not create the file).
func LoadOrCreate(path string) (*RulesFile, error) {
	rf := &RulesFile{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return rf, nil
		}
		return nil, fmt.Errorf("reading rules %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return rf, nil
	}
	if err := json.Unmarshal(data, rf); err != nil {
		return nil, fmt.Errorf("parsing rules %s: %w", path, err)
	}
	return rf, nil
}

// Add validates and appends a new rule entry, then saves the file.
// It checks name uniqueness, name format, action, kind, and pattern validity.
func (rf *RulesFile) Add(entry RuleEntry) error {
	// Validate name format.
	if !nameRE.MatchString(entry.Name) {
		return fmt.Errorf("invalid rule name %q: must match ^[a-z][a-z0-9_-]*$", entry.Name)
	}

	// Check name uniqueness within this file.
	for _, existing := range rf.Rules {
		if existing.Name == entry.Name {
			return fmt.Errorf("duplicate rule name %q", entry.Name)
		}
	}

	// Validate action.
	if !validActions[entry.Action] {
		return fmt.Errorf("invalid action %q: must be deny, ask, or allow", entry.Action)
	}

	// Validate kind.
	if !validKinds[entry.Kind] {
		return fmt.Errorf("invalid kind %q: must be command or rule (legacy: exact, prefix)", entry.Kind)
	}

	// Validate v2 "command" kind.
	if entry.Kind == "command" {
		if entry.When == nil || len(entry.When.Command) == 0 {
			return fmt.Errorf("kind %q requires when.command with at least one command name", entry.Kind)
		}
		for _, cmd := range entry.When.Command {
			if strings.TrimSpace(cmd) == "" {
				return fmt.Errorf("when.command contains empty or whitespace-only value")
			}
		}
	}

	// Set AddedAt if zero.
	if entry.AddedAt.IsZero() {
		entry.AddedAt = time.Now().UTC().Truncate(time.Second)
	}

	rf.Rules = append(rf.Rules, entry)
	return rf.Save()
}

// Remove removes a rule by name and saves. Returns false if not found.
// Returns an error if the rule was found but saving failed.
func (rf *RulesFile) Remove(name string) (bool, error) {
	for i, r := range rf.Rules {
		if r.Name == name {
			rf.Rules = append(rf.Rules[:i], rf.Rules[i+1:]...)
			return true, rf.Save()
		}
	}
	return false, nil
}

// List returns the current rules slice.
func (rf *RulesFile) List() []RuleEntry {
	return rf.Rules
}

// Save writes the rules file atomically via a tmp file and osutil.AtomicRename.
func (rf *RulesFile) Save() error {
	if rf.path == "" {
		return errors.New("rules file has no path")
	}

	dir := filepath.Dir(rf.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(rf, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding rules: %w", err)
	}

	tmpPath := rf.path + ".tmp"
	if err := os.WriteFile(tmpPath, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", tmpPath, err)
	}
	if err := osutil.AtomicRename(tmpPath, rf.path); err != nil {
		return fmt.Errorf("renaming %s → %s: %w", tmpPath, rf.path, err)
	}
	return nil
}

// Path returns the file path of this rules file.
func (rf *RulesFile) Path() string {
	return rf.path
}

// UserRulesPath returns the user-level rules file path (~/.tw/rules.json).
func UserRulesPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".tw", "rules.json"), nil
}

// ProjectRulesPath returns the project-level rules file path.
func ProjectRulesPath() string {
	return filepath.Join(".tw", "rules.json")
}

