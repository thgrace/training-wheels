package packs_test

import (
	"testing"

	"github.com/thgrace/training-wheels/internal/packs"
)

func TestPlatformGitLab_StructuralPatterns(t *testing.T) {
	p := loadPack(t, "platform.gitlab")

	tests := []struct {
		name     string
		cmd      string
		wantName string
		wantSev  packs.Severity
	}{
		{"glab repo delete", "glab repo delete group/project", "glab-repo-delete", packs.SeverityCritical},
		{"glab repo archive", "glab repo archive group/project", "glab-repo-archive", packs.SeverityCritical},
		{"glab release delete", "glab release delete v1.2.3", "glab-release-delete", packs.SeverityCritical},
		{"glab variable delete", "glab variable delete SECRET_TOKEN", "glab-variable-delete", packs.SeverityCritical},
		{"glab variable remove", "glab variable remove SECRET_TOKEN", "glab-variable-delete", packs.SeverityCritical},
		{"glab api delete project", "glab api -X DELETE projects/123", "glab-api-delete-project", packs.SeverityCritical},
		{"glab api delete release", "glab api -X DELETE releases/123", "glab-api-delete-release", packs.SeverityCritical},
		{"glab api delete variable", "glab api -X DELETE variables/123", "glab-api-delete-variable", packs.SeverityCritical},
		{"glab api delete protected branch", "glab api -X DELETE protected_branches/123", "glab-api-delete-protected-branch", packs.SeverityCritical},
		{"glab api delete hook", "glab api -X DELETE hooks/123", "glab-api-delete-hook", packs.SeverityCritical},
		{"gitlab rails runner destructive", "gitlab-rails runner 'project.destroy_all'", "gitlab-rails-runner-destructive", packs.SeverityCritical},
		{"gitlab rake backup restore", "gitlab-rake backup:restore", "gitlab-rake-backup-restore", packs.SeverityCritical},
		{"gitlab rake db drop", "gitlab-rake db:drop", "gitlab-rake-db-destructive", packs.SeverityCritical},
		{"gitlab rake cleanup", "gitlab-rake cleanup:orphaned_projects", "gitlab-rake-cleanup", packs.SeverityCritical},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := p.Check(tc.cmd)
			if m == nil {
				t.Fatalf("Check(%q) returned nil; expected a match with name %q", tc.cmd, tc.wantName)
			}
			if m.Name != tc.wantName {
				t.Errorf("Check(%q): got Name=%q, want %q", tc.cmd, m.Name, tc.wantName)
			}
			if m.Severity != tc.wantSev {
				t.Errorf("Check(%q): got Severity=%s, want %s", tc.cmd, m.Severity, tc.wantSev)
			}
		})
	}
}

func TestPlatformGitLab_SafePatterns(t *testing.T) {
	p := loadPack(t, "platform.gitlab")

	safeCmds := []string{
		"glab repo view group/project",
		"glab release list",
		"glab api projects/123",
		"glab api -X GET projects/123",
		"gitlab-rails runner 'puts Project.count'",
		"gitlab-rake db:migrate",
		"gitlab-rake assets:precompile",
	}

	for _, cmd := range safeCmds {
		t.Run(cmd, func(t *testing.T) {
			m := p.Check(cmd)
			if m != nil {
				t.Fatalf("Check(%q) should be allowed (nil), but got match: name=%q severity=%s", cmd, m.Name, m.Severity)
			}
		})
	}
}
