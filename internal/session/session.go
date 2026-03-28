// Package session provides ephemeral allow entries (session-scoped and
// time-scoped) for the TW command evaluation pipeline. Critical fields
// (kind, value, expiresAt) are signed with HMAC-SHA256 to prevent agent
// tampering; metadata fields (ID, reason, createdAt) are not covered.
package session

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/thgrace/training-wheels/internal/matchutil"
	"github.com/thgrace/training-wheels/internal/osutil"
)

// Entry is a single session-scoped allow or ask entry.
type Entry struct {
	ID        string    `json:"id"`     // "sa-XXXXXXXXXXXXXXXX"
	Action    string    `json:"action"` // "allow" or "ask"
	Kind      string    `json:"kind"`   // "exact", "prefix", "rule"
	Value     string    `json:"value"`
	Reason    string    `json:"reason"`
	ExpiresAt time.Time `json:"expires_at"` // zero = session-scoped (no expiry)
	CreatedAt time.Time `json:"created_at"`
	MAC       string    `json:"mac"` // HMAC-SHA256 hex
}

// Allowlist is the in-memory representation of a session allowlist file.
type Allowlist struct {
	Token   string  `json:"token"`
	Entries []Entry `json:"entries"`
	path    string
}

var fallbackIDCounter uint64
var idGenerator = generateID

// Load reads the allowlist file for the given token, verifies MACs, and
// discards invalid or expired entries. Returns an empty allowlist if the
// file does not exist.
func Load(token string, secret []byte) (*Allowlist, error) {
	p := AllowlistPath(token)
	a := &Allowlist{Token: token, path: p}

	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return a, nil
		}
		return nil, fmt.Errorf("reading session allowlist %s: %w", p, err)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return a, nil
	}

	if err := json.Unmarshal(data, a); err != nil {
		return nil, fmt.Errorf("parsing session allowlist %s: %w", p, err)
	}
	a.path = p // restore after unmarshal (unexported, not in JSON)

	// Filter: keep only entries with valid MACs that are not expired.
	now := time.Now()
	valid := make([]Entry, 0, len(a.Entries))
	for i := range a.Entries {
		e := &a.Entries[i]
		if !verifyMAC(secret, e) {
			continue
		}
		if !e.ExpiresAt.IsZero() && now.After(e.ExpiresAt) {
			continue
		}
		valid = append(valid, *e)
	}
	a.Entries = valid

	return a, nil
}

// Save writes the allowlist to its file path atomically with mode 0600.
func (a *Allowlist) Save() error {
	if a.path == "" {
		return errors.New("allowlist has no path")
	}

	dir := filepath.Dir(a.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding session allowlist: %w", err)
	}

	tmpPath := a.path + ".tmp"
	if err := os.WriteFile(tmpPath, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", tmpPath, err)
	}
	if err := osutil.AtomicRename(tmpPath, a.path); err != nil {
		return fmt.Errorf("renaming %s → %s: %w", tmpPath, a.path, err)
	}
	return nil
}

// Add creates a new entry with a computed HMAC, appends it, and returns
// a pointer to the appended entry.
func (a *Allowlist) Add(secret []byte, action, kind, value, reason string, expiresAt time.Time) *Entry {
	id := idGenerator()
	for a.hasID(id) {
		id = idGenerator()
	}

	entry := Entry{
		ID:        id,
		Action:    action,
		Kind:      kind,
		Value:     value,
		Reason:    reason,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
		MAC:       computeMAC(secret, action, kind, value, expiresAt),
	}
	a.Entries = append(a.Entries, entry)
	return &a.Entries[len(a.Entries)-1]
}

func (a *Allowlist) hasID(id string) bool {
	for i := range a.Entries {
		if a.Entries[i].ID == id {
			return true
		}
	}
	return false
}

// Remove removes an entry by ID. Returns true if found and removed.
func (a *Allowlist) Remove(id string) bool {
	for i, e := range a.Entries {
		if e.ID == id {
			a.Entries = append(a.Entries[:i], a.Entries[i+1:]...)
			return true
		}
	}
	return false
}

// Matches returns the first matching non-expired entry for the given
// command and ruleID. Expired entries are discarded during the scan.
func (a *Allowlist) Matches(command, ruleID string) *Entry {
	now := time.Now()
	for i := range a.Entries {
		e := &a.Entries[i]

		// Check expiry — skip expired entries.
		if !e.ExpiresAt.IsZero() && now.After(e.ExpiresAt) {
			continue
		}

		switch e.Kind {
		case "exact":
			if command == e.Value {
				return e
			}
		case "prefix":
			if strings.HasPrefix(command, e.Value) {
				return e
			}
		case "rule":
			if matchutil.MatchRule(e.Value, ruleID) {
				return e
			}
		}
	}
	return nil
}

// ---------- Token management ----------

// TokenPath returns the path to the session token file under ~/.tw/.
func TokenPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	return filepath.Join(home, ".tw", "session-token")
}

// ReadOrCreateToken reads the session token file. If it does not exist, a new
// 32-character hex token (16 random bytes) is generated and written with mode 0600.
func ReadOrCreateToken() (string, error) {
	p := TokenPath()
	data, err := os.ReadFile(p)
	if err == nil {
		tok := strings.TrimSpace(string(data))
		if tok != "" {
			return tok, nil
		}
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("reading token file %s: %w", p, err)
	}

	// Generate a new token: 16 random bytes → 32 hex chars.
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating token: %w", err)
	}
	tok := hex.EncodeToString(b)

	if dir := filepath.Dir(p); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(p, []byte(tok+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("writing token file %s: %w", p, err)
	}
	return tok, nil
}

// ReadToken reads the session token file. Returns ("", nil) if the file does
// not exist (fail-open for tw hook).
func ReadToken() (string, error) {
	p := TokenPath()
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("reading token file %s: %w", p, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// ---------- Secret management ----------

// SecretPath returns the default path for the HMAC secret key: ~/.tw/allow.key.
func SecretPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback — should rarely happen.
		home = os.TempDir()
	}
	return filepath.Join(home, ".tw", "allow.key")
}

// LoadOrCreateSecret reads a 32-byte HMAC key from the given path.
// If the file does not exist, a new random key is generated and written
// with mode 0600. Parent directories are created as needed.
func LoadOrCreateSecret(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		if len(data) >= 32 {
			return data[:32], nil
		}
		// File exists but is too short — regenerate.
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading secret %s: %w", path, err)
	}

	// Generate a new 32-byte key.
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generating secret: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating directory %s: %w", dir, err)
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, fmt.Errorf("writing secret %s: %w", path, err)
	}
	return key, nil
}

// ---------- Terminal detection ----------

// IsInteractiveTerminal reports whether stdin is connected to an interactive
// terminal (character device).
func IsInteractiveTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// ---------- Path helpers ----------

// AllowlistPath returns the path to the session allowlist file for a given token.
// Uses the first 16 characters of the token (or the full token if shorter).
func AllowlistPath(token string) string {
	prefix := token
	if len(prefix) > 16 {
		prefix = prefix[:16]
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	return filepath.Join(home, ".tw", "allow-"+prefix+".json")
}

// ---------- HMAC helpers ----------

// computeMAC computes an HMAC-SHA256 over "action|kind|value|expiresUnix" and
// returns the hex-encoded result. For a zero ExpiresAt, unix timestamp 0 is used.
func computeMAC(secret []byte, action, kind, value string, expiresAt time.Time) string {
	var unix int64
	if !expiresAt.IsZero() {
		unix = expiresAt.Unix()
	}
	msg := action + "|" + kind + "|" + value + "|" + strconv.FormatInt(unix, 10)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyMAC recomputes the HMAC for the entry and compares it with the stored MAC.
func verifyMAC(secret []byte, e *Entry) bool {
	expected := computeMAC(secret, e.Action, e.Kind, e.Value, e.ExpiresAt)
	return hmac.Equal([]byte(expected), []byte(e.MAC))
}

// ---------- ID generation ----------

// generateID creates a short ID like "sa-0123abcd4567ef89" (16 hex chars).
// Falls back to a timestamp/counter-based ID if random generation fails.
func generateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf(
			"sa-%016x",
			uint64(time.Now().UnixNano())^atomic.AddUint64(&fallbackIDCounter, 1),
		)
	}
	return "sa-" + hex.EncodeToString(b)
}
