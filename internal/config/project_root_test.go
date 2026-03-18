package config

import (
	"os"
	"path/filepath"
	"testing"
)

// chdir changes the working directory and registers a cleanup to restore it.
// Must not be used in parallel tests.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
}

// resolveSymlinks resolves symlinks in a path (needed on macOS where /var is a
// symlink to /private/var, causing os.Getwd to return a different prefix than
// t.TempDir).
func resolveSymlinks(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestFindProjectRoot_ParentHasGit(t *testing.T) {
	parent := resolveSymlinks(t, t.TempDir())
	if err := os.Mkdir(filepath.Join(parent, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(parent, "subdir")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	chdir(t, child)

	root, ok := FindProjectRoot()
	if !ok {
		t.Fatal("FindProjectRoot returned false, want true")
	}
	if root != parent {
		t.Errorf("FindProjectRoot = %q, want %q", root, parent)
	}
}

func TestFindProjectRoot_NoRootFound(t *testing.T) {
	dir := resolveSymlinks(t, t.TempDir())
	nested := filepath.Join(dir, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	chdir(t, nested)

	// Walk from nested up: none of dir, a, b, c have .git.
	// However, an ancestor beyond dir (e.g. /tmp, /home) might have .git
	// in the real filesystem, so we only assert that nothing within the
	// temp tree is returned.
	root, ok := FindProjectRoot()
	if ok && (root == nested || root == filepath.Join(dir, "a", "b") ||
		root == filepath.Join(dir, "a") || root == dir) {
		t.Errorf("FindProjectRoot should not find root in temp tree, got %q", root)
	}
	// If nothing found at all, that's the ideal case.
	if !ok && root != "" {
		t.Errorf("FindProjectRoot returned root=%q with ok=false, want empty string", root)
	}
}

func TestProjectPackDir_AbsoluteWhenRootExists(t *testing.T) {
	dir := resolveSymlinks(t, t.TempDir())
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	chdir(t, dir)

	packDir := ProjectPackDir()
	if !filepath.IsAbs(packDir) {
		t.Errorf("ProjectPackDir = %q, want absolute path", packDir)
	}
	want := filepath.Join(dir, ".tw", "packs")
	if packDir != want {
		t.Errorf("ProjectPackDir = %q, want %q", packDir, want)
	}
}

func TestProjectPackDir_AbsoluteFromSubdirectory(t *testing.T) {
	parent := resolveSymlinks(t, t.TempDir())
	if err := os.Mkdir(filepath.Join(parent, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(parent, "subdir")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	chdir(t, child)

	packDir := ProjectPackDir()
	if !filepath.IsAbs(packDir) {
		t.Errorf("ProjectPackDir = %q, want absolute path", packDir)
	}
	want := filepath.Join(parent, ".tw", "packs")
	if packDir != want {
		t.Errorf("ProjectPackDir = %q, want %q", packDir, want)
	}
}
