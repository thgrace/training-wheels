package shellcontext

import (
	"testing"
)

func FuzzClassify(f *testing.F) {
	seeds := []string{
		"git status",
		"echo hello | grep world",
		`bash -c "rm -rf /"`,
		"ls # comment",
		`echo "$(echo inner)"`,
		"",
		"a && b || c ; d",
		`echo 'single quoted'`,
		`echo "double quoted"`,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 10_000 {
			return
		}

		spans := Classify(input, nil)

		// Property: All span bounds within input
		for i, sp := range spans {
			if sp.Start < 0 || sp.Start > len(input) {
				t.Errorf("span[%d].Start=%d out of bounds (len=%d)", i, sp.Start, len(input))
			}
			if sp.End < sp.Start {
				t.Errorf("span[%d] invalid: Start=%d > End=%d", i, sp.Start, sp.End)
			}
			if sp.End > len(input) {
				t.Errorf("span[%d].End=%d out of bounds (len=%d)", i, sp.End, len(input))
			}
		}

		// Property: Spans should not overlap
		for i := 1; i < len(spans); i++ {
			if spans[i].Start < spans[i-1].End {
				t.Errorf("span[%d] overlaps span[%d]: prev.End=%d, cur.Start=%d",
					i, i-1, spans[i-1].End, spans[i].Start)
			}
		}

		// Property: SpanKind should be a valid value
		for i, sp := range spans {
			switch sp.Kind {
			case SpanExecuted, SpanArgument, SpanInlineCode, SpanData, SpanComment, SpanUnknown:
				// ok
			default:
				t.Errorf("span[%d] has invalid Kind: %d", i, sp.Kind)
			}
		}
	})
}

func FuzzSanitize(f *testing.F) {
	seeds := []string{
		`git commit -m "rm -rf /"`,
		`echo rm -rf /`,
		`grep "DROP TABLE" file`,
		`bash -c "rm -rf /"`,
		"",
		"git status",
		"rm -rf /",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 10_000 {
			return
		}

		result := Sanitize(input, nil)

		// Property: Length preservation
		if len(result) != len(input) {
			t.Errorf("length changed: input=%d result=%d", len(input), len(result))
		}
	})
}
