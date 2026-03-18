package normalize

import (
	"strings"
	"testing"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		wantToks []string // Unquoted values
	}{
		{"git status", []string{"git", "status"}},
		{"git 'reset --hard'", []string{"git", "reset --hard"}},
		{`git "reset --hard"`, []string{"git", "reset --hard"}},
		{`git re\ set`, []string{"git", "re set"}},
		{"", nil},
		{"  git  status  ", []string{"git", "status"}},
		{`echo "hello \"world\""`, []string{"echo", `hello "world"`}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens := tokenize(tt.input)
			if len(tokens) != len(tt.wantToks) {
				t.Fatalf("got %d tokens, want %d", len(tokens), len(tt.wantToks))
			}
			for i, tok := range tokens {
				if tok.Unquoted != tt.wantToks[i] {
					t.Errorf("token[%d].Unquoted = %q, want %q", i, tok.Unquoted, tt.wantToks[i])
				}
			}
		})
	}
}

func TestStripWrapperPrefixes(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		layers int // expected number of stripped layers
	}{
		{"sudo", "sudo rm -rf /", "rm -rf /", 1},
		{"sudo -u root", "sudo -u root rm -rf /", "rm -rf /", 1},
		{"sudo -EH", "sudo -EH git reset --hard", "git reset --hard", 1},
		{"env VAR=val", "env VAR=val git push", "git push", 1},
		{"env -i -u FOO", "env -i -u FOO git push", "git push", 1},
		{"command", "command git status", "git status", 1},
		{"command -v (no strip)", "command -v git", "command -v git", 0},
		{"backslash", `\rm -rf /`, "rm -rf /", 1},
		{"nested sudo+env", "sudo env VAR=val git push", "git push", 2},
		{"empty", "", "", 0},
		{"just sudo", "sudo", "sudo", 0},
		{"sudo --", "sudo -- rm -rf /", "rm -rf /", 1},

		// Inline code in env assignment value — must NOT strip.
		{"env with $(...) in value", "env RESULT=$(rm -rf /) echo done", "env RESULT=$(rm -rf /) echo done", 0},
		{"env with backtick in value", "env RESULT=`rm -rf /` echo done", "env RESULT=`rm -rf /` echo done", 0},
		{"env X=$(...) multi-assign", "env X=$(date) Y=hello rm -rf /", "env X=$(date) Y=hello rm -rf /", 0},
		{"env safe then dangerous assign", "env SAFE=value DANGER=$(rm -rf /) cmd", "env SAFE=value DANGER=$(rm -rf /) cmd", 0},

		// Safe assignments without inline code — SHOULD still strip normally.
		{"env PATH=... rm", "env PATH=/usr/bin rm -rf /", "rm -rf /", 1},
		{"env FOO=bar rm", "env FOO=bar rm -rf /", "rm -rf /", 1},

		// env -S / --split-string bypass prevention.
		{"env -S double-quoted", `env -S "rm -rf /"`, "rm -rf /", 1},
		{"env -S single-quoted", `env -S 'sudo rm -rf /'`, "rm -rf /", 2},
		{"env --split-string", `env --split-string "git reset --hard"`, "git reset --hard", 1},
		{"env -iS combined", `env -iS "rm -rf /"`, "rm -rf /", 1},
		{"env -S trailing space", `env -S "rm -rf / "`, "rm -rf /", 1},
	// sudo long boolean flags (should NOT consume the next token).
		{"sudo --login", "sudo --login rm -rf /", "rm -rf /", 1},
		{"sudo --preserve-env", "sudo --preserve-env rm -rf /", "rm -rf /", 1},
		{"sudo --non-interactive", "sudo --non-interactive rm -rf /", "rm -rf /", 1},
		{"sudo --set-home", "sudo --set-home rm -rf /", "rm -rf /", 1},
		{"sudo --bell", "sudo --bell rm -rf /", "rm -rf /", 1},
		{"sudo --reset-timestamp", "sudo --reset-timestamp rm -rf /", "rm -rf /", 1},

		// sudo long flags that DO take an argument.
		{"sudo --user root", "sudo --user root rm -rf /", "rm -rf /", 1},
		{"sudo --user=root", "sudo --user=root rm -rf /", "rm -rf /", 1},
		{"sudo --group admin", "sudo --group admin rm -rf /", "rm -rf /", 1},

		// Combined boolean + arg long flags.
		{"sudo --preserve-env --user root", "sudo --preserve-env --user root rm -rf /", "rm -rf /", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripWrapperPrefixes(tt.input)
			if result.Normalized != tt.want {
				t.Errorf("Normalized = %q, want %q", result.Normalized, tt.want)
			}
			if len(result.StrippedWrappers) != tt.layers {
				t.Errorf("stripped %d layers, want %d", len(result.StrippedWrappers), tt.layers)
			}
		})
	}
}

func TestNormalizeCommand(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"absolute path", "/usr/bin/git status", "git status"},
		{"absolute path rm", "/usr/local/bin/rm -rf /tmp/x", "rm -rf /tmp/x"},
		{"relative path unchanged", "./git status", "./git status"},
		{"sudo + absolute", "sudo /usr/bin/git reset --hard", "git reset --hard"},
		{"exe stripping", "git.exe status", "git status"},
		{"no change", "git status", "git status"},
		{"sudo + env + path", "sudo env PATH=/usr/bin /usr/bin/git push --force", "git push --force"},
		{"windows absolute path", `C:\Git\bin\git status`, "git status"},
		{"windows backslash path", `C:\Windows\System32\cmd /c echo hello`, "cmd /c echo hello"},
		{"windows forward slash", "C:/tools/rm -rf /tmp/x", "rm -rf /tmp/x"},
		{"windows exe stripping", `C:\tools\git.exe status`, "git status"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeCommand(PreNormalize(tt.input))
			if got != tt.want {
				t.Errorf("NormalizeCommand(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripHeredocs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"basic heredoc",
			"cat <<EOF\nrm -rf /\nEOF",
			"cat <<EOF\n        \nEOF",
		},
		{
			"quoted delimiter single",
			"cat <<'EOF'\nrm -rf /\nEOF",
			"cat <<'EOF'\n        \nEOF",
		},
		{
			"quoted delimiter double",
			"cat <<\"EOF\"\nrm -rf /\nEOF",
			"cat <<\"EOF\"\n        \nEOF",
		},
		{
			"tab-stripped heredoc",
			"cat <<-EOF\n\trm -rf /\n\tEOF",
			"cat <<-EOF\n         \n\tEOF",
		},
		{
			"here-string unchanged",
			"cat <<<word",
			"cat <<<word",
		},
		{
			"heredoc then real command",
			"cat <<EOF\ndata\nEOF\nrm -rf /",
			"cat <<EOF\n    \nEOF\nrm -rf /",
		},
		{
			"no heredoc passthrough",
			"echo hello world",
			"echo hello world",
		},
		{
			"multiline heredoc body",
			"cat <<EOF\nline1\nline2\nline3\nEOF",
			"cat <<EOF\n     \n     \n     \nEOF",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripHeredocs(tt.input)
			if got != tt.want {
				t.Errorf("stripHeredocs():\n  got  %q\n  want %q", got, tt.want)
			}
		})
	}
}

func TestPreNormalize_HeredocNotTreatedAsCommand(t *testing.T) {
	// After PreNormalize, heredoc body content should be spaces, not commands.
	input := "cat <<EOF\nrm -rf /\nEOF"
	got := PreNormalize(input)
	if strings.Contains(got, "rm -rf") {
		t.Errorf("PreNormalize should strip heredoc body content, got %q", got)
	}
}

func TestPreNormalize_CRLF(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"CRLF line continuation",
			"rm \\\r\n-rf /",
			"rm -rf /",
		},
		{
			"bare CR line continuation",
			"rm \\\r-rf /",
			"rm -rf /",
		},
		{
			"CRLF newline as separator",
			"echo hello\r\nrm -rf /",
			"echo hello ; rm -rf /",
		},
		{
			"bare CR as separator",
			"echo hello\rrm -rf /",
			"echo hello ; rm -rf /",
		},
		{
			"mixed CRLF and LF",
			"echo a\r\necho b\necho c",
			"echo a ; echo b ; echo c",
		},
		{
			"no CR passthrough",
			"echo hello world",
			"echo hello world",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PreNormalize(tt.input)
			if got != tt.want {
				t.Errorf("PreNormalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripCarriageReturns(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello\r\nworld", "hello\nworld"},
		{"hello\rworld", "hello\nworld"},
		{"hello\r\nworld\rfoo\nbar", "hello\nworld\nfoo\nbar"},
		{"no carriage returns", "no carriage returns"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripCarriageReturns(tt.input)
			if got != tt.want {
				t.Errorf("stripCarriageReturns(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
