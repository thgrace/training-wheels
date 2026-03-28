package packs_test

import (
	"testing"

	"github.com/thgrace/training-wheels/internal/packs"
	"github.com/thgrace/training-wheels/internal/packs/allpacks"
)

// loadPack is a helper to load a pack from the registry for testing.
func loadPack(t *testing.T, id string) *packs.Pack {
	t.Helper()
	reg := packs.DefaultRegistry()
	if reg.Count() == 0 {
		allpacks.RegisterAll(reg)
	}
	p := reg.Get(id)
	if p == nil {
		t.Fatalf("pack %q not found in registry", id)
	}
	return p
}

func TestRegistryCount(t *testing.T) {
	reg := packs.DefaultRegistry()
	if reg.Count() == 0 {
		allpacks.RegisterAll(reg)
	}
	count := reg.Count()
	if count < 80 {
		t.Fatalf("expected at least 80 registered packs, got %d", count)
	}
	t.Logf("registered %d packs", count)
}

func TestGitPackBlocks(t *testing.T) {
	p := loadPack(t, "core.git")

	// Should block git reset --hard
	m := p.Check("git reset --hard HEAD")
	if m == nil {
		t.Fatal("expected core.git to block 'git reset --hard HEAD'")
	}
	t.Logf("blocked: %s (severity=%s)", m.Name, m.Severity)

	// Should allow git status (no match)
	m = p.Check("git status")
	if m != nil {
		t.Fatalf("expected 'git status' to be allowed, got match: %s", m.Name)
	}
}

func TestFilesystemPackBlocks(t *testing.T) {
	p := loadPack(t, "core.filesystem")

	// Should block rm -rf /
	m := p.Check("rm -rf /")
	if m == nil {
		t.Fatal("expected core.filesystem to block 'rm -rf /'")
	}
	t.Logf("blocked: %s (severity=%s)", m.Name, m.Severity)

	// Should allow rm -rf /tmp/mydir
	m = p.Check("rm -rf /tmp/mydir")
	if m != nil {
		t.Fatalf("expected 'rm -rf /tmp/mydir' to be allowed, got match: %s", m.Name)
	}
}

func TestAllPacksLoadable(t *testing.T) {
	reg := packs.DefaultRegistry()
	if reg.Count() == 0 {
		allpacks.RegisterAll(reg)
	}
	ids := reg.AllIDs()
	for _, id := range ids {
		p := reg.Get(id)
		if p == nil {
			t.Errorf("pack %s returned nil on Get()", id)
			continue
		}
		if p.Name == "" {
			t.Errorf("pack %s has empty Name", id)
		}
		// Verify at least structural patterns exist (all packs should have some).
		if len(p.StructuralPatterns) == 0 {
			t.Errorf("pack %s has no structural patterns", id)
		}
	}
	t.Logf("all %d packs loaded successfully", len(ids))
}

func TestCategoryExpansion(t *testing.T) {
	reg := packs.DefaultRegistry()
	if reg.Count() == 0 {
		allpacks.RegisterAll(reg)
	}
	expanded := reg.ExpandEnabled([]string{"database"})
	if len(expanded) < 4 {
		t.Fatalf("expected at least 4 database sub-packs, got %d: %v", len(expanded), expanded)
	}
	t.Logf("database category expanded to %d packs: %v", len(expanded), expanded)
}
