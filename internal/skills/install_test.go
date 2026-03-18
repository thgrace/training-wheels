package skills

import (
	"os"
	"path/filepath"
	"testing"

	skillassets "github.com/thgrace/training-wheels/skills"
)

func TestInstallTo(t *testing.T) {
	dir := t.TempDir()
	baseDir := filepath.Join(dir, "training-wheels")

	if err := InstallTo(baseDir); err != nil {
		t.Fatalf("InstallTo failed: %v", err)
	}

	path := filepath.Join(baseDir, SkillFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected SKILL.md to exist: %v", err)
	}

	embedded, err := skillassets.SkillContent()
	if err != nil {
		t.Fatalf("SkillContent failed: %v", err)
	}

	if string(data) != string(embedded) {
		t.Error("installed content does not match embedded content")
	}
}

func TestInstallTo_Overwrites(t *testing.T) {
	dir := t.TempDir()
	baseDir := filepath.Join(dir, "training-wheels")

	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(baseDir, SkillFileName)
	if err := os.WriteFile(path, []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := InstallTo(baseDir); err != nil {
		t.Fatalf("InstallTo failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "old content" {
		t.Error("InstallTo did not overwrite stale content")
	}
}

func TestUninstallFrom(t *testing.T) {
	dir := t.TempDir()
	baseDir := filepath.Join(dir, "training-wheels")

	// Install first.
	if err := InstallTo(baseDir); err != nil {
		t.Fatal(err)
	}

	if err := UninstallFrom(baseDir); err != nil {
		t.Fatalf("UninstallFrom failed: %v", err)
	}

	path := filepath.Join(baseDir, SkillFileName)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected SKILL.md to be removed")
	}

	// Directory should also be removed (it was empty).
	if _, err := os.Stat(baseDir); !os.IsNotExist(err) {
		t.Error("expected empty directory to be cleaned up")
	}
}

func TestUninstallFrom_NotInstalled(t *testing.T) {
	dir := t.TempDir()
	baseDir := filepath.Join(dir, "training-wheels")

	// Should not error when nothing is installed.
	if err := UninstallFrom(baseDir); err != nil {
		t.Fatalf("UninstallFrom on missing dir should not error: %v", err)
	}
}

func TestExistsAt(t *testing.T) {
	dir := t.TempDir()
	baseDir := filepath.Join(dir, "training-wheels")

	// Not installed yet.
	if ExistsAt(baseDir) {
		t.Error("ExistsAt should return false before install")
	}

	if err := InstallTo(baseDir); err != nil {
		t.Fatal(err)
	}

	if !ExistsAt(baseDir) {
		t.Error("ExistsAt should return true after install")
	}

	// Tamper with the file.
	path := filepath.Join(baseDir, SkillFileName)
	if err := os.WriteFile(path, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}

	if ExistsAt(baseDir) {
		t.Error("ExistsAt should return false when content differs")
	}
}
