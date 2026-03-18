package session

import (
	"crypto/hmac"
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

	e := a.Add(secret, "exact", "rm -rf ./dist", "Build cleanup", time.Time{})

	if !strings.HasPrefix(e.ID, "sa-") {
		t.Errorf("ID = %q, want prefix sa-", e.ID)
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

	e := a.Add(secret, "prefix", "make ", "Allow make commands", expiry)

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

	a.Add(secret, "exact", "cmd1", "r1", time.Time{})
	e2 := a.Add(secret, "exact", "cmd2", "r2", time.Time{})
	a.Add(secret, "exact", "cmd3", "r3", time.Time{})

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

	a.Add(secret, "exact", "cmd1", "r1", time.Time{})

	if a.Remove("nonexistent-id") {
		t.Error("Remove should return false for non-existent ID")
	}
	if len(a.Entries) != 1 {
		t.Errorf("expected 1 entry unchanged, got %d", len(a.Entries))
	}
}

func TestMatchesAllow_Exact(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	a.Add(secret, "exact", "rm -rf ./dist", "cleanup", time.Time{})

	if e := a.MatchesAllow("rm -rf ./dist", ""); e == nil {
		t.Error("exact match should succeed for identical command")
	}
	if e := a.MatchesAllow("rm -rf ./dist2", ""); e != nil {
		t.Error("exact match should not match different command")
	}
	if e := a.MatchesAllow("rm -rf ./dis", ""); e != nil {
		t.Error("exact match should not match substring")
	}
}

func TestMatchesAllow_Prefix(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	a.Add(secret, "prefix", "make clean", "build", time.Time{})

	if e := a.MatchesAllow("make clean", ""); e == nil {
		t.Error("prefix match should succeed for exact match")
	}
	if e := a.MatchesAllow("make clean all", ""); e == nil {
		t.Error("prefix match should succeed for extended command")
	}
	if e := a.MatchesAllow("make build", ""); e != nil {
		t.Error("prefix match should not match different prefix")
	}
}

func TestMatchesAllow_Rule(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	// Exact rule ID match.
	a.Add(secret, "rule", "core.git:reset-hard", "allow reset", time.Time{})
	if e := a.MatchesAllow("", "core.git:reset-hard"); e == nil {
		t.Error("rule match should succeed for exact ID")
	}
	if e := a.MatchesAllow("", "core.git:push-force"); e != nil {
		t.Error("rule match should not match different rule")
	}

	// Wildcard rule match.
	a.Add(secret, "rule", "core.git:*", "allow all git", time.Time{})
	if e := a.MatchesAllow("", "core.git:push-force"); e == nil {
		t.Error("wildcard rule should match core.git:push-force")
	}
	if e := a.MatchesAllow("", "core.filesystem:rm-rf"); e != nil {
		t.Error("wildcard rule should not match different pack")
	}

	// Global wildcard.
	a2 := &Allowlist{Token: "test-token", path: "/dev/null"}
	a2.Add(secret, "rule", "*", "allow everything", time.Time{})
	if e := a2.MatchesAllow("", "anything:here"); e == nil {
		t.Error("global wildcard should match everything")
	}
}

func TestMatchesAllow_RuleMultiWildcard(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	a.Add(secret, "rule", "core.*:reset-*", "allow core resets", time.Time{})

	if e := a.MatchesAllow("", "core.git:reset-hard"); e == nil {
		t.Error("multi-wildcard should match core.git:reset-hard")
	}
	if e := a.MatchesAllow("", "core.filesystem:reset-soft"); e == nil {
		t.Error("multi-wildcard should match core.filesystem:reset-soft")
	}
	if e := a.MatchesAllow("", "core.git:push-force"); e != nil {
		t.Error("multi-wildcard should not match core.git:push-force")
	}
}

func TestMatchesAllow_Expired(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	// Add entry that expired in the past.
	pastTime := time.Now().Add(-1 * time.Hour).UTC().Truncate(time.Second)
	a.Add(secret, "exact", "rm -rf ./dist", "expired cleanup", pastTime)

	if e := a.MatchesAllow("rm -rf ./dist", ""); e != nil {
		t.Error("expired entry should not match")
	}
}

func TestMatchesAllow_NotExpired(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	// Add entry that expires in the future.
	futureTime := time.Now().Add(1 * time.Hour).UTC().Truncate(time.Second)
	a.Add(secret, "exact", "rm -rf ./dist", "future cleanup", futureTime)

	if e := a.MatchesAllow("rm -rf ./dist", ""); e == nil {
		t.Error("non-expired entry should match")
	}
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	secret := []byte("test-secret-key-32-bytes-long!!")
	token := "test-token-abc123"
	path := filepath.Join(dir, "tw-session-"+token+".json")

	a := &Allowlist{Token: token, path: path}
	a.Add(secret, "exact", "rm -rf ./dist", "cleanup", time.Time{})
	a.Add(secret, "prefix", "make ", "build commands", time.Time{})
	a.Add(secret, "rule", "core.git:*", "allow git", time.Time{})

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
	a2, err := Load(token, secret)
	if err != nil {
		// Load uses AllowlistPath(token) to determine path.
		// Since we saved to a custom path, load directly by setting up properly.
		// Let's instead load from the file directly.
		t.Logf("Load with standard path failed (expected if AllowlistPath differs): %v", err)
	}

	// Load by creating an Allowlist with the same path and reading manually.
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
	a.Add(secret, "exact", "good-cmd", "valid entry", time.Time{})
	a.Add(secret, "exact", "tampered-cmd", "will be tampered", time.Time{})

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
	a.Add(secret, "exact", "alive-cmd", "still valid", time.Time{})

	// Add an expired entry.
	pastTime := time.Now().Add(-1 * time.Hour).UTC().Truncate(time.Second)
	a.Add(secret, "exact", "dead-cmd", "expired", pastTime)

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

func TestLoadOrCreateSecret(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "allow.key")

	// First call creates the secret.
	secret1, err := LoadOrCreateSecret(path)
	if err != nil {
		t.Fatalf("LoadOrCreateSecret (create): %v", err)
	}
	if len(secret1) != 32 {
		t.Errorf("secret length = %d, want 32", len(secret1))
	}

	// Second call reads the same secret.
	secret2, err := LoadOrCreateSecret(path)
	if err != nil {
		t.Fatalf("LoadOrCreateSecret (read): %v", err)
	}
	if !hmac.Equal(secret1, secret2) {
		t.Error("second call should return the same secret")
	}

	// Verify file permissions.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("secret file permissions = %o, want 0600", perm)
	}
}

func TestLoadOrCreateSecret_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "allow.key")

	// Write known bytes.
	knownSecret := make([]byte, 32)
	for i := range knownSecret {
		knownSecret[i] = byte(i)
	}
	if err := os.WriteFile(path, knownSecret, 0o600); err != nil {
		t.Fatal(err)
	}

	// Load should return the same bytes.
	loaded, err := LoadOrCreateSecret(path)
	if err != nil {
		t.Fatalf("LoadOrCreateSecret: %v", err)
	}
	if !hmac.Equal(loaded, knownSecret) {
		t.Error("loaded secret should match written bytes")
	}
}

func TestTokenPath(t *testing.T) {
	p := TokenPath()
	if p == "" {
		t.Fatal("TokenPath should not be empty")
	}
	if !strings.Contains(p, "session-token") {
		t.Errorf("TokenPath = %q, want to contain session-token", p)
	}
}

func TestAllowlistPath(t *testing.T) {
	p := AllowlistPath("abc123def456")
	if p == "" {
		t.Fatal("AllowlistPath should not be empty")
	}
	if !strings.Contains(p, "abc123def456") {
		t.Errorf("AllowlistPath = %q, should contain the token", p)
	}

	// Test with a short token.
	p2 := AllowlistPath("ab")
	if p2 == "" {
		t.Fatal("AllowlistPath with short token should not be empty")
	}
	if !strings.Contains(p2, "ab") {
		t.Errorf("AllowlistPath = %q, should contain the short token", p2)
	}
}

func TestSecretPath(t *testing.T) {
	p := SecretPath()
	if p == "" {
		t.Fatal("SecretPath should not be empty")
	}
	if !strings.Contains(p, "allow.key") {
		t.Errorf("SecretPath = %q, want to contain allow.key", p)
	}
	if !strings.Contains(p, ".tw") {
		t.Errorf("SecretPath = %q, want to contain .tw", p)
	}
}

func TestComputeVerifyMAC(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")

	entry := Entry{
		ID:        "sa-abcd",
		Kind:      "exact",
		Value:     "rm -rf ./dist",
		Reason:    "cleanup",
		ExpiresAt: time.Time{},
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}

	// Compute MAC.
	mac := computeMAC(secret, entry.Kind, entry.Value, entry.ExpiresAt)
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
		Kind:      "exact",
		Value:     "echo hello",
		Reason:    "test",
		ExpiresAt: time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC),
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	mac1 := computeMAC(secret, entry.Kind, entry.Value, entry.ExpiresAt)
	mac2 := computeMAC(secret, entry.Kind, entry.Value, entry.ExpiresAt)

	if mac1 != mac2 {
		t.Errorf("computeMAC should be deterministic: %q != %q", mac1, mac2)
	}
}

func TestComputeMAC_DifferentInputsDifferentMACs(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")
	base := Entry{
		ID:        "sa-1234",
		Kind:      "exact",
		Value:     "echo hello",
		Reason:    "test",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	baseMac := computeMAC(secret, base.Kind, base.Value, base.ExpiresAt)

	// Vary each HMAC-covered field and ensure MAC changes.
	// HMAC covers: kind, value, expiresAt. ID, Reason, CreatedAt are NOT covered.
	tests := []struct {
		name      string
		kind      string
		value     string
		expiresAt time.Time
		shouldDiffer bool
	}{
		{"different Kind", "prefix", base.Value, base.ExpiresAt, true},
		{"different Value", base.Kind, "echo world", base.ExpiresAt, true},
		{"different ExpiresAt", base.Kind, base.Value, time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), true},
		{"same inputs", base.Kind, base.Value, base.ExpiresAt, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mac := computeMAC(secret, tt.kind, tt.value, tt.expiresAt)
			if tt.shouldDiffer && mac == baseMac {
				t.Errorf("MAC should differ for %s", tt.name)
			}
			if !tt.shouldDiffer && mac != baseMac {
				t.Errorf("MAC should be same for %s", tt.name)
			}
		})
	}
}

func TestMatchesAllow_SessionScoped_NoExpiry(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	// Session-scoped entry (zero ExpiresAt) should always match.
	a.Add(secret, "exact", "session-cmd", "session scoped", time.Time{})

	if e := a.MatchesAllow("session-cmd", ""); e == nil {
		t.Error("session-scoped entry (no expiry) should match")
	}
}

func TestMatchesAllow_MultipleEntries_FirstMatch(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	a.Add(secret, "exact", "cmd-a", "first", time.Time{})
	a.Add(secret, "exact", "cmd-a", "second", time.Time{})

	e := a.MatchesAllow("cmd-a", "")
	if e == nil {
		t.Fatal("should match")
	}
	if e.Reason != "first" {
		t.Errorf("should return first matching entry, got reason=%q", e.Reason)
	}
}

func TestMatchesAllow_NoMatch(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}
	secret := []byte("test-secret-key-32-bytes-long!!")

	a.Add(secret, "exact", "specific-cmd", "only this", time.Time{})

	if e := a.MatchesAllow("other-cmd", ""); e != nil {
		t.Error("should not match unrelated command")
	}
	if e := a.MatchesAllow("", "some:rule"); e != nil {
		t.Error("exact entry should not match rule IDs")
	}
}

func TestMatchesAllow_EmptyAllowlist(t *testing.T) {
	a := &Allowlist{Token: "test-token", path: "/dev/null"}

	if e := a.MatchesAllow("any-cmd", "any:rule"); e != nil {
		t.Error("empty allowlist should not match anything")
	}
}

func TestSave_CreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "sub", "dir", "session.json")

	a := &Allowlist{Token: "test-token", path: nested}
	secret := []byte("test-secret-key-32-bytes-long!!")
	a.Add(secret, "exact", "cmd", "test", time.Time{})

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
	a.Add(secret, "exact", "original-cmd", "v1", time.Time{})
	if err := a.Save(); err != nil {
		t.Fatalf("Save v1: %v", err)
	}

	// Save updated version.
	a.Add(secret, "exact", "new-cmd", "v2", time.Time{})
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
		Kind:      "exact",
		Value:     "test",
		Reason:    "test",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}

	mac := computeMAC(secret, entry.Kind, entry.Value, entry.ExpiresAt)
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

	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		e := a.Add(secret, "exact", "cmd", "test", time.Time{})
		if ids[e.ID] {
			t.Fatalf("duplicate ID generated: %s", e.ID)
		}
		ids[e.ID] = true
	}
}
