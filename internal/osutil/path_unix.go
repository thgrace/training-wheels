//go:build !windows

package osutil

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// containsShellMeta reports whether a filesystem path contains characters that
// are special inside shell profile lines. Must be called on the raw absolute
// path BEFORE any shell variable substitution (e.g. $HOME).
func containsShellMeta(path string) bool {
	for _, r := range path {
		switch r {
		case '"', '\'', '`', ';', '|', '&', '<', '>', '(', ')', '{', '}', '!', '\\', '$', '\n', '\r':
			return true
		}
	}
	return false
}

// AddToPath adds a directory to the user's PATH environment variable in their shell profile.
func AddToPath(dir string) (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, fmt.Errorf("getting home directory: %w", err)
	}

	// Expand directory to absolute path if possible.
	absDir, err := filepath.Abs(dir)
	if err == nil {
		dir = absDir
	}

	// Reject paths with shell metacharacters before writing to profile files.
	if containsShellMeta(dir) {
		return false, fmt.Errorf("path %q contains shell metacharacters", dir)
	}

	// Replace home with $HOME for portability in shell files.
	if strings.HasPrefix(dir, home) {
		dir = "$HOME" + dir[len(home):]
	}

	shell := os.Getenv("SHELL")
	var profilePaths []string

	switch {
	case strings.Contains(shell, "zsh"):
		profilePaths = append(profilePaths, filepath.Join(home, ".zshrc"))
	case strings.Contains(shell, "fish"):
		profilePaths = append(profilePaths, filepath.Join(home, ".config", "fish", "config.fish"))
	default:
		// Default to bash/sh common files.
		profilePaths = append(profilePaths,
			filepath.Join(home, ".bashrc"),
			filepath.Join(home, ".bash_profile"),
			filepath.Join(home, ".profile"),
		)
	}

	for _, path := range profilePaths {
		updated, err := updateProfile(path, dir)
		if err == nil && updated {
			return true, nil
		}
	}

	return false, nil
}

func updateProfile(path, dir string) (bool, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// Line to add (POSIX/Zsh style).
	line := fmt.Sprintf(`export PATH="%s:$PATH"`, dir)
	if strings.HasSuffix(path, "config.fish") {
		// Fish style.
		line = fmt.Sprintf(`set -gx PATH %s $PATH`, dir)
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), dir) {
			return false, nil // Already in PATH (at least the directory is mentioned).
		}
	}

	// Append to the end of the file.
	if _, err := f.Seek(0, 2); err != nil {
		return false, err
	}
	if _, err := f.WriteString("\n# Added by Training Wheels\n" + line + "\n"); err != nil {
		return false, err
	}

	return true, nil
}
