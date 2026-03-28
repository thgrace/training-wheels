package packs

import (
	"testing"

	"github.com/thgrace/training-wheels/internal/ast"
)

// ---------------------------------------------------------------------------
// TestPatternCondition_CommandMatch
// ---------------------------------------------------------------------------

func TestPatternCondition_CommandMatch(t *testing.T) {
	tests := []struct {
		name    string
		pc      PatternCondition
		cmd     ast.SimpleCommand
		wantCmd bool // expected result of the command-name check (full Match)
	}{
		{
			name:    "exact match",
			pc:      PatternCondition{Command: []string{"rm"}},
			cmd:     ast.SimpleCommand{Name: "rm"},
			wantCmd: true,
		},
		{
			name:    "case insensitive match",
			pc:      PatternCondition{Command: []string{"rm"}},
			cmd:     ast.SimpleCommand{Name: "rm"},
			wantCmd: true,
		},
		{
			name:    "case insensitive match reverse",
			pc:      PatternCondition{Command: []string{"rm"}},
			cmd:     ast.SimpleCommand{Name: "RM"},
			wantCmd: true,
		},
		{
			name:    "multiple command alternatives",
			pc:      PatternCondition{Command: []string{"rm", "del", "unlink"}},
			cmd:     ast.SimpleCommand{Name: "del"},
			wantCmd: true,
		},
		{
			name:    "no match",
			pc:      PatternCondition{Command: []string{"rm"}},
			cmd:     ast.SimpleCommand{Name: "ls"},
			wantCmd: false,
		},
		{
			name:    "empty command field matches any command",
			pc:      PatternCondition{},
			cmd:     ast.SimpleCommand{Name: "anything"},
			wantCmd: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pc.Match(tt.cmd, false)
			if got != tt.wantCmd {
				t.Errorf("PatternCondition.Match() = %v, want %v", got, tt.wantCmd)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestPatternCondition_SubcommandMatch
// ---------------------------------------------------------------------------

func TestPatternCondition_SubcommandMatch(t *testing.T) {
	tests := []struct {
		name string
		pc   PatternCondition
		cmd  ast.SimpleCommand
		want bool
	}{
		{
			name: "git push",
			pc:   PatternCondition{Command: []string{"git"}, Subcommand: []string{"push"}},
			cmd:  ast.SimpleCommand{Name: "git", Args: []string{"push", "origin", "main"}},
			want: true,
		},
		{
			name: "kubectl delete",
			pc:   PatternCondition{Command: []string{"kubectl"}, Subcommand: []string{"delete"}},
			cmd:  ast.SimpleCommand{Name: "kubectl", Args: []string{"delete", "pod", "my-pod"}},
			want: true,
		},
		{
			name: "docker rm",
			pc:   PatternCondition{Command: []string{"docker"}, Subcommand: []string{"rm", "rmi"}},
			cmd:  ast.SimpleCommand{Name: "docker", Args: []string{"rm", "container1"}},
			want: true,
		},
		{
			name: "docker rmi alternative",
			pc:   PatternCondition{Command: []string{"docker"}, Subcommand: []string{"rm", "rmi"}},
			cmd:  ast.SimpleCommand{Name: "docker", Args: []string{"rmi", "image1"}},
			want: true,
		},
		{
			name: "no args - subcommand check fails",
			pc:   PatternCondition{Command: []string{"git"}, Subcommand: []string{"push"}},
			cmd:  ast.SimpleCommand{Name: "git", Args: []string{}},
			want: false,
		},
		{
			name: "nil args - subcommand check fails",
			pc:   PatternCondition{Command: []string{"git"}, Subcommand: []string{"push"}},
			cmd:  ast.SimpleCommand{Name: "git"},
			want: false,
		},
		{
			name: "wrong subcommand",
			pc:   PatternCondition{Command: []string{"git"}, Subcommand: []string{"push"}},
			cmd:  ast.SimpleCommand{Name: "git", Args: []string{"pull", "origin"}},
			want: false,
		},
		{
			name: "no subcommand field - skip check",
			pc:   PatternCondition{Command: []string{"git"}},
			cmd:  ast.SimpleCommand{Name: "git", Args: []string{"anything"}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pc.Match(tt.cmd, false)
			if got != tt.want {
				t.Errorf("PatternCondition.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestPatternCondition_FlagOR
// ---------------------------------------------------------------------------

func TestPatternCondition_FlagOR(t *testing.T) {
	tests := []struct {
		name string
		pc   PatternCondition
		cmd  ast.SimpleCommand
		want bool
	}{
		{
			name: "matches first flag",
			pc:   PatternCondition{Command: []string{"rm"}, Flag: []string{"-r", "-R", "--recursive"}},
			cmd:  ast.SimpleCommand{Name: "rm", Flags: []string{"-r"}, Args: []string{"/tmp/dir"}},
			want: true,
		},
		{
			name: "matches second flag",
			pc:   PatternCondition{Command: []string{"rm"}, Flag: []string{"-r", "-R", "--recursive"}},
			cmd:  ast.SimpleCommand{Name: "rm", Flags: []string{"-R"}, Args: []string{"/tmp/dir"}},
			want: true,
		},
		{
			name: "matches long flag",
			pc:   PatternCondition{Command: []string{"rm"}, Flag: []string{"-r", "-R", "--recursive"}},
			cmd:  ast.SimpleCommand{Name: "rm", Flags: []string{"--recursive"}, Args: []string{"/tmp/dir"}},
			want: true,
		},
		{
			name: "no matching flag",
			pc:   PatternCondition{Command: []string{"rm"}, Flag: []string{"-r", "-R", "--recursive"}},
			cmd:  ast.SimpleCommand{Name: "rm", Flags: []string{"-f"}, Args: []string{"/tmp/dir"}},
			want: false,
		},
		{
			name: "flag with equals sign matches pattern prefix",
			pc:   PatternCondition{Command: []string{"git"}, Flag: []string{"--force-with-lease"}},
			cmd:  ast.SimpleCommand{Name: "git", Flags: []string{"--force-with-lease=origin"}, Args: []string{"push"}},
			want: true,
		},
		{
			name: "no flags on command",
			pc:   PatternCondition{Command: []string{"rm"}, Flag: []string{"-r"}},
			cmd:  ast.SimpleCommand{Name: "rm", Args: []string{"file.txt"}},
			want: false,
		},
		{
			name: "empty flag field - skip check",
			pc:   PatternCondition{Command: []string{"rm"}},
			cmd:  ast.SimpleCommand{Name: "rm", Args: []string{"file.txt"}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pc.Match(tt.cmd, false)
			if got != tt.want {
				t.Errorf("PatternCondition.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestPatternCondition_AllFlagsAND
// ---------------------------------------------------------------------------

func TestPatternCondition_AllFlagsAND(t *testing.T) {
	tests := []struct {
		name string
		pc   PatternCondition
		cmd  ast.SimpleCommand
		want bool
	}{
		{
			name: "all flags present",
			pc:   PatternCondition{Command: []string{"rm"}, AllFlags: []string{"--no-preserve-root", "-r"}},
			cmd:  ast.SimpleCommand{Name: "rm", Flags: []string{"--no-preserve-root", "-r", "-f"}, Args: []string{"/"}},
			want: true,
		},
		{
			name: "missing one flag",
			pc:   PatternCondition{Command: []string{"rm"}, AllFlags: []string{"--no-preserve-root", "-r"}},
			cmd:  ast.SimpleCommand{Name: "rm", Flags: []string{"-r", "-f"}, Args: []string{"/"}},
			want: false,
		},
		{
			name: "missing all flags",
			pc:   PatternCondition{Command: []string{"rm"}, AllFlags: []string{"--no-preserve-root", "-r"}},
			cmd:  ast.SimpleCommand{Name: "rm", Flags: []string{"-f"}, Args: []string{"/"}},
			want: false,
		},
		{
			name: "no flags on command",
			pc:   PatternCondition{Command: []string{"rm"}, AllFlags: []string{"--no-preserve-root", "-r"}},
			cmd:  ast.SimpleCommand{Name: "rm", Args: []string{"/"}},
			want: false,
		},
		{
			name: "empty all_flags field - skip check",
			pc:   PatternCondition{Command: []string{"rm"}},
			cmd:  ast.SimpleCommand{Name: "rm", Args: []string{"/"}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pc.Match(tt.cmd, false)
			if got != tt.want {
				t.Errorf("PatternCondition.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestPatternCondition_ArgExact
// ---------------------------------------------------------------------------

func TestPatternCondition_ArgExact(t *testing.T) {
	tests := []struct {
		name string
		pc   PatternCondition
		cmd  ast.SimpleCommand
		want bool
	}{
		{
			name: "exact match tilde",
			pc:   PatternCondition{Command: []string{"rm"}, Flag: []string{"-r"}, ArgExact: []string{"~", "$HOME"}},
			cmd:  ast.SimpleCommand{Name: "rm", Flags: []string{"-rf"}, Args: []string{"~"}},
			want: true,
		},
		{
			name: "exact match dollar HOME",
			pc:   PatternCondition{Command: []string{"rm"}, Flag: []string{"-r"}, ArgExact: []string{"~", "$HOME"}},
			cmd:  ast.SimpleCommand{Name: "rm", Flags: []string{"-rf"}, Args: []string{"$HOME"}},
			want: true,
		},
		{
			name: "no exact match",
			pc:   PatternCondition{Command: []string{"rm"}, Flag: []string{"-r"}, ArgExact: []string{"~", "$HOME"}},
			cmd:  ast.SimpleCommand{Name: "rm", Flags: []string{"-rf"}, Args: []string{"/tmp"}},
			want: false,
		},
		{
			name: "empty args on command",
			pc:   PatternCondition{Command: []string{"rm"}, ArgExact: []string{"/"}},
			cmd:  ast.SimpleCommand{Name: "rm", Flags: []string{"-r"}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pc.Match(tt.cmd, false)
			if got != tt.want {
				t.Errorf("PatternCondition.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestPatternCondition_ArgPrefix
// ---------------------------------------------------------------------------

func TestPatternCondition_ArgPrefix(t *testing.T) {
	tests := []struct {
		name string
		pc   PatternCondition
		cmd  ast.SimpleCommand
		want bool
	}{
		{
			name: "arg starts with /",
			pc:   PatternCondition{Command: []string{"rm"}, Flag: []string{"-r"}, ArgPrefix: []string{"/", "~/"}},
			cmd:  ast.SimpleCommand{Name: "rm", Flags: []string{"-rf"}, Args: []string{"/var/data"}},
			want: true,
		},
		{
			name: "arg starts with ~/",
			pc:   PatternCondition{Command: []string{"rm"}, Flag: []string{"-r"}, ArgPrefix: []string{"/", "~/"}},
			cmd:  ast.SimpleCommand{Name: "rm", Flags: []string{"-rf"}, Args: []string{"~/Documents"}},
			want: true,
		},
		{
			name: "arg does not start with prefix",
			pc:   PatternCondition{Command: []string{"rm"}, Flag: []string{"-r"}, ArgPrefix: []string{"/", "~/"}},
			cmd:  ast.SimpleCommand{Name: "rm", Flags: []string{"-rf"}, Args: []string{"relative/path"}},
			want: false,
		},
		{
			name: "multiple args - second matches",
			pc:   PatternCondition{Command: []string{"rm"}, Flag: []string{"-r"}, ArgPrefix: []string{"/"}},
			cmd:  ast.SimpleCommand{Name: "rm", Flags: []string{"-rf"}, Args: []string{"local", "/etc"}},
			want: true,
		},
		{
			name: "no args",
			pc:   PatternCondition{Command: []string{"rm"}, ArgPrefix: []string{"/"}},
			cmd:  ast.SimpleCommand{Name: "rm", Flags: []string{"-r"}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pc.Match(tt.cmd, false)
			if got != tt.want {
				t.Errorf("PatternCondition.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestPatternCondition_ArgContains
// ---------------------------------------------------------------------------

func TestPatternCondition_ArgContains(t *testing.T) {
	tests := []struct {
		name string
		pc   PatternCondition
		cmd  ast.SimpleCommand
		want bool
	}{
		{
			name: "case insensitive contains",
			pc:   PatternCondition{Command: []string{"psql"}, ArgContains: []string{"drop database"}},
			cmd:  ast.SimpleCommand{Name: "psql", Flags: []string{"-c"}, Args: []string{"DROP DATABASE production"}},
			want: true,
		},
		{
			name: "case insensitive - lowercase in arg",
			pc:   PatternCondition{Command: []string{"psql"}, ArgContains: []string{"drop database"}},
			cmd:  ast.SimpleCommand{Name: "psql", Flags: []string{"-c"}, Args: []string{"drop database production"}},
			want: true,
		},
		{
			name: "case insensitive - mixed case",
			pc:   PatternCondition{Command: []string{"psql"}, ArgContains: []string{"drop database"}},
			cmd:  ast.SimpleCommand{Name: "psql", Flags: []string{"-c"}, Args: []string{"DROP DATABASE production"}},
			want: true,
		},
		{
			name: "substring match",
			pc:   PatternCondition{Command: []string{"mysql"}, ArgContains: []string{"drop table"}},
			cmd:  ast.SimpleCommand{Name: "mysql", Flags: []string{"-e"}, Args: []string{"SELECT 1; DROP TABLE users; --"}},
			want: true,
		},
		{
			name: "no match",
			pc:   PatternCondition{Command: []string{"psql"}, ArgContains: []string{"drop database"}},
			cmd:  ast.SimpleCommand{Name: "psql", Flags: []string{"-c"}, Args: []string{"SELECT * FROM users"}},
			want: false,
		},
		{
			name: "no args",
			pc:   PatternCondition{Command: []string{"psql"}, ArgContains: []string{"drop database"}},
			cmd:  ast.SimpleCommand{Name: "psql"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pc.Match(tt.cmd, false)
			if got != tt.want {
				t.Errorf("PatternCondition.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPatternCondition_OutputRedirectContains(t *testing.T) {
	tests := []struct {
		name string
		pc   PatternCondition
		cmd  ast.SimpleCommand
		want bool
	}{
		{
			name: "matches redirect target",
			pc:   PatternCondition{Command: []string{"echo"}, OutputRedirectContains: []string{".tw/overrides.json"}},
			cmd:  ast.SimpleCommand{Name: "echo", OutputRedirects: []string{".tw/overrides.json"}},
			want: true,
		},
		{
			name: "matches redirect target case insensitively",
			pc:   PatternCondition{Command: []string{"echo"}, OutputRedirectContains: []string{".tw/overrides.json"}},
			cmd:  ast.SimpleCommand{Name: "echo", OutputRedirects: []string{".TW/OVERRIDES.JSON"}},
			want: true,
		},
		{
			name: "does not match plain args",
			pc:   PatternCondition{Command: []string{"echo"}, OutputRedirectContains: []string{".tw/overrides.json"}},
			cmd:  ast.SimpleCommand{Name: "echo", Args: []string{".tw/overrides.json"}},
			want: false,
		},
		{
			name: "no redirect target",
			pc:   PatternCondition{Command: []string{"echo"}, OutputRedirectContains: []string{".tw/overrides.json"}},
			cmd:  ast.SimpleCommand{Name: "echo"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pc.Match(tt.cmd, false)
			if got != tt.want {
				t.Errorf("PatternCondition.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestPatternCondition_CombinedConditions
// ---------------------------------------------------------------------------

func TestPatternCondition_CombinedConditions(t *testing.T) {
	tests := []struct {
		name string
		pc   PatternCondition
		cmd  ast.SimpleCommand
		want bool
	}{
		{
			name: "command + flag + arg_prefix all match",
			pc: PatternCondition{
				Command:   []string{"rm"},
				Flag:      []string{"-r", "-R", "--recursive"},
				ArgPrefix: []string{"/"},
			},
			cmd:  ast.SimpleCommand{Name: "rm", Flags: []string{"-rf"}, Args: []string{"/var/data"}},
			want: true,
		},
		{
			name: "command matches but flag does not",
			pc: PatternCondition{
				Command:   []string{"rm"},
				Flag:      []string{"-r", "-R", "--recursive"},
				ArgPrefix: []string{"/"},
			},
			cmd:  ast.SimpleCommand{Name: "rm", Flags: []string{"-f"}, Args: []string{"/var/data"}},
			want: false,
		},
		{
			name: "command + flag match but arg does not",
			pc: PatternCondition{
				Command:   []string{"rm"},
				Flag:      []string{"-r"},
				ArgPrefix: []string{"/"},
			},
			cmd:  ast.SimpleCommand{Name: "rm", Flags: []string{"-r"}, Args: []string{"relative/path"}},
			want: false,
		},
		{
			name: "command + subcommand + flag",
			pc: PatternCondition{
				Command:    []string{"git"},
				Subcommand: []string{"push"},
				Flag:       []string{"--force", "-f"},
			},
			cmd:  ast.SimpleCommand{Name: "git", Flags: []string{"--force"}, Args: []string{"push", "origin", "main"}},
			want: true,
		},
		{
			name: "command + subcommand + flag - wrong subcommand",
			pc: PatternCondition{
				Command:    []string{"git"},
				Subcommand: []string{"push"},
				Flag:       []string{"--force"},
			},
			cmd:  ast.SimpleCommand{Name: "git", Flags: []string{"--force"}, Args: []string{"pull", "origin"}},
			want: false,
		},
		{
			name: "command + all_flags + arg_exact",
			pc: PatternCondition{
				Command:  []string{"rm"},
				AllFlags: []string{"--no-preserve-root", "-r"},
				ArgExact: []string{"/"},
			},
			cmd:  ast.SimpleCommand{Name: "rm", Flags: []string{"--no-preserve-root", "-r"}, Args: []string{"/"}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pc.Match(tt.cmd, false)
			if got != tt.want {
				t.Errorf("PatternCondition.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestPatternCondition_IsEmpty
// ---------------------------------------------------------------------------

func TestPatternCondition_IsEmpty(t *testing.T) {
	tests := []struct {
		name string
		pc   PatternCondition
		want bool
	}{
		{
			name: "zero value is empty",
			pc:   PatternCondition{},
			want: true,
		},
		{
			name: "command set is not empty",
			pc:   PatternCondition{Command: []string{"rm"}},
			want: false,
		},
		{
			name: "subcommand set is not empty",
			pc:   PatternCondition{Subcommand: []string{"push"}},
			want: false,
		},
		{
			name: "flag set is not empty",
			pc:   PatternCondition{Flag: []string{"-r"}},
			want: false,
		},
		{
			name: "all_flags set is not empty",
			pc:   PatternCondition{AllFlags: []string{"-r"}},
			want: false,
		},
		{
			name: "arg_exact set is not empty",
			pc:   PatternCondition{ArgExact: []string{"/"}},
			want: false,
		},
		{
			name: "arg_prefix set is not empty",
			pc:   PatternCondition{ArgPrefix: []string{"/"}},
			want: false,
		},
		{
			name: "arg_contains set is not empty",
			pc:   PatternCondition{ArgContains: []string{"drop"}},
			want: false,
		},
		{
			name: "empty slices are empty",
			pc: PatternCondition{
				Command:     []string{},
				Subcommand:  []string{},
				Flag:        []string{},
				AllFlags:    []string{},
				ArgExact:    []string{},
				ArgPrefix:   []string{},
				ArgContains: []string{},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pc.IsEmpty()
			if got != tt.want {
				t.Errorf("PatternCondition.IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestStructuralPattern_WhenUnless
// ---------------------------------------------------------------------------

func TestStructuralPattern_WhenUnless(t *testing.T) {
	tests := []struct {
		name string
		sp   StructuralPattern
		cmd  ast.SimpleCommand
		want bool
	}{
		{
			name: "when matches, no unless - match",
			sp: StructuralPattern{
				Name: "deny-force-push",
				When: PatternCondition{
					Command:    []string{"git"},
					Subcommand: []string{"push"},
					Flag:       []string{"--force", "-f"},
				},
				Severity: SeverityHigh,
				Reason:   "force push can overwrite remote history",
			},
			cmd:  ast.SimpleCommand{Name: "git", Flags: []string{"--force"}, Args: []string{"push", "origin", "main"}},
			want: true,
		},
		{
			name: "when matches, unless also matches - no match (exempt)",
			sp: StructuralPattern{
				Name: "deny-force-push",
				When: PatternCondition{
					Command:    []string{"git"},
					Subcommand: []string{"push"},
					Flag:       []string{"--force", "-f"},
				},
				Unless: PatternCondition{
					Flag: []string{"--force-with-lease"},
				},
				Severity: SeverityHigh,
				Reason:   "force push can overwrite remote history",
			},
			cmd:  ast.SimpleCommand{Name: "git", Flags: []string{"--force", "--force-with-lease"}, Args: []string{"push", "origin", "main"}},
			want: false,
		},
		{
			name: "when does not match - no match",
			sp: StructuralPattern{
				Name: "deny-force-push",
				When: PatternCondition{
					Command:    []string{"git"},
					Subcommand: []string{"push"},
					Flag:       []string{"--force", "-f"},
				},
				Severity: SeverityHigh,
			},
			cmd:  ast.SimpleCommand{Name: "git", Args: []string{"push", "origin", "main"}},
			want: false,
		},
		{
			name: "unless overriding when - force-with-lease exempts force",
			sp: StructuralPattern{
				Name: "deny-force-push",
				When: PatternCondition{
					Command:    []string{"git"},
					Subcommand: []string{"push"},
					Flag:       []string{"--force", "-f"},
				},
				Unless: PatternCondition{
					Flag: []string{"--force-with-lease"},
				},
				Severity: SeverityHigh,
			},
			cmd:  ast.SimpleCommand{Name: "git", Flags: []string{"--force-with-lease"}, Args: []string{"push", "origin"}},
			want: false,
		},
		{
			name: "unless is empty - does not exempt",
			sp: StructuralPattern{
				Name: "deny-rm-root",
				When: PatternCondition{
					Command:  []string{"rm"},
					Flag:     []string{"-r", "-R", "--recursive"},
					ArgExact: []string{"/"},
				},
				Unless:   PatternCondition{},
				Severity: SeverityCritical,
			},
			cmd:  ast.SimpleCommand{Name: "rm", Flags: []string{"-rf"}, Args: []string{"/"}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sp.MatchCommand(tt.cmd)
			if got != tt.want {
				t.Errorf("StructuralPattern.MatchCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestStructuralPattern_RealWorldPatterns
// ---------------------------------------------------------------------------

func TestStructuralPattern_RealWorldPatterns(t *testing.T) {
	// Pattern: rm -rf / (recursive delete from root)
	rmRootPattern := StructuralPattern{
		Name: "rm-recursive-root",
		When: PatternCondition{
			Command:  []string{"rm"},
			Flag:     []string{"-r", "-R", "--recursive"},
			ArgExact: []string{"/"},
		},
		Reason:   "recursive deletion from filesystem root",
		Severity: SeverityCritical,
		Action:   "deny",
	}

	// Pattern: git push --force (without --force-with-lease exemption)
	gitForcePushPattern := StructuralPattern{
		Name: "git-force-push",
		When: PatternCondition{
			Command:    []string{"git"},
			Subcommand: []string{"push"},
			Flag:       []string{"--force", "-f"},
		},
		Unless: PatternCondition{
			Flag: []string{"--force-with-lease"},
		},
		Reason:   "force push can overwrite remote history",
		Severity: SeverityHigh,
		Action:   "deny",
	}

	// Pattern: psql -c "DROP DATABASE" (sql injection)
	sqlDropPattern := StructuralPattern{
		Name: "sql-drop-database",
		When: PatternCondition{
			Command:     []string{"psql", "mysql", "sqlite3"},
			ArgContains: []string{"drop database", "drop table"},
		},
		Reason:   "destructive SQL statement",
		Severity: SeverityCritical,
		Action:   "deny",
	}

	// Pattern: rm --no-preserve-root -r / (all_flags AND)
	rmNoPreserveRootPattern := StructuralPattern{
		Name: "rm-no-preserve-root",
		When: PatternCondition{
			Command:  []string{"rm"},
			AllFlags: []string{"--no-preserve-root", "-r"},
		},
		Reason:   "rm with --no-preserve-root bypasses safety",
		Severity: SeverityCritical,
		Action:   "deny",
	}

	// Pattern: gh pr create without --draft (enforce draft PRs)
	ghPRNoDraftPattern := StructuralPattern{
		Name: "gh-pr-create-no-draft",
		When: PatternCondition{
			Command:    []string{"gh"},
			Subcommand: []string{"pr"},
			ArgExact:   []string{"create"},
		},
		Unless: PatternCondition{
			Flag: []string{"--draft", "-d"},
		},
		Reason:   "PRs must be created as draft",
		Severity: SeverityMedium,
		Action:   "deny",
	}

	tests := []struct {
		name    string
		pattern StructuralPattern
		cmd     ast.SimpleCommand
		want    bool
	}{
		{
			name:    "rm -rf / matches",
			pattern: rmRootPattern,
			cmd:     ast.SimpleCommand{Name: "rm", Flags: []string{"-rf"}, Args: []string{"/"}, Raw: "rm -rf /"},
			want:    true,
		},
		{
			name:    "rm -rf /tmp does not match (arg is not /)",
			pattern: rmRootPattern,
			cmd:     ast.SimpleCommand{Name: "rm", Flags: []string{"-rf"}, Args: []string{"/tmp"}, Raw: "rm -rf /tmp"},
			want:    false,
		},
		{
			name:    "rm without recursive flag does not match",
			pattern: rmRootPattern,
			cmd:     ast.SimpleCommand{Name: "rm", Flags: []string{"-f"}, Args: []string{"/"}, Raw: "rm -f /"},
			want:    false,
		},
		{
			name:    "git push --force matches",
			pattern: gitForcePushPattern,
			cmd:     ast.SimpleCommand{Name: "git", Flags: []string{"--force"}, Args: []string{"push", "origin", "main"}, Raw: "git push --force origin main"},
			want:    true,
		},
		{
			name:    "git push -f matches",
			pattern: gitForcePushPattern,
			cmd:     ast.SimpleCommand{Name: "git", Flags: []string{"-f"}, Args: []string{"push", "origin", "main"}, Raw: "git push -f origin main"},
			want:    true,
		},
		{
			name:    "git push --force-with-lease does not match (unless exempts)",
			pattern: gitForcePushPattern,
			cmd:     ast.SimpleCommand{Name: "git", Flags: []string{"--force-with-lease"}, Args: []string{"push", "origin", "main"}, Raw: "git push --force-with-lease origin main"},
			want:    false,
		},
		{
			name:    "git push --force --force-with-lease does not match (unless exempts)",
			pattern: gitForcePushPattern,
			cmd:     ast.SimpleCommand{Name: "git", Flags: []string{"--force", "--force-with-lease"}, Args: []string{"push", "origin", "main"}, Raw: "git push --force --force-with-lease origin main"},
			want:    false,
		},
		{
			name:    "git push without force does not match",
			pattern: gitForcePushPattern,
			cmd:     ast.SimpleCommand{Name: "git", Args: []string{"push", "origin", "main"}, Raw: "git push origin main"},
			want:    false,
		},
		{
			name:    "psql DROP DATABASE matches",
			pattern: sqlDropPattern,
			cmd:     ast.SimpleCommand{Name: "psql", Flags: []string{"-c"}, Args: []string{"DROP DATABASE production"}, Raw: `psql -c "DROP DATABASE production"`},
			want:    true,
		},
		{
			name:    "mysql DROP TABLE matches",
			pattern: sqlDropPattern,
			cmd:     ast.SimpleCommand{Name: "mysql", Flags: []string{"-e"}, Args: []string{"DROP TABLE users"}, Raw: `mysql -e "DROP TABLE users"`},
			want:    true,
		},
		{
			name:    "psql SELECT does not match",
			pattern: sqlDropPattern,
			cmd:     ast.SimpleCommand{Name: "psql", Flags: []string{"-c"}, Args: []string{"SELECT * FROM users"}, Raw: `psql -c "SELECT * FROM users"`},
			want:    false,
		},
		{
			name:    "rm --no-preserve-root -r / matches",
			pattern: rmNoPreserveRootPattern,
			cmd:     ast.SimpleCommand{Name: "rm", Flags: []string{"--no-preserve-root", "-r"}, Args: []string{"/"}, Raw: "rm --no-preserve-root -r /"},
			want:    true,
		},
		{
			name:    "rm --no-preserve-root without -r does not match",
			pattern: rmNoPreserveRootPattern,
			cmd:     ast.SimpleCommand{Name: "rm", Flags: []string{"--no-preserve-root"}, Args: []string{"/"}, Raw: "rm --no-preserve-root /"},
			want:    false,
		},
		{
			name:    "rm -r without --no-preserve-root does not match",
			pattern: rmNoPreserveRootPattern,
			cmd:     ast.SimpleCommand{Name: "rm", Flags: []string{"-r"}, Args: []string{"/"}, Raw: "rm -r /"},
			want:    false,
		},
		// gh pr create --draft enforcement
		{
			name:    "gh pr create without --draft is blocked",
			pattern: ghPRNoDraftPattern,
			cmd:     ast.SimpleCommand{Name: "gh", Subcommand: "pr", Flags: []string{"--assignee"}, Args: []string{"create", "@me"}},
			want:    true,
		},
		{
			name:    "gh pr create --draft is allowed (unless exempts)",
			pattern: ghPRNoDraftPattern,
			cmd:     ast.SimpleCommand{Name: "gh", Subcommand: "pr", Flags: []string{"--draft", "--assignee"}, Args: []string{"create", "@me"}},
			want:    false,
		},
		{
			name:    "gh pr create -d is allowed (short flag)",
			pattern: ghPRNoDraftPattern,
			cmd:     ast.SimpleCommand{Name: "gh", Subcommand: "pr", Flags: []string{"-d", "--assignee"}, Args: []string{"create", "@me"}},
			want:    false,
		},
		{
			name:    "gh pr list does not match (not create)",
			pattern: ghPRNoDraftPattern,
			cmd:     ast.SimpleCommand{Name: "gh", Subcommand: "pr", Args: []string{"list"}},
			want:    false,
		},
		{
			name:    "gh issue create does not match (not pr subcommand)",
			pattern: ghPRNoDraftPattern,
			cmd:     ast.SimpleCommand{Name: "gh", Subcommand: "issue", Args: []string{"create"}},
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pattern.MatchCommand(tt.cmd)
			if got != tt.want {
				t.Errorf("StructuralPattern.MatchCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGHPRDraftEnforcement_EndToEnd tests the full pipeline: parse → enrich → match.
// This verifies that a real command string is correctly parsed, the gh enricher
// extracts the subcommand, and the pattern match works end-to-end.
func TestGHPRDraftEnforcement_EndToEnd(t *testing.T) {
	pattern := StructuralPattern{
		Name: "gh-pr-create-no-draft",
		When: PatternCondition{
			Command:    []string{"gh"},
			Subcommand: []string{"pr"},
			ArgExact:   []string{"create"},
		},
		Unless: PatternCondition{
			Flag: []string{"--draft", "-d"},
		},
		Reason:   "PRs must be created as draft",
		Severity: SeverityMedium,
		Action:   "deny",
	}

	tests := []struct {
		name    string
		cmdLine string
		want    bool // true = blocked (pattern matches)
	}{
		{
			name:    "gh pr create --draft --assignee @me (allowed)",
			cmdLine: "gh pr create --draft --assignee @me",
			want:    false,
		},
		{
			name:    "gh pr create --assignee @me (blocked - no draft)",
			cmdLine: "gh pr create --assignee @me",
			want:    true,
		},
		{
			name:    "gh pr create -d --title test (allowed - short flag)",
			cmdLine: `gh pr create -d --title "my feature"`,
			want:    false,
		},
		{
			name:    "gh pr create (blocked - bare create)",
			cmdLine: "gh pr create",
			want:    true,
		},
		{
			name:    "gh -R owner/repo pr create --assignee @me (blocked - global flag before subcommand)",
			cmdLine: "gh -R owner/repo pr create --assignee @me",
			want:    true,
		},
		{
			name:    "gh -R owner/repo pr create --draft (allowed - global flag + draft)",
			cmdLine: "gh -R owner/repo pr create --draft",
			want:    false,
		},
		{
			name:    "gh pr list (allowed - not create)",
			cmdLine: "gh pr list",
			want:    false,
		},
		{
			name:    "gh issue create (allowed - not pr subcommand)",
			cmdLine: "gh issue create --title test",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := ast.Parse([]byte(tt.cmdLine))
			if cc == nil {
				t.Fatalf("Parse(%q) returned nil", tt.cmdLine)
			}
			cc = ast.Unwrap(cc, ast.ShellBash)
			cmds := cc.AllCommands()
			ast.EnrichCommands(cmds)

			matched := false
			for _, cmd := range cmds {
				if pattern.MatchCommand(cmd) {
					matched = true
					break
				}
			}
			if matched != tt.want {
				t.Errorf("command %q: matched=%v, want=%v (cmds=%+v)",
					tt.cmdLine, matched, tt.want, cmds)
			}
		})
	}
}
