package packs_test

import (
	"testing"

	"github.com/thgrace/training-wheels/internal/packs"
)

// ----------------------------------------------------------------------------
// core.git
// ----------------------------------------------------------------------------

func TestCoreGit_PatternRuleIDs(t *testing.T) {
	p := loadPack(t, "core.git")

	tests := []struct {
		name     string
		cmd      string
		wantName string
		wantSev  packs.Severity
	}{
		{"reset hard", "git reset --hard", "reset-hard", packs.SeverityCritical},
		{"reset hard HEAD", "git reset --hard HEAD", "reset-hard", packs.SeverityCritical},
		{"reset hard HEAD~1", "git reset --hard HEAD~1", "reset-hard", packs.SeverityCritical},
		{"reset merge", "git reset --merge", "reset-merge", packs.SeverityHigh},
		{"clean force -f", "git clean -f", "clean-force", packs.SeverityCritical},
		{"clean force -fd", "git clean -fd", "clean-force", packs.SeverityCritical},
		{"clean force -xf", "git clean -xf", "clean-force", packs.SeverityCritical},
		{"clean force --force", "git clean --force", "clean-force", packs.SeverityCritical},
		{"push force long", "git push --force", "push-force-long", packs.SeverityCritical},
		{"push force short", "git push -f", "push-force-short", packs.SeverityCritical},
		{"push mirror", "git push --mirror", "push-mirror", packs.SeverityCritical},
		{"push force origin main", "git push origin main --force", "push-force-long", packs.SeverityCritical},
		{"checkout discard", "git checkout -- file.txt", "checkout-discard", packs.SeverityHigh},
		{"checkout ref discard", "git checkout HEAD -- file.txt", "checkout-discard", packs.SeverityHigh},
		{"restore all files", "git restore .", "restore-worktree", packs.SeverityHigh},
		{"restore worktree explicit", "git restore --worktree .", "restore-worktree-explicit", packs.SeverityHigh},
		{"branch force delete -D", "git branch -D feature", "branch-force-delete", packs.SeverityHigh},
		{"branch force delete protected main", "git branch -D main", "branch-force-delete-protected", packs.SeverityCritical},
		{"branch force delete protected master", "git branch -D master", "branch-force-delete-protected", packs.SeverityCritical},
		{"branch force delete protected production", "git branch -D production", "branch-force-delete-protected", packs.SeverityCritical},
		{"tag delete short", "git tag -d v1.0", "tag-delete", packs.SeverityMedium},
		{"tag delete long", "git tag --delete v1.1", "tag-delete", packs.SeverityMedium},
		{"stash drop", "git stash drop", "stash-drop", packs.SeverityMedium},
		{"stash clear", "git stash clear", "stash-clear", packs.SeverityCritical},
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

func TestCoreGit_SafePatterns(t *testing.T) {
	p := loadPack(t, "core.git")

	safeCmds := []struct {
		name string
		cmd  string
	}{
		{"checkout -b", "git checkout -b new-branch"},
		{"checkout --orphan", "git checkout --orphan empty"},
		{"restore --staged", "git restore --staged file.txt"},
		{"restore -S", "git restore -S file.txt"},
		{"clean -n", "git clean -n"},
		{"clean -nd", "git clean -nd"},
		{"clean --dry-run", "git clean --dry-run"},
		{"restore specific file", "git restore file.txt"},
		{"restore specific path", "git restore src/main.go"},
	}

	for _, tc := range safeCmds {
		t.Run(tc.name, func(t *testing.T) {
			m := p.Check(tc.cmd)
			if m != nil {
				t.Errorf("Check(%q) should be allowed (nil), but got match: name=%q severity=%s", tc.cmd, m.Name, m.Severity)
			}
		})
	}
}

func TestCoreGit_FalsePositiveBatch(t *testing.T) {
	p := loadPack(t, "core.git")

	allowedCmds := []string{
		"git status",
		"git log",
		"git diff",
		"git add .",
		"git commit -m 'test'",
		"git fetch origin",
		"git pull",
		"git branch -a",
		"git branch -d merged-branch", // lowercase -d is safe delete
		"git stash",
		"git stash list",
		"git stash pop",
		"git stash apply",
		"git remote -v",
		"git tag v1.0",
		"git show HEAD",
		"git rebase main",
		"git merge feature",
		"git cherry-pick abc123",
		"cat .gitignore",
	}

	for _, cmd := range allowedCmds {
		t.Run(cmd, func(t *testing.T) {
			m := p.Check(cmd)
			if m != nil {
				t.Errorf("Check(%q) should be allowed (nil), but got match: name=%q severity=%s", cmd, m.Name, m.Severity)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// core.filesystem
// ----------------------------------------------------------------------------

func TestCoreFilesystem_PatternRuleIDs(t *testing.T) {
	p := loadPack(t, "core.filesystem")

	tests := []struct {
		name     string
		cmd      string
		wantName string
		wantSev  packs.Severity
	}{
		{"rm -rf /", "rm -rf /", "rm-rf-root-home", packs.SeverityCritical},
		{"rm -rf ~", "rm -rf ~", "rm-rf-root-home", packs.SeverityCritical},
		{"rm -rf /home", "rm -rf /home", "rm-rf-root-home", packs.SeverityCritical},
		{"rm -rf /etc", "rm -rf /etc", "rm-rf-root-home", packs.SeverityCritical},
		{"rm -rf $HOME", "rm -rf $HOME", "rm-rf-root-home", packs.SeverityCritical},
		{"rm -rf ${HOME}", "rm -rf ${HOME}", "rm-rf-root-home", packs.SeverityCritical},
		{"rm -rf ./build", "rm -rf ./build", "rm-rf-general", packs.SeverityHigh},
		{"rm -rf src", "rm -rf src", "rm-rf-general", packs.SeverityHigh},
		{"rm -fr /", "rm -fr /", "rm-rf-root-home", packs.SeverityCritical},
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

func TestCoreFilesystem_FlagOrderings(t *testing.T) {
	p := loadPack(t, "core.filesystem")

	cmds := []string{
		"rm -rf /important",
		"rm -fr /important",
		"rm -r -f /important",
		"rm -f -r /important",
		"rm --recursive --force /important",
		"rm --force --recursive /important",
	}

	for _, cmd := range cmds {
		t.Run(cmd, func(t *testing.T) {
			m := p.Check(cmd)
			if m == nil {
				t.Errorf("Check(%q) returned nil; expected command to be blocked", cmd)
			}
		})
	}
}

func TestCoreFilesystem_SafeTempPaths(t *testing.T) {
	p := loadPack(t, "core.filesystem")

	safeCmds := []string{
		"rm -rf /tmp/mydir",
		"rm -rf /tmp/build-output",
		"rm -rf /var/tmp/cache",
		"rm -rf $TMPDIR/test",
		"rm -rf ${TMPDIR}/test",
		"rm -fr /tmp/mydir",
	}

	for _, cmd := range safeCmds {
		t.Run(cmd, func(t *testing.T) {
			m := p.Check(cmd)
			if m != nil {
				t.Errorf("Check(%q) should be allowed (nil), but got match: name=%q severity=%s", cmd, m.Name, m.Severity)
			}
		})
	}
}

func TestCoreFilesystem_PathTraversal(t *testing.T) {
	p := loadPack(t, "core.filesystem")

	cmds := []string{
		"rm -rf /tmp/../etc",
		"rm -rf /tmp/foo/../../etc",
	}

	for _, cmd := range cmds {
		t.Run(cmd, func(t *testing.T) {
			m := p.Check(cmd)
			if m == nil {
				t.Errorf("Check(%q) returned nil; path traversal should be blocked", cmd)
			}
		})
	}
}

// repro_git_bypasses.rs — test long-form flag variants.
func TestCoreGit_LongFlagBypasses(t *testing.T) {
	p := loadPack(t, "core.git")

	tests := []struct {
		name     string
		cmd      string
		wantName string
	}{
		{"git clean --force", "git clean --force", "clean-force"},
		{"git branch --delete --force", "git branch --delete --force feature", "branch-force-delete"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := p.Check(tc.cmd)
			if m == nil {
				t.Fatalf("Check(%q) returned nil; expected match with name %q", tc.cmd, tc.wantName)
			}
			if m.Name != tc.wantName {
				t.Errorf("Check(%q): got Name=%q, want %q", tc.cmd, m.Name, tc.wantName)
			}
		})
	}
}

// TestCoreGit_DashCFlagDoesNotBypass tests that git global flags (-C dir)
// before the subcommand don't prevent detection.
func TestCoreGit_DashCFlagDoesNotBypass(t *testing.T) {
	p := loadPack(t, "core.git")

	tests := []struct {
		name string
		cmd  string
	}{
		{"git -C dir reset --hard", "git -C /tmp/repo reset --hard"},
		{"git --work-tree=dir reset --hard", "git --work-tree=/tmp/repo reset --hard"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := p.Check(tc.cmd)
			if m == nil {
				t.Fatalf("Check(%q) returned nil; expected destructive match", tc.cmd)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// core.tw
// ----------------------------------------------------------------------------

func TestCoreTW_PatternRuleIDs(t *testing.T) {
	p := loadPack(t, "core.tw")

	tests := []struct {
		name     string
		cmd      string
		wantName string
		wantSev  packs.Severity
	}{
		{"tw override add allow session", `tw override add allow --session "rm -rf /"`, "tw-override", packs.SeverityCritical},
		{"tw override add allow time", `tw override add allow --time 30m "git push --force"`, "tw-override", packs.SeverityCritical},
		{"tw override add deny permanent", `tw override add deny --permanent "evil-command" --reason "never"`, "tw-override", packs.SeverityCritical},
		{"tw override add ask rule", `tw override add ask --permanent --rule "core.git:reset-hard" --reason "human review"`, "tw-override", packs.SeverityCritical},
		{"tw override clear", "tw override clear", "tw-override", packs.SeverityCritical},
		{"tw override remove", `tw override remove ov-1234`, "tw-override", packs.SeverityCritical},
		{"tw override bare", "tw override", "tw-override", packs.SeverityCritical},
		{"tw rule remove", "tw rule remove core.git:reset-hard", "tw-rule-remove", packs.SeverityCritical},
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

func TestCoreTW_SafePatterns(t *testing.T) {
	p := loadPack(t, "core.tw")

	safeCmds := []struct {
		name string
		cmd  string
	}{
		{"tw test", `tw test "rm -rf /"`},
		{"tw explain", `tw explain "git reset --hard"`},
		{"tw hook", `tw hook "git push --force"`},
		{"tw packs", "tw packs list"},
		{"tw doctor", "tw doctor"},
		{"tw version", "tw version"},
		{"tw help", "tw help"},
		{"tw --help", "tw --help"},
		{"tw -h", "tw -h"},
		{"tw install", "tw install"},
		{"tw uninstall", "tw uninstall"},
		{"tw config", "tw config"},
		{"tw completions", "tw completions"},
		{"tw override list", "tw override list"},
	}

	for _, tc := range safeCmds {
		t.Run(tc.name, func(t *testing.T) {
			m := p.Check(tc.cmd)
			if m != nil {
				t.Errorf("Check(%q) should be allowed (nil), but got match: name=%q severity=%s", tc.cmd, m.Name, m.Severity)
			}
		})
	}
}

func TestCoreTW_FalsePositiveBatch(t *testing.T) {
	p := loadPack(t, "core.tw")

	allowedCmds := []string{
		`tw test "rm -rf /"`,
		`tw explain "git reset --hard"`,
		"tw packs list",
		"tw doctor",
		"tw version",
		"tw help install",
		"tw --help",
		"tw -h",
		`tw hook "some command"`,
		"tw install",
		"tw config",
		"tw completions bash",
		"tw override list",
	}

	for _, cmd := range allowedCmds {
		t.Run(cmd, func(t *testing.T) {
			m := p.Check(cmd)
			if m != nil {
				t.Errorf("Check(%q) should be allowed (nil), but got match: name=%q severity=%s", cmd, m.Name, m.Severity)
			}
		})
	}
}

func TestCoreTW_OverridesWrite(t *testing.T) {
	p := loadPack(t, "core.tw")

	cmds := []struct {
		name     string
		cmd      string
		wantName string
	}{
		{"redirect to overrides", `echo '{}' > .tw/overrides.json`, "tw-overrides-write-redirect"},
		{"append to overrides", `echo '{}' >> .tw/overrides.json`, "tw-overrides-write-redirect"},
		{"tee to overrides", `cat foo | tee .tw/overrides.json`, "tw-overrides-write"},
		{"redirect to allow.key", `echo 'key' > .tw/allow.key`, "tw-allowkey-write-redirect"},
		{"append to allow.key", `echo 'key' >> .tw/allow.key`, "tw-allowkey-write-redirect"},
		{"tee to allow.key", `cat foo | tee .tw/allow.key`, "tw-allowkey-write"},
		{"redirect to rules.json", `echo '{}' > .tw/rules.json`, "tw-rules-write-redirect"},
		{"tee to rules.json", `cat foo | tee .tw/rules.json`, "tw-rules-write"},
	}

	for _, tc := range cmds {
		t.Run(tc.name, func(t *testing.T) {
			m := p.Check(tc.cmd)
			if m == nil {
				t.Fatalf("Check(%q) returned nil; expected a match with name %q", tc.cmd, tc.wantName)
			}
			if m.Name != tc.wantName {
				t.Errorf("Check(%q): got Name=%q, want %q", tc.cmd, m.Name, tc.wantName)
			}
		})
	}
}

func TestCoreFilesystem_FalsePositiveBatch(t *testing.T) {
	p := loadPack(t, "core.filesystem")

	allowedCmds := []string{
		"ls -la",
		"mkdir -p /tmp/test",
		"cp -r src dest",
		"mv file1 file2",
		"rm file.txt",         // no -rf flags
		"rm -i important.txt", // interactive
	}

	for _, cmd := range allowedCmds {
		t.Run(cmd, func(t *testing.T) {
			m := p.Check(cmd)
			if m != nil {
				t.Errorf("Check(%q) should be allowed (nil), but got match: name=%q severity=%s", cmd, m.Name, m.Severity)
			}
		})
	}
}
