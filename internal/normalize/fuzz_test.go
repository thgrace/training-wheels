package normalize

import (
	"testing"
)

func FuzzNormalizeCommand(f *testing.F) {
	// Seed corpus
	seeds := []string{
		"git status",
		"sudo rm -rf /",
		"sudo -u root git reset --hard",
		"env VAR=val git push --force",
		`\rm -rf /`,
		"/usr/bin/git reset --hard",
		"git.exe status",
		"",
		"sudo",
		"env",
		"command git status",
		"sudo env VAR=val /usr/bin/git reset --hard",
		"nice rm -rf /",
		`sudo -EH -u deploy env PATH=/usr/bin git push`,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 10_000 {
			return
		}

		result := NormalizeCommand(PreNormalize(input))

		// Property 1: Idempotence
		again := NormalizeCommand(PreNormalize(result))
		if again != result {
			t.Errorf("not idempotent:\n  input:      %q\n  normalize1: %q\n  normalize2: %q",
				input, result, again)
		}

		// Property 2: Monotone shrinkage
		if len(result) > len(input) {
			t.Errorf("normalization grew: len(%q)=%d > len(%q)=%d",
				result, len(result), input, len(input))
		}
	})
}

func FuzzStripWrapperPrefixes(f *testing.F) {
	seeds := []string{
		"sudo rm -rf /",
		"env VAR=val command",
		`\git status`,
		"sudo -u root env -i git push",
		"",
		"sudo",
		"sudo sudo sudo git status",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 10_000 {
			return
		}

		result := StripWrapperPrefixes(input)

		// The normalized result should not be longer than the input
		if len(result.Normalized) > len(input) {
			t.Errorf("wrapper stripping grew: len(%q)=%d > len(%q)=%d",
				result.Normalized, len(result.Normalized), input, len(input))
		}

		// Idempotence: stripping again should be a no-op
		again := StripWrapperPrefixes(result.Normalized)
		if again.Normalized != result.Normalized {
			t.Errorf("not idempotent:\n  input:  %q\n  strip1: %q\n  strip2: %q",
				input, result.Normalized, again.Normalized)
		}
	})
}

func FuzzTokenize(f *testing.F) {
	seeds := []string{
		"git status",
		`git "reset --hard"`,
		`echo 'hello world'`,
		`git re\ set`,
		"",
		`"unterminated`,
		`'also unterminated`,
		`a\ b\ c`,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 10_000 {
			return
		}

		tokens := tokenize(input)

		// Property: All token spans must be within input bounds
		for i, tok := range tokens {
			if tok.Start < 0 || tok.Start > len(input) {
				t.Errorf("token[%d].Start=%d out of bounds (len=%d)", i, tok.Start, len(input))
			}
			if tok.End < tok.Start || tok.End > len(input) {
				t.Errorf("token[%d] span invalid: Start=%d End=%d (len=%d)", i, tok.Start, tok.End, len(input))
			}
		}

		// Property: Tokens should not overlap and should be ordered
		for i := 1; i < len(tokens); i++ {
			if tokens[i].Start < tokens[i-1].End {
				t.Errorf("token[%d] overlaps token[%d]: prev.End=%d, cur.Start=%d",
					i, i-1, tokens[i-1].End, tokens[i].Start)
			}
		}
	})
}
