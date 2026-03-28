package agentsettings

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectUsesDirectoriesAndBinaries(t *testing.T) {
	home := t.TempDir()

	origLookPath := lookPathFn
	t.Cleanup(func() {
		lookPathFn = origLookPath
	})

	lookPathFn = func(name string) (string, error) {
		switch name {
		case "claude", "gemini":
			return "/tmp/" + name, nil
		default:
			return "", errors.New("not found")
		}
	}

	agents := Detect(home, nil)
	names := []string{agents[0].Name, agents[1].Name}

	if len(agents) != 2 {
		t.Fatalf("got %d agents, want 2", len(agents))
	}
	if names[0] != "claude" || names[1] != "gemini" {
		t.Fatalf("got %v, want [claude gemini]", names)
	}
}

func TestDetectStillUsesAgentDirectories(t *testing.T) {
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	origLookPath := lookPathFn
	t.Cleanup(func() {
		lookPathFn = origLookPath
	})
	lookPathFn = func(name string) (string, error) {
		return "", errors.New("not found")
	}

	agents := Detect(home, nil)
	if len(agents) != 1 || agents[0].Name != "claude" {
		t.Fatalf("got %v, want [claude]", agentNames(agents))
	}
}

func agentNames(agents []Agent) []string {
	names := make([]string, len(agents))
	for i, a := range agents {
		names[i] = a.Name
	}
	return names
}
