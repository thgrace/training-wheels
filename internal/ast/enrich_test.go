package ast

import (
	"testing"
)

func TestEnrichGit(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantSub       string
		wantArgsCount int // remaining args after subcommand extraction
	}{
		{
			name:          "simple git reset",
			input:         "git reset --hard",
			wantSub:       "reset",
			wantArgsCount: 0, // subcommand removed, --hard is a flag
		},
		{
			name:          "git -C dir reset --hard",
			input:         "git -C dir reset --hard",
			wantSub:       "reset",
			wantArgsCount: 1, // "dir" stays as flag-arg for -C; subcommand removed
		},
		{
			name:          "git --git-dir=/path reset --hard (= syntax, no skip)",
			input:         "git --git-dir=/path reset --hard",
			wantSub:       "reset",
			wantArgsCount: 0, // --git-dir=/path uses = syntax so no arg consumed from Args; subcommand removed
		},
		{
			name:          "git -c key=val push",
			input:         "git -c user.name=test push --force",
			wantSub:       "push",
			wantArgsCount: 1, // "user.name=test" stays as flag-arg for -c; subcommand removed
		},
		{
			name:          "git -C dir -c key=val checkout main",
			input:         "git -C myrepo -c core.autocrlf=true checkout main",
			wantSub:       "checkout",
			wantArgsCount: 3, // "myrepo" and "core.autocrlf=true" stay; "main" stays; subcommand removed
		},
		{
			name:          "git status (no global flags)",
			input:         "git status",
			wantSub:       "status",
			wantArgsCount: 0, // subcommand removed, nothing else in Args
		},
		{
			name:          "git with no args",
			input:         "git",
			wantSub:       "",
			wantArgsCount: 0,
		},
		{
			name:          "git --work-tree dir log",
			input:         "git --work-tree /home/user/repo log --oneline",
			wantSub:       "log",
			wantArgsCount: 1, // "/home/user/repo" stays as flag-arg for --work-tree; subcommand removed
		},
		{
			name:          "git --exec-path status (exec-path does not consume next arg)",
			input:         "git --exec-path status",
			wantSub:       "status",
			wantArgsCount: 0, // --exec-path not in globalFlagsWithArg, subcommand removed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := Parse([]byte(tt.input))
			if cc == nil {
				t.Fatal("Parse returned nil")
			}
			cc = Unwrap(cc, ShellBash)
			cmds := cc.AllCommands()
			EnrichCommands(cmds)

			if len(cmds) == 0 {
				if tt.wantSub != "" {
					t.Fatalf("no commands, wanted subcommand %q", tt.wantSub)
				}
				return
			}

			got := cmds[0].Subcommand
			if got != tt.wantSub {
				t.Errorf("Subcommand = %q, want %q (Flags=%v Args=%v)",
					got, tt.wantSub, cmds[0].Flags, cmds[0].Args)
			}

			// Verify subcommand is NOT in Args.
			if tt.wantSub != "" {
				for _, a := range cmds[0].Args {
					if a == tt.wantSub {
						t.Errorf("subcommand %q should not remain in Args: %v",
							tt.wantSub, cmds[0].Args)
					}
				}
			}

			// Verify the remaining Args count matches expectations.
			if gotCount := len(cmds[0].Args); gotCount != tt.wantArgsCount {
				t.Errorf("len(Args) = %d, want %d (Args=%v)",
					gotCount, tt.wantArgsCount, cmds[0].Args)
			}
		})
	}
}

func TestEnrichDocker(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantSub string
	}{
		{"docker rm", "docker rm container1", "rm"},
		{"docker --host h rm", "docker --host tcp://host rm container1", "rm"},
		{"docker ps (no global flags)", "docker ps", "ps"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := Parse([]byte(tt.input))
			if cc == nil {
				t.Fatal("Parse returned nil")
			}
			cmds := cc.AllCommands()
			EnrichCommands(cmds)

			if cmds[0].Subcommand != tt.wantSub {
				t.Errorf("Subcommand = %q, want %q", cmds[0].Subcommand, tt.wantSub)
			}
		})
	}
}

func TestEnrichKubectl(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantSub string
	}{
		{"kubectl delete", "kubectl delete pod foo", "delete"},
		{"kubectl -n ns delete", "kubectl -n production delete pod foo", "delete"},
		{"kubectl --namespace ns delete", "kubectl --namespace kube-system delete deployment bar", "delete"},
		{"kubectl get (no flags)", "kubectl get pods", "get"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := Parse([]byte(tt.input))
			if cc == nil {
				t.Fatal("Parse returned nil")
			}
			cmds := cc.AllCommands()
			EnrichCommands(cmds)

			if cmds[0].Subcommand != tt.wantSub {
				t.Errorf("Subcommand = %q, want %q (Args=%v)", cmds[0].Subcommand, tt.wantSub, cmds[0].Args)
			}
		})
	}
}

func TestEnrichHelm(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantSub string
	}{
		{"helm install", "helm install my-release ./chart", "install"},
		{"helm -n ns upgrade", "helm -n production upgrade my-release ./chart", "upgrade"},
		{"helm --namespace ns delete", "helm --namespace default delete my-release", "delete"},
		{"helm --kube-context ctx list", "helm --kube-context staging list", "list"},
		{"helm repo add", "helm repo add stable https://charts.helm.sh/stable", "repo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := Parse([]byte(tt.input))
			if cc == nil {
				t.Fatal("Parse returned nil")
			}
			cmds := cc.AllCommands()
			EnrichCommands(cmds)

			if cmds[0].Subcommand != tt.wantSub {
				t.Errorf("Subcommand = %q, want %q (Args=%v)", cmds[0].Subcommand, tt.wantSub, cmds[0].Args)
			}
		})
	}
}

func TestEnrichRestic(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantSub string
	}{
		{"restic backup", "restic backup /home", "backup"},
		{"restic -r repo backup", "restic -r /mnt/backup backup /home", "backup"},
		{"restic --repo repo snapshots", "restic --repo s3:bucket snapshots", "snapshots"},
		{"restic forget", "restic forget --keep-last 10", "forget"},
		{"restic --password-file f prune", "restic --password-file /etc/restic.pw prune", "prune"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := Parse([]byte(tt.input))
			if cc == nil {
				t.Fatal("Parse returned nil")
			}
			cmds := cc.AllCommands()
			EnrichCommands(cmds)

			if cmds[0].Subcommand != tt.wantSub {
				t.Errorf("Subcommand = %q, want %q (Args=%v)", cmds[0].Subcommand, tt.wantSub, cmds[0].Args)
			}
		})
	}
}

func TestEnrichGh(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantSub string
	}{
		{"gh pr create", "gh pr create --title test", "pr"},
		{"gh -R owner/repo issue list", "gh -R owner/repo issue list", "issue"},
		{"gh --repo owner/repo run list", "gh --repo owner/repo run list", "run"},
		{"gh auth login", "gh auth login", "auth"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := Parse([]byte(tt.input))
			if cc == nil {
				t.Fatal("Parse returned nil")
			}
			cmds := cc.AllCommands()
			EnrichCommands(cmds)

			if cmds[0].Subcommand != tt.wantSub {
				t.Errorf("Subcommand = %q, want %q (Args=%v)", cmds[0].Subcommand, tt.wantSub, cmds[0].Args)
			}
		})
	}
}

func TestEnrichTerraform(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantSub string
	}{
		{"terraform apply", "terraform apply", "apply"},
		{"terraform -chdir=path apply", "terraform -chdir=/infra apply", "apply"},
		{"terraform -chdir path plan", "terraform -chdir /infra plan", "plan"},
		{"terraform init", "terraform init", "init"},
		{"tofu apply", "tofu apply", "apply"},
		{"tofu -chdir=path destroy", "tofu -chdir=/infra destroy", "destroy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := Parse([]byte(tt.input))
			if cc == nil {
				t.Fatal("Parse returned nil")
			}
			cmds := cc.AllCommands()
			EnrichCommands(cmds)

			if cmds[0].Subcommand != tt.wantSub {
				t.Errorf("Subcommand = %q, want %q (Args=%v Flags=%v)",
					cmds[0].Subcommand, tt.wantSub, cmds[0].Args, cmds[0].Flags)
			}
		})
	}
}

func TestEnrichUnknownCommand(t *testing.T) {
	// Unknown commands should not be enriched — Subcommand stays empty.
	cc := Parse([]byte("mycli --flag arg1 arg2"))
	if cc == nil {
		t.Fatal("Parse returned nil")
	}
	cmds := cc.AllCommands()
	EnrichCommands(cmds)

	if cmds[0].Subcommand != "" {
		t.Errorf("unknown command should not have Subcommand, got %q", cmds[0].Subcommand)
	}
}

func TestEnrichPreservesOtherFields(t *testing.T) {
	cc := Parse([]byte("git -C myrepo reset --hard HEAD"))
	if cc == nil {
		t.Fatal("Parse returned nil")
	}
	cc = Unwrap(cc, ShellBash)
	cmds := cc.AllCommands()
	EnrichCommands(cmds)

	cmd := cmds[0]
	if cmd.Name != "git" {
		t.Errorf("Name = %q, want %q", cmd.Name, "git")
	}
	if cmd.Subcommand != "reset" {
		t.Errorf("Subcommand = %q, want %q", cmd.Subcommand, "reset")
	}
	// --hard should still be in Flags.
	hasHard := false
	for _, f := range cmd.Flags {
		if f == "--hard" {
			hasHard = true
		}
	}
	if !hasHard {
		t.Errorf("expected --hard in Flags, got %v", cmd.Flags)
	}
	// HEAD should be in Args.
	hasHead := false
	for _, a := range cmd.Args {
		if a == "HEAD" {
			hasHead = true
		}
	}
	if !hasHead {
		t.Errorf("expected HEAD in Args, got %v", cmd.Args)
	}
}
