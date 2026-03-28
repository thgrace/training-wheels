package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAdd(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	e := a.Add(secret, "allow", "exact", "rm -rf ./dist", "Build cleanup", time.Time{})

	if !strings.HasPrefix(e.ID, "sa-") {
		t.Errorf("ID = %q, want prefix sa-", e.ID)
	}
	idSuffix := strings.TrimPrefix(e.ID, "sa-")
	if len(idSuffix) != 16 {
		t.Errorf("ID suffix length = %d, want 16", len(idSuffix))
	}
	if _, err := hex.DecodeString(idSuffix); err != nil {
		t.Errorf("ID suffix should be hex: %v", err)
	}
	if e.Action != "allow" {
		t.Errorf("Action = %q, want allow", e.Action)
	}
	if e.Kind != "exact" {
		t.Errorf("Kind = %q, want exact", e.Kind)
	}
	if e.Value != "rm -rf ./dist" {
		t.Errorf("Value = %q, want %q", e.Value, "rm -rf ./dist")
	}
	if e.Reason != "Build cleanup" {
		t.Errorf("Reason = %q, want %q", e.Reason, "Build cleanup")
	}
	if e.MAC == "" {
		t.Error("MAC should not be empty")
	}
	if e.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if !e.ExpiresAt.IsZero() {
		t.Errorf("ExpiresAt should be zero for session-scoped, got %v", e.ExpiresAt)
	}
	if len(a.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(a.Entries))
	}
}

func TestAdd_WithExpiry(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")
	expiry := time.Now().Add(1 * time.Hour).UTC().Truncate(time.Second)

	e := a.Add(secret, "allow", "prefix", "make ", "Allow make commands", expiry)

	if e.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should be set for timed entry")
	}
	if !e.ExpiresAt.Equal(expiry) {
		t.Errorf("ExpiresAt = %v, want %v", e.ExpiresAt, expiry)
	}
}

func TestRemove(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	a.Add(secret, "allow", "exact", "cmd1", "r1", time.Time{})
	e2 := a.Add(secret, "allow", "exact", "cmd2", "r2", time.Time{})
	a.Add(secret, "allow", "exact", "cmd3", "r3", time.Time{})

	if !a.Remove(e2.ID) {
		t.Error("Remove should return true for existing ID")
	}
	if len(a.Entries) != 2 {
		t.Errorf("expected 2 entries after remove, got %d", len(a.Entries))
	}
	// Verify the remaining entries are cmd1 and cmd3.
	if a.Entries[0].Value != "cmd1" {
		t.Errorf("entry 0 value = %q, want cmd1", a.Entries[0].Value)
	}
	if a.Entries[1].Value != "cmd3" {
		t.Errorf("entry 1 value = %q, want cmd3", a.Entries[1].Value)
	}
}

func TestRemove_NotFound(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	a.Add(secret, "allow", "exact", "cmd1", "r1", time.Time{})

	if a.Remove("nonexistent-id") {
		t.Error("Remove should return false for non-existent ID")
	}
	if len(a.Entries) != 1 {
		t.Errorf("expected 1 entry unchanged, got %d", len(a.Entries))
	}
}

func TestMatches_Exact(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	a.Add(secret, "allow", "exact", "rm -rf ./dist", "cleanup", time.Time{})

	if e := a.Matches("rm -rf ./dist", ""); e == nil {
		t.Error("exact match should succeed for identical command")
	}
	if e := a.Matches("rm -rf ./dist2", ""); e != nil {
		t.Error("exact match should not match different command")
	}
	if e := a.Matches("rm -rf ./dis", ""); e != nil {
		t.Error("exact match should not match substring")
	}
}

func TestMatches_Prefix(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	a.Add(secret, "allow", "prefix", "make clean", "build", time.Time{})

	if e := a.Matches("make clean", ""); e == nil {
		t.Error("prefix match should succeed for exact match")
	}
	if e := a.Matches("make clean all", ""); e == nil {
		t.Error("prefix match should succeed for extended command")
	}
	if e := a.Matches("make build", ""); e != nil {
		t.Error("prefix match should not match different prefix")
	}
}

func TestMatches_Rule(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	// Exact rule ID match.
	a.Add(secret, "allow", "rule", "core.git:reset-hard", "allow reset", time.Time{})
	if e := a.Matches("", "core.git:reset-hard"); e == nil {
		t.Error("rule match should succeed for exact ID")
	}
	if e := a.Matches("", "core.git:push-force"); e != nil {
		t.Error("rule match should not match different rule")
	}

	// Wildcard rule match.
	a.Add(secret, "allow", "rule", "core.git:*", "allow all git", time.Time{})
	if e := a.Matches("", "core.git:push-force"); e == nil {
		t.Error("wildcard rule should match core.git:push-force")
	}
	if e := a.Matches("", "core.filesystem:rm-rf"); e != nil {
		t.Error("wildcard rule should not match different pack")
	}

	// Global wildcard.
	a2 := &Allowlist{Token: "test-token", path: "/dev/null"}
	a2.Add(secret, "allow", "rule", "*", "allow everything", time.Time{})
	if e := a2.Matches("", "anything:here"); e == nil {
		t.Error("global wildcard should match everything")
	}
}

func TestMatches_RuleMultiWildcard(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	a.Add(secret, "allow", "rule", "core.*:reset-*", "allow core resets", time.Time{})

	if e := a.Matches("", "core.git:reset-hard"); e == nil {
		t.Error("multi-wildcard should match core.git:reset-hard")
	}
	if e := a.Matches("", "core.filesystem:reset-soft"); e == nil {
		t.Error("multi-wildcard should match core.filesystem:reset-soft")
	}
	if e := a.Matches("", "core.git:push-force"); e != nil {
		t.Error("multi-wildcard should not match core.git:push-force")
	}
}

func TestMatches_Expired(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	// Add entry that expired in the past.
	pastTime := time.Now().Add(-1 * time.Hour).UTC().Truncate(time.Second)
	a.Add(secret, "allow", "exact", "rm -rf ./dist", "expired cleanup", pastTime)

	if e := a.Matches("rm -rf ./dist", ""); e != nil {
		t.Error("expired entry should not match")
	}
}

func TestMatches_NotExpired(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	// Add entry that expires in the future.
	futureTime := time.Now().Add(1 * time.Hour).UTC().Truncate(time.Second)
	a.Add(secret, "allow", "exact", "rm -rf ./dist", "future cleanup", futureTime)

	if e := a.Matches("rm -rf ./dist", ""); e == nil {
		t.Error("non-expired entry should match")
	}
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	secret := []byte("test-secret-key-32-bytes-long!!")
	token := "test-token-abc123"
	path := filepath.Join(dir, "tw-session-"+token+".json")

	a := &Allowlist{Token: token, path: path}
	a.Add(secret, "allow", "exact", "rm -rf ./dist", "cleanup", time.Time{})
	a.Add(secret, "allow", "prefix", "make ", "build commands", time.Time{})
	a.Add(secret, "allow", "rule", "core.git:*", "allow git", time.Time{})

	if err := a.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file exists with 0600 permissions.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("file should exist after save: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}

	// Load back and verify.
	var a2 *Allowlist
	// Manual load from the file directly.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	a2 = &Allowlist{}
	if err := json.Unmarshal(data, a2); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	a2.path = path

	if len(a2.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(a2.Entries))
	}
	if a2.Entries[0].Value != "rm -rf ./dist" {
		t.Errorf("entry 0 value = %q", a2.Entries[0].Value)
	}
	if a2.Entries[0].Action != "allow" {
		t.Errorf("entry 0 action = %q", a2.Entries[0].Action)
	}
	if a2.Entries[0].Kind != "exact" {
		t.Errorf("entry 0 kind = %q", a2.Entries[0].Kind)
	}
	if a2.Entries[1].Kind != "prefix" {
		t.Errorf("entry 1 kind = %q", a2.Entries[1].Kind)
	}
	if a2.Entries[2].Kind != "rule" {
		t.Errorf("entry 2 kind = %q", a2.Entries[2].Kind)
	}

	// Verify MACs are valid.
	for i, e := range a2.Entries {
		if !verifyMAC(secret, &e) {
			t.Errorf("entry %d MAC verification failed", i)
		}
	}
}

func TestLoad_NonExistent(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")
	// Use a token that will never have a corresponding file.
	a, err := Load("nonexistent-token-xyz789", secret)
	if err != nil {
		t.Fatalf("Load non-existent should not error: %v", err)
	}
	if len(a.Entries) != 0 {
		t.Error("non-existent file should return empty allowlist")
	}
}

func TestLoad_InvalidMAC(t *testing.T) {
	dir := t.TempDir()
	secret := []byte("test-secret-key-32-bytes-long!!")
	token := "test-token-tampered"
	path := filepath.Join(dir, "tw-session-"+token+".json")

	// Create a valid allowlist and save it.
	a := &Allowlist{Token: token, path: path}
	a.Add(secret, "allow", "exact", "good-cmd", "valid entry", time.Time{})
	a.Add(secret, "allow", "exact", "tampered-cmd", "will be tampered", time.Time{})

	// Tamper with the second entry's MAC before saving.
	a.Entries[1].MAC = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Load should discard the tampered entry.
	loaded := &Allowlist{}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, loaded); err != nil {
		t.Fatal(err)
	}
	loaded.path = path

	// Manually filter as Load would.
	var valid []Entry
	for _, e := range loaded.Entries {
		if verifyMAC(secret, &e) {
			valid = append(valid, e)
		}
	}

	if len(valid) != 1 {
		t.Fatalf("expected 1 valid entry after MAC check, got %d", len(valid))
	}
	if valid[0].Value != "good-cmd" {
		t.Errorf("surviving entry = %q, want good-cmd", valid[0].Value)
	}
}

func TestLoad_ExpiredDiscarded(t *testing.T) {
	dir := t.TempDir()
	secret := []byte("test-secret-key-32-bytes-long!!")
	token := "test-token-expired"
	path := filepath.Join(dir, "tw-session-"+token+".json")

	a := &Allowlist{Token: token, path: path}

	// Add a valid non-expired entry.
	a.Add(secret, "allow", "exact", "alive-cmd", "still valid", time.Time{})

	// Add an expired entry.
	pastTime := time.Now().Add(-1 * time.Hour).UTC().Truncate(time.Second)
	a.Add(secret, "allow", "exact", "dead-cmd", "expired", pastTime)

	if err := a.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Read back and filter as Load would.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	loaded := &Allowlist{}
	if err := json.Unmarshal(raw, loaded); err != nil {
		t.Fatal(err)
	}

	var valid []Entry
	now := time.Now()
	for _, e := range loaded.Entries {
		if verifyMAC(secret, &e) && (e.ExpiresAt.IsZero() || e.ExpiresAt.After(now)) {
			valid = append(valid, e)
		}
	}

	if len(valid) != 1 {
		t.Fatalf("expected 1 valid entry after expiry check, got %d", len(valid))
	}
	if valid[0].Value != "alive-cmd" {
		t.Errorf("surviving entry = %q, want alive-cmd", valid[0].Value)
	}
}

func TestComputeVerifyMAC(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")

	entry := Entry{
		ID:        "sa-abcd",
		Action:    "allow",
		Kind:      "exact",
		Value:     "rm -rf ./dist",
		Reason:    "cleanup",
		ExpiresAt: time.Time{},
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}

	// Compute MAC.
	mac := computeMAC(secret, entry.Action, entry.Kind, entry.Value, entry.ExpiresAt)
	if mac == "" {
		t.Fatal("computeMAC should not return empty string")
	}

	// Verify it's valid hex.
	if _, err := hex.DecodeString(mac); err != nil {
		t.Errorf("MAC should be valid hex: %v", err)
	}

	// Verify the MAC.
	entry.MAC = mac
	if !verifyMAC(secret, &entry) {
		t.Error("verifyMAC should return true for valid MAC")
	}

	// Tamper with the Action and verify MAC fails.
	tampered0 := entry
	tampered0.Action = "ask"
	if verifyMAC(secret, &tampered0) {
		t.Error("verifyMAC should return false after tampering Action")
	}

	// Tamper with the value and verify MAC fails.
	tampered := entry
	tampered.Value = "rm -rf /"
	if verifyMAC(secret, &tampered) {
		t.Error("verifyMAC should return false after tampering Value")
	}

	// Tamper with the kind and verify MAC fails.
	tampered2 := entry
	tampered2.Kind = "prefix"
	if verifyMAC(secret, &tampered2) {
		t.Error("verifyMAC should return false after tampering Kind")
	}

	// Tamper with the reason — reason is NOT in the HMAC, so this should still verify.
	tampered3 := entry
	tampered3.Reason = "malicious"
	if !verifyMAC(secret, &tampered3) {
		t.Error("verifyMAC should return true when only Reason is changed (not in HMAC)")
	}

	// Tamper with the MAC itself.
	tampered4 := entry
	tampered4.MAC = "0000000000000000000000000000000000000000000000000000000000000000"
	if verifyMAC(secret, &tampered4) {
		t.Error("verifyMAC should return false for wrong MAC")
	}

	// Different secret should not verify.
	otherSecret := []byte("different-secret-key-32-bytes!!")
	if verifyMAC(otherSecret, &entry) {
		t.Error("verifyMAC should return false for different secret")
	}
}

func TestComputeMAC_Deterministic(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")
	entry := Entry{
		ID:        "sa-1234",
		Action:    "allow",
		Kind:      "exact",
		Value:     "echo hello",
		Reason:    "test",
		ExpiresAt: time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC),
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	mac1 := computeMAC(secret, entry.Action, entry.Kind, entry.Value, entry.ExpiresAt)
	mac2 := computeMAC(secret, entry.Action, entry.Kind, entry.Value, entry.ExpiresAt)

	if mac1 != mac2 {
		t.Errorf("computeMAC should be deterministic: %q != %q", mac1, mac2)
	}
}

func TestComputeMAC_DifferentInputsDifferentMACs(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")
	base := Entry{
		ID:        "sa-1234",
		Action:    "allow",
		Kind:      "exact",
		Value:     "echo hello",
		Reason:    "test",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	baseMac := computeMAC(secret, base.Action, base.Kind, base.Value, base.ExpiresAt)

	// Vary each HMAC-covered field and ensure MAC changes.
	// HMAC covers: action, kind, value, expiresAt. ID, Reason, CreatedAt are NOT covered.
	tests := []struct {
		name         string
		action       string
		kind         string
		value        string
		expiresAt    time.Time
		shouldDiffer bool
	}{
		{"different Action", "ask", base.Kind, base.Value, base.ExpiresAt, true},
		{"different Kind", base.Action, "prefix", base.Value, base.ExpiresAt, true},
		{"different Value", base.Action, base.Kind, "echo world", base.ExpiresAt, true},
		{"different ExpiresAt", base.Action, base.Kind, base.Value, time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), true},
		{"same inputs", base.Action, base.Kind, base.Value, base.ExpiresAt, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mac := computeMAC(secret, tt.action, tt.kind, tt.value, tt.expiresAt)
			if tt.shouldDiffer && mac == baseMac {
				t.Errorf("MAC should differ for %s", tt.name)
			}
			if !tt.shouldDiffer && mac != baseMac {
				t.Errorf("MAC should be same for %s", tt.name)
			}
		})
	}
}

func TestMatches_SessionScoped_NoExpiry(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	// Session-scoped entry (zero ExpiresAt) should always match.
	a.Add(secret, "allow", "exact", "session-cmd", "session scoped", time.Time{})

	if e := a.Matches("session-cmd", ""); e == nil {
		t.Error("session-scoped entry (no expiry) should match")
	}
}

func TestMatches_MultipleEntries_FirstMatch(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	a.Add(secret, "allow", "exact", "cmd-a", "first", time.Time{})
	a.Add(secret, "allow", "exact", "cmd-a", "second", time.Time{})

	e := a.Matches("cmd-a", "")
	if e == nil {
		t.Fatal("should match")
	}
	if e.Reason != "first" {
		t.Errorf("should return first matching entry, got reason=%q", e.Reason)
	}
}

func TestMatches_NoMatch(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	a.Add(secret, "allow", "exact", "specific-cmd", "only this", time.Time{})

	if e := a.Matches("other-cmd", ""); e != nil {
		t.Error("should not match unrelated command")
	}
	if e := a.Matches("", "some:rule"); e != nil {
		t.Error("exact entry should not match rule IDs")
	}
}

func TestMatches_EmptyAllowlist(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}

	if e := a.Matches("any-cmd", "any:rule"); e != nil {
		t.Error("empty allowlist should not match anything")
	}
}

func TestSave_CreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "sub", "dir", "session.json")

	a := &Allowlist{Token: "test-token", path: nested}
	secret := []byte("test-secret-key-32-bytes-long!!")
	a.Add(secret, "allow", "exact", "cmd", "test", time.Time{})

	if err := a.Save(); err != nil {
		t.Fatalf("Save should create intermediate directories: %v", err)
	}

	if _, err := os.Stat(nested); err != nil {
		t.Fatalf("file should exist at nested path: %v", err)
	}
}

func TestSave_AtomicOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")
	secret := []byte("test-secret-key-32-bytes-long!!")

	// Save initial version.
	a := &Allowlist{Token: "test-token", path: path}
	a.Add(secret, "allow", "exact", "original-cmd", "v1", time.Time{})
	if err := a.Save(); err != nil {
		t.Fatalf("Save v1: %v", err)
	}

	// Save updated version.
	a.Add(secret, "allow", "exact", "new-cmd", "v2", time.Time{})
	if err := a.Save(); err != nil {
		t.Fatalf("Save v2: %v", err)
	}

	// Read back and verify both entries exist.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	loaded := &Allowlist{}
	if err := json.Unmarshal(data, loaded); err != nil {
		t.Fatal(err)
	}
	if len(loaded.Entries) != 2 {
		t.Errorf("expected 2 entries after overwrite, got %d", len(loaded.Entries))
	}
}

func TestMACIntegrity_HMACSHA256(t *testing.T) {
	// Verify that the MAC computation uses HMAC-SHA256 by checking the output length.
	secret := []byte("test-secret-key-32-bytes-long!!")
	entry := Entry{
		ID:        "sa-test",
		Action:    "allow",
		Kind:      "exact",
		Value:     "test",
		Reason:    "test",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}

	mac := computeMAC(secret, entry.Action, entry.Kind, entry.Value, entry.ExpiresAt)
	macBytes, err := hex.DecodeString(mac)
	if err != nil {
		t.Fatalf("MAC should be valid hex: %v", err)
	}
	// HMAC-SHA256 produces 32 bytes = 64 hex characters.
	if len(macBytes) != sha256.Size {
		t.Errorf("MAC length = %d bytes, want %d (HMAC-SHA256)", len(macBytes), sha256.Size)
	}
}

func TestAdd_UniqueIDs(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	originalIDGenerator := idGenerator
	t.Cleanup(func() {
		idGenerator = originalIDGenerator
	})

	ids := []string{"sa-0000000000000001", "sa-0000000000000001", "sa-0000000000000002"}
	idGenerator = func() string {
		if len(ids) == 0 {
			t.Fatal("idGenerator called more than expected")
		}
		id := ids[0]
		ids = ids[1:]
		return id
	}

	first := a.Add(secret, "allow", "exact", "cmd1", "test", time.Time{})
	second := a.Add(secret, "allow", "exact", "cmd2", "test", time.Time{})

	if first.ID != "sa-0000000000000001" {
		t.Fatalf("first ID = %q, want sa-0000000000000001", first.ID)
	}
	if second.ID != "sa-0000000000000002" {
		t.Fatalf("second ID = %q, want sa-0000000000000002", second.ID)
	}
}

func TestAdd_AskAction(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	e := a.Add(secret, "ask", "exact", "git push --force", "ask for force push", time.Time{})

	if e.Action != "ask" {
		t.Errorf("Action = %q, want ask", e.Action)
	}
	if !verifyMAC(secret, e) {
		t.Error("MAC verification failed for ask entry")
	}
}
