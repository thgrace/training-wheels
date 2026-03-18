package shellcontext

// Sanitize replaces known-safe data regions in cmd with spaces.
// If shell is nil, it uses the default shell for the platform.
func Sanitize(cmd string, shell Shell) string {
	if shell == nil {
		shell = DefaultShell()
	}
	spans := Classify(cmd, shell)
	if len(spans) == 0 {
		return cmd
	}

	masks := computeMasks(cmd, spans, shell)
	if len(masks) == 0 {
		return cmd
	}

	out := []byte(cmd)
	for _, m := range masks {
		for i := m.Start; i < m.End && i < len(out); i++ {
			out[i] = ' '
		}
	}
	return string(out)
}

// maskRange is a byte range to replace with spaces.
type maskRange struct {
	Start, End int
}

type tokenInfo struct {
	span Span
	text string
	unq  string // unquoted text for identification
}

// computeMasks walks the classified spans and determines which ranges to mask.
func computeMasks(cmd string, spans []Span, shell Shell) []maskRange {
	var masks []maskRange

	// Collect non-whitespace tokens with their span info.
	var tokens []tokenInfo
	for _, sp := range spans {
		text := cmd[sp.Start:sp.End]
		if isAllSpace(text) {
			if sp.Kind == SpanComment || sp.Kind == SpanData {
				masks = append(masks, maskRange{sp.Start, sp.End})
			}
			continue
		}
		tokens = append(tokens, tokenInfo{
			span: sp,
			text: text,
			unq:  unquoteSimple(text, shell),
		})
	}

	i := 0
	for i < len(tokens) {
		var cmdName string
		segStart := i

		segEnd := i
		for segEnd < len(tokens) {
			t := tokens[segEnd]
			if isOperatorToken(t.text) {
				break
			}
			if t.span.Kind == SpanExecuted && cmdName == "" {
				cmdName = t.unq
			}
			segEnd++
		}

		entry := LookupSafeCommand(cmdName)
		if entry != nil {
			masks = append(masks, maskSegment(tokens[segStart:segEnd], entry, cmdName)...)
		}

		for j := segStart; j < segEnd; j++ {
			sp := tokens[j].span
			if sp.Kind == SpanData || sp.Kind == SpanComment {
				masks = append(masks, maskRange{sp.Start, sp.End})
			}
		}

		if segEnd < len(tokens) {
			segEnd++
		}
		i = segEnd
	}

	return dedup(masks)
}

// maskSegment applies safe-registry masking to a segment's tokens.
func maskSegment(tokens []tokenInfo, entry *SafeCommandEntry, cmdName string) []maskRange {
	var masks []maskRange

	switch entry.Mode {
	case SafeArgAll:
		for _, t := range tokens {
			if t.span.Kind == SpanArgument {
				masks = append(masks, maskRange{t.span.Start, t.span.End})
			}
		}

	case SafeArgFlags:
		maskNext := false
		for _, t := range tokens {
			if t.span.Kind == SpanExecuted {
				continue
			}
			if maskNext && t.span.Kind == SpanArgument {
				masks = append(masks, maskRange{t.span.Start, t.span.End})
				maskNext = false
				continue
			}
			maskNext = false
			if t.span.Kind == SpanArgument && IsSafeFlag(cmdName, t.unq) {
				maskNext = true
			}
		}
	}

	return masks
}

func isOperatorToken(s string) bool {
	return s == "|" || s == "||" || s == "&&" || s == ";" || s == "&"
}

func isAllSpace(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isSpace(s[i]) {
			return false
		}
	}
	return true
}

func dedup(masks []maskRange) []maskRange {
	if len(masks) == 0 {
		return nil
	}
	result := make([]maskRange, 0, len(masks))
	cur := masks[0]
	for _, m := range masks[1:] {
		if m.Start <= cur.End {
			if m.End > cur.End {
				cur.End = m.End
			}
		} else {
			result = append(result, cur)
			cur = m
		}
	}
	result = append(result, cur)
	return result
}
