// Package override provides persistent override management for TW.
// Entries are stored in JSON files at user (~/.tw/overrides.json)
// and project (.tw/overrides.json) levels.
// Each entry has an action (allow or deny) and a selector (exact, prefix, or rule).
package override

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thgrace/training-wheels/internal/osutil"
)

// Action identifies whether an override allows or denies.
type Action int

const (
	ActionAllow Action = iota
	ActionDeny
)

func (a Action) String() string {
	switch a {
	case ActionAllow:
		return "allow"
	case ActionDeny:
		return "deny"
	default:
		return "unknown"
	}
}

// SelectorKind identifies the type of override match.
type SelectorKind int

const (
	SelectorExact  SelectorKind = iota // Exact command match
	SelectorPrefix                     // Command prefix match
	SelectorRule                       // Rule ID match (pack:pattern), supports * wildcard
)

func (k SelectorKind) String() string {
	switch k {
	case SelectorExact:
		return "exact"
	case SelectorPrefix:
		return "prefix"
	case SelectorRule:
		return "rule"
	default:
		return "unknown"
	}
}

// Entry is a single override entry.
type Entry struct {
	ID      string    `json:"id"`
	Action  string    `json:"action"` // "allow" or "deny"
	Kind    string    `json:"kind"`   // "exact", "prefix", or "rule"
	Value   string    `json:"value"`  // The command, prefix, or rule ID pattern
	Reason  string    `json:"reason"` // Human explanation
	AddedAt time.Time `json:"added_at"`
}

// Matches checks if this entry matches a given command and rule ID.
func (e *Entry) Matches(command string, ruleID string) bool {
	switch e.Kind {
	case "exact":
		return command == e.Value
	case "prefix":
		return strings.HasPrefix(command, e.Value)
	case "rule":
		return matchRule(e.Value, ruleID)
	default:
		return false
	}
}

// matchRule matches a rule ID pattern against a concrete rule ID.
// Supports * as a wildcard for any sequence of characters (including empty).
// Multiple wildcards are supported.
// e.g., "core.git:*" matches "core.git:reset-hard"
// e.g., "core.*:*" matches "core.git:reset-hard"
// e.g., "core.*:reset-*" matches "core.git:reset-hard"
func matchRule(pattern, ruleID string) bool {
	if pattern == "*" {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return pattern == ruleID
	}
	// Split pattern on * to get literal segments, then verify
	// ruleID contains all segments in order.
	segments := strings.Split(pattern, "*")

	// The ruleID must start with the first segment.
	if !strings.HasPrefix(ruleID, segments[0]) {
		return false
	}
	// Walk through ruleID, matching each segment in order.
	remaining := ruleID[len(segments[0]):]
	for _, seg := range segments[1:] {
		idx := strings.Index(remaining, seg)
		if idx < 0 {
			return false
		}
		remaining = remaining[idx+len(seg):]
	}
	// If the pattern does not end with *, the ruleID must end exactly
	// at the last segment (no trailing characters allowed).
	if !strings.HasSuffix(pattern, "*") && remaining != "" {
		return false
	}
	return true
}

// Overrides is the in-memory representation of an overrides file.
type Overrides struct {
	Entries []Entry `json:"entries"`
	path    string
}

// Load reads overrides from the given JSON file path.
// Returns an empty Overrides if the file doesn't exist.
func Load(path string) (*Overrides, error) {
	o := &Overrides{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return o, nil
		}
		return nil, fmt.Errorf("reading overrides %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return o, nil
	}
	if err := json.Unmarshal(data, o); err != nil {
		return nil, fmt.Errorf("parsing overrides %s: %w", path, err)
	}
	return o, nil
}

// Save writes the overrides to its file path atomically.
func (o *Overrides) Save() error {
	if o.path == "" {
		return errors.New("overrides has no path")
	}

	dir := filepath.Dir(o.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding overrides: %w", err)
	}

	tmpPath := o.path + ".tmp"
	if err := os.WriteFile(tmpPath, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", tmpPath, err)
	}
	if err := osutil.AtomicRename(tmpPath, o.path); err != nil {
		return fmt.Errorf("renaming %s → %s: %w", tmpPath, o.path, err)
	}
	return nil
}

// Path returns the file path of this overrides file.
func (o *Overrides) Path() string {
	return o.path
}

// Add creates a new entry and appends it to the overrides.
func (o *Overrides) Add(action Action, kind SelectorKind, value, reason string) *Entry {
	entry := Entry{
		ID:      generateID(),
		Action:  action.String(),
		Kind:    kind.String(),
		Value:   value,
		Reason:  reason,
		AddedAt: time.Now().UTC().Truncate(time.Second),
	}
	o.Entries = append(o.Entries, entry)
	return &o.Entries[len(o.Entries)-1]
}

// Remove removes an entry by ID. Returns true if found and removed.
func (o *Overrides) Remove(id string) bool {
	for i, e := range o.Entries {
		if e.ID == id {
			o.Entries = append(o.Entries[:i], o.Entries[i+1:]...)
			return true
		}
	}
	return false
}

// MatchesDeny checks if any deny entry matches the given command+ruleID.
// Returns the first matching deny entry, or nil.
func (o *Overrides) MatchesDeny(command, ruleID string) *Entry {
	for i := range o.Entries {
		if o.Entries[i].Action == "deny" && o.Entries[i].Matches(command, ruleID) {
			return &o.Entries[i]
		}
	}
	return nil
}

// MatchesAllow checks if any allow entry matches the given command+ruleID.
// Returns the first matching allow entry, or nil.
func (o *Overrides) MatchesAllow(command, ruleID string) *Entry {
	for i := range o.Entries {
		if o.Entries[i].Action == "allow" && o.Entries[i].Matches(command, ruleID) {
			return &o.Entries[i]
		}
	}
	return nil
}

// generateID creates a short stable ID like "ov-7f3a".
func generateID() string {
	b := make([]byte, 2)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("ov-%04x", time.Now().UnixNano()&0xFFFF)
	}
	return "ov-" + hex.EncodeToString(b)
}

// UserOverridesPath returns the user-level overrides file path (~/.tw/overrides.json).
func UserOverridesPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".tw", "overrides.json"), nil
}

// ProjectOverridesPath returns the project-level overrides file path.
func ProjectOverridesPath() string {
	return filepath.Join(".tw", "overrides.json")
}

// LoadMerged loads both user and project overrides.
func LoadMerged() (user *Overrides, project *Overrides, err error) {
	userPath, err := UserOverridesPath()
	if err != nil {
		return nil, nil, err
	}
	user, err = Load(userPath)
	if err != nil {
		return nil, nil, fmt.Errorf("loading user overrides: %w", err)
	}

	project, err = Load(ProjectOverridesPath())
	if err != nil {
		return nil, nil, fmt.Errorf("loading project overrides: %w", err)
	}

	return user, project, nil
}

// CheckDeny checks if a command is denied by either the user or project overrides.
// Project overrides are checked first (higher precedence).
func CheckDeny(command, ruleID string, user, project *Overrides) *Entry {
	if project != nil {
		if e := project.MatchesDeny(command, ruleID); e != nil {
			return e
		}
	}
	if user != nil {
		if e := user.MatchesDeny(command, ruleID); e != nil {
			return e
		}
	}
	return nil
}

// CheckAllow checks if a command is allowed by either the user or project overrides.
// Project overrides are checked first (higher precedence).
func CheckAllow(command, ruleID string, user, project *Overrides) *Entry {
	if project != nil {
		if e := project.MatchesAllow(command, ruleID); e != nil {
			return e
		}
	}
	if user != nil {
		if e := user.MatchesAllow(command, ruleID); e != nil {
			return e
		}
	}
	return nil
}
