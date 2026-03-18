package packs_test

import (
	"testing"
	"time"

	"github.com/thgrace/training-wheels/internal/packs"
)

func TestAllPacks_MetadataValidity(t *testing.T) {
	reg := packs.DefaultRegistry()
	for _, id := range reg.AllIDs() {
		p := reg.Get(id)
		if p == nil {
			t.Errorf("pack %s: Get() returned nil", id)
			continue
		}
		t.Run(id, func(t *testing.T) {
			if p.Name == "" {
				t.Error("empty Name")
			}
			if len(p.Keywords) == 0 {
				t.Error("no keywords")
			}
			if len(p.DestructivePatterns) == 0 {
				t.Error("no destructive patterns")
			}

			// Check uniqueness of pattern names
			names := make(map[string]bool)
			for _, dp := range p.DestructivePatterns {
				if dp.Name == "" {
					t.Error("destructive pattern with empty name")
				}
				if dp.Reason == "" {
					t.Errorf("pattern %q has empty reason", dp.Name)
				}
				if names[dp.Name] {
					t.Errorf("duplicate destructive pattern name: %q", dp.Name)
				}
				names[dp.Name] = true
			}
			for _, sp := range p.SafePatterns {
				if sp.Name == "" {
					t.Error("safe pattern with empty name")
				}
				if names[sp.Name] {
					t.Errorf("pattern name %q used in both safe and destructive", sp.Name)
				}
				names[sp.Name] = true
			}
		})
	}
}

func TestAllPacks_PatternsCompile(t *testing.T) {
	reg := packs.DefaultRegistry()
	for _, id := range reg.AllIDs() {
		p := reg.Get(id)
		if p == nil {
			continue
		}
		t.Run(id, func(t *testing.T) {
			for _, dp := range p.DestructivePatterns {
				// IsMatch will trigger lazy compilation; if it panics or errors, test fails
				_ = dp.Regex.IsMatch("")
			}
			for _, sp := range p.SafePatterns {
				_ = sp.Regex.IsMatch("")
			}
		})
	}
}

func TestAllPacks_BatchSafeCommands(t *testing.T) {
	reg := packs.DefaultRegistry()
	allIDs := reg.AllIDs()

	safeCommands := []string{
		"git status",
		"git log --oneline -10",
		"git diff",
		"git add .",
		"git commit -m 'update readme'",
		"git fetch origin",
		"git pull origin main",
		"git branch -a",
		"ls -la",
		"cat README.md",
		"mkdir -p src/test",
		"cp file1.txt file2.txt",
		"mv old.txt new.txt",
		"echo hello world",
		"pwd",
		"cd /tmp",
		"which git",
		"docker ps",
		"docker images",
		"kubectl get pods",
		"kubectl describe pod my-pod",
		"npm install",
		"npm test",
		"go build ./...",
		"cargo test",
	}

	for _, cmd := range safeCommands {
		t.Run(cmd, func(t *testing.T) {
			for _, id := range allIDs {
				p := reg.Get(id)
				if p == nil {
					continue
				}
				if m := p.Check(cmd); m != nil {
					t.Errorf("safe command %q blocked by pack %s pattern %q: %s",
						cmd, id, m.Name, m.Reason)
				}
			}
		})
	}
}

func TestAllPacks_PerformanceBudget(t *testing.T) {
	reg := packs.DefaultRegistry()

	// Normal command
	normalCmd := "git reset --hard HEAD"

	// Pathological: very long command with repeated spaces
	var longCmd string
	for i := 0; i < 100; i++ {
		longCmd += "a "
	}
	longCmd += "git reset --hard"

	for _, id := range reg.AllIDs() {
		p := reg.Get(id)
		if p == nil {
			continue
		}
		t.Run(id+"/normal", func(t *testing.T) {
			start := time.Now()
			_ = p.Check(normalCmd)
			elapsed := time.Since(start)
			if elapsed > 50*time.Millisecond {
				t.Errorf("pack %s took %v for normal command (budget: 50ms)", id, elapsed)
			}
		})
		t.Run(id+"/pathological", func(t *testing.T) {
			start := time.Now()
			_ = p.Check(longCmd)
			elapsed := time.Since(start)
			if elapsed > 100*time.Millisecond {
				t.Errorf("pack %s took %v for pathological input (budget: 100ms)", id, elapsed)
			}
		})
	}
}

func TestAllPacks_SeverityDistribution(t *testing.T) {
	reg := packs.DefaultRegistry()
	counts := map[packs.Severity]int{}

	for _, id := range reg.AllIDs() {
		p := reg.Get(id)
		if p == nil {
			continue
		}
		for _, dp := range p.DestructivePatterns {
			counts[dp.Severity]++
		}
	}

	if counts[packs.SeverityCritical] == 0 {
		t.Error("no critical severity patterns found across all packs")
	}
	if counts[packs.SeverityHigh] == 0 {
		t.Error("no high severity patterns found across all packs")
	}
	t.Logf("Severity distribution: critical=%d high=%d medium=%d low=%d",
		counts[packs.SeverityCritical], counts[packs.SeverityHigh],
		counts[packs.SeverityMedium], counts[packs.SeverityLow])
}
