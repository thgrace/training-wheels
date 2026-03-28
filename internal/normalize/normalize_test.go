package normalize

import (
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
			var got []string
			for _, tok := range tokens {
				got = append(got, tok.Unquoted)
			}
			if len(got) != len(tt.wantToks) {
				t.Fatalf("tokenize(%q) got %d tokens, want %d:\n  got:  %v\n  want: %v",
					tt.input, len(got), len(tt.wantToks), got, tt.wantToks)
			}
			for i := range tt.wantToks {
				if got[i] != tt.wantToks[i] {
					t.Errorf("token[%d] = %q, want %q", i, got[i], tt.wantToks[i])
				}
			}
		})
	}
}

func TestPreParseCleanup(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"strips CRLF", "hello\r\nworld", "hello\nworld"},
		{"strips bare CR", "hello\rworld", "hello\nworld"},
		{"no change", "hello world", "hello world"},
		{"empty", "", ""},
		{"windows backslashes", `C:\tools\git status`, "C:/tools/git status"},
		{"preserves unix paths", "/usr/bin/git status", "/usr/bin/git status"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PreParseCleanup(tt.input)
			if got != tt.want {
				t.Errorf("PreParseCleanup(%q) = %q, want %q", tt.input, got, tt.want)
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
