package packs_test

import (
	"testing"

	"github.com/thgrace/training-wheels/internal/packs"
)

func TestRegistryCount(t *testing.T) {
	reg := packs.DefaultRegistry()
	count := reg.Count()
	if count < 80 {
		t.Fatalf("expected at least 80 registered packs, got %d", count)
	}
	t.Logf("registered %d packs", count)
}

func TestGitPackBlocks(t *testing.T) {
	reg := packs.DefaultRegistry()
	p := reg.Get("core.git")
	if p == nil {
		t.Fatal("core.git pack not found")
	}

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
	reg := packs.DefaultRegistry()
	p := reg.Get("core.filesystem")
	if p == nil {
		t.Fatal("core.filesystem pack not found")
	}

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
		// Verify at least destructive patterns exist (all packs should have some)
		if len(p.DestructivePatterns) == 0 {
			t.Errorf("pack %s has no destructive patterns", id)
		}
	}
	t.Logf("all %d packs loaded successfully", len(ids))
}

func TestCategoryExpansion(t *testing.T) {
	reg := packs.DefaultRegistry()
	expanded := reg.ExpandEnabled([]string{"database"})
	if len(expanded) < 4 {
		t.Fatalf("expected at least 4 database sub-packs, got %d: %v", len(expanded), expanded)
	}
	t.Logf("database category expanded to %d packs: %v", len(expanded), expanded)
}
