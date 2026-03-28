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
			if len(p.StructuralPatterns) == 0 {
				t.Error("no patterns")
			}

			// Check uniqueness of pattern names
			names := make(map[string]bool)
			for _, sp := range p.StructuralPatterns {
				if sp.Name == "" {
					t.Error("structural pattern with empty name")
				}
				if sp.Reason == "" {
					t.Errorf("pattern %q has empty reason", sp.Name)
				}
				if names[sp.Name] {
					t.Errorf("duplicate structural pattern name: %q", sp.Name)
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
			// v2 packs have structural patterns which don't need regex compilation.
			// Just verify they have names.
			for _, sp := range p.StructuralPatterns {
				if sp.Name == "" {
					t.Error("structural pattern with empty name")
				}
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
	normalBudget := 500 * time.Millisecond
	pathologicalBudget := 750 * time.Millisecond

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
			if elapsed > normalBudget {
				t.Errorf("pack %s took %v for normal command (budget: %v)", id, elapsed, normalBudget)
			}
		})
		t.Run(id+"/pathological", func(t *testing.T) {
			start := time.Now()
			_ = p.Check(longCmd)
			elapsed := time.Since(start)
			if elapsed > pathologicalBudget {
				t.Errorf("pack %s took %v for pathological input (budget: %v)", id, elapsed, pathologicalBudget)
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
		for _, sp := range p.StructuralPatterns {
			counts[sp.Severity]++
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
